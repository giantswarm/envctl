package view

import (
	"context" // For service mock context.Context
	"errors"  // For placeholder error
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"envctl/internal/color" // For ServiceManagerAPI if needed by tests (nil for now)
	"envctl/internal/reporting"

	// Added for KubeManagerAPI type for nil
	"envctl/internal/k8smanager"     // Using k8smanager
	"envctl/internal/mcpserver"      // For mcpserver.MCPServerConfig
	"envctl/internal/portforwarding" // For portforwarding.PortForwardingConfig
	"envctl/internal/tui/model"

	// Added for nil logChan type
	"github.com/charmbracelet/lipgloss"
)

// updateGoldenFiles is a flag to indicate that golden files should be updated.
var updateGoldenFiles = flag.Bool("update", false, "Update golden files")

// mockKubeManager is a mock for KubeManagerAPI, implementing only what view tests might need indirectly.
// For view tests, we mostly care that model fields (like MCHealth) are populated correctly prior to rendering.
// The actual KubeManagerAPI calls usually happen in the controller/model updates, not in view.Render.

type mockKubeManager struct{} // This mock will implement k8smanager.KubeManagerAPI

func (m *mockKubeManager) Login(clusterName string) (stdout string, stderr string, err error) {
	return "", "", nil
}
func (m *mockKubeManager) ListClusters() (*k8smanager.ClusterList, error) {
	return &k8smanager.ClusterList{}, nil
}
func (m *mockKubeManager) GetCurrentContext() (string, error)           { return "test-context", nil }
func (m *mockKubeManager) SwitchContext(targetContextName string) error { return nil }
func (m *mockKubeManager) GetAvailableContexts() ([]string, error) {
	return []string{"test-context"}, nil
}
func (m *mockKubeManager) BuildMcContextName(mcShortName string) string {
	return "teleport.giantswarm.io-" + mcShortName
}
func (m *mockKubeManager) BuildWcContextName(mcShortName, wcShortName string) string {
	return "teleport.giantswarm.io-" + mcShortName + "-" + wcShortName
}
func (m *mockKubeManager) StripTeleportPrefix(contextName string) string {
	return strings.TrimPrefix(contextName, "teleport.giantswarm.io-")
}
func (m *mockKubeManager) HasTeleportPrefix(contextName string) bool {
	return strings.HasPrefix(contextName, "teleport.giantswarm.io-")
}
func (m *mockKubeManager) GetClusterNodeHealth(ctx context.Context, kubeContextName string) (k8smanager.NodeHealth, error) {
	// Return a default healthy state for tests, or vary based on kubeContextName if needed for specific tests.
	return k8smanager.NodeHealth{ReadyNodes: 1, TotalNodes: 1, Error: nil}, nil
}

func (m *mockKubeManager) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
	return "mockProvider", nil
}

func (m *mockKubeManager) SetReporter(reporter reporting.ServiceReporter) {
	// Mock implementation, can be empty.
}

// checkGoldenFile compares the actual output with the golden file.
// If -update flag is set, it updates the golden file.
func checkGoldenFile(t *testing.T, goldenFile string, actualOutput string) {
	t.Helper()

	// Normalize line endings to LF for consistent comparisons
	actualOutputNormalized := strings.ReplaceAll(actualOutput, "\r\n", "\n")

	if *updateGoldenFiles {
		// Ensure the testdata directory exists
		if err := os.MkdirAll(filepath.Dir(goldenFile), 0755); err != nil {
			t.Fatalf("Failed to create testdata directory: %v", err)
		}
		if err := ioutil.WriteFile(goldenFile, []byte(actualOutputNormalized), 0644); err != nil {
			t.Fatalf("Failed to update golden file %s: %v", goldenFile, err)
		}
		t.Logf("Updated golden file: %s", goldenFile)
		return
	}

	expectedOutputBytes, err := ioutil.ReadFile(goldenFile)
	if err != nil {
		// If file doesn't exist, and not in update mode, create it so the user can inspect.
		if errOsStat := os.MkdirAll(filepath.Dir(goldenFile), 0755); errOsStat != nil {
			t.Fatalf("Failed to create testdata directory for new golden file: %v", errOsStat)
		}
		if errWrite := ioutil.WriteFile(goldenFile, []byte(actualOutputNormalized), 0644); errWrite != nil {
			t.Fatalf("Failed to write initial golden file %s: %v", goldenFile, errWrite)
		}
		t.Errorf("Golden file %s did not exist. Created it with current output. Verify and re-run. Or use -update flag.", goldenFile)
		return
	}

	expectedOutputNormalized := strings.ReplaceAll(string(expectedOutputBytes), "\r\n", "\n")

	if actualOutputNormalized != expectedOutputNormalized {
		var diffDetails strings.Builder
		diffDetails.WriteString(fmt.Sprintf("Output does not match golden file %s.\n", goldenFile))

		expectedLines := strings.Split(expectedOutputNormalized, "\n")
		actualLines := strings.Split(actualOutputNormalized, "\n")

		maxLines := len(expectedLines)
		if len(actualLines) > maxLines {
			maxLines = len(actualLines)
		}

		diffDetails.WriteString("Line-by-line differences:\n")
		foundDiff := false
		for i := 0; i < maxLines; i++ {
			var eLine, aLine string
			lineDiff := false
			if i < len(expectedLines) {
				eLine = expectedLines[i]
			} else {
				eLine = "<no line>"
				lineDiff = true
			}
			if i < len(actualLines) {
				aLine = actualLines[i]
			} else {
				aLine = "<no line>"
				lineDiff = true
			}

			if !lineDiff && eLine != aLine {
				lineDiff = true
			}

			if lineDiff {
				foundDiff = true
				diffDetails.WriteString(fmt.Sprintf("Line %d:\n", i+1))
				diffDetails.WriteString(fmt.Sprintf("  Expected: %s\n", strconv.Quote(eLine)))
				diffDetails.WriteString(fmt.Sprintf("  Actual:   %s\n", strconv.Quote(aLine)))
			}
		}

		if !foundDiff {
			diffDetails.WriteString("No line-by-line differences found, but content differs (possibly trailing whitespace or normalization issue).\n")
		}

		diffDetails.WriteString("\nFull Expected Output:\n")
		diffDetails.WriteString(expectedOutputNormalized)
		diffDetails.WriteString("\nFull Actual Output:\n")
		diffDetails.WriteString(actualOutputNormalized)

		t.Error(diffDetails.String())
	}
}

func TestRenderHeader_Simple(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes

	initialTime := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)

	m := model.InitialModel("MCmgmt", "WCwork", "test-context", false, mcpserver.GetMCPServerConfig(), nil, &mockKubeManager{}, nil)
	m.CurrentAppMode = model.ModeMainDashboard
	m.Width = 100 // Provide a fixed width for consistent rendering
	m.MCHealth = model.ClusterHealthInfo{ReadyNodes: 3, TotalNodes: 3, LastUpdated: initialTime}
	m.WCHealth = model.ClusterHealthInfo{ReadyNodes: 1, TotalNodes: 2, LastUpdated: initialTime}
	m.StatusBarMessage = "Test status message"
	m.ActivityLog = []string{"log1", "log2"}
	m.ColorMode = "TestColorMode (Dark: true)" // Simulate a color mode string
	m.FocusedPanelKey = model.McPaneFocusKey

	// Force dark background for consistent testing of adaptive colors
	originalHasDarkBackground := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	color.Initialize(true) // Re-initialize colors based on the forced dark mode
	defer func() {
		lipgloss.SetHasDarkBackground(originalHasDarkBackground) // Restore
		color.Initialize(originalHasDarkBackground)              // Restore colors
	}()

	headerOutput := renderHeader(m, m.Width-color.AppStyle.GetHorizontalFrameSize())
	goldenFile := filepath.Join("testdata", "header_simple.golden")
	checkGoldenFile(t, goldenFile, headerOutput)
}

func TestRenderContextPanesRow_Simple(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes

	initialTime := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)
	m := model.InitialModel("MCmgmt", "WCwork", "test-context", false, mcpserver.GetMCPServerConfig(), nil, &mockKubeManager{}, nil)
	m.CurrentAppMode = model.ModeMainDashboard
	m.Width = 120
	m.Height = 30
	m.MCHealth = model.ClusterHealthInfo{ReadyNodes: 3, TotalNodes: 3, LastUpdated: initialTime, StatusError: nil}
	m.WCHealth = model.ClusterHealthInfo{ReadyNodes: 1, TotalNodes: 2, LastUpdated: initialTime, StatusError: nil}
	m.FocusedPanelKey = model.McPaneFocusKey

	// Force dark background for consistent testing of adaptive colors
	originalHasDarkBackground := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	color.Initialize(true)
	defer func() {
		lipgloss.SetHasDarkBackground(originalHasDarkBackground)
		color.Initialize(originalHasDarkBackground)
	}()

	// Dimensions for the row based on typical calculation in Render func
	contentWidth := m.Width - color.AppStyle.GetHorizontalFrameSize()
	rowHeight := 5 // A typical minimum height for this row

	output := renderContextPanesRow(m, contentWidth, rowHeight)
	goldenFile := filepath.Join("testdata", "context_panes_row_simple.golden")
	checkGoldenFile(t, goldenFile, output)
}

func TestRenderPortForwardingRow_Simple(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes

	pfConfigs := []portforwarding.PortForwardingConfig{
		{Label: "Service One"},
		{Label: "My Pod"},
	}
	m := model.InitialModel("MCmgmt", "", "test-context", false, mcpserver.GetMCPServerConfig(), pfConfigs, &mockKubeManager{}, nil)
	m.CurrentAppMode = model.ModeMainDashboard
	m.Width = 120
	m.Height = 30

	m.PortForwards = map[string]*model.PortForwardProcess{
		"svc/service1-8080": {
			Label:       "Service One",
			LocalPort:   8080,
			RemotePort:  80,
			TargetHost:  "svc/service1",
			ContextName: "test-context",
			StatusMsg:   "Running",
			Active:      true,
			Running:     true,
		},
		"pod/mypod-9090": {
			Label:       "My Pod",
			LocalPort:   9090,
			RemotePort:  3000,
			TargetHost:  "pod/mypod",
			ContextName: "test-context",
			StatusMsg:   "Error: connection refused",
			Active:      true, // Still configured to be active
			Running:     false,
			Err:         errors.New("placeholder error"), // Using a standard error
		},
	}
	m.PortForwardOrder = []string{"svc/service1-8080", "pod/mypod-9090"}
	m.FocusedPanelKey = "svc/service1-8080" // Focus the first port-forward

	// Force dark background for consistent testing of adaptive colors
	originalHasDarkBackground := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	color.Initialize(true)
	defer func() {
		lipgloss.SetHasDarkBackground(originalHasDarkBackground)
		color.Initialize(originalHasDarkBackground)
	}()

	contentWidth := m.Width - color.AppStyle.GetHorizontalFrameSize()
	rowHeight := 7 // A typical height for this row

	output := renderPortForwardingRow(m, contentWidth, rowHeight)
	goldenFile := filepath.Join("testdata", "port_forwarding_row_simple.golden")
	checkGoldenFile(t, goldenFile, output)
}

func TestRenderMcpProxiesRow_Simple(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes

	m := model.InitialModel("MCmgmt", "WCwork", "test-context", false, mcpserver.GetMCPServerConfig(), nil, &mockKubeManager{}, nil)
	m.CurrentAppMode = model.ModeMainDashboard
	m.Width = 120
	m.Height = 30

	// Define some MCP servers
	mcpKey1 := "k8s-api"
	mcpKey2 := "etcd"
	mcpKey3 := "other-proxy"

	m.McpServers = map[string]*model.McpServerProcess{
		mcpKey1: {
			Label:     mcpKey1,
			Active:    true,
			StatusMsg: "Running (PID: 123)",
		},
		mcpKey2: {
			Label:     mcpKey2,
			Active:    false, // Not configured to be active
			StatusMsg: "Inactive",
		},
		mcpKey3: {
			Label:     mcpKey3,
			Active:    true,
			StatusMsg: "Error: Failed to start",
			Err:       errors.New("failed to start proxy"),
		},
	}
	m.McpProxyOrder = []string{mcpKey1, mcpKey2, mcpKey3}
	m.FocusedPanelKey = mcpKey1 // Focus the first MCP server

	// Force dark background for consistent testing of adaptive colors (even though NoColor is set, some styles might depend on this)
	originalHasDarkBackground := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	color.Initialize(true)
	defer func() {
		lipgloss.SetHasDarkBackground(originalHasDarkBackground)
		color.Initialize(originalHasDarkBackground)
	}()

	contentWidth := m.Width - color.AppStyle.GetHorizontalFrameSize()
	rowHeight := 5 // A typical height for this row

	output := renderMcpProxiesRow(m, contentWidth, rowHeight)
	goldenFile := filepath.Join("testdata", "mcp_proxies_row_simple.golden")
	checkGoldenFile(t, goldenFile, output)
}

func TestRenderStatusBar_Simple(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes

	m := model.InitialModel("MC", "WC", "ctx", false, mcpserver.GetMCPServerConfig(), nil, &mockKubeManager{}, nil)
	m.Width = 80
	m.CurrentAppMode = model.ModeMainDashboard // Or any mode that shows status bar
	m.StatusBarMessage = "This is an INFO message."
	m.StatusBarMessageType = model.StatusBarInfo
	m.Keys = model.DefaultKeyMap() // Needed for ShortHelp() in status bar

	// Force dark background for consistent testing (styles might depend on this)
	originalHasDarkBackground := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	color.Initialize(true)
	defer func() {
		lipgloss.SetHasDarkBackground(originalHasDarkBackground)
		color.Initialize(originalHasDarkBackground)
	}()

	output := renderStatusBar(m, m.Width)
	goldenFile := filepath.Join("testdata", "status_bar_info.golden")
	checkGoldenFile(t, goldenFile, output)

	// Test with an Error message type
	m.StatusBarMessage = "This is an ERROR message!"
	m.StatusBarMessageType = model.StatusBarError
	outputError := renderStatusBar(m, m.Width)
	goldenFileError := filepath.Join("testdata", "status_bar_error.golden")
	checkGoldenFile(t, goldenFileError, outputError)
}

func TestRender_HelpOverlay(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes

	m := model.InitialModel("MC", "WC", "ctx", false, mcpserver.GetMCPServerConfig(), nil, &mockKubeManager{}, nil)
	m.Width = 100
	m.Height = 30
	m.CurrentAppMode = model.ModeHelpOverlay
	m.Keys = model.DefaultKeyMap() // Help overlay uses m.Keys.FullHelp()

	// Force dark background for consistent testing
	originalHasDarkBackground := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	color.Initialize(true)
	defer func() {
		lipgloss.SetHasDarkBackground(originalHasDarkBackground)
		color.Initialize(originalHasDarkBackground)
	}()

	output := Render(m) // Call the main Render function
	goldenFile := filepath.Join("testdata", "render_help_overlay.golden")
	checkGoldenFile(t, goldenFile, output)
}

func TestRender_LogOverlay(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes

	m := model.InitialModel("MC", "WC", "ctx", false, mcpserver.GetMCPServerConfig(), nil, &mockKubeManager{}, nil)
	m.Width = 100
	m.Height = 30
	m.CurrentAppMode = model.ModeLogOverlay
	m.ActivityLog = []string{
		"[INFO] Log line 1",
		"[WARN] A warning message here",
		"[ERRO] An error occurred!",
		strings.Repeat("This is a very long log line to test wrapping and viewport behavior. ", 5),
	}
	// InitialLogViewport sets up the viewport, but Render also adjusts it.
	// We set some initial dimensions for the viewport within the model.
	// The Render function will further refine LogViewport.Width and LogViewport.Height.
	// Let an overlay width/height be calculated as in Render function
	overlayWidth := int(float64(m.Width) * 0.8)   // 80
	overlayHeight := int(float64(m.Height) * 0.7) // 21
	m.LogViewport.Width = overlayWidth - 2        // Assuming 2 is for border/padding
	m.LogViewport.Height = overlayHeight - 2      // Assuming 2 is for border/padding
	m.LogViewport.SetContent(strings.Join(m.ActivityLog, "\n"))

	m.Keys = model.DefaultKeyMap() // For status bar

	// Force dark background for consistent testing
	originalHasDarkBackground := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	color.Initialize(true)
	defer func() {
		lipgloss.SetHasDarkBackground(originalHasDarkBackground)
		color.Initialize(originalHasDarkBackground)
	}()

	output := Render(m) // Call the main Render function
	goldenFile := filepath.Join("testdata", "render_log_overlay.golden")
	checkGoldenFile(t, goldenFile, output)
}

func TestRender_McpConfigOverlay(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes

	m := model.InitialModel("MC", "WC", "ctx", false, mcpserver.GetMCPServerConfig(), nil, &mockKubeManager{}, nil)
	m.Width = 100
	m.Height = 30
	m.CurrentAppMode = model.ModeMcpConfigOverlay
	// The Render function for McpConfigOverlay will call view.GenerateMcpConfigJson()
	// and set it to m.McpConfigViewport if it's empty. We can rely on that.
	// Or, pre-populate for more control, but let's test the default generation path.

	m.Keys = model.DefaultKeyMap() // For status bar

	// Force dark background for consistent testing
	originalHasDarkBackground := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	color.Initialize(true)
	defer func() {
		lipgloss.SetHasDarkBackground(originalHasDarkBackground)
		color.Initialize(originalHasDarkBackground)
	}()

	output := Render(m) // Call the main Render function
	goldenFile := filepath.Join("testdata", "render_mcp_config_overlay.golden")
	checkGoldenFile(t, goldenFile, output)
}

func TestRenderCombinedLogPanel_Simple(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes

	m := model.InitialModel("MC", "WC", "ctx", false, mcpserver.GetMCPServerConfig(), nil, &mockKubeManager{}, nil)
	m.Width = 100
	m.Height = 40 // Need enough height for this panel to be rendered
	m.CurrentAppMode = model.ModeMainDashboard
	m.ActivityLog = []string{
		"[INIT] Application starting...",
		"[DEBUG] Debug mode enabled.",
		"[INFO] MC connection successful.",
	}

	// Setup MainLogViewport as it would be in the main Render function
	contentWidthForPanel := m.Width - color.AppStyle.GetHorizontalFrameSize() // e.g. 98
	logSectionHeight := 10                                                    // Arbitrary height for the log panel section

	m.MainLogViewport.Width = contentWidthForPanel - color.PanelStatusDefaultStyle.GetHorizontalFrameSize() // Adjust for panel padding
	m.MainLogViewport.Height = logSectionHeight - color.PanelStatusDefaultStyle.GetVerticalBorderSize() - lipgloss.Height(color.LogPanelTitleStyle.Render(" ")) - 1
	if m.MainLogViewport.Height < 0 {
		m.MainLogViewport.Height = 0
	}
	m.MainLogViewport.SetContent(strings.Join(m.ActivityLog, "\n"))
	m.ActivityLogDirty = false // Assume content is processed
	m.MainLogViewportLastWidth = m.MainLogViewport.Width

	// Force dark background for consistent testing
	originalHasDarkBackground := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	color.Initialize(true)
	defer func() {
		lipgloss.SetHasDarkBackground(originalHasDarkBackground)
		color.Initialize(originalHasDarkBackground)
	}()

	output := renderCombinedLogPanel(m, contentWidthForPanel, logSectionHeight)
	goldenFile := filepath.Join("testdata", "combined_log_panel_simple.golden")
	checkGoldenFile(t, goldenFile, output)
}

func TestRender_ModeQuitting(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes
	m := model.InitialModel("", "", "", false, mcpserver.GetMCPServerConfig(), nil, &mockKubeManager{}, nil)
	m.Width = 80
	m.Height = 24
	m.CurrentAppMode = model.ModeQuitting
	m.QuittingMessage = "Shutting down, please wait..."

	// Force dark background for consistent testing
	originalHasDarkBackground := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	color.Initialize(true)
	defer func() {
		lipgloss.SetHasDarkBackground(originalHasDarkBackground)
		color.Initialize(originalHasDarkBackground)
	}()

	output := Render(m)
	goldenFile := filepath.Join("testdata", "render_quitting.golden")
	checkGoldenFile(t, goldenFile, output)
}

func TestRender_ModeInitializing(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes

	t.Run("NoSize", func(t *testing.T) {
		m := model.InitialModel("", "", "", false, mcpserver.GetMCPServerConfig(), nil, &mockKubeManager{}, nil)
		m.Width = 0 // Critical: test case for when window size is not yet known
		m.Height = 0
		m.CurrentAppMode = model.ModeInitializing

		originalHasDarkBackground := lipgloss.HasDarkBackground()
		lipgloss.SetHasDarkBackground(true)
		color.Initialize(true)
		defer func() {
			lipgloss.SetHasDarkBackground(originalHasDarkBackground)
			color.Initialize(originalHasDarkBackground)
		}()

		output := Render(m)
		goldenFile := filepath.Join("testdata", "render_initializing_no_size.golden")
		checkGoldenFile(t, goldenFile, output)
	})

	t.Run("WithSize", func(t *testing.T) {
		m := model.InitialModel("", "", "", false, mcpserver.GetMCPServerConfig(), nil, &mockKubeManager{}, nil)
		m.Width = 80
		m.Height = 24
		m.CurrentAppMode = model.ModeInitializing

		originalHasDarkBackground := lipgloss.HasDarkBackground()
		lipgloss.SetHasDarkBackground(true)
		color.Initialize(true)
		defer func() {
			lipgloss.SetHasDarkBackground(originalHasDarkBackground)
			color.Initialize(originalHasDarkBackground)
		}()

		output := Render(m)
		goldenFile := filepath.Join("testdata", "render_initializing_with_size.golden")
		checkGoldenFile(t, goldenFile, output)
	})
}

func TestRender_ModeUnknown(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes
	m := model.InitialModel("", "", "", false, mcpserver.GetMCPServerConfig(), nil, &mockKubeManager{}, nil)
	m.Width = 80
	m.Height = 24
	m.CurrentAppMode = model.AppMode(999) // An undefined AppMode value

	originalHasDarkBackground := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	color.Initialize(true)
	defer func() {
		lipgloss.SetHasDarkBackground(originalHasDarkBackground)
		color.Initialize(originalHasDarkBackground)
	}()

	output := Render(m)
	goldenFile := filepath.Join("testdata", "render_unknown_mode.golden")
	checkGoldenFile(t, goldenFile, output)
}

func TestRender_MainDashboard_Normal(t *testing.T) {
	// NO_COLOR=true in Makefile should handle this, but for isolated test runs:
	// originalNoColor := os.Getenv("NO_COLOR")
	// os.Setenv("NO_COLOR", "true")
	// defer os.Setenv("NO_COLOR", originalNoColor)

	mcpCfgs := mcpserver.GetMCPServerConfig()
	pfCfgs := portforwarding.GetPortForwardConfig("MC", "WC")
	mockKube := &mockKubeManager{}

	m := model.InitialModel("MC", "WC", "teleport.giantswarm.io-MC-WC", false, mcpCfgs, pfCfgs, mockKube, nil)

	// Manually replicate SetupPortForwards logic for the test model
	m.PortForwards = make(map[string]*model.PortForwardProcess)
	m.PortForwardOrder = nil
	m.PortForwardOrder = append(m.PortForwardOrder, model.McPaneFocusKey)
	if m.WorkloadClusterName != "" { // Check wcName from model
		m.PortForwardOrder = append(m.PortForwardOrder, model.WcPaneFocusKey)
	}
	for _, pfCfg := range m.PortForwardingConfig { // This is already set by InitialModel from pfCfgs
		m.PortForwardOrder = append(m.PortForwardOrder, pfCfg.Label)
		// Initialize dummy PortForwardProcess entries, specific values set later in test
		m.PortForwards[pfCfg.Label] = &model.PortForwardProcess{Label: pfCfg.Label, Config: pfCfg, Active: true, StatusMsg: "Awaiting Setup..."}
	}

	m.Width = 120
	m.Height = 35
	m.CurrentAppMode = model.ModeMainDashboard

	if len(m.PortForwardOrder) > 0 {
		m.FocusedPanelKey = m.PortForwardOrder[0]
	} else {
		t.Logf("Warning: PortForwardOrder is empty in test. FocusedPanelKey not set to first PF.")
		m.FocusedPanelKey = model.McPaneFocusKey // Fallback focus for test
	}

	// Simulate some service states
	if proc, ok := m.PortForwards["Prometheus (MC)"]; ok {
		proc.Running = true
		proc.StatusMsg = "Running (PID: 123)"
	}
	if proc, ok := m.PortForwards["Alloy Metrics (WC)"]; ok {
		proc.Running = false
		proc.Err = errors.New("connection refused")
		proc.StatusMsg = "Error: connection refused"
	}
	if mcpProc, ok := m.McpServers["kubernetes"]; ok {
		mcpProc.Active = true
		mcpProc.StatusMsg = "Proxy Running - Healthy"
		mcpProc.Pid = 456
	}

	// Ensure all necessary styles are initialized (they should be by default by lipgloss or our color package)
	// Forcing dark background for consistent test output
	originalHasDarkBackground := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	color.Initialize(true) // Re-initialize colors for dark mode
	defer func() {
		lipgloss.SetHasDarkBackground(originalHasDarkBackground)
		color.Initialize(originalHasDarkBackground) // Restore original theme
	}()

	output := Render(m) // Call the main Render function
	goldenFile := filepath.Join("testdata", "render_main_dashboard_normal.golden")
	checkGoldenFile(t, goldenFile, output)
}

// TestMain is needed to parse the -update flag
func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}
