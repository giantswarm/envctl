package view

import (
	"envctl/internal/api"
	"envctl/internal/tui/design"
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
)

// PortForwardListItem represents a port forward in the list
type PortForwardListItem struct {
	BaseListItem
	LocalPort  int
	RemotePort int
	TargetType string
	TargetName string
}

// Title returns the display title for the port forward
func (i PortForwardListItem) Title() string {
	return fmt.Sprintf("%s (%d:%d)", i.Name, i.LocalPort, i.RemotePort)
}

// Description returns the display description for the port forward
func (i PortForwardListItem) Description() string {
	return fmt.Sprintf("%s/%s", i.TargetType, i.TargetName)
}

// ConvertPortForwardToListItem converts API port forward to list item
func ConvertPortForwardToListItem(pf *api.PortForwardServiceInfo) PortForwardListItem {
	// Map API status to our common status
	var status ServiceStatus
	switch strings.ToLower(pf.State) {
	case "running":
		status = StatusRunning
	case "failed":
		status = StatusFailed
	case "starting", "waiting":
		status = StatusStarting
	case "stopped", "exited", "killed":
		status = StatusStopped
	default:
		status = StatusUnknown
	}

	// Map API health to our common health
	var health ServiceHealth
	if strings.ToLower(pf.Health) == "healthy" {
		health = HealthHealthy
	} else if strings.ToLower(pf.Health) == "unhealthy" {
		health = HealthUnhealthy
	} else if status == StatusRunning {
		health = HealthChecking
	} else {
		health = HealthUnknown
	}

	// Use custom icon if provided, otherwise default
	icon := pf.Icon
	if icon == "" {
		icon = design.IconLink
	}

	return PortForwardListItem{
		BaseListItem: BaseListItem{
			ID:          pf.Label,
			Name:        pf.Name,
			Status:      status,
			Health:      health,
			Icon:        design.SafeIcon(icon),
			Description: fmt.Sprintf("%s/%s", pf.TargetType, pf.TargetName),
			Details: fmt.Sprintf("Port: %d:%d, Target: %s/%s",
				pf.LocalPort, pf.RemotePort, pf.TargetType, pf.TargetName),
		},
		LocalPort:  pf.LocalPort,
		RemotePort: pf.RemotePort,
		TargetType: pf.TargetType,
		TargetName: pf.TargetName,
	}
}

// BuildPortForwardsList creates a list model for port forwards (deprecated - removed functionality)
func BuildPortForwardsList(m *model.Model, width, height int, focused bool) *ServiceListModel {
	// Port forwarding has been removed as part of the generic orchestrator refactoring
	items := []list.Item{}
	l := CreateStyledList("Port Forwards (Deprecated)", items, width, height, focused)

	return &ServiceListModel{
		List:     l,
		ListType: "portforwards",
	}
}
