package view

import (
	"envctl/internal/tui/model"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MCPServerWithDepsListItem represents an MCP server with its dependencies in the list
type MCPServerWithDepsListItem struct {
	MCPServerListItem
	PortForwards []PortForwardListItem
	IsExpanded   bool
}

// Title returns the display title for the MCP server with expansion indicator
func (i MCPServerWithDepsListItem) Title() string {
	expansionIcon := "▶"
	if i.IsExpanded && len(i.PortForwards) > 0 {
		expansionIcon = "▼"
	}
	return fmt.Sprintf("%s %s %s MCP", expansionIcon, i.GetIcon(), i.GetName())
}

// Description returns the display description including dependency count
func (i MCPServerWithDepsListItem) Description() string {
	desc := i.MCPServerListItem.BaseListItem.Description
	if len(i.PortForwards) > 0 {
		if i.IsExpanded {
			// When expanded, show status
			desc = fmt.Sprintf("%s • Tools: %d", desc, countTools(i.GetName()))
		} else {
			// When collapsed, show dependency count
			activeCount := 0
			for _, pf := range i.PortForwards {
				if pf.GetStatus() == StatusRunning {
					activeCount++
				}
			}
			desc = fmt.Sprintf("%s • %d/%d deps", desc, activeCount, len(i.PortForwards))
		}
	}
	return desc
}

// Helper to count tools (placeholder - should get from actual data)
func countTools(mcpName string) int {
	// This should be retrieved from the actual MCP server info
	// For now, return a placeholder
	switch mcpName {
	case "kubernetes":
		return 15
	case "prometheus":
		return 23
	case "grafana":
		return 19
	default:
		return 0
	}
}

// BuildMCPServersWithDependenciesList creates a hierarchical list of MCP servers with their port forward dependencies
func BuildMCPServersWithDependenciesList(m *model.Model, width, height int, focused bool) *ServiceListModel {
	items := []list.Item{}

	// Map port forwards to their dependent MCP servers
	pfByMCP := make(map[string][]PortForwardListItem)

	// First, organize port forwards by their MCP dependencies
	for _, config := range m.PortForwardingConfig {
		pfItem := PortForwardListItem{}

		if pf, exists := m.PortForwards[config.Name]; exists {
			pfItem = ConvertPortForwardToListItem(pf)
		} else {
			// Create placeholder item
			pfItem = PortForwardListItem{
				BaseListItem: BaseListItem{
					ID:          config.Name,
					Name:        config.Name,
					Status:      StatusStopped,
					Health:      HealthUnknown,
					Icon:        SafeIcon(config.Icon),
					Description: fmt.Sprintf("%s/%s", config.TargetType, config.TargetName),
					Details:     fmt.Sprintf("Port: %s:%s (Not Started)", config.LocalPort, config.RemotePort),
				},
				LocalPort:  0,
				RemotePort: 0,
				TargetType: config.TargetType,
				TargetName: config.TargetName,
			}
		}

		// Find which MCP servers depend on this port forward
		for _, mcpConfig := range m.MCPServerConfig {
			for _, requiredPF := range mcpConfig.RequiresPortForwards {
				if requiredPF == config.Name {
					pfByMCP[mcpConfig.Name] = append(pfByMCP[mcpConfig.Name], pfItem)
					break
				}
			}
		}
	}

	// Now build the hierarchical list
	for _, config := range m.MCPServerConfig {
		mcpItem := MCPServerWithDepsListItem{
			PortForwards: pfByMCP[config.Name],
			IsExpanded:   false, // Start collapsed
		}

		if mcp, exists := m.MCPServers[config.Name]; exists {
			mcpItem.MCPServerListItem = ConvertMCPServerToListItem(mcp)
		} else {
			// Create placeholder item
			mcpItem.MCPServerListItem = MCPServerListItem{
				BaseListItem: BaseListItem{
					ID:          config.Name,
					Name:        config.Name,
					Status:      StatusStopped,
					Health:      HealthUnknown,
					Icon:        SafeIcon(config.Icon),
					Description: "Not Started",
					Details:     fmt.Sprintf("MCP Server: %s (Not Started)", config.Name),
				},
			}
		}

		items = append(items, mcpItem)

		// If expanded, add port forward items as well
		if mcpItem.IsExpanded {
			for _, pf := range mcpItem.PortForwards {
				items = append(items, pf)
			}
		}
	}

	// Create the list with a custom delegate
	l := list.New(items, MCPWithDepsItemDelegate{}, width, height)
	l.Title = "MCP Servers & Dependencies"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	// Style the title
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170")).
		PaddingLeft(1)

	if focused {
		l.Styles.Title = l.Styles.Title.
			Background(lipgloss.Color("238")).
			Foreground(lipgloss.Color("205"))
	}

	// Status bar styling
	l.Styles.StatusBar = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "60", Dark: "110"}).
		PaddingLeft(2)

	return &ServiceListModel{
		List:     l,
		ListType: "mcpservers",
	}
}

// MCPWithDepsItemDelegate handles rendering for MCP servers with dependencies
type MCPWithDepsItemDelegate struct{}

func (d MCPWithDepsItemDelegate) Height() int                             { return 1 }
func (d MCPWithDepsItemDelegate) Spacing() int                            { return 0 }
func (d MCPWithDepsItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d MCPWithDepsItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	// Check if it's an MCP server or a port forward
	switch item := listItem.(type) {
	case MCPServerWithDepsListItem:
		// Render MCP server
		d.renderMCPServer(w, m, index, item)
	case PortForwardListItem:
		// Render port forward as a sub-item
		d.renderPortForward(w, m, index, item)
	default:
		// Fallback to common delegate
		CommonItemDelegate{}.Render(w, m, index, listItem)
	}
}

func (d MCPWithDepsItemDelegate) renderMCPServer(w io.Writer, m list.Model, index int, item MCPServerWithDepsListItem) {
	var content strings.Builder

	// Build the display
	content.WriteString(item.Title())
	content.WriteString(" ")

	// Status icon
	statusIcon := GetStatusIcon(item.GetStatus())
	statusStyle := GetStatusColor(item.GetStatus())
	content.WriteString(statusStyle.Render(statusIcon))

	// Description
	if desc := item.Description(); desc != "" {
		content.WriteString(" ")
		content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(desc))
	}

	// Render with selection indicator
	str := content.String()
	if index == m.Index() {
		str = lipgloss.NewStyle().
			Foreground(lipgloss.Color("170")).
			Bold(true).
			Render("▶ " + str)
	} else {
		str = "  " + str
	}

	fmt.Fprint(w, str)
}

func (d MCPWithDepsItemDelegate) renderPortForward(w io.Writer, m list.Model, index int, item PortForwardListItem) {
	var content strings.Builder

	// Indent for sub-item
	content.WriteString("    └─ ")

	// Icon and name
	if item.GetIcon() != "" {
		content.WriteString(item.GetIcon())
		content.WriteString(" ")
	}
	content.WriteString(fmt.Sprintf("%s (%d:%d)", item.GetName(), item.LocalPort, item.RemotePort))

	// Status
	content.WriteString(" ")
	statusIcon := GetStatusIcon(item.GetStatus())
	statusStyle := GetStatusColor(item.GetStatus())
	content.WriteString(statusStyle.Render(statusIcon))

	// Target info
	content.WriteString(" ")
	content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(
		fmt.Sprintf("Target: %s/%s", item.TargetType, item.TargetName)))

	// Health status
	if item.GetStatus() == StatusRunning {
		content.WriteString(" ")
		healthIcon := GetHealthIcon(item.GetHealth())
		healthStyle := GetHealthColor(item.GetHealth())
		content.WriteString(healthStyle.Render("Status: " + healthIcon + string(item.GetHealth())))
	}

	// Render with selection indicator
	str := content.String()
	if index == m.Index() {
		// Remove the indent for selected items
		str = lipgloss.NewStyle().
			Foreground(lipgloss.Color("170")).
			Bold(true).
			Render("▶   " + strings.TrimPrefix(str, "    "))
	}

	fmt.Fprint(w, str)
}
