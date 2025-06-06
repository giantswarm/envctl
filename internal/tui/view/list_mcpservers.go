package view

import (
	"envctl/internal/api"
	"envctl/internal/tui/design"
	"envctl/internal/tui/model"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
)

// MCPServerListItem represents an MCP server in the list
type MCPServerListItem struct {
	BaseListItem
}

// Title returns the display title for the MCP server
func (i MCPServerListItem) Title() string {
	return i.Name + " MCP"
}

// Description returns the display description for the MCP server
func (i MCPServerListItem) Description() string {
	return i.BaseListItem.Description
}

// ConvertMCPServerToListItem converts API MCP server to list item
func ConvertMCPServerToListItem(mcp *api.MCPServerInfo) MCPServerListItem {
	// Map API status to our common status
	var status ServiceStatus
	switch strings.ToLower(mcp.State) {
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
	if strings.ToLower(mcp.Health) == "healthy" {
		health = HealthHealthy
	} else if strings.ToLower(mcp.Health) == "unhealthy" {
		health = HealthUnhealthy
	} else if status == StatusRunning {
		health = HealthChecking
	} else {
		health = HealthUnknown
	}

	// Use custom icon if provided, otherwise default
	icon := mcp.Icon
	if icon == "" {
		icon = "⚙"
	}

	// Build description
	description := ""
	if mcp.Error != "" {
		description = "Error: " + mcp.Error
	} else if mcp.Enabled {
		description = "Enabled"
	} else {
		description = "Disabled"
	}

	return MCPServerListItem{
		BaseListItem: BaseListItem{
			ID:          mcp.Label,
			Name:        mcp.Name,
			Status:      status,
			Health:      health,
			Icon:        design.SafeIcon(icon),
			Description: description,
			Details:     fmt.Sprintf("MCP Server: %s", mcp.Name),
		},
	}
}

// BuildMCPServersList creates a list model for MCP servers
func BuildMCPServersList(m *model.Model, width, height int, focused bool) *ServiceListModel {
	items := []list.Item{}

	// Add MCP servers from config order
	for _, config := range m.MCPServerConfig {
		if mcp, exists := m.MCPServers[config.Name]; exists {
			items = append(items, ConvertMCPServerToListItem(mcp))
		} else {
			// Create placeholder item for configured but not running MCP server
			items = append(items, MCPServerListItem{
				BaseListItem: BaseListItem{
					ID:          config.Name,
					Name:        config.Name,
					Status:      StatusStopped,
					Health:      HealthUnknown,
					Icon:        design.SafeIcon(config.Icon),
					Description: "Not Started",
					Details:     fmt.Sprintf("MCP Server: %s (Not Started)", config.Name),
				},
			})
		}
	}

	l := CreateStyledList("MCP Servers", items, width, height, focused)

	return &ServiceListModel{
		List:     l,
		ListType: "mcpservers",
	}
}
