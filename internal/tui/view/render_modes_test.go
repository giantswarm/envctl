package view

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"envctl/internal/color"
	"envctl/internal/config"
	"envctl/internal/tui/model"

	"github.com/charmbracelet/lipgloss"
)

func TestRender_ModeQuitting(t *testing.T) {
	// NO_COLOR=true in Makefile should handle disabling ANSI codes
	defaultEnvctlCfg := config.GetDefaultConfig("", "")
	m := model.InitialModel("", "", "", false, defaultEnvctlCfg, nil)
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

	defaultEnvctlCfg := config.GetDefaultConfig("", "")
	t.Run("NoSize", func(t *testing.T) {
		m := model.InitialModel("", "", "", false, defaultEnvctlCfg, nil)
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
		m := model.InitialModel("", "", "", false, defaultEnvctlCfg, nil)
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
	defaultEnvctlCfg := config.GetDefaultConfig("", "")
	m := model.InitialModel("", "", "", false, defaultEnvctlCfg, nil)
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
	defaultEnvctlCfg := config.GetDefaultConfig("MC", "WC")
	m := model.InitialModel("MC", "WC", "teleport.giantswarm.io-MC-WC", false, defaultEnvctlCfg, nil)

	m.Width = 120
	m.Height = 35
	m.CurrentAppMode = model.ModeMainDashboard

	prometheusPFName := "mc-prometheus"
	alloyMetricsPFName := "alloy-metrics-wc"
	kubernetesMcpName := "kubernetes"

	// Set states for services that are expected to be in defaultEnvctlCfg
	if proc, ok := m.PortForwards[prometheusPFName]; ok {
		proc.Running = true
		proc.StatusMsg = "Running (PID: 123)"
	} else {
		t.Logf("Default port forward '%s' not found in m.PortForwards after InitialModel for state update.", prometheusPFName)
	}
	if proc, ok := m.PortForwards[alloyMetricsPFName]; ok {
		proc.Running = false
		proc.Err = errors.New("connection refused")
		proc.StatusMsg = "Error: connection refused"
	} else {
		t.Logf("Default port forward '%s' not found in m.PortForwards after InitialModel for state update.", alloyMetricsPFName)
	}

	if mcpProc, ok := m.McpServers[kubernetesMcpName]; ok {
		// mcpProc.Active = true // Active is determined by config.Enabled, already handled by InitialModel
		mcpProc.StatusMsg = "Proxy Running - Healthy"
		// mcpProc.Pid = 456 // PID is part of McpServerProcess, set by runtime logic not test setup
	} else {
		t.Logf("Default MCP server '%s' not found in m.McpServers after InitialModel for state update.", kubernetesMcpName)
	}

	// Set focus
	if _, fok := m.PortForwards[prometheusPFName]; fok {
		m.FocusedPanelKey = prometheusPFName
	} else if len(m.PortForwardOrder) > 0 {
		// Try to pick a valid focus key from PortForwardOrder, skipping special keys if possible
		validFocusSet := false
		for _, key := range m.PortForwardOrder {
			if key != model.McPaneFocusKey && key != model.WcPaneFocusKey {
				m.FocusedPanelKey = key
				validFocusSet = true
				break
			}
		}
		if !validFocusSet {
			m.FocusedPanelKey = model.McPaneFocusKey // Fallback
		}
	} else {
		m.FocusedPanelKey = model.McPaneFocusKey // Fallback
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
