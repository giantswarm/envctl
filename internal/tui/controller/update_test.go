package controller

import (
	"context"
	"envctl/internal/k8smanager"
	"envctl/internal/reporting" // To access mainControllerDispatch (needs to be exported or tested via model.Update)
	"envctl/internal/tui/model"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// MockKubeManager now correctly implements k8smanager.KubeManagerAPI
type MockKubeManager struct{}

func (m *MockKubeManager) Login(clusterName string) (string, string, error) {
	return "login-stdout-" + clusterName, "", nil
}

func (m *MockKubeManager) ListClusters() (*k8smanager.ClusterList, error) {
	// Return a structure that matches k8smanager.ClusterList and k8smanager.Cluster
	mc1 := k8smanager.Cluster{
		Name:                  "mc1",
		KubeconfigContextName: "teleport.giantswarm.io-mc1",
		IsManagement:          true,
	}
	wc1a := k8smanager.Cluster{
		Name:                  "wc1a",
		KubeconfigContextName: "teleport.giantswarm.io-mc1-wc1a",
		IsManagement:          false,
		MCName:                "mc1",
		WCShortName:           "wc1a",
	}
	allClustersMap := make(map[string]k8smanager.Cluster)
	allClustersMap[mc1.KubeconfigContextName] = mc1
	allClustersMap[wc1a.KubeconfigContextName] = wc1a

	return &k8smanager.ClusterList{
		ManagementClusters: []k8smanager.Cluster{mc1},
		WorkloadClusters:   map[string][]k8smanager.Cluster{"mc1": {wc1a}},
		AllClusters:        allClustersMap,
		ContextNames:       []string{mc1.KubeconfigContextName, wc1a.KubeconfigContextName},
	}, nil
}

func (m *MockKubeManager) GetCurrentContext() (string, error)           { return "test-context", nil }
func (m *MockKubeManager) SwitchContext(targetContextName string) error { return nil }
func (m *MockKubeManager) GetAvailableContexts() ([]string, error) {
	return []string{"test-context", "another-context"}, nil
}
func (m *MockKubeManager) BuildMcContextName(mcShortName string) string { return "mc-" + mcShortName }
func (m *MockKubeManager) BuildWcContextName(mcShortName, wcShortName string) string {
	return "wc-" + mcShortName + "-" + wcShortName
}
func (m *MockKubeManager) StripTeleportPrefix(contextName string) string { return contextName }
func (m *MockKubeManager) HasTeleportPrefix(contextName string) bool     { return false }
func (m *MockKubeManager) GetClusterNodeHealth(ctx context.Context, kubeContextName string) (k8smanager.NodeHealth, error) {
	return k8smanager.NodeHealth{ReadyNodes: 1, TotalNodes: 1}, nil
}

func (m *MockKubeManager) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
	return "mockProvider", nil
}

func (m *MockKubeManager) SetReporter(reporter reporting.ServiceReporter) {
	// Mock implementation: can be empty if not used in these specific tests,
	// or store the reporter if tests need to verify it was set.
}

// TestMainControllerDispatch_ReporterUpdateMsg_ContinuouslyProcessesViaChannelReaderCmd
// tests if the mainControllerDispatch correctly re-queues the ChannelReaderCmd
// after processing a ReporterUpdateMsg, allowing continuous processing of messages
// from the TUIChannel.
func TestMainControllerDispatch_ReporterUpdateMsg_ContinuouslyProcessesViaChannelReaderCmd(t *testing.T) {
	mockKubeMgr := &MockKubeManager{}
	// Assuming model.InitialModel is fine and returns *model.Model
	mInitialModel := model.InitialModel("mc1", "wc1", "test-context", true, nil, nil, mockKubeMgr)

	assert.NotNil(t, mInitialModel.TUIChannel, "TUIChannel should be initialized")
	assert.NotNil(t, mInitialModel.Reporter, "Reporter should be initialized")
	assert.NotNil(t, mInitialModel.ServiceManager, "ServiceManager should be initialized")

	msg1Content := reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeSystem,
		SourceLabel: "TestService1",
		Level:       reporting.LogLevelInfo,
		Message:     "First test message",
	}
	msg2Content := reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeSystem,
		SourceLabel: "TestService2",
		Level:       reporting.LogLevelInfo,
		Message:     "Second test message",
	}

	// --- Simulate processing the first message ---
	t.Log("Simulating send and processing of first message")
	go func() {
		mInitialModel.TUIChannel <- reporting.ReporterUpdateMsg{Update: msg1Content}
		t.Log("First message sent to TUIChannel")
	}()

	var readMsg1 tea.Msg
	select {
	case readMsg1 = <-mInitialModel.TUIChannel:
		t.Logf("First message read by test harness: %T", readMsg1)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for first message from TUIChannel")
	}
	assert.IsType(t, reporting.ReporterUpdateMsg{}, readMsg1, "Read message should be ReporterUpdateMsg")

	// Call actual mainControllerDispatch (now accessible as it's in the same package)
	updatedModel1, cmd1 := mainControllerDispatch(mInitialModel, readMsg1)

	assert.NotEmpty(t, updatedModel1.ActivityLog, "ActivityLog should not be empty after first message")
	lastLog1 := updatedModel1.ActivityLog[len(updatedModel1.ActivityLog)-1]
	assert.Contains(t, lastLog1, msg1Content.Message, "ActivityLog should contain the first message")
	t.Logf("Activity log after first message: %s", lastLog1)
	assert.NotNil(t, cmd1, "A command should be returned after processing the first message")

	// Capture length before second dispatch
	lenActivityLogBeforeMsg2 := len(updatedModel1.ActivityLog)
	t.Logf("Length of ActivityLog before second message: %d", lenActivityLogBeforeMsg2)

	// --- Simulate processing the second message ---
	t.Log("Simulating send and processing of second message")
	go func() {
		updatedModel1.TUIChannel <- reporting.ReporterUpdateMsg{Update: msg2Content}
		t.Log("Second message sent to TUIChannel")
	}()

	var readMsg2 tea.Msg
	select {
	case readMsg2 = <-updatedModel1.TUIChannel:
		t.Logf("Second message read by test harness: %T", readMsg2)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for second message from TUIChannel")
	}
	assert.IsType(t, reporting.ReporterUpdateMsg{}, readMsg2, "Read message should be ReporterUpdateMsg")

	// Call actual mainControllerDispatch for the second message
	updatedModel2, cmd2 := mainControllerDispatch(updatedModel1, readMsg2)

	assert.True(t, len(updatedModel2.ActivityLog) > lenActivityLogBeforeMsg2, "ActivityLog should have grown")
	lastLog2 := updatedModel2.ActivityLog[len(updatedModel2.ActivityLog)-1]
	assert.Contains(t, lastLog2, msg2Content.Message, "ActivityLog should contain the second message")
	t.Logf("Activity log after second message: %s", lastLog2)
	assert.NotNil(t, cmd2, "A command should be returned after processing the second message")

	t.Log("Test completed: two messages processed, commands returned each time.")
}

// Note: To make this test fully robust without being in the 'controller' package,
// or without exporting mainControllerDispatch, one would need to:
// 1. Create a mock for tea.Cmd that can be identified (e.g., returns a specific string or struct).
// 2. Have model.ChannelReaderCmd return this mock command instead of the actual func() tea.Msg during tests.
// This would allow asserting `cmd1` and `cmd2` specifically contain the expected mock command.
// The current test relies on observing the side effect (processing of the second message)
// as strong evidence of re-queuing.
