package tui

import (
	"envctl/internal/mcpserver"
	"fmt"
	"time"

	"envctl/internal/utils"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// getManagementClusterContextIdentifier generates the MC part of a kube context name.
// func (m *model) getManagementClusterContextIdentifier() string {
//     return m.managementCluster
// }

// getWorkloadClusterContextIdentifier generates the WC context identifier based on MC and WC names.
// func (m *model) getWorkloadClusterContextIdentifier() string {
//     if m.workloadCluster == "" {
//         return ""
//     }
//     if m.managementCluster != "" && strings.HasPrefix(m.workloadCluster, m.managementCluster+"-") {
//         return m.workloadCluster
//     }
//     if m.managementCluster != "" {
//         return m.managementCluster + "-" + m.workloadCluster
//     }
//     return m.workloadCluster
// }

// InitialModel constructs the initial model with sensible defaults.
func InitialModel(mcName, wcName, kubeContext string, tuiDebug bool) model {
	ti := textinput.New()
	ti.Placeholder = "Management Cluster"
	ti.CharLimit = 156
	ti.Width = 50

	// Buffered channel to avoid blocking goroutines.
	tuiMsgChannel := make(chan tea.Msg, 100)

	// Force dark background for lipgloss; helps with colour-consistency.
	colorProfile := lipgloss.ColorProfile().String()
	lipgloss.SetHasDarkBackground(true)
	colorMode := fmt.Sprintf("%s (Dark: %v)", colorProfile, true)

	// Spinner setup.
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	m := model{
		managementClusterName:    mcName,
		workloadClusterName:      wcName,
		currentKubeContext:       kubeContext,
		portForwards:             make(map[string]*portForwardProcess),
		portForwardOrder:         make([]string, 0),
		mcpServers:               make(map[string]*mcpServerProcess),
		activityLog:              make([]string, 0),
		activityLogDirty:         true,
		logViewportLastWidth:     0,
		mainLogViewportLastWidth: 0,
		MCHealth:                 clusterHealthInfo{IsLoading: true},
		currentAppMode:           ModeInitializing,
		newConnectionInput:       ti,
		currentInputStep:         mcInputStep,
		TUIChannel:               tuiMsgChannel,
		debugMode:                tuiDebug,
		colorMode:                colorMode,
		logViewport:              viewport.New(0, 0),
		mainLogViewport:          viewport.New(0, 0),
		spinner:                  s,
		isLoading:                true,
		keys:                     DefaultKeyMap(),
		help:                     help.New(),
		mcpConfigViewport:        viewport.New(0, 0),
	}

	m.help.ShowAll = true
	// Styling tweaks live in styles.go; pull palette from there for consistency.
	helpOverlayBg := HelpOverlayBg
	descTextFg := HelpOverlayDescFg
	ellipsisFg := HelpOverlayEllipsisFg
	separatorFg := HelpOverlaySeparatorFg
	keyCapTextFg := HelpOverlayKeyFg

	m.help.Styles = help.Styles{
		Ellipsis:       lipgloss.NewStyle().Foreground(ellipsisFg).Background(helpOverlayBg),
		FullDesc:       lipgloss.NewStyle().Foreground(descTextFg).Background(helpOverlayBg),
		FullKey:        lipgloss.NewStyle().Foreground(keyCapTextFg).Bold(true),
		ShortDesc:      lipgloss.NewStyle().Foreground(descTextFg).Background(helpOverlayBg),
		ShortKey:       lipgloss.NewStyle().Foreground(keyCapTextFg).Bold(true),
		ShortSeparator: lipgloss.NewStyle().Foreground(separatorFg).Background(helpOverlayBg).SetString(" • "),
	}

	// Configure initial port-forwards.
	setupPortForwards(&m, mcName, wcName)

	// Dependency graph (MCP ↔︎ PF).
	m.dependencyGraph = buildDependencyGraph(&m)

	if wcName != "" {
		m.WCHealth = clusterHealthInfo{IsLoading: true}
	}

	if len(m.portForwardOrder) > 0 {
		m.focusedPanelKey = m.portForwardOrder[0]
	} else if mcName != "" {
		m.focusedPanelKey = mcPaneFocusKey
	}

	// Build navigation order for MCP proxies.
	for _, cfg := range mcpserver.PredefinedMcpServers {
		m.mcpProxyOrder = append(m.mcpProxyOrder, cfg.Name)
	}

	return m
}

// channelReaderCmd returns a Bubbletea command that forwards messages from the given channel.
func channelReaderCmd(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

// Init implements tea.Model and starts asynchronous bootstrap tasks.
func (m model) Init() tea.Cmd {
	var cmds []tea.Cmd

	cmds = append(cmds, getCurrentKubeContextCmd())
	cmds = append(cmds, fetchClusterListCmd())

	// Initial health checks using fully built context names
	if m.managementClusterName != "" {
		mcTargetContext := utils.BuildMcContext(m.managementClusterName)
		cmds = append(cmds, fetchNodeStatusCmd(mcTargetContext, true, m.managementClusterName))
	}
	if m.workloadClusterName != "" && m.managementClusterName != "" {
		wcTargetContext := utils.BuildWcContext(m.managementClusterName, m.workloadClusterName)
		// The third argument to fetchNodeStatusCmd is the short name for message tagging
		cmds = append(cmds, fetchNodeStatusCmd(wcTargetContext, false, m.workloadClusterName))
	}

	// Start port-forwards.
	cmds = append(cmds, getInitialPortForwardCmds(&m)...)

	// Start MCP proxies.
	if proxyCmds := startMcpProxiesCmd(m.TUIChannel); len(proxyCmds) > 0 {
		cmds = append(cmds, proxyCmds...)
	}

	// Ticker for periodic health updates.
	tickCmd := tea.Tick(healthUpdateInterval, func(t time.Time) tea.Msg { return requestClusterHealthUpdate{} })
	cmds = append(cmds, tickCmd)

	// Listen for async messages.
	cmds = append(cmds, channelReaderCmd(m.TUIChannel))

	// Spinner.
	cmds = append(cmds, m.spinner.Tick)

	return tea.Batch(cmds...)
}
