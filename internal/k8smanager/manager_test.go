package k8smanager_test

import (
	"context"
	"envctl/internal/k8smanager"
	"envctl/internal/kube"      // To access original functions for overriding
	"envctl/internal/reporting" // For reporting.ServiceReporter
	"envctl/internal/utils"     // To access original functions for overriding
	"fmt"
	"reflect" // For DeepEqual

	// For TestKubeManager_GetAvailableContexts_Success
	"strings" // For TestKubeManager_GetAvailableContexts_Error
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
	originalDetermineClusterProvider                        func(ctx context.Context, kubeContextName string) (string, error)
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

// mockServiceReporter is a simple mock for reporting.ServiceReporter
type mockServiceReporter struct {
	ReportFunc func(update reporting.ManagedServiceUpdate)
	stateStore reporting.StateStore
}

func (m *mockServiceReporter) Report(update reporting.ManagedServiceUpdate) {
	if m.ReportFunc != nil {
		m.ReportFunc(update)
	}
	// Update the state store if available
	if m.stateStore != nil {
		m.stateStore.SetServiceState(update)
	}
}

func (m *mockServiceReporter) ReportHealth(update reporting.HealthStatusUpdate) {
	// For now, just ignore health reports in tests
	// They're not relevant to the k8smanager tests
}

func (m *mockServiceReporter) GetStateStore() reporting.StateStore {
	if m.stateStore == nil {
		m.stateStore = reporting.NewStateStore()
	}
	return m.stateStore
}

// Make DetermineClusterProvider mockable for tests
var mockableDetermineClusterProvider func(ctx context.Context, kubeContextName string) (string, error)

func newTestKubeManager() k8smanager.KubeManagerAPI {
	// For tests, we can pass a nil reporter, and NewKubeManager will use a ConsoleReporter,
	// or we can pass a specific mock reporter if we want to assert reporting calls.
	// Using nil here as most existing tests don't check reporter calls yet.
	return k8smanager.NewKubeManager(nil)
}

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
	// No longer need the mockableDetermineClusterProvider indirection as kube.DetermineClusterProvider is now a var.
	// if originalDetermineClusterProvider == nil {
	// 	originalDetermineClusterProvider = kube.DetermineClusterProvider
	// 	// Assign the initial mockable function to the original one.
	// 	// Tests can then override mockableDetermineClusterProvider.
	// 	mockableDetermineClusterProvider = originalDetermineClusterProvider
	// 	// The actual kube.DetermineClusterProvider will call our mockable one.
	// 	kube.DetermineClusterProvider = func(ctx context.Context, kubeContextName string) (string, error) {
	// 		return mockableDetermineClusterProvider(ctx, kubeContextName)
	// 	}
	// }

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

	// if originalDetermineClusterProvider != nil {
	// 	kube.DetermineClusterProvider = originalDetermineClusterProvider
	// 	mockableDetermineClusterProvider = nil // Reset mockable version
	// }
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
	// if originalDetermineClusterProvider != nil {
	// 	kube.DetermineClusterProvider = originalDetermineClusterProvider
	// 	mockableDetermineClusterProvider = nil // Reset mockable version
	// }
}

func TestKubeManager_Login(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()

	km := newTestKubeManager()
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

func TestKubeManager_Login_Error(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()

	expectedErr := fmt.Errorf("tsh login error")
	utils.LoginToKubeCluster = func(clusterName string) (string, string, error) {
		t.Logf("mock LoginToKubeCluster called for error test with: %s", clusterName)
		return "", "error details", expectedErr
	}

	reportedError := false
	mockReporter := &mockServiceReporter{
		ReportFunc: func(update reporting.ManagedServiceUpdate) {
			t.Logf("mockReporter received update: %+v", update)
			if update.State == reporting.StateFailed && update.ErrorDetail == expectedErr {
				reportedError = true
			}
		},
	}
	km := k8smanager.NewKubeManager(mockReporter)

	_, _, err := km.Login("errorcluster")
	if err == nil {
		t.Fatal("Login expected an error, but got nil")
	}
	if err != expectedErr {
		t.Errorf("Login expected error %v, got %v", expectedErr, err)
	}
	if !reportedError {
		t.Errorf("Expected reporter to be called with StateFailed and the specific error")
	}
}

func TestKubeManager_ListClusters(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()

	km := newTestKubeManager()
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

func TestKubeManager_ListClusters_Error(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()

	expectedErr := fmt.Errorf("get cluster info error")
	utils.GetClusterInfo = func() (*utils.ClusterInfo, error) {
		t.Log("mock GetClusterInfo called for error test")
		return nil, expectedErr
	}

	km := newTestKubeManager()
	_, err := km.ListClusters()

	if err == nil {
		t.Fatal("ListClusters expected an error, but got nil")
	}
	if err != expectedErr {
		t.Errorf("ListClusters expected error %v, got %v", expectedErr, err)
	}
}

func TestKubeManager_GetCurrentContext(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()
	km := newTestKubeManager()
	ctx, err := km.GetCurrentContext()
	if err != nil || ctx != "test-current-context" {
		t.Errorf("GetCurrentContext() got %v, %v, want test-current-context, nil", ctx, err)
	}
}

func TestKubeManager_SwitchContext(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()
	km := newTestKubeManager()
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

func TestKubeManager_SwitchContext_Error(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()

	expectedErr := fmt.Errorf("kubectl switch error")
	kube.SwitchKubeContext = func(target string) error {
		t.Logf("mock SwitchKubeContext called for error test with: %s", target)
		return expectedErr
	}

	reportedError := false
	mockReporter := &mockServiceReporter{
		ReportFunc: func(update reporting.ManagedServiceUpdate) {
			t.Logf("mockReporter received update: %+v", update)
			if update.State == reporting.StateFailed && update.ErrorDetail == expectedErr {
				reportedError = true
			}
		},
	}
	km := k8smanager.NewKubeManager(mockReporter)

	target := "error-context"
	err := km.SwitchContext(target)

	if err == nil {
		t.Fatalf("SwitchContext(%s) expected an error, but got nil", target)
	}
	if err != expectedErr {
		t.Errorf("SwitchContext(%s) expected error %v, got %v", target, expectedErr, err)
	}
	if !reportedError {
		t.Errorf("Expected reporter to be called with StateFailed and the specific error")
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

	originalGetNodeStatus := kube.GetNodeStatus
	kube.GetNodeStatus = func(clientset kubernetes.Interface) (readyNodes int, totalNodes int, err error) {
		t.Log("mock GetNodeStatus called for success test")
		return 3, 5, nil
	}
	defer func() { kube.GetNodeStatus = originalGetNodeStatus }()

	km := newTestKubeManager()
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

	km := newTestKubeManager()
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

	expectedStatusErr := fmt.Errorf("kube API error from GetNodeStatus")
	originalGetNodeStatus := kube.GetNodeStatus
	kube.GetNodeStatus = func(clientset kubernetes.Interface) (readyNodes int, totalNodes int, err error) {
		t.Log("mock GetNodeStatus called, returning specific error")
		return 1, 2, expectedStatusErr
	}
	defer func() { kube.GetNodeStatus = originalGetNodeStatus }()

	km := newTestKubeManager()
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

// --- Additional tests for simple passthrough functions ---

func TestKubeManager_BuildMcContextName(t *testing.T) {
	km := newTestKubeManager()
	mcShortName := "mycluster"
	expected := utils.BuildMcContext(mcShortName) // Uses the actual util function for expected value
	result := km.BuildMcContextName(mcShortName)
	if result != expected {
		t.Errorf("BuildMcContextName(%q) = %q, want %q", mcShortName, result, expected)
	}
}

func TestKubeManager_BuildWcContextName(t *testing.T) {
	km := newTestKubeManager()
	mcShortName := "mc"
	wcShortName := "wc"
	expected := utils.BuildWcContext(mcShortName, wcShortName) // Uses the actual util function
	result := km.BuildWcContextName(mcShortName, wcShortName)
	if result != expected {
		t.Errorf("BuildWcContextName(%q, %q) = %q, want %q", mcShortName, wcShortName, result, expected)
	}
}

func TestKubeManager_StripTeleportPrefix(t *testing.T) {
	km := newTestKubeManager()
	contextName := "teleport.giantswarm.io-mycluster"
	expected := utils.StripTeleportPrefix(contextName) // Uses the actual util function
	result := km.StripTeleportPrefix(contextName)
	if result != expected {
		t.Errorf("StripTeleportPrefix(%q) = %q, want %q", contextName, result, expected)
	}

	contextNameNoPrefix := "mycluster"
	expectedNoPrefix := utils.StripTeleportPrefix(contextNameNoPrefix)
	resultNoPrefix := km.StripTeleportPrefix(contextNameNoPrefix)
	if resultNoPrefix != expectedNoPrefix {
		t.Errorf("StripTeleportPrefix(%q) = %q, want %q", contextNameNoPrefix, resultNoPrefix, expectedNoPrefix)
	}
}

func TestKubeManager_HasTeleportPrefix(t *testing.T) {
	km := newTestKubeManager()

	contextWithPrefix := "teleport.giantswarm.io-mycluster"
	if !km.HasTeleportPrefix(contextWithPrefix) {
		t.Errorf("HasTeleportPrefix(%q) = false, want true", contextWithPrefix)
	}

	contextWithoutPrefix := "mycluster"
	if km.HasTeleportPrefix(contextWithoutPrefix) {
		t.Errorf("HasTeleportPrefix(%q) = true, want false", contextWithoutPrefix)
	}

	emptyContext := ""
	if km.HasTeleportPrefix(emptyContext) {
		t.Errorf("HasTeleportPrefix(%q) = true, want false", emptyContext)
	}
}

// --- End of additional tests ---

// --- Tests for GetAvailableContexts ---

func TestKubeManager_GetAvailableContexts_Success(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()

	originalGetStartingConfig := k8smanager.K8sGetStartingConfigForList
	k8smanager.K8sGetStartingConfigForList = func() (*api.Config, error) {
		return &api.Config{
			Contexts: map[string]*api.Context{
				"ctx-b": {Cluster: "cluster-b"},
				"ctx-a": {Cluster: "cluster-a"},
				"ctx-c": {Cluster: "cluster-c"},
			},
		}, nil
	}
	defer func() { k8smanager.K8sGetStartingConfigForList = originalGetStartingConfig }()

	km := newTestKubeManager()
	contexts, err := km.GetAvailableContexts()

	if err != nil {
		t.Fatalf("GetAvailableContexts failed: %v", err)
	}

	expectedContexts := []string{"ctx-a", "ctx-b", "ctx-c"}
	// The function sorts them, so we expect sorted order
	if !reflect.DeepEqual(contexts, expectedContexts) {
		t.Errorf("Expected contexts %v, got %v", expectedContexts, contexts)
	}
}

func TestKubeManager_GetAvailableContexts_Error(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()

	expectedErr := fmt.Errorf("failed to load kubeconfig")
	originalGetStartingConfig := k8smanager.K8sGetStartingConfigForList
	k8smanager.K8sGetStartingConfigForList = func() (*api.Config, error) {
		return nil, expectedErr
	}
	defer func() { k8smanager.K8sGetStartingConfigForList = originalGetStartingConfig }()

	km := newTestKubeManager()
	_, err := km.GetAvailableContexts()

	if err == nil {
		t.Fatal("GetAvailableContexts expected an error, but got nil")
	}

	wantErrorMsg := "failed to get starting kubeconfig: " + expectedErr.Error()
	if !strings.Contains(err.Error(), wantErrorMsg) {
		t.Errorf("Expected error message containing %q, got %q", wantErrorMsg, err.Error())
	}
}

// --- End of GetAvailableContexts tests ---

func TestKubeManager_GetClusterNodeHealth_ClientsetError(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()

	// Mock K8sNewNonInteractiveDeferredLoadingClientConfig to return a working rest.Config
	currentOriginalLoader := k8smanager.K8sNewNonInteractiveDeferredLoadingClientConfig
	k8smanager.K8sNewNonInteractiveDeferredLoadingClientConfig = func(loader clientcmd.ClientConfigLoader, overrides *clientcmd.ConfigOverrides) clientcmd.ClientConfig {
		return &mockClientConfig{
			clientConfigFuncOverride: func() (*rest.Config, error) {
				t.Log("mockClientConfig.ClientConfig called for clientset error test")
				return &rest.Config{Host: "fake-clientset-error-host"}, nil
			},
		}
	}
	defer func() { k8smanager.K8sNewNonInteractiveDeferredLoadingClientConfig = currentOriginalLoader }()

	expectedErr := fmt.Errorf("clientset creation error")
	originalNewClientset := k8smanager.NewK8sClientsetFromConfig
	k8smanager.NewK8sClientsetFromConfig = func(c *rest.Config) (kubernetes.Interface, error) {
		t.Log("mock NewK8sClientsetFromConfig called for error test")
		return nil, expectedErr
	}
	defer func() { k8smanager.NewK8sClientsetFromConfig = originalNewClientset }()

	reportedError := false
	mockReporter := &mockServiceReporter{
		ReportFunc: func(update reporting.ManagedServiceUpdate) {
			t.Logf("mockReporter received update for GetClusterNodeHealth_ClientsetError: %+v", update)
			if update.State == reporting.StateFailed && update.ErrorDetail != nil && update.ErrorDetail.Error() == fmt.Errorf("failed to create Kubernetes clientset for context test-ctx-clientset-error: %w", expectedErr).Error() {
				reportedError = true
			}
		},
	}
	km := k8smanager.NewKubeManager(mockReporter)

	health, err := km.GetClusterNodeHealth(context.Background(), "test-ctx-clientset-error")

	if err == nil {
		t.Fatalf("GetClusterNodeHealth expected error from NewK8sClientsetFromConfig, got nil")
	}
	if err.Error() != fmt.Errorf("failed to create Kubernetes clientset for context test-ctx-clientset-error: %w", expectedErr).Error() {
		t.Errorf("Unexpected error message. Got: %v", err)
	}
	if health.Error == nil || health.Error.Error() != fmt.Errorf("failed to create Kubernetes clientset for context test-ctx-clientset-error: %w", expectedErr).Error() {
		t.Errorf("Expected health.Error to be '%v', got '%v'", expectedErr, health.Error)
	}
	if !reportedError {
		t.Errorf("Expected reporter to be called with StateFailed and the specific wrapped error")
	}
}

// --- Tests for DetermineClusterProvider ---

func TestKubeManager_DetermineClusterProvider_Success(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()

	expectedProvider := "aws"
	// Store and defer restore of the actual kube.DetermineClusterProvider
	originalDCP := kube.DetermineClusterProvider
	kube.DetermineClusterProvider = func(ctx context.Context, kubeContextName string) (string, error) {
		t.Logf("mock DetermineClusterProvider called for success test with context: %s", kubeContextName)
		return expectedProvider, nil
	}
	defer func() { kube.DetermineClusterProvider = originalDCP }()

	reportedRunning := false
	mockReporter := &mockServiceReporter{
		ReportFunc: func(update reporting.ManagedServiceUpdate) {
			t.Logf("mockReporter received update: %+v", update)
			if update.State == reporting.StateRunning {
				reportedRunning = true
			}
		},
	}
	km := k8smanager.NewKubeManager(mockReporter)
	provider, err := km.DetermineClusterProvider(context.Background(), "test-ctx-provider-success")

	if err != nil {
		t.Fatalf("DetermineClusterProvider failed: %v", err)
	}
	if provider != expectedProvider {
		t.Errorf("Expected provider %s, got %s", expectedProvider, provider)
	}
	if !reportedRunning {
		t.Errorf("Expected reporter to be called with StateRunning")
	}
}

func TestKubeManager_DetermineClusterProvider_Error(t *testing.T) {
	setupKubeManagerMocks(t)
	defer restoreKubeManagerOriginals()

	expectedErr := fmt.Errorf("determine provider error")
	// Store and defer restore of the actual kube.DetermineClusterProvider
	originalDCP := kube.DetermineClusterProvider
	kube.DetermineClusterProvider = func(ctx context.Context, kubeContextName string) (string, error) {
		t.Logf("mock DetermineClusterProvider called for error test with context: %s", kubeContextName)
		return "", expectedErr
	}
	defer func() { kube.DetermineClusterProvider = originalDCP }()

	reportedError := false
	mockReporter := &mockServiceReporter{
		ReportFunc: func(update reporting.ManagedServiceUpdate) {
			t.Logf("mockReporter received update: %+v", update)
			if update.State == reporting.StateFailed && update.ErrorDetail == expectedErr {
				reportedError = true
			}
		},
	}
	km := k8smanager.NewKubeManager(mockReporter)
	_, err := km.DetermineClusterProvider(context.Background(), "test-ctx-provider-error")

	if err == nil {
		t.Fatal("DetermineClusterProvider expected an error, but got nil")
	}
	if err != expectedErr {
		t.Errorf("DetermineClusterProvider expected error %v, got %v", expectedErr, err)
	}
	if !reportedError {
		t.Errorf("Expected reporter to be called with StateFailed and the specific error")
	}
}

// --- End of DetermineClusterProvider tests ---
