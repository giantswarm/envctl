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

	// Determine panel style based on state
	var panelStyle lipgloss.Style
	if aggregatorInfo.connectedServers > 0 {
		panelStyle = color.PanelStatusRunningStyle
	} else {
		panelStyle = color.PanelStatusInitializingStyle
	}

	if isFocused {
		if aggregatorInfo.connectedServers > 0 {
			panelStyle = color.FocusedPanelStatusRunningStyle
		} else {
			panelStyle = color.FocusedPanelStatusInitializingStyle
		}
	}

	// Build content
	var b strings.Builder

	// Title
	b.WriteString(color.PortTitleStyle.Render(SafeIcon(IconLink) + " MCP Aggregator"))
	b.WriteString("\n")

	// Endpoint
	b.WriteString(fmt.Sprintf("Endpoint: %s", aggregatorInfo.endpoint))
	b.WriteString("\n")

	// Status
	statusIcon := SafeIcon(IconHourglass)
	statusText := "Initializing"
	if aggregatorInfo.connectedServers > 0 {
		statusIcon = SafeIcon(IconCheck)
		statusText = fmt.Sprintf("Connected: %d/%d", aggregatorInfo.connectedServers, aggregatorInfo.totalServers)
	}
	b.WriteString(fmt.Sprintf("Status: %s%s", statusIcon, statusText))
	b.WriteString("\n")

	// Tools count
	b.WriteString(fmt.Sprintf("Tools: %d", aggregatorInfo.toolCount))

	// Calculate content width
	frame := panelStyle.GetHorizontalFrameSize()
	contentWidth := width - frame
	if contentWidth < 0 {
		contentWidth = 0
	}

	return panelStyle.Copy().Width(contentWidth).Render(b.String())
}

// aggregatorInfo holds information about the aggregator service
type aggregatorInfo struct {
	endpoint         string
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
		totalServers:     m.AggregatorInfo.ServersTotal,
		connectedServers: m.AggregatorInfo.ServersConnected,
		toolCount:        m.AggregatorInfo.ToolsCount,
		resourceCount:    m.AggregatorInfo.ResourcesCount,
		promptCount:      m.AggregatorInfo.PromptsCount,
	}
}
