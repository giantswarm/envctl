package kube

import (
	// "envctl/internal/reporting" // No longer needed by this test file if using old signature

	"context"

	// "envctl/internal/k8smanager" // Temporarily commented out to break import cycle

	// Assuming this is needed for other parts of the test file or by k8smanager types if re-enabled

	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func TestGetNodeStatus(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node1"},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node2"},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
				},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node3"},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				},
			},
		},
	)

	// Call GetNodeStatus with the fake clientset
	ready, total, err := GetNodeStatus(clientset)
	if err != nil {
		t.Fatalf("GetNodeStatus() error = %v, wantErr nil", err)
	}
	if ready != 2 {
		t.Errorf("GetNodeStatus() ready = %v, want 2", ready)
	}
	if total != 3 {
		t.Errorf("GetNodeStatus() total = %v, want 3", total)
	}
}

// Mocking helper variables are no longer needed here as GetNodeStatus accepts an interface.
// var newForConfig = func(config clientcmd.ClientConfig) (kubernetes.Interface, error) { ... }
// var kubeConfigClientConfig = func(kc clientcmd.ClientConfig) (*rest.Config, error) { ... }

// This is a simplified GetNodeStatus that uses the mockable helpers.
// The actual GetNodeStatus in kube.go needs to be modified to use these vars.
// For this test to pass, the GetNodeStatus in kube.go would be refactored like this:
/*
func GetNodeStatus(kubeContext string) (readyNodes int, totalNodes int, err error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: kubeContext}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	// USE THE MOCKABLE HELPER HERE
	// restConfig, err := kubeConfig.ClientConfig()
	restConfig, err := kubeConfigClientConfig(kubeConfig)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get REST config for context %q: %w", kubeContext, err)
	}
	restConfig.Timeout = 15 * time.Second

	// USE THE MOCKABLE HELPER HERE
	// clientset, err := kubernetes.NewForConfig(restConfig)
	clientset, err := newForConfig(kubeConfig) // Pass kubeConfig, as newForConfig mock expects it for our setup
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create Kubernetes clientset for context %q: %w", kubeContext, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	nodeList, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	// ... rest of function
	return readyNodes, totalNodes, nil
}
*/

func Test_resolveServiceToPodNameForPortForward(t *testing.T) {
	tests := []struct {
		name                string
		namespace           string
		serviceArg          string
		remotePodTargetPort uint16
		initialObjects      []runtime.Object // Pods, Services to pre-populate
		wantPodName         string
		wantErr             bool
		desc                string
	}{
		{
			name:                "direct pod name",
			namespace:           "default",
			serviceArg:          "pod/my-actual-pod",
			remotePodTargetPort: 8080,
			wantPodName:         "my-actual-pod",
			wantErr:             false,
			desc:                "Should return the pod name directly if type is pod.",
		},
		{
			name:                "service with ready pod",
			namespace:           "dev",
			serviceArg:          "service/my-service",
			remotePodTargetPort: 80,
			initialObjects: []runtime.Object{
				&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "my-service", Namespace: "dev"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "my-app"}, Ports: []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(80)}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod1", Namespace: "dev", Labels: map[string]string{"app": "my-app"}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}, ContainerStatuses: []corev1.ContainerStatus{{Ready: true}}}},
			},
			wantPodName: "pod1",
			wantErr:     false,
			desc:        "Should find a ready pod backing the service.",
		},
		{
			name:                "service with multiple pods, one ready",
			namespace:           "prod",
			serviceArg:          "service/frontend",
			remotePodTargetPort: 3000,
			initialObjects: []runtime.Object{
				&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "frontend", Namespace: "prod"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"tier": "frontend"}, Ports: []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(3000)}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "frontend-pod-a", Namespace: "prod", Labels: map[string]string{"tier": "frontend"}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}}, ContainerStatuses: []corev1.ContainerStatus{{Ready: false}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "frontend-pod-b", Namespace: "prod", Labels: map[string]string{"tier": "frontend"}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}, ContainerStatuses: []corev1.ContainerStatus{{Ready: true}}}},
			},
			wantPodName: "frontend-pod-b",
			wantErr:     false,
			desc:        "Should pick a ready pod from multiple backing pods.",
		},
		{
			name:                "service with no selector",
			namespace:           "default",
			serviceArg:          "service/no-selector-svc",
			remotePodTargetPort: 8080,
			initialObjects:      []runtime.Object{&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "no-selector-svc", Namespace: "default"}, Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080)}}}}},
			wantErr:             true,
			desc:                "Should return error if service has no selector.",
		},
		{
			name:                "service with no matching pods",
			namespace:           "default",
			serviceArg:          "service/no-pods-svc",
			remotePodTargetPort: 8080,
			initialObjects:      []runtime.Object{&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "no-pods-svc", Namespace: "default"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "nonexistent"}, Ports: []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080)}}}}},
			wantErr:             true,
			desc:                "Should return error if service selector matches no pods.",
		},
		{
			name:                "service with no ready pods",
			namespace:           "default",
			serviceArg:          "service/not-ready-svc",
			remotePodTargetPort: 8080,
			initialObjects: []runtime.Object{
				&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "not-ready-svc", Namespace: "default"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "not-ready"}, Ports: []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080)}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-not-ready", Namespace: "default", Labels: map[string]string{"app": "not-ready"}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}}, ContainerStatuses: []corev1.ContainerStatus{{Ready: false}}}},
			},
			wantErr: true,
			desc:    "Should return error if service has pods but none are ready.",
		},
		{
			name:                "invalid serviceArg format",
			namespace:           "default",
			serviceArg:          "justname",
			remotePodTargetPort: 8080,
			wantErr:             true,
			desc:                "Should return error for invalid serviceArg format.",
		},
		{
			name:                "service with pod ready but container statuses empty",
			namespace:           "default",
			serviceArg:          "service/empty-container-status-svc",
			remotePodTargetPort: 8080,
			initialObjects: []runtime.Object{
				&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "empty-container-status-svc", Namespace: "default"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "empty-cs"}, Ports: []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080)}}}},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "pod-empty-cs", Namespace: "default", Labels: map[string]string{"app": "empty-cs"}},
					Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "main"}}}, // Has a container defined
					Status: corev1.PodStatus{
						Phase:             corev1.PodRunning,
						Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
						ContainerStatuses: []corev1.ContainerStatus{}, // Empty container statuses
					},
				},
			},
			wantErr: true, // Should result in "no ready pods found"
			desc:    "Should not select pod if PodReady but ContainerStatuses is empty while Spec.Containers is not.",
		},
		{
			name:                "service with pod ready but one container not ready",
			namespace:           "default",
			serviceArg:          "service/container-not-ready-svc",
			remotePodTargetPort: 8080,
			initialObjects: []runtime.Object{
				&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "container-not-ready-svc", Namespace: "default"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "container-not-ready"}, Ports: []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080)}}}},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "pod-container-not-ready", Namespace: "default", Labels: map[string]string{"app": "container-not-ready"}},
					Status: corev1.PodStatus{
						Phase:      corev1.PodRunning,
						Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
						ContainerStatuses: []corev1.ContainerStatus{
							{Name: "main", Ready: true},
							{Name: "sidecar", Ready: false}, // One container not ready
						},
					},
				},
			},
			wantErr: true, // Should result in "no ready pods found"
			desc:    "Should not select pod if PodReady but one of its containers is not ready.",
		},
		{
			name:                "service with ready pod, but targetPort mismatch (current code ignores mismatch)",
			namespace:           "default",
			serviceArg:          "service/port-mismatch-svc",
			remotePodTargetPort: 9999, // This port is not in service spec's TargetPort
			initialObjects: []runtime.Object{
				&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "port-mismatch-svc", Namespace: "default"}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "port-mismatch"}, Ports: []corev1.ServicePort{{Port: 80, TargetPort: intstr.FromInt(8080)}}}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-port-mismatch", Namespace: "default", Labels: map[string]string{"app": "port-mismatch"}}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}, ContainerStatuses: []corev1.ContainerStatus{{Ready: true}}}},
			},
			wantPodName: "pod-port-mismatch", // Currently, the code does not enforce TargetPort match strictly
			wantErr:     false,               // So, it should still find the pod based on selector if strict check is off
			desc:        "Should select pod even if remotePodTargetPort doesn't match Service TargetPort (due to current relaxed check).",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset(tt.initialObjects...)
			podName, err := resolveServiceToPodNameForPortForward(context.Background(), clientset, tt.namespace, tt.serviceArg, tt.remotePodTargetPort)

			if (err != nil) != tt.wantErr {
				t.Errorf("%s: resolveServiceToPodNameForPortForward() error = %v, wantErr %v", tt.desc, err, tt.wantErr)
				return
			}
			if !tt.wantErr && podName != tt.wantPodName {
				t.Errorf("%s: resolveServiceToPodNameForPortForward() podName = %q, want %q", tt.desc, podName, tt.wantPodName)
			}
		})
	}
}

func TestStartPortForward_InputValidation(t *testing.T) {
	// Revert mockSendUpdate signature
	mockSendUpdate := func(status, outputLog string, isError, isReady bool) { /* no-op */ }

	tests := []struct {
		name        string
		kubeContext string
		namespace   string
		serviceArg  string
		portString  string
		pfLabel     string
		wantErr     bool
		desc        string
	}{
		{
			name:        "valid input smoke test (expect further errors)",
			kubeContext: "test-ctx", namespace: "default", serviceArg: "pod/mypod", portString: "8080:80", pfLabel: "test",
			wantErr: true, // Will error out trying to get REST config for a fake context
			desc:    "Basic valid format, should pass initial parsing but fail later.",
		},
		{
			name:        "invalid port string format - missing colon",
			kubeContext: "test-ctx", namespace: "default", serviceArg: "pod/mypod", portString: "808080", pfLabel: "test",
			wantErr: true,
			desc:    "Should error on port string without colon.",
		},
		{
			name:        "invalid port string format - too many colons",
			kubeContext: "test-ctx", namespace: "default", serviceArg: "pod/mypod", portString: "8080:80:30", pfLabel: "test",
			wantErr: true,
			desc:    "Should error on port string with too many colons.",
		},
		{
			name:        "invalid local port - not a number",
			kubeContext: "test-ctx", namespace: "default", serviceArg: "pod/mypod", portString: "bad:80", pfLabel: "test",
			wantErr: true,
			desc:    "Should error if local port is not a number.",
		},
		{
			name:        "invalid remote port - not a number",
			kubeContext: "test-ctx", namespace: "default", serviceArg: "pod/mypod", portString: "8080:bad", pfLabel: "test",
			wantErr: true,
			desc:    "Should error if remote port is not a number.",
		},
		{
			name:        "invalid local port - out of range",
			kubeContext: "test-ctx", namespace: "default", serviceArg: "pod/mypod", portString: "70000:80", pfLabel: "test",
			wantErr: true, // strconv.ParseUint for uint16 will error for 70000
			desc:    "Should error if local port is out of uint16 range.",
		},
		{
			name:        "invalid remote port - out of range",
			kubeContext: "test-ctx", namespace: "default", serviceArg: "pod/mypod", portString: "8080:70000", pfLabel: "test",
			wantErr: true, // strconv.ParseUint for uint16 will error for 70000
			desc:    "Should error if remote port is out of uint16 range.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stopChan, _, err := StartPortForward(context.Background(), tt.kubeContext, tt.namespace, tt.serviceArg, tt.portString, tt.pfLabel, mockSendUpdate)
			if (err != nil) != tt.wantErr {
				t.Errorf("%s: StartPortForward() error = %v, wantErr %v", tt.desc, err, tt.wantErr)
			}
			if stopChan != nil {
				close(stopChan)
			}
		})
	}
}

// TestDirectLogger_Write tests the directLogger's Write method.

// If `k8smanager` types are used in signatures (e.g. k8smanager.ClusterList), the import is needed.
