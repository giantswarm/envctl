package kube

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api" // Import for api.Config
	// No longer need clientcmd for this specific test if clientset is passed in
	// "k8s.io/client-go/tools/clientcmd"
	// "k8s.io/client-go/rest"
)

func TestGetNodeStatusClientGo(t *testing.T) {
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

	// Call GetNodeStatusClientGo with the fake clientset
	ready, total, err := GetNodeStatusClientGo(clientset)
	if err != nil {
		t.Fatalf("GetNodeStatusClientGo() error = %v, wantErr nil", err)
	}
	if ready != 2 {
		t.Errorf("GetNodeStatusClientGo() ready = %v, want 2", ready)
	}
	if total != 3 {
		t.Errorf("GetNodeStatusClientGo() total = %v, want 3", total)
	}
}

// Mocking helper variables are no longer needed here as GetNodeStatusClientGo accepts an interface.
// var newForConfig = func(config clientcmd.ClientConfig) (kubernetes.Interface, error) { ... }
// var kubeConfigClientConfig = func(kc clientcmd.ClientConfig) (*rest.Config, error) { ... }

// This is a simplified GetNodeStatusClientGo that uses the mockable helpers.
// The actual GetNodeStatusClientGo in kube.go needs to be modified to use these vars.
// For this test to pass, the GetNodeStatusClientGo in kube.go would be refactored like this:
/*
func GetNodeStatusClientGo(kubeContext string) (readyNodes int, totalNodes int, err error) {
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

func TestTuiLogWriter_Write(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		asError     bool
		wantLogs    []string
		wantAsError []bool // Tracks the asError flag for each log line
	}{
		{
			name:        "single line",
			input:       "hello world\n",
			asError:     false,
			wantLogs:    []string{"hello world"},
			wantAsError: []bool{false},
		},
		{
			name:        "multiple lines",
			input:       "line1\nline2\nline3\n",
			asError:     true,
			wantLogs:    []string{"line1", "line2", "line3"},
			wantAsError: []bool{true, true, true},
		},
		{
			name:        "empty input",
			input:       "",
			asError:     false,
			wantLogs:    nil, // Expect no calls to sendUpdate
			wantAsError: nil,
		},
		{
			name:        "line without newline suffix",
			input:       "incomplete line",
			asError:     false,
			wantLogs:    []string{"incomplete line"},
			wantAsError: []bool{false},
		},
		{
			name:        "client-go info log prefix",
			input:       "I1234 56:78.901 client-go/stuff.go:123] this is the actual log\n",
			asError:     false,
			wantLogs:    []string{"client-go/stuff.go:123] this is the actual log"},
			wantAsError: []bool{false},
		},
		{
			name:        "client-go info log prefix without enough parts",
			input:       "I1234 client-go log\n", // Does not match SplitN( ,3) expectation
			asError:     false,
			wantLogs:    []string{"log"},
			wantAsError: []bool{false},
		},
		{
			name:        "mixed newlines and empty lines",
			input:       "line one\n\nline three\n",
			asError:     false,
			wantLogs:    []string{"line one", "line three"}, // Empty line should be skipped
			wantAsError: []bool{false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotLogs []string
			var gotAsError []bool
			mockSendUpdate := func(status, outputLog string, isError, isReady bool) {
				gotLogs = append(gotLogs, outputLog)
				gotAsError = append(gotAsError, isError)
				if status != "" {
					t.Errorf("tuiLogWriter.Write() sendUpdate got status %q, want empty", status)
				}
				if isReady {
					t.Error("tuiLogWriter.Write() sendUpdate got isReady true, want false")
				}
			}

			w := &tuiLogWriter{
				label:      "test",
				sendUpdate: mockSendUpdate,
				asError:    tt.asError,
			}

			n, err := w.Write([]byte(tt.input))
			if err != nil {
				t.Fatalf("Write() error = %v", err)
			}
			if n != len(tt.input) {
				t.Errorf("Write() bytes written = %v, want %v", n, len(tt.input))
			}

			if len(gotLogs) != len(tt.wantLogs) {
				t.Fatalf("Write() got %d log lines, want %d. Got logs: %v", len(gotLogs), len(tt.wantLogs), gotLogs)
			}

			for i := range gotLogs {
				if gotLogs[i] != tt.wantLogs[i] {
					t.Errorf("Write() gotLog[%d] = %q, want %q", i, gotLogs[i], tt.wantLogs[i])
				}
				if gotAsError[i] != tt.wantAsError[i] {
					t.Errorf("Write() gotAsError[%d] = %v, want %v", i, gotAsError[i], tt.wantAsError[i])
				}
			}
		})
	}
}

func TestGetPodNameForPortForward(t *testing.T) {
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
			podName, err := getPodNameForPortForward(clientset, tt.namespace, tt.serviceArg, tt.remotePodTargetPort)

			if (err != nil) != tt.wantErr {
				t.Errorf("%s: getPodNameForPortForward() error = %v, wantErr %v", tt.desc, err, tt.wantErr)
				return
			}
			if !tt.wantErr && podName != tt.wantPodName {
				t.Errorf("%s: getPodNameForPortForward() podName = %q, want %q", tt.desc, podName, tt.wantPodName)
			}
		})
	}
}

func TestStartPortForwardClientGo_InputValidation(t *testing.T) {
	mockSendUpdate := func(status, outputLog string, isError, isReady bool) { /* no-op for these tests */ }

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
			stopChan, _, err := StartPortForwardClientGo(tt.kubeContext, tt.namespace, tt.serviceArg, tt.portString, tt.pfLabel, mockSendUpdate)
			if (err != nil) != tt.wantErr {
				t.Errorf("%s: StartPortForwardClientGo() error = %v, wantErr %v", tt.desc, err, tt.wantErr)
			}
			// If a stop channel is returned, it should be closed to prevent goroutine leaks if the test panics or http client is used.
			if stopChan != nil {
				close(stopChan)
			}
		})
	}
}

func TestDetermineProviderFromNode(t *testing.T) {
	tests := []struct {
		name         string
		node         *corev1.Node
		wantProvider string
	}{
		{
			name:         "aws via providerID",
			node:         &corev1.Node{Spec: corev1.NodeSpec{ProviderID: "aws:///us-east-1/i-12345"}},
			wantProvider: "aws",
		},
		{
			name:         "azure via providerID",
			node:         &corev1.Node{Spec: corev1.NodeSpec{ProviderID: "azure:///subscriptions/subid/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm-name"}},
			wantProvider: "azure",
		},
		{
			name:         "gcp via providerID",
			node:         &corev1.Node{Spec: corev1.NodeSpec{ProviderID: "gce://project-id/us-central1-a/instance-1"}},
			wantProvider: "gcp",
		},
		{
			name:         "vsphere via providerID",
			node:         &corev1.Node{Spec: corev1.NodeSpec{ProviderID: "vsphere://guid"}},
			wantProvider: "vsphere",
		},
		{
			name:         "openstack via providerID",
			node:         &corev1.Node{Spec: corev1.NodeSpec{ProviderID: "openstack:///id"}},
			wantProvider: "openstack",
		},
		{
			name:         "aws via eks label",
			node:         &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"eks.amazonaws.com/nodegroup": "my-group"}}},
			wantProvider: "aws",
		},
		{
			name:         "aws via compute label",
			node:         &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"alpha.kubernetes.io/instance-type": "m5.large", "topology.kubernetes.io/zone": "us-east-1a", "failure-domain.beta.kubernetes.io/zone": "us-east-1a", "node.kubernetes.io/instance-type": "m5.large", "beta.kubernetes.io/instance-type": "m5.large", "failure-domain.beta.kubernetes.io/region": "us-east-1", "topology.kubernetes.io/region": "us-east-1", "amazonaws.com/compute": "ec2"}}},
			wantProvider: "aws",
		},
		{
			name:         "azure via label",
			node:         &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"kubernetes.azure.com/cluster": "aks-cluster"}}},
			wantProvider: "azure",
		},
		{
			name:         "gcp via gke label",
			node:         &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"cloud.google.com/gke-nodepool": "pool-1"}}},
			wantProvider: "gcp",
		},
		{
			name:         "unknown provider - no ID, no matching labels",
			node:         &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}}},
			wantProvider: "unknown",
		},
		{
			name:         "unknown provider - empty node spec and meta",
			node:         &corev1.Node{},
			wantProvider: "unknown",
		},
		{
			name:         "providerID present but not matched, fallback to unknown label",
			node:         &corev1.Node{Spec: corev1.NodeSpec{ProviderID: "somecloud://id"}, ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}}},
			wantProvider: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotProvider := determineProviderFromNode(tt.node); gotProvider != tt.wantProvider {
				t.Errorf("determineProviderFromNode() = %v, want %v", gotProvider, tt.wantProvider)
			}
		})
	}
}

func TestGetCurrentKubeContext(t *testing.T) {
	tests := []struct {
		name            string
		setupKubeconfig func(t *testing.T) (cleanup func())
		wantContext     string
		wantErr         bool
	}{
		{
			name: "valid kubeconfig with current context set",
			setupKubeconfig: func(t *testing.T) func() {
				tmpFile, err := ioutil.TempFile("", "kubeconfig-")
				if err != nil {
					t.Fatalf("Failed to create temp kubeconfig: %v", err)
				}
				config := api.Config{
					CurrentContext: "test-context",
					Contexts:       map[string]*api.Context{"test-context": {Cluster: "test-cluster"}},
					Clusters:       map[string]*api.Cluster{"test-cluster": {Server: "https://localhost:8080"}},
				}
				if err := clientcmd.WriteToFile(config, tmpFile.Name()); err != nil {
					t.Fatalf("Failed to write temp kubeconfig: %v", err)
				}
				t.Setenv("KUBECONFIG", tmpFile.Name())
				return func() { os.Remove(tmpFile.Name()) }
			},
			wantContext: "test-context",
			wantErr:     false,
		},
		{
			name: "kubeconfig with current context not set",
			setupKubeconfig: func(t *testing.T) func() {
				tmpFile, err := ioutil.TempFile("", "kubeconfig-")
				if err != nil {
					t.Fatalf("Failed to create temp kubeconfig: %v", err)
				}
				config := api.Config{
					Contexts: map[string]*api.Context{"another-context": {Cluster: "another-cluster"}},
					Clusters: map[string]*api.Cluster{"another-cluster": {Server: "https://localhost:8081"}},
					// CurrentContext is empty
				}
				if err := clientcmd.WriteToFile(config, tmpFile.Name()); err != nil {
					t.Fatalf("Failed to write temp kubeconfig: %v", err)
				}
				t.Setenv("KUBECONFIG", tmpFile.Name())
				return func() { os.Remove(tmpFile.Name()) }
			},
			wantContext: "",
			wantErr:     true, // Expect error as current context is not set
		},
		{
			name: "KUBECONFIG not set, default path does not exist (simulated by empty KUBECONFIG to non-existent file)",
			setupKubeconfig: func(t *testing.T) func() {
				// Point KUBECONFIG to a path that won't exist or is empty
				nonExistentPath := filepath.Join(t.TempDir(), "does_not_exist_kubeconfig")
				t.Setenv("KUBECONFIG", nonExistentPath)
				// For a more robust test of default path failure, we might need to manipulate user's home dir resolution,
				// but clientcmd.NewDefaultPathOptions() might also check KUBECONFIG first.
				// If KUBECONFIG is set (even to non-existent), it might take precedence over default ~/.kube/config.
				// This case effectively tests when GetStartingConfig fails to load anything.
				return func() {}
			},
			wantContext: "",
			wantErr:     true,
		},
		{
			name: "empty KUBECONFIG var, expect default path behavior (hard to test default path in isolation)",
			setupKubeconfig: func(t *testing.T) func() {
				originalKubeconfig := os.Getenv("KUBECONFIG")
				t.Setenv("KUBECONFIG", "") // Unset KUBECONFIG or set to empty
				// This test will now depend on the actual default kubeconfig of the test runner's environment
				// which is not ideal for a unit test. A true test would mock the filesystem or home dir.
				// For now, this tests that it *can* proceed if KUBECONFIG is empty, deferring to clientcmd's defaults.
				// We will just check it doesn't panic, error expectation depends on runner's env.
				return func() { t.Setenv("KUBECONFIG", originalKubeconfig) }
			},
			// wantContext: depends on runner's environment, cannot assert reliably
			// wantErr: depends on runner's environment
			// For this specific case, we'll just ensure it doesn't panic and returns *something* or an error.
			// A more sophisticated test would mock clientcmd.DefaultPathOptions.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupKubeconfig != nil {
				cleanup := tt.setupKubeconfig(t)
				defer cleanup()
			}

			gotContext, err := GetCurrentKubeContext()

			// Special handling for the unreliable default path test case
			if tt.name == "empty KUBECONFIG var, expect default path behavior (hard to test default path in isolation)" {
				// Just ensure it doesn't panic. Error or context value is env-dependent.
				t.Logf("Default path behavior test: gotContext=%q, err=%v (environment dependent)", gotContext, err)
				return
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("GetCurrentKubeContext() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && gotContext != tt.wantContext {
				t.Errorf("GetCurrentKubeContext() gotContext = %v, want %v", gotContext, tt.wantContext)
			}
		})
	}
}

func TestSwitchKubeContext(t *testing.T) {
	tests := []struct {
		name              string
		initialKubeconfig *api.Config // nil for non-existent or error cases initially
		contextToSwitch   string
		setupFS           func(t *testing.T) (kubeconfigPath string, cleanup func()) // For non-existent file test
		wantErr           bool
		verifySwitch      func(t *testing.T, kubeconfigPath string) // Check CurrentContext after switch
	}{
		{
			name: "switch to existing context",
			initialKubeconfig: &api.Config{
				CurrentContext: "ctx1",
				Contexts: map[string]*api.Context{
					"ctx1": {Cluster: "cluster1"},
					"ctx2": {Cluster: "cluster2"},
				},
				Clusters: map[string]*api.Cluster{
					"cluster1": {Server: "server1"},
					"cluster2": {Server: "server2"},
				},
			},
			contextToSwitch: "ctx2",
			wantErr:         false,
			verifySwitch: func(t *testing.T, kubeconfigPath string) {
				config, err := clientcmd.LoadFromFile(kubeconfigPath)
				if err != nil {
					t.Fatalf("Failed to load kubeconfig for verification: %v", err)
				}
				if config.CurrentContext != "ctx2" {
					t.Errorf("Expected current context to be 'ctx2', got '%s'", config.CurrentContext)
				}
			},
		},
		{
			name: "switch to non-existent context",
			initialKubeconfig: &api.Config{
				CurrentContext: "ctx1",
				Contexts:       map[string]*api.Context{"ctx1": {Cluster: "cluster1"}},
				Clusters:       map[string]*api.Cluster{"cluster1": {Server: "server1"}},
			},
			contextToSwitch: "non-existent-ctx",
			wantErr:         true,
		},
		{
			name: "kubeconfig file does not exist",
			setupFS: func(t *testing.T) (string, func()) {
				kubeconfigPath := filepath.Join(t.TempDir(), "non_existent_kubeconfig")
				// Ensure it doesn't exist (though TempDir is new, this is explicit)
				os.Remove(kubeconfigPath)
				return kubeconfigPath, func() {}
			},
			contextToSwitch: "any-ctx",
			wantErr:         true, // clientcmd.LoadFromFile called by GetStartingConfig will fail
		},
		{
			name: "empty kubeconfig file (invalid)",
			setupFS: func(t *testing.T) (string, func()) {
				tmpFile, err := ioutil.TempFile("", "empty-kubeconfig-")
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				// Leave it empty
				return tmpFile.Name(), func() { os.Remove(tmpFile.Name()) }
			},
			contextToSwitch: "any-ctx",
			wantErr:         true, // clientcmd.LoadFromFile will likely fail or return empty config
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var kubeconfigPath string
			cleanup := func() {}

			if tt.setupFS != nil {
				kubeconfigPath, cleanup = tt.setupFS(t)
			} else if tt.initialKubeconfig != nil {
				tmpFile, err := ioutil.TempFile("", "kubeconfig-")
				if err != nil {
					t.Fatalf("Failed to create temp kubeconfig: %v", err)
				}
				kubeconfigPath = tmpFile.Name()
				if err := clientcmd.WriteToFile(*tt.initialKubeconfig, kubeconfigPath); err != nil {
					t.Fatalf("Failed to write temp kubeconfig: %v", err)
				}
				cleanup = func() { os.Remove(kubeconfigPath) }
			} else {
				// Should not happen if test cases are well-defined
				t.Fatal("Test case misconfigured: either initialKubeconfig or setupFS must be provided")
			}
			defer cleanup()
			t.Setenv("KUBECONFIG", kubeconfigPath)

			err := SwitchKubeContext(tt.contextToSwitch)

			if (err != nil) != tt.wantErr {
				t.Errorf("SwitchKubeContext() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && tt.verifySwitch != nil {
				tt.verifySwitch(t, kubeconfigPath)
			}
		})
	}
}
