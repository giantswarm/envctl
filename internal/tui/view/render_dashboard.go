package view

import (
	"envctl/internal/color"
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderMainDashboard renders the main dashboard
func renderMainDashboard(m *model.Model) string {
	// Use 95% of terminal dimensions to ensure everything fits
	width := int(float64(m.Width) * 0.95)
	height := int(float64(m.Height) * 0.95)

	// Calculate component heights
	headerHeight := 1
	statusHeight := 1
	contentHeight := height - headerHeight - statusHeight

	// Split content: 40% aggregator, 60% bottom panels
	aggregatorHeight := int(float64(contentHeight) * 0.4)
	if aggregatorHeight < 8 {
		aggregatorHeight = 8
	}
	bottomHeight := contentHeight - aggregatorHeight
	if bottomHeight < 10 {
		bottomHeight = 10
	}

	// 1. Header
	header := renderHeader(m, width)

	// 2. Aggregator Panel
	aggregatorPanel := renderAggregator(m, width, aggregatorHeight)

	// 3. Bottom panels (Clusters and MCP Servers)
	bottomPanel := renderBottomPanels(m, width, bottomHeight)

	// 4. Status bar
	statusBar := renderStatusBar(m, width)

	// Join all parts
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		aggregatorPanel,
		bottomPanel,
		statusBar,
	)
}

func renderHeader(m *model.Model, width int) string {
	title := "envctl TUI - Press h for Help | Tab to Navigate | q to Quit"
	if m.IsLoading {
		title = m.Spinner.View() + " " + title
	}

	return color.HeaderStyle.Copy().
		Width(width).
		MaxWidth(width).
		Render(title)
}

func renderAggregator(m *model.Model, width int, height int) string {
	// Apply border and styling first to get the actual content area
	style := color.PanelStyle.Copy()
	if m.FocusedPanelKey == "mcp-aggregator" {
		style = color.FocusedPanelStyle.Copy()
	}

	// Calculate inner dimensions (accounting for border)
	innerWidth := width - style.GetHorizontalFrameSize()
	innerHeight := height - style.GetVerticalFrameSize()

	// Build aggregator content
	var lines []string

	// Title line with endpoint
	titleLine := "▣ MCP Aggregator"
	endpoint := fmt.Sprintf("[http://localhost:%d/sse]", m.AggregatorConfig.Port)
	padding := innerWidth - lipgloss.Width(titleLine) - lipgloss.Width(endpoint)
	if padding > 0 {
		titleLine += strings.Repeat(" ", padding) + endpoint
	}
	lines = append(lines, titleLine)
	lines = append(lines, "") // Empty line

	// Status info
	if m.AggregatorInfo != nil {
		// Status and Health line
		statusIcon := SafeIcon(IconPlay)
		if m.AggregatorInfo.State == "Stopped" || m.AggregatorInfo.State == "stopped" {
			statusIcon = SafeIcon(IconStop)
		} else if m.AggregatorInfo.State == "Failed" || m.AggregatorInfo.State == "failed" {
			statusIcon = SafeIcon(IconCross)
		}

		healthIcon := SafeIcon(IconCheck)
		if m.AggregatorInfo.Health != "Healthy" && m.AggregatorInfo.Health != "healthy" {
			healthIcon = SafeIcon(IconWarning)
		}

		statusLine := fmt.Sprintf("Status: %s %-12s    Health: %s %s",
			statusIcon, m.AggregatorInfo.State,
			healthIcon, m.AggregatorInfo.Health)
		lines = append(lines, statusLine)

		// Servers and Tools line
		serversIcon := SafeIcon(IconCheck)
		if m.AggregatorInfo.ServersConnected < m.AggregatorInfo.ServersTotal {
			serversIcon = SafeIcon(IconWarning)
		}

		serversLine := fmt.Sprintf("Servers: %s %d/%d Connected    Tools: %s %d Available",
			serversIcon, m.AggregatorInfo.ServersConnected, m.AggregatorInfo.ServersTotal,
			SafeIcon(IconGear), m.AggregatorInfo.ToolsCount)
		lines = append(lines, serversLine)
	} else {
		lines = append(lines, "Status: "+SafeIcon(IconHourglass)+" Initializing...")
		lines = append(lines, "Servers: 0/0 Connected    Tools: 0 Available")
	}

	// Add connected servers mini-dashboard if space allows
	if len(lines) < innerHeight-3 && len(m.MCPServers) > 0 {
		lines = append(lines, "") // Empty line
		// Create a properly sized box for the mini-dashboard
		boxWidth := innerWidth - 2
		if boxWidth > 50 {
			boxWidth = 50 // Cap at 50 chars wide
		}
		header := "─ Connected MCP Servers ─"
		headerPadding := boxWidth - len(header) - 2
		if headerPadding < 0 {
			headerPadding = 0
		}
		lines = append(lines, "┌"+header+strings.Repeat("─", headerPadding)+"┐")

		var serverStatuses []string
		for _, config := range m.MCPServerConfig {
			if mcp, exists := m.MCPServers[config.Name]; exists {
				icon := SafeIcon(config.Icon)
				if icon == "" {
					icon = SafeIcon(IconGear)
				}

				statusIcon := ""
				if mcp.State == "Running" || mcp.State == "running" {
					statusIcon = SafeIcon(IconCheck)
				} else if mcp.State == "Failed" || mcp.State == "failed" {
					statusIcon = SafeIcon(IconCross)
				} else {
					statusIcon = SafeIcon(IconHourglass)
				}

				serverStatuses = append(serverStatuses, fmt.Sprintf("%s %s %s", icon, config.Name, statusIcon))
			}
		}

		if len(serverStatuses) > 0 {
			serversLine := strings.Join(serverStatuses, "    ")
			// Center the servers line if it fits
			if lipgloss.Width(serversLine) < innerWidth-4 {
				leftPad := (innerWidth - 4 - lipgloss.Width(serversLine)) / 2
				serversLine = strings.Repeat(" ", leftPad) + serversLine
			}
			// Ensure the line fits within the box
			if lipgloss.Width(serversLine) > boxWidth-4 {
				serversLine = serversLine[:boxWidth-7] + "..."
			}
			// Pad to fill the box width
			padding := boxWidth - 4 - lipgloss.Width(serversLine)
			if padding > 0 {
				serversLine += strings.Repeat(" ", padding)
			}
			lines = append(lines, "│ "+serversLine+" │")
		}
		lines = append(lines, "└"+strings.Repeat("─", boxWidth-2)+"┘")
	}

	// Add empty lines if needed to fill height
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}

	// Join lines and render with style
	content := strings.Join(lines[:innerHeight], "\n")

	return style.
		Width(width).
		Height(height).
		Render(content)
}

func renderBottomPanels(m *model.Model, width int, height int) string {
	// Split width: 33% clusters, 67% MCP servers
	clustersWidth := width / 3
	mcpWidth := width - clustersWidth - 1 // -1 for gap

	// Render clusters panel
	clustersContent := renderClustersContent(m, clustersWidth-2, height-2) // -2 for border
	clustersStyle := color.PanelStyle.Copy()
	if m.FocusedPanelKey == "clusters" {
		clustersStyle = color.FocusedPanelStyle.Copy()
	}
	clustersPanel := clustersStyle.
		Width(clustersWidth).
		Height(height).
		Render(clustersContent)

	// Render MCP servers panel
	mcpContent := renderMCPServersContent(m, mcpWidth-2, height-2) // -2 for border
	mcpStyle := color.PanelStyle.Copy()
	if m.FocusedPanelKey == "mcpservers" {
		mcpStyle = color.FocusedPanelStyle.Copy()
	}
	mcpPanel := mcpStyle.
		Width(mcpWidth).
		Height(height).
		Render(mcpContent)

	// Join with gap
	return lipgloss.JoinHorizontal(lipgloss.Top, clustersPanel, " ", mcpPanel)
}

func renderClustersContent(m *model.Model, width int, height int) string {
	var content strings.Builder

	// Title
	content.WriteString("⎈ Kubernetes Clusters\n\n")

	// List clusters
	lineCount := 2 // title + blank line
	for _, label := range m.K8sConnectionOrder {
		if lineCount >= height {
			break
		}

		if conn, exists := m.K8sConnections[label]; exists {
			selected := m.FocusedPanelKey == "clusters" && getSelectedLabel(m) == label
			prefix := "  "
			if selected {
				prefix = "▶ "
			}

			line := fmt.Sprintf("%s%s %s", prefix, SafeIcon(IconKubernetes), conn.Label)
			// Check if this is the management cluster
			if conn.Label == m.ManagementClusterName || strings.Contains(conn.Label, "(MC)") {
				line += " (MC)"
			} else if conn.Label == m.WorkloadClusterName || strings.Contains(conn.Label, "(WC)") {
				line += " (WC)"
			}

			// Add status
			if conn.State == "Connected" || conn.State == "connected" {
				line += fmt.Sprintf(" %s", SafeIcon(IconCheck))
			} else {
				line += fmt.Sprintf(" %s", SafeIcon(IconHourglass))
			}

			content.WriteString(line + "\n")
			lineCount++

			// Add node info if space
			if lineCount < height && selected {
				nodeInfo := fmt.Sprintf("    Nodes: %d/%d Ready", conn.ReadyNodes, conn.TotalNodes)
				content.WriteString(nodeInfo + "\n")
				lineCount++
			}
		}
	}

	return content.String()
}

func renderMCPServersContent(m *model.Model, width int, height int) string {
	var content strings.Builder

	// Title
	content.WriteString("☸ MCP Servers & Dependencies\n\n")

	// List MCP servers
	lineCount := 2 // title + blank line
	for _, config := range m.MCPServerConfig {
		if lineCount >= height {
			break
		}

		selected := m.FocusedPanelKey == "mcpservers" && getSelectedLabel(m) == config.Name
		prefix := "  "
		if selected {
			prefix = "▶ "
		}

		// Get server info
		var statusIcon string
		if mcp, exists := m.MCPServers[config.Name]; exists {
			if mcp.State == "Running" || mcp.State == "running" {
				statusIcon = SafeIcon(IconCheck)
			} else if mcp.State == "Failed" || mcp.State == "failed" {
				statusIcon = SafeIcon(IconCross)
			} else {
				statusIcon = SafeIcon(IconHourglass)
			}
		} else {
			statusIcon = SafeIcon(IconStop)
		}

		icon := config.Icon
		if icon == "" {
			icon = IconGear
		}

		line := fmt.Sprintf("%s%s %s MCP %s", prefix, SafeIcon(icon), config.Name, statusIcon)
		content.WriteString(line + "\n")
		lineCount++

		// Show port forward dependencies if selected and space available
		if selected && lineCount < height && len(config.RequiresPortForwards) > 0 {
			for _, pfName := range config.RequiresPortForwards {
				if lineCount >= height {
					break
				}

				pfLine := fmt.Sprintf("    └─ %s %s", SafeIcon(IconLink), pfName)
				if pf, exists := m.PortForwards[pfName]; exists {
					if pf.State == "Running" || pf.State == "running" {
						pfLine += fmt.Sprintf(" (%d:%d) %s", pf.LocalPort, pf.RemotePort, SafeIcon(IconCheck))
					} else {
						pfLine += " " + SafeIcon(IconCross)
					}
				}

				content.WriteString(pfLine + "\n")
				lineCount++
			}
		}
	}

	return content.String()
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

// renderStatusBar renders the status bar at the bottom
func renderStatusBar(m *model.Model, width int) string {
	// Simple status bar for now
	leftText := "✓ Up"
	rightText := fmt.Sprintf("MC: %s", m.ManagementClusterName)
	if m.WorkloadClusterName != "" {
		rightText += fmt.Sprintf(" / WC: %s", m.WorkloadClusterName)
	}

	// Calculate padding
	padding := width - lipgloss.Width(leftText) - lipgloss.Width(rightText)
	if padding < 1 {
		padding = 1
	}

	statusText := leftText + strings.Repeat(" ", padding) + rightText

	return lipgloss.NewStyle().
		Width(width).
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230")).
		Render(statusText)
}
