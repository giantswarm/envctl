package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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