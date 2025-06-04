package view

import (
	"envctl/internal/color"
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderAggregatorPanel renders the MCP aggregator panel
func RenderAggregatorPanel(m *model.Model, width int, isFocused bool) string {
	// Get aggregator info from service data
	aggregatorInfo := getAggregatorInfo(m)
	if aggregatorInfo == nil {
		return ""
	}

	// Determine panel style based on state (like port forwards and MCP servers)
	var baseStyleForPanel, focusedBaseStyleForPanel lipgloss.Style
	var contentFg lipgloss.Style

	// Use actual service state from state transitions
	stateLower := strings.ToLower(aggregatorInfo.state)
	switch {
	case stateLower == "failed":
		baseStyleForPanel = color.PanelStatusErrorStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusErrorStyle
		contentFg = color.StatusMsgErrorStyle
	case stateLower == "running":
		baseStyleForPanel = color.PanelStatusRunningStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusRunningStyle
		contentFg = color.StatusMsgRunningStyle
	case stateLower == "starting":
		baseStyleForPanel = color.PanelStatusInitializingStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusInitializingStyle
		contentFg = color.StatusMsgInitializingStyle
	case stateLower == "stopped" || stateLower == "stopping":
		baseStyleForPanel = color.PanelStatusExitedStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusExitedStyle
		contentFg = color.StatusMsgExitedStyle
	default:
		baseStyleForPanel = color.PanelStatusInitializingStyle
		focusedBaseStyleForPanel = color.FocusedPanelStatusInitializingStyle
		contentFg = color.StatusMsgInitializingStyle
	}

	finalPanelStyle := baseStyleForPanel
	if isFocused {
		finalPanelStyle = focusedBaseStyleForPanel
	}

	// Override foreground color for content
	finalPanelStyle = finalPanelStyle.Copy().Foreground(contentFg.GetForeground())

	// Build content
	var b strings.Builder

	// Title
	b.WriteString(color.PortTitleStyle.Render(SafeIcon(IconLink) + " MCP Aggregator"))
	b.WriteString("\n")

	// Endpoint
	b.WriteString(fmt.Sprintf("Endpoint: %s", aggregatorInfo.endpoint))
	b.WriteString("\n")

	// Status line (like port forwards)
	var statusIcon string
	statusText := strings.TrimSpace(aggregatorInfo.state)
	switch {
	case stateLower == "running":
		statusIcon = SafeIcon(IconPlay)
	case stateLower == "failed":
		statusIcon = SafeIcon(IconCross)
	case stateLower == "starting":
		statusIcon = SafeIcon(IconHourglass)
	case stateLower == "stopped" || stateLower == "stopping":
		statusIcon = SafeIcon(IconStop)
	default:
		statusIcon = SafeIcon(IconHourglass)
		if statusText == "" {
			statusText = "Initializing"
		}
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Status: %s%s", statusIcon, statusText)))
	b.WriteString("\n")

	// Health line (like port forwards)
	var healthIcon, healthText string
	healthLower := strings.ToLower(aggregatorInfo.health)
	if stateLower == "running" && healthLower == "healthy" {
		healthIcon = SafeIcon(IconCheck)
		healthText = "Healthy"
	} else if stateLower == "failed" || healthLower == "unhealthy" {
		healthIcon = SafeIcon(IconCross)
		healthText = "Unhealthy"
	} else {
		healthIcon = SafeIcon(IconHourglass)
		healthText = "Checking..."
	}
	b.WriteString(contentFg.Render(fmt.Sprintf("Health: %s%s", healthIcon, healthText)))
	b.WriteString("\n")

	// Connected servers (if running)
	if stateLower == "running" {
		serversIcon := SafeIcon(IconCheck)
		if aggregatorInfo.connectedServers < aggregatorInfo.totalServers {
			serversIcon = SafeIcon(IconWarning)
		}
		b.WriteString(contentFg.Render(fmt.Sprintf("Servers: %s%d/%d", serversIcon, aggregatorInfo.connectedServers, aggregatorInfo.totalServers)))
		b.WriteString("\n")
	}

	// Tools count
	b.WriteString(contentFg.Render(fmt.Sprintf("Tools: %d", aggregatorInfo.toolCount)))

	// Calculate content width
	frame := finalPanelStyle.GetHorizontalFrameSize()
	contentWidth := width - frame
	if contentWidth < 0 {
		contentWidth = 0
	}

	return finalPanelStyle.Copy().Width(contentWidth).Render(b.String())
}

// aggregatorInfo holds information about the aggregator service
type aggregatorInfo struct {
	endpoint         string
	state            string
	health           string
	totalServers     int
	connectedServers int
	toolCount        int
	resourceCount    int
	promptCount      int
	servers          []serverInfo
}

// serverInfo holds information about a connected MCP server
type serverInfo struct {
	name      string
	connected bool
	toolCount int
}

// getAggregatorInfo extracts aggregator information from the model
func getAggregatorInfo(m *model.Model) *aggregatorInfo {
	// Check if aggregator info is available
	if m.AggregatorInfo == nil {
		// Fallback to checking if aggregator is configured
		if m.AggregatorConfig.Port > 0 {
			return &aggregatorInfo{
				endpoint:         fmt.Sprintf("http://localhost:%d/sse", m.AggregatorConfig.Port),
				state:            "Stopped",
				health:           "Unknown",
				totalServers:     0,
				connectedServers: 0,
				toolCount:        0,
			}
		}
		return nil
	}

	// Use the aggregator info from the API
	return &aggregatorInfo{
		endpoint:         m.AggregatorInfo.Endpoint,
		state:            m.AggregatorInfo.State,
		health:           m.AggregatorInfo.Health,
		totalServers:     m.AggregatorInfo.ServersTotal,
		connectedServers: m.AggregatorInfo.ServersConnected,
		toolCount:        m.AggregatorInfo.ToolsCount,
		resourceCount:    m.AggregatorInfo.ResourcesCount,
		promptCount:      m.AggregatorInfo.PromptsCount,
	}
}
