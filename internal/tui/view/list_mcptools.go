package view

import (
	"envctl/internal/api"
	"envctl/internal/tui/model"
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MCPToolListItem represents a single MCP tool in the list
type MCPToolListItem struct {
	ToolName string
	ToolDesc string
	Blocked  bool
}

// Title returns the title for the list item
func (i MCPToolListItem) Title() string {
	if i.Blocked {
		return i.ToolName + " [BLOCKED]"
	}
	return i.ToolName
}

// Description returns the description for the list item
func (i MCPToolListItem) Description() string {
	return i.ToolDesc
}

// FilterValue returns the value to use for filtering
func (i MCPToolListItem) FilterValue() string {
	return i.ToolName + " " + i.ToolDesc
}

// MCPToolDelegate is the item delegate for MCP tools
type MCPToolDelegate struct {
	focused bool
}

// Height returns the number of lines each item occupies
func (d MCPToolDelegate) Height() int { return 2 }

// Spacing returns the spacing between items
func (d MCPToolDelegate) Spacing() int { return 1 }

// Update handles messages for the delegate
func (d MCPToolDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

// Render renders a single MCP tool item
func (d MCPToolDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	mcpTool, ok := item.(MCPToolListItem)
	if !ok {
		return
	}

	// Determine styles
	var titleStyle, descStyle lipgloss.Style
	isSelected := index == m.Index()

	if isSelected {
		if d.focused {
			titleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("170")).
				Bold(true).
				PaddingLeft(2)
			descStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				PaddingLeft(4)
		} else {
			titleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				PaddingLeft(2)
			descStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("238")).
				PaddingLeft(4)
		}
	} else {
		titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			PaddingLeft(2)
		descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238")).
			PaddingLeft(4)
	}

	// Icon
	icon := "✔ "
	iconColor := lipgloss.Color("42") // green
	if mcpTool.Blocked {
		icon = "🚫 "
		iconColor = lipgloss.Color("160") // red
	}

	// Title with icon
	title := mcpTool.ToolName
	if mcpTool.Blocked {
		title = titleStyle.Copy().Foreground(iconColor).Render(icon) +
			titleStyle.Copy().Strikethrough(true).Render(title) +
			lipgloss.NewStyle().Foreground(lipgloss.Color("160")).Bold(true).Render(" [BLOCKED]")
	} else {
		title = titleStyle.Copy().Foreground(iconColor).Render(icon) +
			titleStyle.Render(title)
	}

	// Description (truncated if needed)
	desc := mcpTool.ToolDesc
	maxDescWidth := 80
	if len(desc) > maxDescWidth {
		desc = desc[:maxDescWidth-3] + "..."
	}
	desc = descStyle.Render(desc)

	// Selection indicator
	if isSelected && d.focused {
		fmt.Fprintf(w, "▶ %s\n%s", title[2:], desc)
	} else {
		fmt.Fprintf(w, "%s\n%s", title, desc)
	}
}

// BuildMCPToolsList creates a list model for MCP tools
func BuildMCPToolsList(m *model.Model, width, height int, focused bool) *ServiceListModel {
	items := make([]list.Item, 0, len(m.MCPToolsWithStatus))

	// Group tools by blocked status - show unblocked first
	var unblockedTools, blockedTools []api.ToolWithStatus
	for _, tool := range m.MCPToolsWithStatus {
		if tool.Blocked {
			blockedTools = append(blockedTools, tool)
		} else {
			unblockedTools = append(unblockedTools, tool)
		}
	}

	// Add unblocked tools first
	for _, tool := range unblockedTools {
		items = append(items, MCPToolListItem{
			ToolName: tool.Name,
			ToolDesc: tool.Description,
			Blocked:  false,
		})
	}

	// Then add blocked tools
	for _, tool := range blockedTools {
		items = append(items, MCPToolListItem{
			ToolName: tool.Name,
			ToolDesc: tool.Description,
			Blocked:  true,
		})
	}

	// Create delegate
	delegate := MCPToolDelegate{
		focused: focused,
	}

	// Create list
	l := list.New(items, delegate, width, height)

	// Title with status
	title := "MCP Tools"
	if m.AggregatorInfo != nil && m.AggregatorInfo.YoloMode {
		title += " [YOLO MODE]"
	}
	l.Title = title

	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	// Apply styles
	if focused {
		l.Styles.Title = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("170")).
			Background(lipgloss.Color("238")).
			PaddingLeft(1).
			PaddingRight(1)
	} else {
		l.Styles.Title = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("241")).
			PaddingLeft(1).
			PaddingRight(1)
	}

	l.Styles.StatusBar = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "60", Dark: "110"}).
		PaddingLeft(2)

	// Show count in status
	blockedCount := len(blockedTools)
	if blockedCount > 0 {
		l.SetStatusBarItemName("tool", fmt.Sprintf("tools (%d blocked)", blockedCount))
	} else {
		l.SetStatusBarItemName("tool", "tools")
	}

	return &ServiceListModel{
		List:     l,
		ListType: "mcptools",
	}
}
