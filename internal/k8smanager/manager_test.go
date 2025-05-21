package k8smanager_test

import (
	"context"
	"envctl/internal/k8smanager"
	"envctl/internal/kube"  // To access original functions for overriding
	"envctl/internal/utils" // To access original functions for overriding
	"fmt"
	"reflect" // For DeepEqual
	"testing"

	"k8s.io/client-go/kubernetes"      // Added for kubernetes.Interface
	"k8s.io/client-go/kubernetes/fake" // For fake clientset
	"k8s.io/client-go/rest"            // For rest.Config type in mock override
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api" // For api.Config in mock
	// "k8s.io/client-go/tools/clientcmd" - If we mock at a higher level for GetAvailableContexts
)

// --- Test Mocks/Doubles for underlying kube and utils functions ---

// Store original functions
var (
	originalLoginToKubeCluster                              func(string) (string, string, error)
	originalGetClusterInfo                                  func() (*utils.ClusterInfo, error)
	originalGetCurrentCtx                                   func() (string, error)
	originalSwitchCtx                                       func(string) error
	originalBuildMcCtx                                      func(string) string
	originalBuildWcCtx                                      func(string, string) string
	originalStripPrefix                                     func(string) string
	originalHasPrefix                                       func(string) bool
	originalGetNodeStatus                                   func(clientset interface{}) (int, int, error) // Simplified clientset to interface{}
	originalNewK8sClientsetFromConfig                       func(c *rest.Config) (kubernetes.Interface, error)
	originalK8sNewNonInteractiveDeferredLoadingClientConfig func(loader clientcmd.ClientConfigLoader, overrides *clientcmd.ConfigOverrides) clientcmd.ClientConfig // Kept this type
)

// --- Mock clientcmd.ClientConfig ---
type mockClientConfig struct {
	// No longer embedding clientcmd.ClientConfig
	clientConfigFuncOverride func() (*rest.Config, error)
	rawConfigFuncOverride    func() (api.Config, error)
	namespaceFuncOverride    func() (string, bool, error)
	// ConfigAccess() is harder to mock simply, will panic if called by tests
}

func (mcc *mockClientConfig) ClientConfig() (*rest.Config, error) {
	if mcc.clientConfigFuncOverride != nil {
		return mcc.clientConfigFuncOverride()
	}
	return &rest.Config{Host: "fake-host"}, nil
}

func (mcc *mockClientConfig) RawConfig() (api.Config, error) {
	if mcc.rawConfigFuncOverride != nil {
		return mcc.rawConfigFuncOverride()
	}
	return api.Config{}, nil
}

func (mcc *mockClientConfig) Namespace() (string, bool, error) {
	if mcc.namespaceFuncOverride != nil {
		return mcc.namespaceFuncOverride()
	}
	return "default", true, nil // Default stub implementation
}

func (mcc *mockClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	// This is harder to mock simply. If tests don't hit this, panicking is a way to find out.
	panic("ConfigAccess() not implemented on mockClientConfig")
}

var _ clientcmd.ClientConfig = &mockClientConfig{} // Ensure it satisfies the interface

func setupKubeManagerMocks(t *testing.T) {
	// Store originals first time
	if originalLoginToKubeCluster == nil {
		originalLoginToKubeCluster = utils.LoginToKubeCluster
		originalGetClusterInfo = utils.GetClusterInfo
		originalGetCurrentCtx = kube.GetCurrentKubeContext
		originalSwitchCtx = kube.SwitchKubeContext
		originalBuildMcCtx = utils.BuildMcContext
		originalBuildWcCtx = utils.BuildWcContext
		originalStripPrefix = utils.StripTeleportPrefix
		originalHasPrefix = utils.HasTeleportPrefix
		// originalGetNodeStatus = kube.GetNodeStatusClientGo // This is tricky due to clientset type
	}

	if originalNewK8sClientsetFromConfig == nil {
		originalNewK8sClientsetFromConfig = k8smanager.NewK8sClientsetFromConfig
	}
	if originalK8sNewNonInteractiveDeferredLoadingClientConfig == nil {
		originalK8sNewNonInteractiveDeferredLoadingClientConfig = k8smanager.K8sNewNonInteractiveDeferredLoadingClientConfig
	}

	// Default mocks (can be overridden per test)
	utils.LoginToKubeCluster = func(clusterName string) (string, string, error) {
		t.Logf("mock LoginToKubeCluster called with: %s", clusterName)
		return "login-stdout-" + clusterName, "login-stderr-" + clusterName, nil
	}
	utils.GetClusterInfo = func() (*utils.ClusterInfo, error) {
		t.Log("mock GetClusterInfo called")
		return &utils.ClusterInfo{
			ManagementClusters: []string{"mc1"},
			WorkloadClusters:   map[string][]string{"mc1": {"wc1a"}},
		}, nil
	}
	kube.GetCurrentKubeContext = func() (string, error) {
		t.Log("mock GetCurrentKubeContext called")
		return "test-current-context", nil
	}
	kube.SwitchKubeContext = func(target string) error {
		t.Logf("mock SwitchKubeContext called with: %s", target)
		return nil
	}
	// utils context functions are simple passthroughs, usually no need to mock unless testing specific prefix logic.

	// Mocking GetNodeStatusClientGo is harder due to the kubernetes.Interface argument.
	// For GetClusterNodeHealth tests, we might need to mock at the NewForConfig level or accept its complexity.
	// For now, we will skip direct mocking of GetNodeStatusClientGo here and test GetClusterNodeHealth more as an integration.

	// Mock for clientset creation
	k8smanager.NewK8sClientsetFromConfig = func(c *rest.Config) (kubernetes.Interface, error) {
		t.Log("mock NewK8sClientsetFromConfig called")
		return fake.NewSimpleClientset(), nil // Return a fake clientset
	}

	// Default mock for K8sNewNonInteractiveDeferredLoadingClientConfig if needed for other tests
	// TestGetClusterNodeHealth_* will override this specifically.
	k8smanager.K8sNewNonInteractiveDeferredLoadingClientConfig = func(loader clientcmd.ClientConfigLoader, overrides *clientcmd.ConfigOverrides) clientcmd.ClientConfig {
		t.Log("default mock K8sNewNonInteractiveDeferredLoadingClientConfig called")
		return &mockClientConfig{
			clientConfigFuncOverride: func() (*rest.Config, error) {
				return &rest.Config{Host: "default-fake-host"}, nil // Default success
			},
		}
	}
}

func restoreKubeManagerOriginals() {
	utils.LoginToKubeCluster = originalLoginToKubeCluster
	utils.GetClusterInfo = originalGetClusterInfo
	kube.GetCurrentKubeContext = originalGetCurrentCtx
	kube.SwitchKubeContext = originalSwitchCtx
	// kube.GetNodeStatusClientGo = originalGetNodeStatus // Restore if mocked
	k8smanager.NewK8sClientsetFromConfig = originalNewK8sClientsetFromConfig // Restore original
	if originalK8sNewNonInteractiveDeferredLoadingClientConfig != nil {
		k8smanager.K8sNewNonInteractiveDeferredLoadingClientConfig = originalK8sNewNonInteractiveDeferredLoadingClientConfig
	}
}

func TestKubeManager_Login(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()

	km := k8smanager.NewKubeManager()
	stdout, stderr, err := km.Login("mycluster")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if stdout != "login-stdout-mycluster" {
		t.Errorf("Expected stdout %s, got %s", "login-stdout-mycluster", stdout)
	}
	if stderr != "login-stderr-mycluster" {
		t.Errorf("Expected stderr %s, got %s", "login-stderr-mycluster", stderr)
	}
}

func TestKubeManager_ListClusters(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()

	km := k8smanager.NewKubeManager()
	cl, err := km.ListClusters()
	if err != nil {
		t.Fatalf("ListClusters failed: %v", err)
	}
	if cl == nil {
		t.Fatal("ListClusters returned nil list")
	}
	expectedMCs := []k8smanager.Cluster{
		{Name: "mc1", KubeconfigContextName: "teleport.giantswarm.io-mc1", IsManagement: true},
	}
	if !reflect.DeepEqual(cl.ManagementClusters, expectedMCs) {
		t.Errorf("ManagementClusters mismatch:\nGot:  %v\nWant: %v", cl.ManagementClusters, expectedMCs)
	}

	expectedWCs := map[string][]k8smanager.Cluster{
		"mc1": {{Name: "wc1a", KubeconfigContextName: "teleport.giantswarm.io-mc1-wc1a", IsManagement: false, MCName: "mc1", WCShortName: "wc1a"}},
	}
	if !reflect.DeepEqual(cl.WorkloadClusters, expectedWCs) {
		t.Errorf("WorkloadClusters mismatch:\nGot:  %v\nWant: %v", cl.WorkloadClusters, expectedWCs)
	}
}

func TestKubeManager_GetCurrentContext(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()
	km := k8smanager.NewKubeManager()
	ctx, err := km.GetCurrentContext()
	if err != nil || ctx != "test-current-context" {
		t.Errorf("GetCurrentContext() got %v, %v, want test-current-context, nil", ctx, err)
	}
}

func TestKubeManager_SwitchContext(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()
	km := k8smanager.NewKubeManager()
	target := "new-context"
	var switchedTo string
	kube.SwitchKubeContext = func(s string) error { // Override mock for this test
		switchedTo = s
		return nil
	}
	if err := km.SwitchContext(target); err != nil || switchedTo != target {
		t.Errorf("SwitchContext(%s) failed or did not switch to target: %v, switchedTo: %s", target, err, switchedTo)
	}
}

func TestKubeManager_GetClusterNodeHealth_Success(t *testing.T) {
	setupKubeManagerMocks(t) // This sets up a default mock for K8sNewNonInteractiveDeferredLoadingClientConfig
	defer restoreKubeManagerOriginals()

	// Specific mock for this test
	currentOriginalLoader := k8smanager.K8sNewNonInteractiveDeferredLoadingClientConfig
	k8smanager.K8sNewNonInteractiveDeferredLoadingClientConfig = func(loader clientcmd.ClientConfigLoader, overrides *clientcmd.ConfigOverrides) clientcmd.ClientConfig {
		return &mockClientConfig{
			clientConfigFuncOverride: func() (*rest.Config, error) {
				t.Log("mockClientConfig.ClientConfig called for success test")
				return &rest.Config{Host: "fake-success-host"}, nil
			},
		}
	}
	// No need to defer restore here if restoreKubeManagerOriginals handles it,
	// but specific override means we should restore it to what setupKubeManagerMocks set, or original.
	// Better to restore to the global original at the end of test.
	defer func() { k8smanager.K8sNewNonInteractiveDeferredLoadingClientConfig = currentOriginalLoader }()

	originalGetNodeStatus := kube.GetNodeStatusClientGo
	kube.GetNodeStatusClientGo = func(clientset kubernetes.Interface) (readyNodes int, totalNodes int, err error) {
		t.Log("mock GetNodeStatusClientGo called for success test")
		return 3, 5, nil
	}
	defer func() { kube.GetNodeStatusClientGo = originalGetNodeStatus }()

	km := k8smanager.NewKubeManager()
	health, err := km.GetClusterNodeHealth(context.Background(), "test-ctx-success")

	if err != nil {
		t.Fatalf("GetClusterNodeHealth failed: %v", err)
	}
	if health.ReadyNodes != 3 {
		t.Errorf("Expected ReadyNodes 3, got %d", health.ReadyNodes)
	}
	if health.TotalNodes != 5 {
		t.Errorf("Expected TotalNodes 5, got %d", health.TotalNodes)
	}
	if health.Error != nil {
		t.Errorf("Expected health.Error to be nil, got %v", health.Error)
	}
}

func TestKubeManager_GetClusterNodeHealth_ClientConfigError(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()

	expectedErr := fmt.Errorf("clientConfig error")
	currentOriginalLoader := k8smanager.K8sNewNonInteractiveDeferredLoadingClientConfig
	k8smanager.K8sNewNonInteractiveDeferredLoadingClientConfig = func(loader clientcmd.ClientConfigLoader, overrides *clientcmd.ConfigOverrides) clientcmd.ClientConfig {
		return &mockClientConfig{
			clientConfigFuncOverride: func() (*rest.Config, error) {
				t.Log("mockClientConfig.ClientConfig called for error test")
				return nil, expectedErr
			},
		}
	}
	defer func() { k8smanager.K8sNewNonInteractiveDeferredLoadingClientConfig = currentOriginalLoader }()

	km := k8smanager.NewKubeManager()
	_, err := km.GetClusterNodeHealth(context.Background(), "test-ctx-clientconfig-error")

	if err == nil {
		t.Fatalf("GetClusterNodeHealth expected error from ClientConfig, got nil")
	}
	if err.Error() != fmt.Errorf("failed to get REST config for context test-ctx-clientconfig-error: %w", expectedErr).Error() {
		t.Errorf("Unexpected error message. Got: %v", err)
	}
}

func TestKubeManager_GetClusterNodeHealth_NodeStatusError(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()

	currentOriginalLoader := k8smanager.K8sNewNonInteractiveDeferredLoadingClientConfig
	k8smanager.K8sNewNonInteractiveDeferredLoadingClientConfig = func(loader clientcmd.ClientConfigLoader, overrides *clientcmd.ConfigOverrides) clientcmd.ClientConfig {
		return &mockClientConfig{
			clientConfigFuncOverride: func() (*rest.Config, error) {
				return &rest.Config{Host: "fake-nodestatus-host"}, nil
			},
		}
	}
	defer func() { k8smanager.K8sNewNonInteractiveDeferredLoadingClientConfig = currentOriginalLoader }()

	expectedStatusErr := fmt.Errorf("kube API error from GetNodeStatusClientGo")
	originalGetNodeStatus := kube.GetNodeStatusClientGo
	kube.GetNodeStatusClientGo = func(clientset kubernetes.Interface) (readyNodes int, totalNodes int, err error) {
		t.Log("mock GetNodeStatusClientGo called, returning specific error")
		return 1, 2, expectedStatusErr
	}
	defer func() { kube.GetNodeStatusClientGo = originalGetNodeStatus }()

	km := k8smanager.NewKubeManager()
	health, err := km.GetClusterNodeHealth(context.Background(), "test-ctx-nodestatus-error")

	if err == nil {
		t.Fatal("GetClusterNodeHealth expected error from GetNodeStatusClientGo, got nil")
	}
	if health.Error == nil || health.Error.Error() != expectedStatusErr.Error() {
		t.Errorf("Expected health.Error '%v', got '%v'", expectedStatusErr, health.Error)
	}
	if health.ReadyNodes != 1 || health.TotalNodes != 2 {
		t.Errorf("Expected Ready=1, Total=2 even on error, got Ready=%d, Total=%d", health.ReadyNodes, health.TotalNodes)
	}
}

// TODO: Add tests for:
// - GetAvailableContexts (might need to mock clientcmd interaction)
// - BuildMcContextName, BuildWcContextName, StripTeleportPrefix, HasTeleportPrefix (these are simple passthroughs, low priority unless logic changes)
// - GetClusterNodeHealth (this is an integration test if kube.GetNodeStatusClientGo is not easily mockable here)
// - Error paths for all methods
