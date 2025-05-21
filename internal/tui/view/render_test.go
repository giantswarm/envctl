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

	"envctl/internal/color"
	"envctl/internal/mcpserver"      // For McpServerConfig for service mock
	"envctl/internal/portforwarding" // For PortForwardConfig for service mock
	"envctl/internal/service"        // For service.Services struct
	"envctl/internal/tui/model"

	"github.com/charmbracelet/lipgloss"
)

// updateGoldenFiles is a flag to indicate that golden files should be updated.
var updateGoldenFiles = flag.Bool("update", false, "Update golden files")

// mockClusterService, mockPFService, mockProxyService for model setup
type mockClusterService struct{}

func (m *mockClusterService) CurrentContext() (string, error)   { return "test-context", nil }
func (m *mockClusterService) SwitchContext(mc, wc string) error { return nil }
func (m *mockClusterService) Health(ctx context.Context, cluster string) (service.ClusterHealthInfo, error) {
	return service.ClusterHealthInfo{IsLoading: false, Error: nil}, nil
}

type mockPFService struct{}

func (m *mockPFService) Start(cfg portforwarding.PortForwardConfig, cb portforwarding.PortForwardUpdateFunc) (stopChan chan struct{}, err error) {
	return make(chan struct{}), nil
}
func (m *mockPFService) Status(id string) portforwarding.PortForwardProcessUpdate {
	return portforwarding.PortForwardProcessUpdate{InstanceKey: id, StatusMsg: "mocked pf status", Running: true}
}

type mockProxyService struct{}

func (m *mockProxyService) Start(cfg mcpserver.PredefinedMcpServer, updateFn func(mcpserver.McpProcessUpdate)) (stopChan chan struct{}, pid int, err error) {
	return make(chan struct{}), 0, nil
}
func (m *mockProxyService) Status(name string) (running bool, err error) { return true, nil }

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

	m := model.InitialModel("MCmgmt", "WCwork", "test-context", false)
	m.CurrentAppMode = model.ModeMainDashboard
	m.Width = 100 // Provide a fixed width for consistent rendering
	m.MCHealth = model.ClusterHealthInfo{ReadyNodes: 3, TotalNodes: 3, LastUpdated: initialTime}
	m.WCHealth = model.ClusterHealthInfo{ReadyNodes: 1, TotalNodes: 2, LastUpdated: initialTime}
	m.StatusBarMessage = "Test status message"
	m.ActivityLog = []string{"log1", "log2"}
	m.ColorMode = "TestColorMode (Dark: true)" // Simulate a color mode string
	m.FocusedPanelKey = model.McPaneFocusKey

	// Set up mock services as controller does
	m.Services = service.Services{
		Cluster: &mockClusterService{},
		PF:      &mockPFService{},
		Proxy:   &mockProxyService{},
	}

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
	m := model.InitialModel("MCmgmt", "WCwork", "test-context", false)
	m.CurrentAppMode = model.ModeMainDashboard
	m.Width = 120
	m.Height = 30
	m.MCHealth = model.ClusterHealthInfo{ReadyNodes: 3, TotalNodes: 3, LastUpdated: initialTime, StatusError: nil}
	m.WCHealth = model.ClusterHealthInfo{ReadyNodes: 1, TotalNodes: 2, LastUpdated: initialTime, StatusError: nil}
	m.FocusedPanelKey = model.McPaneFocusKey
	m.Services = service.Services{
		Cluster: &mockClusterService{},
		PF:      &mockPFService{},
		Proxy:   &mockProxyService{},
	}

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

	m := model.InitialModel("MCmgmt", "", "test-context", false) // No WC for this simple PF test
	m.CurrentAppMode = model.ModeMainDashboard
	m.Width = 120
	m.Height = 30
	m.Services = service.Services{
		Cluster: &mockClusterService{},
		PF:      &mockPFService{},
		Proxy:   &mockProxyService{},
	}

	pfKey1 := "svc/service1-8080"
	pfKey2 := "pod/mypod-9090"
	m.PortForwards = map[string]*model.PortForwardProcess{
		pfKey1: {
			Label:       "Service One",
			LocalPort:   8080,
			RemotePort:  80,
			TargetHost:  "svc/service1",
			ContextName: "test-context",
			StatusMsg:   "Running",
			Active:      true,
			Running:     true,
		},
		pfKey2: {
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
	m.PortForwardOrder = []string{pfKey1, pfKey2}
	m.FocusedPanelKey = pfKey1 // Focus the first port-forward

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

	m := model.InitialModel("MCmgmt", "WCwork", "test-context", false)
	m.CurrentAppMode = model.ModeMainDashboard
	m.Width = 120
	m.Height = 30
	m.Services = service.Services{
		Cluster: &mockClusterService{},
		PF:      &mockPFService{},
		Proxy:   &mockProxyService{},
	}

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

	m := model.InitialModel("MC", "WC", "ctx", false)
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

	m := model.InitialModel("MC", "WC", "ctx", false)
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

	m := model.InitialModel("MC", "WC", "ctx", false)
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
	m.Services = service.Services{
		Cluster: &mockClusterService{},
		PF:      &mockPFService{},
		Proxy:   &mockProxyService{},
	}

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

	m := model.InitialModel("MC", "WC", "ctx", false)
	m.Width = 100
	m.Height = 30
	m.CurrentAppMode = model.ModeMcpConfigOverlay
	// The Render function for McpConfigOverlay will call view.GenerateMcpConfigJson()
	// and set it to m.McpConfigViewport if it's empty. We can rely on that.
	// Or, pre-populate for more control, but let's test the default generation path.

	m.Keys = model.DefaultKeyMap() // For status bar
	m.Services = service.Services{
		Cluster: &mockClusterService{},
		PF:      &mockPFService{},
		Proxy:   &mockProxyService{},
	}

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

	m := model.InitialModel("MC", "WC", "ctx", false)
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

	m.Services = service.Services{
		Cluster: &mockClusterService{},
		PF:      &mockPFService{},
		Proxy:   &mockProxyService{},
	}

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
	m := model.InitialModel("", "", "", false)
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
		m := model.InitialModel("", "", "", false)
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
		m := model.InitialModel("", "", "", false)
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
	m := model.InitialModel("", "", "", false)
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

func TestRender_MainDashboard_Full(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes

	initialTime := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)
	m := model.InitialModel("MCmgmt", "WCwork", "test-context", false)
	m.CurrentAppMode = model.ModeMainDashboard
	m.Width = 120 // Sufficient width
	m.Height = 50 // Sufficient height for all sections including main log panel

	// Setup Health
	m.MCHealth = model.ClusterHealthInfo{ReadyNodes: 3, TotalNodes: 3, LastUpdated: initialTime}
	m.WCHealth = model.ClusterHealthInfo{ReadyNodes: 1, TotalNodes: 2, LastUpdated: initialTime}

	// Setup Port Forwards (from controller.SetupPortForwards for consistency, but simplified here)
	pfKey1 := "Prometheus (MC)"
	pfKey2 := "Grafana (MC)"
	m.PortForwards = map[string]*model.PortForwardProcess{
		pfKey1: {Label: pfKey1, LocalPort: 8080, RemotePort: 8080, StatusMsg: "Running", Active: true, Running: true},
		pfKey2: {Label: pfKey2, LocalPort: 3000, RemotePort: 3000, StatusMsg: "Error", Active: true, Running: false, Err: errors.New("pf error")},
	}
	m.PortForwardOrder = []string{model.McPaneFocusKey, model.WcPaneFocusKey, pfKey1, pfKey2} // Match typical order

	// Setup MCP Servers (from mcpserver.PredefinedMcpServers for consistency, simplified)
	mcpKeyK8s := "kubernetes-mcp" // Assuming this matches a predefined name used in view logic if any special handling
	mcpKeyProm := "prometheus-mcp"
	m.McpServers = map[string]*model.McpServerProcess{
		mcpKeyK8s:  {Label: "kubernetes API", Active: true, StatusMsg: "Running (PID: 123)"},
		mcpKeyProm: {Label: "prometheus", Active: false, StatusMsg: "Inactive"},
	}
	m.McpProxyOrder = []string{mcpKeyK8s, mcpKeyProm} // Match typical order from PredefinedMcpServers

	// Setup Activity Log for MainLogViewport
	m.ActivityLog = []string{
		"[INFO] System initialized.",
		"[DEBUG] Test debug message for main log.",
	}
	m.ActivityLogDirty = true // To trigger log processing in Render

	m.FocusedPanelKey = pfKey1 // Focus a port-forward
	m.Keys = model.DefaultKeyMap()
	m.StatusBarMessage = "Main dashboard ready."
	m.StatusBarMessageType = model.StatusBarSuccess

	m.Services = service.Services{
		Cluster: &mockClusterService{},
		PF:      &mockPFService{},
		Proxy:   &mockProxyService{},
	}

	// Force dark background for consistent testing
	originalHasDarkBackground := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	color.Initialize(true)
	defer func() {
		lipgloss.SetHasDarkBackground(originalHasDarkBackground)
		color.Initialize(originalHasDarkBackground)
	}()

	output := Render(m)
	goldenFile := filepath.Join("testdata", "render_main_dashboard_full.golden")
	checkGoldenFile(t, goldenFile, output)
}

// TestMain is needed to parse the -update flag
func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}
