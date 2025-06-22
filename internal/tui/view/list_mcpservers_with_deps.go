package view

import (
	"envctl/internal/tui/design"
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
}

// Title returns the display title for the MCP server with expansion indicator
func (i MCPServerWithDepsListItem) Title() string {
	return fmt.Sprintf("%s MCP", i.GetName())
}

// Description returns the display description including dependency count
func (i MCPServerWithDepsListItem) Description() string {
	return i.MCPServerListItem.BaseListItem.Description
}

// BuildMCPServersWithDependenciesList creates a hierarchical list of MCP servers with their port forward dependencies
func BuildMCPServersWithDependenciesList(m *model.Model, width, height int, focused bool) *ServiceListModel {
	items := []list.Item{}

	// Now build the hierarchical list
	for _, config := range m.MCPServerConfig {
		mcpItem := MCPServerWithDepsListItem{}

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
					Icon:        design.SafeIcon("🔧"), // Default icon since Icon field removed in Phase 3
					Description: "Not Started",
					Details:     fmt.Sprintf("MCP Server: %s (Not Started)", config.Name),
				},
			}
		}

		items = append(items, mcpItem)
	}

	// Create the list with a custom delegate
	l := list.New(items, MCPWithDepsItemDelegate{}, width, height)
	l.Title = "MCP Servers & Dependencies"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	// Style the title
	l.Styles.Title = design.TitleStyle.Copy().
		PaddingLeft(1)

	if focused {
		l.Styles.Title = l.Styles.Title.
			Background(design.ColorHighlight).
			Foreground(design.ColorPrimary)
	}

	// Status bar styling
	l.Styles.StatusBar = lipgloss.NewStyle().
		Foreground(design.ColorTextSecondary).
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
		content.WriteString(design.TextSecondaryStyle.Render(desc))
	}

	// Render with selection indicator
	str := content.String()
	if index == m.Index() {
		str = design.ListItemSelectedStyle.Render("▶ " + str)
	} else {
		str = "  " + str
	}

	fmt.Fprint(w, str)
}
