package view

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"envctl/internal/color"
	"envctl/internal/config"
	"envctl/internal/tui/model"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderHeader_Simple(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes

	initialTime := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)

	// Use GetDefaultConfig to get initial config.EnvctlConfig
	defaultEnvctlCfg := config.GetDefaultConfig("MCmgmt", "WCwork")
	m := model.InitialModel("MCmgmt", "WCwork", "test-context", false, defaultEnvctlCfg, nil)
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
	defaultEnvctlCfg := config.GetDefaultConfig("MCmgmt", "WCwork") // Get default EnvctlConfig
	m := model.InitialModel("MCmgmt", "WCwork", "test-context", false, defaultEnvctlCfg, nil)
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
	// Define PortForwardDefinitions for the EnvctlConfig
	pfDefs := []config.PortForwardDefinition{
		{Name: "Service One", Enabled: true, LocalPort: "8080", RemotePort: "80", TargetType: "service", TargetName: "service1", KubeContextTarget: "test-context", Icon: "🔗"},
		{Name: "My Pod", Enabled: true, LocalPort: "9090", RemotePort: "3000", TargetType: "pod", TargetName: "mypod", KubeContextTarget: "test-context", Icon: "📦"},
	}
	// Create an EnvctlConfig with these specific port forwards for the test
	envctlCfgWithPfs := config.EnvctlConfig{
		PortForwards: pfDefs,
		// Use default MCPServers for this test, or define specific ones if needed
		MCPServers:     config.GetDefaultConfig("MCmgmt", "").MCPServers,
		GlobalSettings: config.GetDefaultConfig("MCmgmt", "").GlobalSettings,
	}

	m := model.InitialModel("MCmgmt", "", "test-context", false, envctlCfgWithPfs, nil)
	m.CurrentAppMode = model.ModeMainDashboard
	m.Width = 120
	m.Height = 30

	// model.InitialModel populates m.PortForwards and m.PortForwardOrder from envctlCfgWithPfs.PortForwards.
	// It also populates m.McpServers and m.McpProxyOrder.
	// Now, set states for the specific PFs defined in pfDefs.
	if proc, ok := m.PortForwards["Service One"]; ok {
		proc.StatusMsg = "Running"
		proc.Running = true
	} else {
		t.Fatalf("Port forward 'Service One' not found in m.PortForwards after InitialModel")
	}
	if proc, ok := m.PortForwards["My Pod"]; ok {
		proc.StatusMsg = "Error: connection refused"
		proc.Running = false
		proc.Err = errors.New("placeholder error")
	} else {
		t.Fatalf("Port forward 'My Pod' not found in m.PortForwards after InitialModel")
	}

	m.FocusedPanelKey = "Service One" // Focus the first port-forward by Name

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
	// Use specific MCPServers for this test to control their state
	mcpDefs := []config.MCPServerDefinition{
		{Name: "k8s-api", Enabled: true, Type: config.MCPServerTypeLocalCommand, Command: []string{"cmd1"}, Icon: "☸"},
		{Name: "etcd", Enabled: false, Type: config.MCPServerTypeLocalCommand, Command: []string{"cmd2"}, Icon: "🗄️"}, // Test with a disabled one
		{Name: "other-proxy", Enabled: true, Type: config.MCPServerTypeLocalCommand, Command: []string{"cmd3"}, Icon: "⚙️"},
	}
	envctlCfgWithMcps := config.EnvctlConfig{
		MCPServers:     mcpDefs,
		PortForwards:   config.GetDefaultConfig("MCmgmt", "WCwork").PortForwards,
		GlobalSettings: config.GetDefaultConfig("MCmgmt", "WCwork").GlobalSettings,
	}

	m := model.InitialModel("MCmgmt", "WCwork", "test-context", false, envctlCfgWithMcps, nil)
	m.CurrentAppMode = model.ModeMainDashboard
	m.Width = 120
	m.Height = 30

	// model.InitialModel populates m.McpServers and m.McpProxyOrder.
	// Set states for the MCPs defined in mcpDefs.
	if proc, ok := m.McpServers["k8s-api"]; ok {
		proc.StatusMsg = "Running (PID: 123)"
		// proc.Pid = 123 // PID is part of McpServerProcess in model
	} else {
		t.Fatalf("MCP server 'k8s-api' not found after InitialModel")
	}

	// "etcd" is disabled, so it shouldn't be in m.McpServers if InitialModel filters by Enabled
	// Check if InitialModel adds disabled services to m.McpServers map (it currently does, view layer skips them)
	// If it does, its status should be "Inactive" or similar by default. The view test expects 3 cells.
	if proc, ok := m.McpServers["etcd"]; ok {
		proc.StatusMsg = "Inactive (disabled)"
	} else if _, presentInConfig := getConfigByName(mcpDefs, "etcd"); presentInConfig {
		// If InitialModel *doesn't* add disabled ones, this check is different
		// For now, assume InitialModel adds all from config, and view handles enabled state
		// t.Logf("MCP server 'etcd' (disabled) not found in m.McpServers, which might be okay if InitialModel filters.")
	}

	if proc, ok := m.McpServers["other-proxy"]; ok {
		proc.StatusMsg = "Error: Failed to start"
		proc.Err = errors.New("failed to start proxy")
	} else {
		t.Fatalf("MCP server 'other-proxy' not found after InitialModel")
	}

	m.FocusedPanelKey = "k8s-api"

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

// Helper to find config by name for MCP test assertions
func getConfigByName(configs []config.MCPServerDefinition, name string) (*config.MCPServerDefinition, bool) {
	for _, cfg := range configs {
		if cfg.Name == name {
			return &cfg, true
		}
	}
	return nil, false
}

func TestRenderStatusBar_Simple(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes

	defaultEnvctlCfg := config.GetDefaultConfig("MC", "WC")
	m := model.InitialModel("MC", "WC", "ctx", false, defaultEnvctlCfg, nil)
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
