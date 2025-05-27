package kube

import (
	"context"
	"envctl/internal/reporting"
)

// If `k8smanager` types are used in signatures (e.g. k8smanager.ClusterList), the import is needed.

type MockKubeManager struct{}

func (m *MockKubeManager) Login(clusterName string) (string, string, error) { return "", "", nil }
func (m *MockKubeManager) ListClusters() (interface{}, error) {
	return nil, nil
}
func (m *MockKubeManager) GetCurrentContext() (string, error)           { return "test-context", nil }
func (m *MockKubeManager) SwitchContext(targetContextName string) error { return nil }
func (m *MockKubeManager) GetAvailableContexts() ([]string, error) {
	return []string{"test-context"}, nil
}
func (m *MockKubeManager) BuildMcContextName(mcShortName string) string { return "mc-" + mcShortName }
func (m *MockKubeManager) BuildWcContextName(mcShortName, wcShortName string) string {
	return "wc-" + mcShortName + "-" + wcShortName
}
func (m *MockKubeManager) StripTeleportPrefix(contextName string) string { return contextName }
func (m *MockKubeManager) HasTeleportPrefix(contextName string) bool     { return false }
func (m *MockKubeManager) GetClusterNodeHealth(ctx context.Context, kubeContextName string) (interface{}, error) {
	// Return a simple struct that matches basic fields if any test relies on it, or just an empty interface.
	type fakeNodeHealth struct {
		ReadyNodes, TotalNodes int
		Error                  error
	}
	return fakeNodeHealth{ReadyNodes: 1, TotalNodes: 1}, nil
}
func (m *MockKubeManager) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
	return "mockProvider", nil
}
func (m *MockKubeManager) SetReporter(reporter reporting.ServiceReporter) {}

// var _ k8smanager.KubeManagerAPI = (*MockKubeManager)(nil) // Temporarily comment out to break cycle
