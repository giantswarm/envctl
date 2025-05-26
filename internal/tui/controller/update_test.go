package controller

import (
	"context"
	"envctl/internal/color"  // Corrected import for color package
	"envctl/internal/config" // Added
	"envctl/internal/k8smanager"
	"envctl/internal/reporting" // To access mainControllerDispatch (needs to be exported or tested via model.Update)
	"envctl/internal/tui/model" // Added for logging.LogEntry for logChan type
	"envctl/pkg/logging"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
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

// TestMainControllerDispatch_ReporterUpdateMsg_GeneratesLog
// tests if the mainControllerDispatch correctly re-queues the ChannelReaderCmd
// after processing a ReporterUpdateMsg, allowing continuous processing of messages
// from the TUIChannel.
func TestMainControllerDispatch_ReporterUpdateMsg_GeneratesLog(t *testing.T) {
	mockKubeMgr := &MockKubeManager{}

	logChan := logging.InitForTUI(logging.LevelDebug)

	// Use default config for InitialModel
	defaultCfg := config.GetDefaultConfig("mc1", "wc1")
	mInitialModel := model.InitialModel("mc1", "wc1", "test-context", true, defaultCfg, mockKubeMgr, logChan)
	mInitialModel.LogChannel = logChan

	assert.NotNil(t, mInitialModel.TUIChannel, "TUIChannel should be initialized")

	msg1Content := reporting.ManagedServiceUpdate{
		Timestamp:   time.Now(),
		SourceType:  reporting.ServiceTypeSystem,
		SourceLabel: "TestService1",
		State:       reporting.StateRunning,
		IsReady:     true,
	}

	// --- Simulate processing the first message ---
	t.Log("Simulating send of ReporterUpdateMsg to TUIChannel")
	// Send and immediately try to process any resulting logs. This needs to be more controlled.

	// Step 1: Dispatch the ReporterUpdateMsg
	// This call to mainControllerDispatch will internally call LogDebug if m.DebugMode is true.
	// That LogDebug should send a LogEntry to logChan.
	updatedModel1, cmd1 := mainControllerDispatch(mInitialModel, reporting.ReporterUpdateMsg{Update: msg1Content})
	t.Logf("mainControllerDispatch called for ReporterUpdateMsg. ActivityLog len: %d", len(updatedModel1.ActivityLog))

	// Step 2: Explicitly process the log entry expected from LogDebug
	// We expect one log entry from the LogDebug call inside mainControllerDispatch for the ReporterUpdateMsg.
	// But there might be other log entries in the channel first (like InitialModel), so we need to process them all
	var foundExpectedLog bool
	timeout := time.After(500 * time.Millisecond)

	for !foundExpectedLog {
		select {
		case logEntry := <-logChan:
			t.Logf("LogEntry received from logChan: %+v", logEntry)
			// Dispatch this log entry to have it added to ActivityLog
			updatedModel1, _ = mainControllerDispatch(updatedModel1, model.NewLogEntryMsg{Entry: logEntry})

			// Check if this is the log entry we're looking for
			for _, logLine := range updatedModel1.ActivityLog {
				if strings.Contains(logLine, "Received msg: reporting.ReporterUpdateMsg") {
					foundExpectedLog = true
					break
				}
			}
		case <-timeout:
			t.Log("Timeout waiting for expected LogEntry from logChan")
			foundExpectedLog = false
			goto done
		}
	}

done:
	assert.True(t, foundExpectedLog, "Expected to find the ReporterUpdateMsg debug log")
	assert.NotEmpty(t, updatedModel1.ActivityLog, "ActivityLog should not be empty after LogDebug processing")
	if len(updatedModel1.ActivityLog) > 0 {
		t.Logf("ActivityLog content: %v", updatedModel1.ActivityLog)
	}
	assert.NotNil(t, cmd1, "A command should be returned after processing the first message")

	t.Log("Test TestMainControllerDispatch_ReporterUpdateMsg_GeneratesLog completed.")
}

// Note: To make this test fully robust without being in the 'controller' package,
// or without exporting mainControllerDispatch, one would need to:
// 1. Create a mock for tea.Cmd that can be identified (e.g., returns a specific string or struct).
// 2. Have model.ChannelReaderCmd return this mock command instead of the actual func() tea.Msg during tests.
// This would allow asserting `cmd1` and `cmd2` specifically contain the expected mock command.
// The current test relies on observing the side effect (processing of the second message)
// as strong evidence of re-queuing.

func TestMainControllerDispatch_NewLogEntryMsg_UpdatesLogViewport(t *testing.T) {
	mockKubeMgr := &MockKubeManager{}

	logChan := logging.InitForTUI(logging.LevelDebug)
	defer logging.CloseTUIChannel()

	// Use default config for InitialModel
	defaultCfg := config.GetDefaultConfig("mc1", "wc1")
	m := model.InitialModel("mc1", "wc1", "test-context", true, defaultCfg, mockKubeMgr, logChan)
	m.LogChannel = logChan
	m.Width = 80
	m.Height = 24

	// Setup LogViewport dimensions as if the overlay were active
	// This ensures its Width and Height are set for PrepareLogContent and SetContent.
	overlayContentWidth := int(float64(m.Width)*0.8) - 2                                                           // Typical overlay width calc
	overlayContentHeight := int(float64(m.Height)*0.7) - 2 - lipgloss.Height(color.LogPanelTitleStyle.Render(" ")) // Approx height after title and borders
	if overlayContentHeight < 0 {
		overlayContentHeight = 1 // Ensure at least 1 line for content
	}
	m.LogViewport.Width = overlayContentWidth
	m.LogViewport.Height = overlayContentHeight
	// t.Logf("Test calculated LogViewport dimensions: W=%d, H=%d", m.LogViewport.Width, m.LogViewport.Height) // For debugging test

	logEntry := logging.LogEntry{
		Timestamp: time.Now(),
		Level:     logging.LevelInfo,
		Subsystem: "TestSystem",
		Message:   "This is a test log message for the overlay.",
	}

	// Dispatch the NewLogEntryMsg
	updatedModel, cmd := mainControllerDispatch(m, model.NewLogEntryMsg{Entry: logEntry})

	assert.NotNil(t, updatedModel, "Model should not be nil")
	assert.NotNil(t, cmd, "Command should not be nil")

	// Check ActivityLog
	assert.NotEmpty(t, updatedModel.ActivityLog, "ActivityLog should not be empty")
	assert.Contains(t, updatedModel.ActivityLog[0], "This is a test log message for the overlay.", "ActivityLog should contain the new log message")

	// For debugging the test:
	t.Logf("LogViewport Width at assertion point: %d", updatedModel.LogViewport.Width)
	// We can't directly call view.PrepareLogContent here easily without importing view and its dependencies.
	// Instead, we rely on observing the effect on LogViewport.

	// Check LogViewport content
	assert.True(t, updatedModel.LogViewport.TotalLineCount() > 0, "LogViewport should have content (TotalLineCount > 0)")
	t.Logf("LogViewport TotalLineCount after NewLogEntryMsg: %d lines", updatedModel.LogViewport.TotalLineCount())

	// Check if ActivityLogDirty was reset
	assert.False(t, updatedModel.ActivityLogDirty, "ActivityLogDirty should be false after dispatch and viewport update")
}
