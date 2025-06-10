package mcpserver

import (
	"context"
	"envctl/internal/api"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// APIAdapter adapts the MCPServerService to implement api.MCPServiceHandler
type APIAdapter struct {
	service *MCPServerService
}

// NewAPIAdapter creates a new MCP server API adapter
func NewAPIAdapter(s *MCPServerService) *APIAdapter {
	return &APIAdapter{service: s}
}

// GetModelID returns the model ID for the MCP service
func (a *APIAdapter) GetModelID() string {
	// Model ID is not stored in MCPServerService, return empty string
	return ""
}

// GetProvider returns the provider for the MCP service
func (a *APIAdapter) GetProvider() string {
	// Provider is not stored in MCPServerService, return empty string
	return ""
}

// GetURL returns the URL for the MCP service
func (a *APIAdapter) GetURL() string {
	// URL is not stored in MCPServerService, return empty string
	return ""
}

// GetClusterLabel returns the cluster label for the MCP service
func (a *APIAdapter) GetClusterLabel() string {
	// Cluster label is not stored in MCPServerService, return empty string
	return ""
}

// GetMCPTools returns the list of tools exposed by the MCP server
func (a *APIAdapter) GetMCPTools() []api.MCPTool {
	// Get the MCP client
	client := a.service.GetMCPClient()
	if client == nil {
		return nil
	}

	// Try to get tools from the client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tools, err := api.GetToolsFromClient(ctx, client)
	if err != nil {
		return nil
	}

	return tools
}

// GetResources returns the list of resources exposed by the MCP server
func (a *APIAdapter) GetResources() []api.MCPResource {
	// Get the MCP client
	client := a.service.GetMCPClient()
	if client == nil {
		return nil
	}

	// Use the client to list resources
	type resourceLister interface {
		ListResources(ctx context.Context) ([]mcp.Resource, error)
	}

	lister, ok := client.(resourceLister)
	if !ok {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// List available resources
	resources, err := lister.ListResources(ctx)
	if err != nil {
		return nil
	}

	// Convert to our MCPResource type
	result := make([]api.MCPResource, 0, len(resources))
	for _, res := range resources {
		result = append(result, api.MCPResource{
			URI:         res.URI,
			Name:        res.Name,
			Description: res.Description,
			MimeType:    res.MIMEType,
		})
	}

	return result
}

// Register registers this adapter with the API package
func (a *APIAdapter) Register() {
	api.RegisterMCPService(a.service.GetLabel(), a)
}

// Unregister removes this adapter from the API package
func (a *APIAdapter) Unregister() {
	api.UnregisterMCPService(a.service.GetLabel())
}

// ToolProvider interface implementation
// These are no-ops since the global adapter handles tools

// GetTools returns no tools - the global adapter handles MCP tools
func (a *APIAdapter) GetTools() []api.ToolMetadata {
	return nil
}

// ExecuteTool is not implemented at the instance level
func (a *APIAdapter) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (*api.CallToolResult, error) {
	return nil, nil
}

// MCPServiceHandler interface implementation (delegated to global adapter)

// ListServers is handled by the global adapter
func (a *APIAdapter) ListServers(ctx context.Context) ([]*api.MCPServerInfo, error) {
	// This should be handled by the global MCP service adapter
	return nil, nil
}

// GetServerInfo is handled by the global adapter
func (a *APIAdapter) GetServerInfo(ctx context.Context, label string) (*api.MCPServerInfo, error) {
	// This should be handled by the global MCP service adapter
	return nil, nil
}

// GetServerTools is handled by the global adapter
func (a *APIAdapter) GetServerTools(ctx context.Context, serverName string) ([]api.MCPTool, error) {
	// This should be handled by the global MCP service adapter
	return nil, nil
}
