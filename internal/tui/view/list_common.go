package view

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ServiceStatus represents the common status for all services
type ServiceStatus string

const (
	StatusRunning  ServiceStatus = "running"
	StatusStopped  ServiceStatus = "stopped"
	StatusFailed   ServiceStatus = "failed"
	StatusStarting ServiceStatus = "starting"
	StatusUnknown  ServiceStatus = "unknown"
)

// ServiceHealth represents the common health status
type ServiceHealth string

const (
	HealthHealthy   ServiceHealth = "healthy"
	HealthUnhealthy ServiceHealth = "unhealthy"
	HealthDegraded  ServiceHealth = "degraded"
	HealthChecking  ServiceHealth = "checking"
	HealthUnknown   ServiceHealth = "unknown"
)

// CommonListItem represents a common interface for all list items
type CommonListItem interface {
	list.Item
	GetID() string
	GetName() string
	GetStatus() ServiceStatus
	GetHealth() ServiceHealth
	GetIcon() string
	GetDescription() string
	GetDetails() string
}

// BaseListItem provides common implementation for list items
type BaseListItem struct {
	ID          string
	Name        string
	Status      ServiceStatus
	Health      ServiceHealth
	Icon        string
	Description string
	Details     string
}

func (i BaseListItem) GetID() string            { return i.ID }
func (i BaseListItem) GetName() string          { return i.Name }
func (i BaseListItem) GetStatus() ServiceStatus { return i.Status }
func (i BaseListItem) GetHealth() ServiceHealth { return i.Health }
func (i BaseListItem) GetIcon() string          { return i.Icon }
func (i BaseListItem) GetDescription() string   { return i.Description }
func (i BaseListItem) GetDetails() string       { return i.Details }
func (i BaseListItem) FilterValue() string      { return i.Name + " " + i.Description }

// GetStatusIcon returns the icon for a given status
func GetStatusIcon(status ServiceStatus) string {
	switch status {
	case StatusRunning:
		return SafeIcon(IconCheck)
	case StatusFailed:
		return SafeIcon(IconCross)
	case StatusStarting:
		return SafeIcon(IconHourglass)
	case StatusStopped:
		return SafeIcon(IconStop)
	default:
		return SafeIcon(IconWarning)
	}
}

// GetHealthIcon returns the icon for a given health status
func GetHealthIcon(health ServiceHealth) string {
	switch health {
	case HealthHealthy:
		return SafeIcon(IconCheck)
	case HealthUnhealthy:
		return SafeIcon(IconCross)
	case HealthDegraded:
		return SafeIcon(IconWarning)
	case HealthChecking:
		return SafeIcon(IconHourglass)
	default:
		return SafeIcon(IconQuestion)
	}
}

// GetStatusColor returns the style for a given status
func GetStatusColor(status ServiceStatus) lipgloss.Style {
	switch status {
	case StatusRunning:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("42")) // Green
	case StatusFailed:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("160")) // Red
	case StatusStarting:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange
	case StatusStopped:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Gray
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange
	}
}

// GetHealthColor returns the style for a given health status
func GetHealthColor(health ServiceHealth) lipgloss.Style {
	switch health {
	case HealthHealthy:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("42")) // Green
	case HealthUnhealthy:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("160")) // Red
	case HealthDegraded:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange
	case HealthChecking:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("33")) // Blue
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Gray
	}
}

// CommonItemDelegate handles rendering for all list items
type CommonItemDelegate struct{}

func (d CommonItemDelegate) Height() int                             { return 1 }
func (d CommonItemDelegate) Spacing() int                            { return 0 }
func (d CommonItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d CommonItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(CommonListItem)
	if !ok {
		return
	}

	// Build the item display
	var content strings.Builder

	// Icon and name
	if item.GetIcon() != "" {
		content.WriteString(item.GetIcon())
		content.WriteString(" ")
	}
	content.WriteString(item.GetName())

	// Status icon
	content.WriteString(" ")
	statusIcon := GetStatusIcon(item.GetStatus())
	statusStyle := GetStatusColor(item.GetStatus())
	content.WriteString(statusStyle.Render(statusIcon))

	// Description if available
	if item.GetDescription() != "" {
		content.WriteString(" ")
		content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(item.GetDescription()))
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

// CreateStyledList creates a styled list with common settings
func CreateStyledList(title string, items []list.Item, width, height int, focused bool) list.Model {
	l := list.New(items, CommonItemDelegate{}, width, height)
	l.Title = title
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false) // We'll show help in status bar
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("170")).
		PaddingLeft(1)

	// Apply focus styling
	if focused {
		l.Styles.Title = l.Styles.Title.
			Background(lipgloss.Color("238")).
			Foreground(lipgloss.Color("205"))
	}

	// Status bar styling
	l.Styles.StatusBar = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "60", Dark: "110"}).
		PaddingLeft(2)

	return l
}

// ServiceListModel wraps a list.Model with service-specific functionality
type ServiceListModel struct {
	List     list.Model
	ListType string // "clusters", "portforwards", "mcpservers"
}

// Update handles common list updates
func (m *ServiceListModel) Update(msg tea.Msg) (*ServiceListModel, tea.Cmd) {
	var cmd tea.Cmd
	m.List, cmd = m.List.Update(msg)
	return m, cmd
}

// View renders the list
func (m *ServiceListModel) View() string {
	return m.List.View()
}

// SetSize updates the list dimensions
func (m *ServiceListModel) SetSize(width, height int) {
	m.List.SetWidth(width)
	m.List.SetHeight(height)
}

// GetSelectedItem returns the currently selected item
func (m *ServiceListModel) GetSelectedItem() CommonListItem {
	selected := m.List.SelectedItem()
	if selected == nil {
		return nil
	}
	item, ok := selected.(CommonListItem)
	if !ok {
		return nil
	}
	return item
}

// SetFocused updates the focus state styling
func (m *ServiceListModel) SetFocused(focused bool) {
	if focused {
		m.List.Styles.Title = m.List.Styles.Title.
			Background(lipgloss.Color("238")).
			Foreground(lipgloss.Color("205"))
	} else {
		m.List.Styles.Title = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("170")).
			PaddingLeft(1)
	}
}
