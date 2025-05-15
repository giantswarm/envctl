package utils

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // Important for various auth providers
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// SendUpdateFunc defines the callback signature for sending status updates to the TUI.
type SendUpdateFunc func(status, outputLog string, isError, isReady bool)

// tuiLogWriter is an io.Writer that sends data to the TUI via the SendUpdateFunc.
type tuiLogWriter struct {
	label      string
	sendUpdate SendUpdateFunc
	asError    bool // True if this writer is for stderr
}

func (w *tuiLogWriter) Write(p []byte) (n int, err error) {
	lines := strings.Split(strings.TrimSuffix(string(p), "\n"), "\n")
	for _, line := range lines {
		if line != "" { // Avoid sending empty log lines
			// For plain log output, status is empty, isReady is false.
			// Strip out client-go internal formatting and provide cleaner messages
			cleanLine := line
			
			// Remove timestamp prefixes often seen in client-go logs
			if strings.Contains(cleanLine, "I") && strings.Contains(cleanLine, "client-go") {
				parts := strings.SplitN(cleanLine, " ", 3)
				if len(parts) >= 3 {
					cleanLine = parts[2]
				}
			}
			
			// Send as log output without additional label prefix since handler will add it
			w.sendUpdate("", cleanLine, w.asError, false)
		}
	}
	return len(p), nil
}

// StartPortForwardClientGo establishes a port forward using client-go.
// It returns a channel to stop the port forward, an initial status message, and any setup error.
func StartPortForwardClientGo(
	kubeContext string,
	namespace string,
	serviceArg string, // e.g., "service/my-svc" or "pod/my-pod"
	portString string, // e.g., "8080:8080"
	pfLabel string,
	sendUpdate SendUpdateFunc,
) (chan struct{}, string, error) {

	// 1. Parse Ports
	portParts := strings.Split(portString, ":")
	if len(portParts) != 2 {
		errMsg := fmt.Errorf("invalid port string %q, expected format local:remote", portString)
		// sendUpdate for early errors is good for logging, but TUI state might be better set by direct return
		return nil, "", errMsg
	}
	localPortStr, remotePortStr := portParts[0], portParts[1]

	localPort, err := strconv.ParseUint(localPortStr, 10, 16)
	if err != nil {
		return nil, "", fmt.Errorf("invalid local port %q: %w", localPortStr, err)
	}
	remotePort, err := strconv.ParseUint(remotePortStr, 10, 16)
	if err != nil {
		return nil, "", fmt.Errorf("invalid remote port %q: %w", remotePortStr, err)
	}
	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}

	// 2. Kubernetes Config
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// ExplicitPath can be set here if envctl uses a specific kubeconfig path
	// loadingRules.ExplicitPath = clientcmd.RecommendedHomeFile 
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: kubeContext}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get REST config for context %q: %w", kubeContext, err)
	}
	restConfig.Timeout = 30 * time.Second // Example timeout for connection attempts

	// 3. Kubernetes Clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	// 4. Determine Target Pod
	podName, err := getPodNameForPortForward(clientset, namespace, serviceArg, uint16(remotePort))
	if err != nil {
		return nil, "", fmt.Errorf("failed to determine target pod for %q in %q: %w", serviceArg, namespace, err)
	}

	// 5. Create PortForwarder URL
	// Example URL: POST https://<server>/api/v1/namespaces/<namespace>/pods/<pod>/portforward
	reqURL := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward").
		URL()
		
	// 6. Create Dialer & PortForwarder
	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create SPDY round tripper: %w", err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, reqURL)

	stopChan := make(chan struct{}, 1) // Buffered to allow send without immediate receive
	readyChan := make(chan struct{})

	stdOutWriter := &tuiLogWriter{label: pfLabel, sendUpdate: sendUpdate, asError: false}
	stdErrWriter := &tuiLogWriter{label: pfLabel, sendUpdate: sendUpdate, asError: true}

	// Using NewOnAddresses to specify listen on 127.0.0.1
	// localPort can be 0 to pick a random available port.
	// If localPort is 0, GetPorts() must be used after ready.
	addresses := []string{"127.0.0.1"} // Listen on localhost

	forwarder, err := portforward.NewOnAddresses(dialer, addresses, ports, stopChan, readyChan, stdOutWriter, stdErrWriter)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create port forwarder: %w", err)
	}

	initialStatusLog := fmt.Sprintf("Initializing %s:%s -> %s/%s (pod %s)...", localPortStr, remotePortStr, namespace, serviceArg, podName)
	sendUpdate(initialStatusLog, "", false, false)

	// Send useful debug messages without overwhelming the log
	sendUpdate("", fmt.Sprintf("Starting port forward to pod %s", podName), false, false)

	// 7. Run Asynchronously
	go func() {
		sendUpdate("", "Starting ForwardPorts process...", false, false)
		if err = forwarder.ForwardPorts(); err != nil {
			sendUpdate("", fmt.Sprintf("ForwardPorts error: %v", err), true, false)
			select {
			case <-stopChan:
				sendUpdate("Stopped.", "Port forwarding terminated by request after error.", false, false)
			default:
				sendUpdate("Error.", fmt.Sprintf("Forwarding failed: %v", err), true, false)
			}
		} else {
			sendUpdate("", "ForwardPorts completed gracefully", false, false)
			sendUpdate("Stopped.", "Port forwarding connection closed.", false, false)
		}
	}()

	go func() {
		sendUpdate("", "Monitoring ready/stop channels...", false, false)
		select {
		case <-stopChan:
			sendUpdate("", "Stop signal received.", false, false)
			return
		case <-readyChan:
			sendUpdate("", "Ready signal received!", false, false)
			actualPorts, portErr := forwarder.GetPorts()
			var fwdDetail string
			if portErr == nil && len(actualPorts) > 0 {
				fwdDetail = fmt.Sprintf("Forwarding from 127.0.0.1:%d to pod port %d", actualPorts[0].Local, actualPorts[0].Remote)
			} else {
				fwdDetail = fmt.Sprintf("Forwarding from 127.0.0.1:%s to pod port %s", localPortStr, remotePortStr)
				if portErr != nil {
					sendUpdate("", fmt.Sprintf("Warning: could not get bound local port: %v", portErr), true, false)
				}
			}
			sendUpdate(fwdDetail, "", false, true) // isReady = true
			sendUpdate("", "Waiting for stop signal (port-forward is active)", false, false)
			<-stopChan // Wait for stop signal after ready
			sendUpdate("", "Stop signal received after port-forward was active.", false, false)
			return
		case <-time.After(60 * time.Second):
			sendUpdate("", "Timeout (60s) waiting for ready signal.", true, false)
			sendUpdate(
				"Timeout.",
				"Port-forward timed out after 60s waiting for ready signal.",
				true, // isError = true
				false, // isReady = false
			)
			return
		}
	}()

	return stopChan, initialStatusLog, nil
}

// getPodNameForPortForward resolves a service argument to a pod name.
// serviceArg can be "service/my-service" or "pod/my-pod".
// remotePodTargetPort is used to check if a service exposes this port.
func getPodNameForPortForward(clientset kubernetes.Interface, namespace, serviceArg string, remotePodTargetPort uint16) (string, error) {
	parts := strings.SplitN(serviceArg, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid service/pod string %q, expected type/name (e.g., pod/my-pod or service/my-service)", serviceArg)
	}
	resourceType, resourceName := parts[0], parts[1]

	if strings.ToLower(resourceType) == "pod" {
		// For a pod, just return its name. The port is already known.
		// We could verify the pod exists, but port-forward will fail if not.
		return resourceName, nil
	} else if strings.ToLower(resourceType) == "service" {
		svc, err := clientset.CoreV1().Services(namespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to get service %s/%s: %w", namespace, resourceName, err)
		}

		// Check if the service's TargetPort matches the remotePodTargetPort
		// This is a simple check; a service might have named target ports.
		portMatches := false
		for _, servicePort := range svc.Spec.Ports {
			if servicePort.TargetPort.Type == 0 && servicePort.TargetPort.IntVal == int32(remotePodTargetPort) { // IntVal is int32
				portMatches = true
				break
			}
			// TODO: Handle named TargetPort by looking up pod's container ports.
			// For now, we only match numeric TargetPort.
		}
		if !portMatches {
			// Temporarily disabling this strict check, as users might provide a pod port that the service maps to,
			// but not explicitly the service's TargetPort if it's named or different.
			// The port forward will ultimately be to the pod on that remotePodTargetPort.
			// return "", fmt.Errorf("service %s/%s does not have a TargetPort matching %d", namespace, resourceName, remotePodTargetPort)
		}


		if len(svc.Spec.Selector) == 0 {
			return "", fmt.Errorf("service %s/%s has no selector, cannot find backing pods", namespace, resourceName)
		}

		selector := labels.SelectorFromSet(svc.Spec.Selector)
		podList, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			return "", fmt.Errorf("failed to list pods for service %s/%s: %w", namespace, resourceName, err)
		}
		if len(podList.Items) == 0 {
			return "", fmt.Errorf("no pods found for service %s/%s with selector %s", namespace, resourceName, selector.String())
		}

		// Pick a ready pod
		for _, pod := range podList.Items {
			if pod.Status.Phase == corev1.PodRunning {
				isReady := false
				for _, cond := range pod.Status.Conditions {
					if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
						isReady = true
						break
					}
				}
				if isReady {
					// Also check if containers are ready (optional, but good)
					allContainersReady := true
					if len(pod.Status.ContainerStatuses) == 0 && len(pod.Spec.Containers) > 0 {
						// Pod is running but container statuses not yet reported, might be initializing
						allContainersReady = false 
					}
					for _, cs := range pod.Status.ContainerStatuses {
						if !cs.Ready {
							allContainersReady = false
							break
						}
					}
					if allContainersReady {
						return pod.Name, nil
					}
				}
			}
		}
		return "", fmt.Errorf("no ready pods found for service %s/%s (selector: %s)", namespace, resourceName, selector.String())
	}
	return "", fmt.Errorf("unsupported resource type %q in %q", resourceType, serviceArg)
}

// Ensure other utility functions (GetCurrentKubeContext, SwitchKubeContext, GetNodeStatus, LoginToKubeCluster, GetClusterInfo)
// are also eventually refactored to use client-go where appropriate, or their exec-based nature is confirmed.
// The StartPortForwardClientGo is the main focus of this change. 