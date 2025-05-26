package controller

import (
	"context"
	"envctl/internal/dependency"
	"envctl/internal/kube"
	"envctl/internal/reporting"
	"envctl/internal/tui/model"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test the health check flow and UI updates
func TestHandleNodeStatusMsg_ServiceLifecycle(t *testing.T) {
	// Create a test model
	testModel := &model.Model{
		ManagementClusterName: "test-mc",
		DependencyGraph:       dependency.New(),
		MCHealth:              model.ClusterHealthInfo{},
		WCHealth:              model.ClusterHealthInfo{},
		ClusterInfo:           &kube.ClusterInfo{},
	}

	// Test 1: Health check fails - UI should update
	t.Run("UnhealthyConnection_UpdatesUI", func(t *testing.T) {
		// Send unhealthy status
		msg := model.NodeStatusMsg{
			ClusterShortName: "test-mc",
			ForMC:            true,
			ReadyNodes:       0,
			TotalNodes:       3,
			Err:              errors.New("connection timeout"),
		}

		updatedModel, _ := handleNodeStatusMsg(testModel, msg)

		// Verify UI state was updated
		assert.NotNil(t, updatedModel.MCHealth.StatusError)
		assert.Equal(t, 0, updatedModel.MCHealth.ReadyNodes)
		assert.Equal(t, 3, updatedModel.MCHealth.TotalNodes)
		assert.False(t, updatedModel.MCHealth.IsLoading)
	})

	// Test 2: Health check succeeds - UI should update
	t.Run("HealthyConnection_UpdatesUI", func(t *testing.T) {
		// Send healthy status
		msg := model.NodeStatusMsg{
			ClusterShortName: "test-mc",
			ForMC:            true,
			ReadyNodes:       3,
			TotalNodes:       3,
			Err:              nil,
		}

		updatedModel, _ := handleNodeStatusMsg(testModel, msg)

		// Verify UI state was updated
		assert.Nil(t, updatedModel.MCHealth.StatusError)
		assert.Equal(t, 3, updatedModel.MCHealth.ReadyNodes)
		assert.Equal(t, 3, updatedModel.MCHealth.TotalNodes)
		assert.False(t, updatedModel.MCHealth.IsLoading)
	})

	// Test 3: Workload cluster health update
	t.Run("WorkloadCluster_UpdatesUI", func(t *testing.T) {
		testModel.WorkloadClusterName = "test-wc"

		// Send WC healthy status
		msg := model.NodeStatusMsg{
			ClusterShortName: "test-wc",
			ForMC:            false,
			ReadyNodes:       5,
			TotalNodes:       5,
			Err:              nil,
		}

		updatedModel, _ := handleNodeStatusMsg(testModel, msg)

		// Verify WC health was updated
		assert.Nil(t, updatedModel.WCHealth.StatusError)
		assert.Equal(t, 5, updatedModel.WCHealth.ReadyNodes)
		assert.Equal(t, 5, updatedModel.WCHealth.TotalNodes)
		assert.False(t, updatedModel.WCHealth.IsLoading)
	})

	// Test that the model is updated correctly
	assert.NotNil(t, testModel.ClusterInfo)
}

// Mock KubeManager for testing
type mockKubeManager struct {
	buildMcContextName func(string) string
	buildWcContextName func(string, string) string
}

func (m *mockKubeManager) Login(clusterName string) (stdout string, stderr string, err error) {
	return "", "", nil
}

func (m *mockKubeManager) ListClusters() (interface{}, error) {
	return nil, nil
}

func (m *mockKubeManager) GetCurrentContext() (string, error) {
	return "", nil
}

func (m *mockKubeManager) SwitchContext(targetContextName string) error {
	return nil
}

func (m *mockKubeManager) GetAvailableContexts() ([]string, error) {
	return nil, nil
}

func (m *mockKubeManager) BuildMcContextName(mcShortName string) string {
	if m.buildMcContextName != nil {
		return m.buildMcContextName(mcShortName)
	}
	return "teleport.giantswarm.io-" + mcShortName
}

func (m *mockKubeManager) BuildWcContextName(mcShortName, wcShortName string) string {
	if m.buildWcContextName != nil {
		return m.buildWcContextName(mcShortName, wcShortName)
	}
	return "teleport.giantswarm.io-" + mcShortName + "-" + wcShortName
}

func (m *mockKubeManager) StripTeleportPrefix(contextName string) string {
	return contextName
}

func (m *mockKubeManager) HasTeleportPrefix(contextName string) bool {
	return false
}

func (m *mockKubeManager) GetClusterNodeHealth(ctx context.Context, kubeContextName string) (interface{}, error) {
	return struct {
		ReadyNodes int
		TotalNodes int
		Error      error
	}{}, nil
}

func (m *mockKubeManager) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
	return "", nil
}

func (m *mockKubeManager) SetReporter(reporter reporting.ServiceReporter) {
}

func TestHandleClusterListResultMsg(t *testing.T) {
	tests := []struct {
		name     string
		msg      model.ClusterListResultMsg
		expected *kube.ClusterInfo
	}{
		{
			name: "successful cluster list",
			msg: model.ClusterListResultMsg{
				Info: &kube.ClusterInfo{
					ManagementClusters: []string{"mc1", "mc2"},
					WorkloadClusters: map[string][]string{
						"mc1": {"wc1", "wc2"},
						"mc2": {"wc3"},
					},
				},
				Err: nil,
			},
			expected: &kube.ClusterInfo{
				ManagementClusters: []string{"mc1", "mc2"},
				WorkloadClusters: map[string][]string{
					"mc1": {"wc1", "wc2"},
					"mc2": {"wc3"},
				},
			},
		},
		{
			name: "error fetching cluster list",
			msg: model.ClusterListResultMsg{
				Info: nil,
				Err:  assert.AnError,
			},
			expected: &kube.ClusterInfo{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &model.Model{
				ClusterInfo: &kube.ClusterInfo{},
			}

			updatedModel := handleClusterListResultMsg(m, tt.msg)

			assert.Equal(t, tt.expected, updatedModel.ClusterInfo)
		})
	}
}
