package view

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"envctl/internal/color"
	"envctl/internal/config"
	"envctl/internal/tui/model"

	"github.com/charmbracelet/lipgloss"
)

func TestRender_HelpOverlay(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes

	defaultEnvctlCfg := config.GetDefaultConfig("MC", "WC")
	m := model.InitialModel("MC", "WC", "ctx", false, defaultEnvctlCfg, nil)
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
	m := model.InitialModel("MC", "WC", "ctx", false, config.GetDefaultConfig("MC", "WC"), nil)
	m.Width = 100
	m.Height = 30
	m.CurrentAppMode = model.ModeLogOverlay
	m.ActivityLog = []string{
		"[INFO] Log line 1",
		"[WARN] A warning message here",
		"[ERRO] An error occurred!",
		strings.Repeat("This is a very long log line to test wrapping and viewport behavior. ", 5),
	}
	// m.ActivityLogDirty is true by default from InitialModel, so Render should prepare and set content.

	// REMOVED direct LogViewport.SetContent and dimension setting here.
	// view.Render will now handle setting viewport dimensions and content based on m.ActivityLogDirty.

	m.Keys = model.DefaultKeyMap()

	originalHasDarkBackground := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	color.Initialize(true)
	defer func() {
		lipgloss.SetHasDarkBackground(originalHasDarkBackground)
		color.Initialize(originalHasDarkBackground)
	}()

	output := Render(m)
	goldenFile := filepath.Join("testdata", "render_log_overlay.golden")
	checkGoldenFile(t, goldenFile, output)
}

func TestRender_McpConfigOverlay(t *testing.T) {
	defaultEnvctlCfg := config.GetDefaultConfig("MC", "WC")
	m := model.InitialModel("MC", "WC", "ctx", false, defaultEnvctlCfg, nil)
	m.Width = 100
	m.Height = 30
	m.CurrentAppMode = model.ModeMcpConfigOverlay

	// GenerateMcpConfigJson now takes []config.MCPServerDefinition which is m.MCPServerConfig
	// To avoid importing controller, we create a representative JSON string.
	mcpJsonEntries := []string{}
	for _, srvCfg := range m.MCPServerConfig { // m.MCPServerConfig is now []config.MCPServerDefinition
		if !srvCfg.Enabled {
			continue
		}
		// Simplified URL for test, actual URL generation logic is in controller.GenerateMcpConfigJson
		url := "local://" + srvCfg.Name
		if srvCfg.Type == config.MCPServerTypeLocalCommand {
			for envKey, envVal := range srvCfg.Env {
				if strings.HasSuffix(strings.ToUpper(envKey), "_URL") && strings.HasPrefix(envVal, "http") {
					url = envVal
					break
				}
			}
		} else if srvCfg.Type == config.MCPServerTypeContainer && len(srvCfg.ContainerPorts) > 0 {
			parts := strings.Split(srvCfg.ContainerPorts[0], ":")
			hostPort := parts[0]
			url = fmt.Sprintf("http://localhost:%s", hostPort)
		}
		mcpJsonEntries = append(mcpJsonEntries, fmt.Sprintf(`{"name":"%s-mcp","url":"%s","description":"%s server"}`, srvCfg.Name, url, srvCfg.Type))
	}
	placeholderConfigJSON := fmt.Sprintf(`{"mcpServers":[%s]}`, strings.Join(mcpJsonEntries, ","))

	m.McpConfigViewport.SetContent(placeholderConfigJSON)
	m.McpConfigViewport.GotoTop()

	m.Keys = model.DefaultKeyMap()

	originalHasDarkBackground := lipgloss.HasDarkBackground()
	lipgloss.SetHasDarkBackground(true)
	color.Initialize(true)
	defer func() {
		lipgloss.SetHasDarkBackground(originalHasDarkBackground)
		color.Initialize(originalHasDarkBackground)
	}()

	output := Render(m)
	goldenFile := filepath.Join("testdata", "render_mcp_config_overlay.golden")
	checkGoldenFile(t, goldenFile, output)
}

func TestRenderCombinedLogPanel_Simple(t *testing.T) {
	defaultEnvctlCfg := config.GetDefaultConfig("MC", "WC")
	m := model.InitialModel("MC", "WC", "ctx", false, defaultEnvctlCfg, nil)
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
