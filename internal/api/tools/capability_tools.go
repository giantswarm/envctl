package tools

import (
	"context"
	"fmt"
	"time"

	"envctl/internal/capability"
	"envctl/pkg/logging"

	"github.com/mark3labs/mcp-go/mcp"
)

// CapabilityTools provides MCP tools for capability management
type CapabilityTools struct {
	registry *capability.Registry
	resolver *capability.Resolver
	// We'll need access to the orchestrator API, but we'll add that when integrating
	// orchestratorAPI api.OrchestratorAPI
}

// NewCapabilityTools creates a new capability tools handler
func NewCapabilityTools(registry *capability.Registry, resolver *capability.Resolver) *CapabilityTools {
	return &CapabilityTools{
		registry: registry,
		resolver: resolver,
	}
}

// GetCapabilityTools returns all capability management tools
func (ct *CapabilityTools) GetCapabilityTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "capability_register",
			Description: "Register a capability that this MCP server provides",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"auth_provider", "discovery_provider", "portforward_provider", "cluster_provider"},
						"description": "Type of capability being registered",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Human-readable name for this capability",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Description of what this capability provides",
					},
					"features": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "List of features this provider supports",
					},
					"config": map[string]interface{}{
						"type":        "object",
						"description": "Provider-specific configuration",
					},
					"metadata": map[string]interface{}{
						"type": "object",
						"additionalProperties": map[string]interface{}{
							"type": "string",
						},
						"description": "Additional metadata about the capability",
					},
				},
				Required: []string{"type", "name", "features"},
			},
		},
		{
			Name:        "capability_unregister",
			Description: "Unregister a previously registered capability",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"capability_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the capability to unregister",
					},
				},
				Required: []string{"capability_id"},
			},
		},
		{
			Name:        "capability_update",
			Description: "Update the status of a registered capability",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"capability_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the capability to update",
					},
					"state": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"active", "unhealthy", "inactive"},
						"description": "New state of the capability",
					},
					"error": map[string]interface{}{
						"type":        "string",
						"description": "Error message if unhealthy",
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Updated metadata",
					},
				},
				Required: []string{"capability_id", "state"},
			},
		},
		{
			Name:        "capability_list",
			Description: "List all registered capabilities",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Filter by capability type (optional)",
					},
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "Filter by provider name (optional)",
					},
					"state": map[string]interface{}{
						"type":        "string",
						"description": "Filter by state (optional)",
					},
				},
			},
		},
		{
			Name:        "capability_get",
			Description: "Get detailed information about a capability",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"capability_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the capability",
					},
				},
				Required: []string{"capability_id"},
			},
		},
		{
			Name:        "capability_find_matching",
			Description: "Find capabilities that match specific requirements",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Required capability type",
					},
					"features": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Required features",
					},
					"config": map[string]interface{}{
						"type":        "object",
						"description": "Required configuration",
					},
				},
				Required: []string{"type"},
			},
		},
		{
			Name:        "capability_request",
			Description: "Request a capability for a service",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"service": map[string]interface{}{
						"type":        "string",
						"description": "Service requesting the capability",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Type of capability needed",
					},
					"features": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "Required features",
					},
					"config": map[string]interface{}{
						"type":        "object",
						"description": "Request-specific configuration",
					},
					"timeout": map[string]interface{}{
						"type":        "integer",
						"description": "Timeout in seconds (default: 300)",
					},
				},
				Required: []string{"service", "type"},
			},
		},
		{
			Name:        "capability_release",
			Description: "Release a previously requested capability",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"service": map[string]interface{}{
						"type":        "string",
						"description": "Service releasing the capability",
					},
					"handle_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the capability handle to release",
					},
				},
				Required: []string{"service", "handle_id"},
			},
		},
	}
}

// HandleCapabilityRegister handles capability registration
func (ct *CapabilityTools) HandleCapabilityRegister(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters
	typeStr, ok := req.Params.Arguments.(map[string]interface{})["type"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid or missing 'type' parameter")
	}

	name, ok := req.Params.Arguments.(map[string]interface{})["name"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid or missing 'name' parameter")
	}

	description, _ := req.Params.Arguments.(map[string]interface{})["description"].(string)

	featuresRaw, ok := req.Params.Arguments.(map[string]interface{})["features"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid or missing 'features' parameter")
	}

	features := make([]string, len(featuresRaw))
	for i, f := range featuresRaw {
		features[i], ok = f.(string)
		if !ok {
			return nil, fmt.Errorf("invalid feature at index %d", i)
		}
	}

	config, _ := req.Params.Arguments.(map[string]interface{})["config"].(map[string]interface{})
	metadataRaw, _ := req.Params.Arguments.(map[string]interface{})["metadata"].(map[string]interface{})

	metadata := make(map[string]string)
	for k, v := range metadataRaw {
		if strVal, ok := v.(string); ok {
			metadata[k] = strVal
		}
	}

	// Determine provider from context (this would come from the MCP server name in real implementation)
	// For now, we'll use a placeholder
	provider := "mcp-server" // TODO: Get actual MCP server name from context

	// Create capability
	cap := &capability.Capability{
		Type:        capability.CapabilityType(typeStr),
		Provider:    provider,
		Name:        name,
		Description: description,
		Features:    features,
		Config:      config,
		Metadata:    metadata,
	}

	// Register capability
	err := ct.registry.Register(cap)
	if err != nil {
		return nil, fmt.Errorf("failed to register capability: %w", err)
	}

	logging.Info("CapabilityTools", "Registered capability: %s (ID: %s)", name, cap.ID)

	// Return result
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Successfully registered capability '%s' with ID: %s", name, cap.ID),
			},
		},
	}, nil
}

// HandleCapabilityUnregister handles capability unregistration
func (ct *CapabilityTools) HandleCapabilityUnregister(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	capabilityID, ok := req.Params.Arguments.(map[string]interface{})["capability_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid or missing 'capability_id' parameter")
	}

	err := ct.registry.Unregister(capabilityID)
	if err != nil {
		return nil, fmt.Errorf("failed to unregister capability: %w", err)
	}

	logging.Info("CapabilityTools", "Unregistered capability: %s", capabilityID)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Successfully unregistered capability with ID: %s", capabilityID),
			},
		},
	}, nil
}

// HandleCapabilityUpdate handles capability status updates
func (ct *CapabilityTools) HandleCapabilityUpdate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments.(map[string]interface{})

	capabilityID, ok := args["capability_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid or missing 'capability_id' parameter")
	}

	stateStr, ok := args["state"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid or missing 'state' parameter")
	}

	errorMsg, _ := args["error"].(string)
	metadataRaw, _ := args["metadata"].(map[string]interface{})

	metadata := make(map[string]string)
	for k, v := range metadataRaw {
		if strVal, ok := v.(string); ok {
			metadata[k] = strVal
		}
	}

	// Map state string to CapabilityState
	var state capability.CapabilityState
	var health capability.HealthStatus
	switch stateStr {
	case "active":
		state = capability.CapabilityStateActive
		health = capability.HealthStatusHealthy
	case "unhealthy":
		state = capability.CapabilityStateUnhealthy
		health = capability.HealthStatusUnhealthy
	case "inactive":
		state = capability.CapabilityStateInactive
		health = capability.HealthStatusUnknown
	default:
		return nil, fmt.Errorf("invalid state: %s", stateStr)
	}

	// Update capability
	status := capability.CapabilityStatus{
		State:  state,
		Error:  errorMsg,
		Health: health,
	}

	err := ct.registry.Update(capabilityID, status)
	if err != nil {
		return nil, fmt.Errorf("failed to update capability: %w", err)
	}

	logging.Info("CapabilityTools", "Updated capability %s to state: %s", capabilityID, state)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Successfully updated capability %s to state: %s", capabilityID, state),
			},
		},
	}, nil
}

// HandleCapabilityList handles listing capabilities
func (ct *CapabilityTools) HandleCapabilityList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := req.Params.Arguments.(map[string]interface{})

	typeFilter, _ := args["type"].(string)
	providerFilter, _ := args["provider"].(string)
	stateFilter, _ := args["state"].(string)

	// Get all capabilities
	var capabilities []*capability.Capability

	if typeFilter != "" {
		capabilities = ct.registry.ListByType(capability.CapabilityType(typeFilter))
	} else if providerFilter != "" {
		capabilities = ct.registry.ListByProvider(providerFilter)
	} else {
		capabilities = ct.registry.ListAll()
	}

	// Apply state filter if provided
	if stateFilter != "" {
		var filtered []*capability.Capability
		for _, cap := range capabilities {
			if string(cap.Status.State) == stateFilter {
				filtered = append(filtered, cap)
			}
		}
		capabilities = filtered
	}

	// Format response as text
	var responseText string
	if len(capabilities) == 0 {
		responseText = "No capabilities found"
	} else {
		responseText = fmt.Sprintf("Found %d capabilities:\n\n", len(capabilities))
		for _, cap := range capabilities {
			responseText += fmt.Sprintf("ID: %s\nType: %s\nProvider: %s\nName: %s\nState: %s\nHealth: %s\n\n",
				cap.ID, cap.Type, cap.Provider, cap.Name, cap.Status.State, cap.Status.Health)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: responseText,
			},
		},
	}, nil
}

// HandleCapabilityGet handles getting a specific capability
func (ct *CapabilityTools) HandleCapabilityGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	capabilityID, ok := req.Params.Arguments.(map[string]interface{})["capability_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid or missing 'capability_id' parameter")
	}

	cap, exists := ct.registry.Get(capabilityID)
	if !exists {
		return nil, fmt.Errorf("capability not found: %s", capabilityID)
	}

	// Format capability details as text
	responseText := fmt.Sprintf(`Capability Details:
ID: %s
Type: %s
Provider: %s
Name: %s
Description: %s
Features: %v
State: %s
Health: %s
Error: %s
Last Check: %s`,
		cap.ID, cap.Type, cap.Provider, cap.Name, cap.Description,
		cap.Features, cap.Status.State, cap.Status.Health,
		cap.Status.Error, cap.Status.LastCheck.Format(time.RFC3339))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: responseText,
			},
		},
	}, nil
}

// HandleCapabilityFindMatching handles finding matching capabilities
func (ct *CapabilityTools) HandleCapabilityFindMatching(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments.(map[string]interface{})

	typeStr, ok := args["type"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid or missing 'type' parameter")
	}

	featuresRaw, _ := args["features"].([]interface{})
	features := make([]string, len(featuresRaw))
	for i, f := range featuresRaw {
		if str, ok := f.(string); ok {
			features[i] = str
		}
	}

	config, _ := args["config"].(map[string]interface{})

	// Create request
	request := capability.CapabilityRequest{
		Type:     capability.CapabilityType(typeStr),
		Features: features,
		Config:   config,
	}

	// Find matching capabilities
	matching := ct.registry.FindMatching(request)

	// Format response as text
	var responseText string
	if len(matching) == 0 {
		responseText = fmt.Sprintf("No capabilities found matching type '%s'", typeStr)
	} else {
		responseText = fmt.Sprintf("Found %d matching capabilities:\n\n", len(matching))
		for _, cap := range matching {
			responseText += fmt.Sprintf("ID: %s\nProvider: %s\nName: %s\nFeatures: %v\nState: %s\n\n",
				cap.ID, cap.Provider, cap.Name, cap.Features, cap.Status.State)
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: responseText,
			},
		},
	}, nil
}

// HandleCapabilityRequest handles requesting a capability for a service
func (ct *CapabilityTools) HandleCapabilityRequest(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments.(map[string]interface{})

	service, ok := args["service"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid or missing 'service' parameter")
	}

	typeStr, ok := args["type"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid or missing 'type' parameter")
	}

	featuresRaw, _ := args["features"].([]interface{})
	features := make([]string, len(featuresRaw))
	for i, f := range featuresRaw {
		if str, ok := f.(string); ok {
			features[i] = str
		}
	}

	config, _ := args["config"].(map[string]interface{})
	timeout, _ := args["timeout"].(float64) // JSON numbers come as float64

	// Create request
	request := capability.CapabilityRequest{
		Type:     capability.CapabilityType(typeStr),
		Features: features,
		Config:   config,
	}

	if timeout > 0 {
		request.Timeout = time.Duration(timeout) * time.Second
	}

	// Request capability
	handle, err := ct.resolver.RequestCapability(service, request)
	if err != nil {
		return nil, fmt.Errorf("failed to request capability: %w", err)
	}

	logging.Info("CapabilityTools", "Service %s requested capability %s, handle: %s", service, typeStr, handle.ID)

	responseText := fmt.Sprintf(`Successfully requested %s capability for service %s
Handle ID: %s
Provider: %s
Type: %s`, typeStr, service, handle.ID, handle.Provider, handle.Type)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: responseText,
			},
		},
	}, nil
}

// HandleCapabilityRelease handles releasing a capability
func (ct *CapabilityTools) HandleCapabilityRelease(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.Params.Arguments.(map[string]interface{})

	service, ok := args["service"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid or missing 'service' parameter")
	}

	handleID, ok := args["handle_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid or missing 'handle_id' parameter")
	}

	err := ct.resolver.ReleaseCapability(service, handleID)
	if err != nil {
		return nil, fmt.Errorf("failed to release capability: %w", err)
	}

	logging.Info("CapabilityTools", "Service %s released capability handle: %s", service, handleID)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("Successfully released capability handle %s for service %s", handleID, service),
			},
		},
	}, nil
}
