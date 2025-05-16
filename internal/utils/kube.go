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

// SendUpdateFunc defines a callback function signature used by port-forwarding logic
// to send status updates (including log messages, errors, and readiness status)
// back to the caller, typically the TUI, for display and state management.
type SendUpdateFunc func(status, outputLog string, isError, isReady bool)

// tuiLogWriter is an io.Writer implementation that wraps the SendUpdateFunc.
// It's used to capture stdout/stderr from the client-go port forwarding process
// and relay each line as a log message to the TUI.
type tuiLogWriter struct {
	label      string         // Label to prefix messages, identifying the source port-forward.
	sendUpdate SendUpdateFunc // The callback function to send formatted log messages.
	asError    bool           // If true, indicates this writer handles stderr-like messages, potentially flagging them as errors.
}

// Write processes the byte slice p, splits it into lines, and sends each line
// via the sendUpdate callback. It also performs minor cleaning of client-go internal log prefixes.
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

// StartPortForwardClientGo establishes a port-forward to a Kubernetes pod using the client-go library.
// This function handles the entire setup: parsing ports, loading Kubernetes configuration for the specified context,
// creating a clientset, resolving the service/pod name to a target pod, constructing the port-forwarding URL,
// and finally, creating and starting the port forwarder in a new goroutine.
// It returns a channel that can be used to stop the port-forwarding process, an initial status message indicating
// the setup attempt, and any error encountered during the synchronous part of the setup.
// Asynchronous updates (status changes, logs, errors, readiness) are sent via the provided `sendUpdate` callback.
//
// Parameters:
// - kubeContext: The name of the Kubernetes context to use.
// - namespace: The Kubernetes namespace where the target service/pod resides.
// - serviceArg: A string specifying the target, e.g., "service/my-service" or "pod/my-pod".
// - portString: The port mapping, e.g., "localPort:remotePort" (e.g., "8080:80").
// - pfLabel: A user-friendly label for this port-forward, used in updates sent via `sendUpdate`.
// - sendUpdate: The callback function (SendUpdateFunc) for sending asynchronous updates.
//
// Returns:
// - chan struct{}: A channel that, when closed, signals the port-forwarding goroutine to stop.
// - string: An initial status message (e.g., "Initializing...") from the synchronous setup phase.
// - error: Any error that occurred during the synchronous setup before the goroutine was started.
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
				true,  // isError = true
				false, // isReady = false
			)
			return
		}
	}()

	return stopChan, initialStatusLog, nil
}

// getPodNameForPortForward resolves a service argument (like "service/my-svc" or "pod/my-pod")
// to a specific, preferably ready, pod name that can be used as a target for port forwarding.
// If the argument is a service, it lists pods matching the service's selector and picks a running/ready one.
// - clientset: An initialized Kubernetes clientset.
// - namespace: The namespace to look for the service/pod in.
// - serviceArg: The string identifying the target (e.g., "service/my-service", "pod/my-pod").
// - remotePodTargetPort: The port on the pod that the port-forward aims to connect to. Used to (softly) check service port exposure.
// Returns the name of a suitable pod or an error if one cannot be found.
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

// GetNodeStatusClientGo retrieves the number of ready and total nodes in a cluster using client-go.
// - kubeContext: The Kubernetes context to target.
// Returns the count of ready nodes, total nodes, and an error if any occurs.
func GetNodeStatusClientGo(kubeContext string) (readyNodes int, totalNodes int, err error) {
	// 1. Kubernetes Config
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: kubeContext}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get REST config for context %q: %w", kubeContext, err)
	}
	restConfig.Timeout = 15 * time.Second // Shorter timeout for non-interactive calls

	// 2. Kubernetes Clientset
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create Kubernetes clientset for context %q: %w", kubeContext, err)
	}

	// 3. List Nodes
	nodeList, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list nodes in context %q: %w", kubeContext, err)
	}

	totalNodes = len(nodeList.Items)
	for _, node := range nodeList.Items {
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				readyNodes++
				break // Found a ready condition, move to the next node
			}
		}
	}

	return readyNodes, totalNodes, nil
}

// Note: Other utility functions within this package (e.g., GetCurrentKubeContext, SwitchKubeContext,
// GetNodeStatus, LoginToKubeCluster, GetClusterInfo) are also essential for the application's functionality.
// They primarily interact with external commands (`kubectl`, `tsh`) or system configurations.
// While `StartPortForwardClientGo` has been refactored to use client-go directly, these other functions
// currently retain their exec-based implementation. Future enhancements could involve migrating more of these
// to use client-go for more robust and integrated Kubernetes interactions where applicable.
