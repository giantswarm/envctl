package view

import (
	"envctl/internal/tui/components"
	"envctl/internal/tui/design"
	"envctl/internal/tui/model"
	"fmt"
	"strings"
)

// renderMainDashboard renders the main dashboard using the new component system
func renderMainDashboard(m *model.Model) string {
	// Ensure minimum dimensions
	width := m.Width
	height := m.Height
	if width < 80 {
		width = 80
	}
	if height < 24 {
		height = 24
	}

	// Create layout manager with safe dimensions
	layout := components.NewLayout(width, height)

	// Calculate dimensions
	headerHeight := 1
	statusBarHeight := 1
	_ = layout.CalculateContentArea(headerHeight, statusBarHeight)

	// Split content area: 40% aggregator, 60% bottom panels
	aggregatorHeight, bottomHeight := layout.SplitHorizontal(0.4)

	// 1. Render Header
	header := renderDashboardHeader(m, width)

	// 2. Render Aggregator Panel
	aggregatorPanel := renderAggregatorPanel(m, width, aggregatorHeight)

	// 3. Render Bottom Panels (Clusters and MCP Servers)
	bottomPanels := renderBottomPanelsNew(m, width, bottomHeight)

	// 4. Render Status Bar
	statusBar := renderDashboardStatusBar(m, width)

	// Join all components
	return components.JoinVertical(
		header,
		aggregatorPanel,
		bottomPanels,
		statusBar,
	)
}

// renderDashboardHeader renders the main header
func renderDashboardHeader(m *model.Model, width int) string {
	header := components.NewHeader("envctl TUI").
		WithSubtitle("Press h for Help | Tab to Navigate | q to Quit").
		WithWidth(width)

	if m.IsLoading {
		header = header.WithSpinner(m.Spinner.View())
	}

	return header.Render()
}

// renderAggregatorPanel renders the MCP aggregator panel
func renderAggregatorPanel(m *model.Model, width, height int) string {
	// Determine panel type based on aggregator state
	panelType := components.PanelTypeDefault
	if m.AggregatorInfo != nil {
		switch m.AggregatorInfo.State {
		case "Running", "running":
			panelType = components.PanelTypeSuccess
		case "Failed", "failed":
			panelType = components.PanelTypeError
		case "Starting", "starting":
			panelType = components.PanelTypeWarning
		}
	}

	// Build content
	content := buildAggregatorContent(m, width-4) // Account for border and padding

	// Create panel
	panel := components.NewPanel("MCP Aggregator").
		WithContent(content).
		WithDimensions(width, height).
		WithType(panelType).
		WithIcon("▣").
		SetFocused(m.FocusedPanelKey == "mcp-aggregator")

	return panel.Render()
}

// buildAggregatorContent builds the content for the aggregator panel
func buildAggregatorContent(m *model.Model, innerWidth int) string {
	var lines []string

	// Endpoint info
	endpoint := fmt.Sprintf("[http://localhost:%d/sse]", m.AggregatorConfig.Port)
	lines = append(lines, design.TextSecondaryStyle.Render(endpoint))
	lines = append(lines, "") // Empty line

	if m.AggregatorInfo != nil {
		// Status line
		statusIndicator := components.NewStatusIndicator(
			components.StatusFromString(m.AggregatorInfo.State),
		).WithText(m.AggregatorInfo.State)

		healthIndicator := components.NewStatusIndicator(
			components.StatusFromString(m.AggregatorInfo.Health),
		).WithText(m.AggregatorInfo.Health)

		statusLine := fmt.Sprintf("Status: %s    Health: %s",
			statusIndicator.Render(),
			healthIndicator.Render())
		lines = append(lines, statusLine)

		// Servers and Tools line
		serversStatus := components.StatusTypeHealthy
		if m.AggregatorInfo.ServersConnected < m.AggregatorInfo.ServersTotal {
			serversStatus = components.StatusTypeDegraded
		}
		serversIndicator := components.NewStatusIndicator(serversStatus).IconOnly()

		toolsText := fmt.Sprintf("%d Available", m.AggregatorInfo.ToolsCount)
		if m.AggregatorInfo.BlockedTools > 0 {
			toolsText = fmt.Sprintf("%d Available (%d blocked)",
				m.AggregatorInfo.ToolsCount, m.AggregatorInfo.BlockedTools)
		}
		if m.AggregatorInfo.YoloMode {
			toolsText += " [YOLO]"
		}

		serversLine := fmt.Sprintf("Servers: %s %d/%d Connected    Tools: %s %s",
			serversIndicator.Render(),
			m.AggregatorInfo.ServersConnected,
			m.AggregatorInfo.ServersTotal,
			design.SafeIcon(design.IconGear),
			toolsText)
		lines = append(lines, serversLine)

		// Resources and Prompts line
		resourcesLine := fmt.Sprintf("Resources: %d    Prompts: %d",
			m.AggregatorInfo.ResourcesCount,
			m.AggregatorInfo.PromptsCount)
		lines = append(lines, resourcesLine)
	} else {
		statusIndicator := components.NewStatusIndicator(components.StatusTypeStarting).
			WithText("Initializing...")
		lines = append(lines, "Status: "+statusIndicator.Render())
		lines = append(lines, "Servers: 0/0 Connected    Tools: 0 Available")
		lines = append(lines, "Resources: 0    Prompts: 0")
	}

	// Add tools preview if space and data available
	if len(m.MCPToolsWithStatus) > 0 {
		lines = append(lines, "")
		lines = append(lines, design.SubtitleStyle.Render("─ Available Tools ─"))

		// Show first few tools
		toolsToShow := 3
		if toolsToShow > len(m.MCPToolsWithStatus) {
			toolsToShow = len(m.MCPToolsWithStatus)
		}

		for i := 0; i < toolsToShow; i++ {
			tool := m.MCPToolsWithStatus[i]
			var toolLine string

			if tool.Blocked {
				icon := components.NewStatusIndicator(components.StatusTypeFailed).
					WithIcon(design.SafeIcon(design.IconBan)).IconOnly()
				toolLine = fmt.Sprintf("  %s %s [BLOCKED]", icon.Render(), tool.Name)
			} else {
				icon := components.NewStatusIndicator(components.StatusTypeHealthy).IconOnly()
				toolLine = fmt.Sprintf("  %s %s", icon.Render(), tool.Name)
			}

			lines = append(lines, toolLine)
		}

		if len(m.MCPToolsWithStatus) > toolsToShow {
			remaining := len(m.MCPToolsWithStatus) - toolsToShow
			lines = append(lines, design.TextSecondaryStyle.Render(
				fmt.Sprintf("  ...and %d more tools", remaining)))
		}
	}

	return strings.Join(lines, "\n")
}

// renderBottomPanelsNew renders the clusters and MCP servers panels
func renderBottomPanelsNew(m *model.Model, width, height int) string {
	layout := components.NewLayout(width, height)

	// Split width: 33% clusters, 67% MCP servers
	clustersWidth, mcpWidth := layout.SplitVertical(0.33)

	// Render clusters panel
	clustersPanel := renderClustersPanel(m, clustersWidth, height)

	// Render MCP servers panel
	mcpPanel := renderMCPServersPanel(m, mcpWidth-1, height) // -1 for gap

	// Join with gap
	return components.JoinHorizontal(1, clustersPanel, mcpPanel)
}

// renderClustersPanel renders the Kubernetes clusters panel
func renderClustersPanel(m *model.Model, width, height int) string {
	content := buildClustersContent(m, width-4)

	// Determine panel type based on overall cluster health
	panelType := components.PanelTypeDefault
	hasConnected := false
	hasFailed := false

	for _, conn := range m.K8sConnections {
		if conn.State == "Connected" || conn.State == "connected" {
			hasConnected = true
		} else if conn.State == "Failed" || conn.State == "failed" {
			hasFailed = true
		}
	}

	if hasFailed {
		panelType = components.PanelTypeError
	} else if hasConnected {
		panelType = components.PanelTypeSuccess
	}

	panel := components.NewPanel("Kubernetes Clusters").
		WithContent(content).
		WithDimensions(width, height).
		WithType(panelType).
		WithIcon(design.SafeIcon(design.IconKubernetes)).
		SetFocused(m.FocusedPanelKey == "clusters")

	return panel.Render()
}

// buildClustersContent builds the content for the clusters panel
func buildClustersContent(m *model.Model, innerWidth int) string {
	var lines []string

	for _, label := range m.K8sConnectionOrder {
		if conn, exists := m.K8sConnections[label]; exists {
			selected := m.FocusedPanelKey == "clusters" && getSelectedLabel(m) == label

			// Build line
			var line string
			if selected {
				line = design.ListItemSelectedStyle.Render("▶ ")
			} else {
				line = "  "
			}

			// Add cluster icon and name
			line += fmt.Sprintf("%s %s", design.SafeIcon(design.IconKubernetes), conn.Label)

			// Add status
			statusIndicator := components.NewStatusIndicator(
				components.StatusFromString(conn.State),
			).IconOnly()
			line += " " + statusIndicator.Render()

			lines = append(lines, line)

			// Add node info if selected
			if selected && (conn.State == "Connected" || conn.State == "connected") {
				nodeInfo := design.TextSecondaryStyle.Render(
					fmt.Sprintf("    Nodes: %d/%d Ready", conn.ReadyNodes, conn.TotalNodes))
				lines = append(lines, nodeInfo)
			}
		}
	}

	if len(lines) == 0 {
		lines = append(lines, design.TextSecondaryStyle.Render("No clusters configured"))
	}

	return strings.Join(lines, "\n")
}

// renderMCPServersPanel renders the MCP servers panel
func renderMCPServersPanel(m *model.Model, width, height int) string {
	content := buildMCPServersContent(m, width-4)

	// Determine panel type
	panelType := components.PanelTypeDefault
	hasRunning := false
	hasFailed := false

	for _, mcp := range m.MCPServers {
		if mcp.State == "Running" || mcp.State == "running" {
			hasRunning = true
		} else if mcp.State == "Failed" || mcp.State == "failed" {
			hasFailed = true
		}
	}

	if hasFailed {
		panelType = components.PanelTypeError
	} else if hasRunning {
		panelType = components.PanelTypeSuccess
	}

	panel := components.NewPanel("MCP Servers & Dependencies").
		WithContent(content).
		WithDimensions(width, height).
		WithType(panelType).
		WithIcon("☸").
		SetFocused(m.FocusedPanelKey == "mcpservers")

	return panel.Render()
}

// buildMCPServersContent builds the content for the MCP servers panel
func buildMCPServersContent(m *model.Model, innerWidth int) string {
	var lines []string

	for _, config := range m.MCPServerConfig {
		selected := m.FocusedPanelKey == "mcpservers" && getSelectedLabel(m) == config.Name

		// Build line
		var line string
		if selected {
			line = design.ListItemSelectedStyle.Render("▶ ")
		} else {
			line = "  "
		}

		// Get server status
		var statusIndicator *components.StatusIndicator
		if mcp, exists := m.MCPServers[config.Name]; exists {
			statusIndicator = components.NewStatusIndicator(
				components.StatusFromString(mcp.State),
			).IconOnly()
		} else {
			statusIndicator = components.NewStatusIndicator(
				components.StatusTypeStopped,
			).IconOnly()
		}

		// Add icon and name
		icon := "🔧" // Default icon since Icon field removed in Phase 3
		if icon == "" {
			icon = design.IconGear
		}
		line += fmt.Sprintf("%s %s MCP %s", design.SafeIcon(icon), config.Name, statusIndicator.Render())

		lines = append(lines, line)

		// Port forward dependencies have been removed
		if false {
			for _, pfName := range []string{} {
				pfLine := fmt.Sprintf("    └─ %s %s", design.SafeIcon(design.IconLink), pfName)

				if pf, exists := m.PortForwards[pfName]; exists {
					if pf.State == "Running" || pf.State == "running" {
						pfLine += design.TextSuccessStyle.Render(
							fmt.Sprintf(" (%d:%d) %s", pf.LocalPort, pf.RemotePort, design.SafeIcon(design.IconCheck)))
					} else {
						pfLine += " " + design.TextErrorStyle.Render(design.SafeIcon(design.IconCross))
					}
				}

				lines = append(lines, pfLine)
			}
		}
	}

	if len(lines) == 0 {
		lines = append(lines, design.TextSecondaryStyle.Render("No MCP servers configured"))
	}

	return strings.Join(lines, "\n")
}

// renderDashboardStatusBar renders the status bar
func renderDashboardStatusBar(m *model.Model, width int) string {
	statusBar := components.NewStatusBar(width)

	if m.StatusBarMessage != "" {
		statusBar = statusBar.WithMessage(m.StatusBarMessage, m.StatusBarMessageType)
	} else {
		// Default status
		leftText := design.TextSuccessStyle.Render("✓ Up")
		statusBar = statusBar.WithLeftText(leftText)
	}

	return statusBar.Render()
}

// Keep existing helper functions below...

// renderHeader renders the header - OLD VERSION kept for compatibility
func renderHeader(m *model.Model, width int) string {
	return renderDashboardHeader(m, width)
}

// renderAggregator renders the aggregator - OLD VERSION kept for compatibility
func renderAggregator(m *model.Model, width int, height int) string {
	return renderAggregatorPanel(m, width, height)
}

// renderBottomPanels renders bottom panels - OLD VERSION kept for compatibility
func renderBottomPanels(m *model.Model, width int, height int) string {
	return renderBottomPanelsNew(m, width, height)
}

// renderStatusBar renders status bar - OLD VERSION kept for compatibility
func renderStatusBar(m *model.Model, width int) string {
	return renderDashboardStatusBar(m, width)
}

// Keep existing helper functions
func renderClustersContent(m *model.Model, width int, height int) string {
	return buildClustersContent(m, width)
}

func renderMCPServersContent(m *model.Model, width int, height int) string {
	return buildMCPServersContent(m, width)
}

func getSelectedLabel(m *model.Model) string {
	// Get the selected item from the appropriate list
	switch m.FocusedPanelKey {
	case "clusters":
		if m.ClustersList != nil {
			listModel := m.ClustersList.(*ServiceListModel)
			if item := listModel.GetSelectedItem(); item != nil {
				return item.GetID()
			}
		}
	case "mcpservers":
		if m.MCPServersList != nil {
			listModel := m.MCPServersList.(*ServiceListModel)
			if item := listModel.GetSelectedItem(); item != nil {
				return item.GetID()
			}
		}
	}
	return ""
}
