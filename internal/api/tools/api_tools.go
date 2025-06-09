package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"envctl/internal/api"
	"envctl/internal/config"

	"github.com/mark3labs/mcp-go/mcp"
)

// APITools provides MCP tools for accessing envctl's API functionality
type APITools struct {
	orchestratorAPI       api.OrchestratorAPI
	mcpServiceAPI         api.MCPServiceAPI
	k8sServiceAPI         api.K8sServiceAPI
	portForwardServiceAPI api.PortForwardServiceAPI
	configServiceAPI      api.ConfigServiceAPI
}

// NewAPITools creates API tools with the necessary API interfaces
func NewAPITools() *APITools {
	return &APITools{
		orchestratorAPI:       api.NewOrchestratorAPI(),
		mcpServiceAPI:         api.NewMCPServiceAPI(),
		k8sServiceAPI:         api.NewK8sServiceAPI(),
		portForwardServiceAPI: api.NewPortForwardServiceAPI(),
		configServiceAPI:      api.NewConfigServiceAPI(),
	}
}

// GetAPITools returns all API tools
func (at *APITools) GetAPITools() []mcp.Tool {
	tools := []mcp.Tool{}

	// Service Management Tools
	tools = append(tools, at.getServiceManagementTools()...)

	// Cluster Management Tools
	tools = append(tools, at.getClusterManagementTools()...)

	// MCP Server Tools
	tools = append(tools, at.getMCPServerTools()...)

	// K8s Connection Tools
	tools = append(tools, at.getK8sConnectionTools()...)

	// Port Forward Tools
	tools = append(tools, at.getPortForwardTools()...)

	// Configuration Tools
	tools = append(tools, at.getConfigurationTools()...)

	return tools
}

// Service Management Tools
func (at *APITools) getServiceManagementTools() []mcp.Tool {
	return []mcp.Tool{
		mcp.NewTool("service_list",
			mcp.WithDescription("List all services with their current status"),
		),
		mcp.NewTool("service_start",
			mcp.WithDescription("Start a specific service"),
			mcp.WithString("label",
				mcp.Required(),
				mcp.Description("Service label to start"),
			),
		),
		mcp.NewTool("service_stop",
			mcp.WithDescription("Stop a specific service"),
			mcp.WithString("label",
				mcp.Required(),
				mcp.Description("Service label to stop"),
			),
		),
		mcp.NewTool("service_restart",
			mcp.WithDescription("Restart a specific service"),
			mcp.WithString("label",
				mcp.Required(),
				mcp.Description("Service label to restart"),
			),
		),
		mcp.NewTool("service_status",
			mcp.WithDescription("Get detailed status of a specific service"),
			mcp.WithString("label",
				mcp.Required(),
				mcp.Description("Service label to get status for"),
			),
		),
	}
}

// Cluster Management Tools
func (at *APITools) getClusterManagementTools() []mcp.Tool {
	return []mcp.Tool{
		mcp.NewTool("cluster_list",
			mcp.WithDescription("List available clusters by role"),
			mcp.WithString("role",
				mcp.Required(),
				mcp.Description("Cluster role: talos, management, workload, or observability"),
				mcp.Enum("talos", "management", "workload", "observability"),
			),
		),
		mcp.NewTool("cluster_switch",
			mcp.WithDescription("Switch active cluster for a role"),
			mcp.WithString("role",
				mcp.Required(),
				mcp.Description("Cluster role: talos, management, workload, or observability"),
				mcp.Enum("talos", "management", "workload", "observability"),
			),
			mcp.WithString("cluster_name",
				mcp.Required(),
				mcp.Description("Name of the cluster to switch to"),
			),
		),
		mcp.NewTool("cluster_active",
			mcp.WithDescription("Get currently active cluster for a role"),
			mcp.WithString("role",
				mcp.Required(),
				mcp.Description("Cluster role: talos, management, workload, or observability"),
				mcp.Enum("talos", "management", "workload", "observability"),
			),
		),
	}
}

// MCP Server Tools
func (at *APITools) getMCPServerTools() []mcp.Tool {
	return []mcp.Tool{
		mcp.NewTool("mcp_server_list",
			mcp.WithDescription("List all MCP servers"),
		),
		mcp.NewTool("mcp_server_info",
			mcp.WithDescription("Get detailed information about an MCP server"),
			mcp.WithString("label",
				mcp.Required(),
				mcp.Description("MCP server label"),
			),
		),
		mcp.NewTool("mcp_server_tools",
			mcp.WithDescription("List tools exposed by an MCP server"),
			mcp.WithString("server_name",
				mcp.Required(),
				mcp.Description("MCP server name"),
			),
		),
	}
}

// K8s Connection Tools
func (at *APITools) getK8sConnectionTools() []mcp.Tool {
	return []mcp.Tool{
		mcp.NewTool("k8s_connection_list",
			mcp.WithDescription("List all Kubernetes connections"),
		),
		mcp.NewTool("k8s_connection_info",
			mcp.WithDescription("Get information about a specific Kubernetes connection"),
			mcp.WithString("label",
				mcp.Required(),
				mcp.Description("K8s connection label"),
			),
		),
		mcp.NewTool("k8s_connection_by_context",
			mcp.WithDescription("Find Kubernetes connection by context name"),
			mcp.WithString("context",
				mcp.Required(),
				mcp.Description("Kubernetes context name"),
			),
		),
	}
}

// Port Forward Tools
func (at *APITools) getPortForwardTools() []mcp.Tool {
	return []mcp.Tool{
		mcp.NewTool("portforward_list",
			mcp.WithDescription("List all port forwards"),
		),
		mcp.NewTool("portforward_info",
			mcp.WithDescription("Get information about a specific port forward"),
			mcp.WithString("label",
				mcp.Required(),
				mcp.Description("Port forward label"),
			),
		),
	}
}

// Configuration Tools
func (at *APITools) getConfigurationTools() []mcp.Tool {
	return []mcp.Tool{
		// Get configuration tools
		mcp.NewTool("config_get",
			mcp.WithDescription("Get the entire envctl configuration"),
		),
		mcp.NewTool("config_get_clusters",
			mcp.WithDescription("Get all configured clusters"),
		),
		mcp.NewTool("config_get_active_clusters",
			mcp.WithDescription("Get active clusters mapping"),
		),
		mcp.NewTool("config_get_mcp_servers",
			mcp.WithDescription("Get all MCP server definitions"),
		),
		mcp.NewTool("config_get_port_forwards",
			mcp.WithDescription("Get all port forward definitions"),
		),
		mcp.NewTool("config_get_workflows",
			mcp.WithDescription("Get all workflow definitions"),
		),
		mcp.NewTool("config_get_aggregator",
			mcp.WithDescription("Get aggregator configuration"),
		),
		mcp.NewTool("config_get_global_settings",
			mcp.WithDescription("Get global settings"),
		),

		// Update configuration tools
		mcp.NewTool("config_update_mcp_server",
			mcp.WithDescription("Update or add an MCP server definition"),
			mcp.WithObject("server",
				mcp.Required(),
				mcp.Description("MCP server definition object"),
			),
		),
		mcp.NewTool("config_update_port_forward",
			mcp.WithDescription("Update or add a port forward definition"),
			mcp.WithObject("port_forward",
				mcp.Required(),
				mcp.Description("Port forward definition object"),
			),
		),
		mcp.NewTool("config_update_workflow",
			mcp.WithDescription("Update or add a workflow definition"),
			mcp.WithObject("workflow",
				mcp.Required(),
				mcp.Description("Workflow definition object"),
			),
		),
		mcp.NewTool("config_update_aggregator",
			mcp.WithDescription("Update aggregator configuration"),
			mcp.WithObject("aggregator",
				mcp.Required(),
				mcp.Description("Aggregator configuration object"),
			),
		),
		mcp.NewTool("config_update_global_settings",
			mcp.WithDescription("Update global settings"),
			mcp.WithObject("settings",
				mcp.Required(),
				mcp.Description("Global settings object"),
			),
		),

		// Delete configuration tools
		mcp.NewTool("config_delete_mcp_server",
			mcp.WithDescription("Delete an MCP server by name"),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("MCP server name to delete"),
			),
		),
		mcp.NewTool("config_delete_port_forward",
			mcp.WithDescription("Delete a port forward by name"),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Port forward name to delete"),
			),
		),
		mcp.NewTool("config_delete_workflow",
			mcp.WithDescription("Delete a workflow by name"),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Workflow name to delete"),
			),
		),
		mcp.NewTool("config_delete_cluster",
			mcp.WithDescription("Delete a cluster by name"),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Cluster name to delete"),
			),
		),

		// Save configuration
		mcp.NewTool("config_save",
			mcp.WithDescription("Save the current configuration to file"),
		),
	}
}

// HandleServiceList handles the service_list tool call
func (at *APITools) HandleServiceList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	services := at.orchestratorAPI.GetAllServices()

	result := map[string]interface{}{
		"services": services,
		"total":    len(services),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleServiceStart handles the service_start tool call
func (at *APITools) HandleServiceStart(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	label, err := req.RequireString("label")
	if err != nil {
		return mcp.NewToolResultError("label is required"), nil
	}

	if err := at.orchestratorAPI.StartService(label); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start service: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully started service '%s'", label)), nil
}

// HandleServiceStop handles the service_stop tool call
func (at *APITools) HandleServiceStop(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	label, err := req.RequireString("label")
	if err != nil {
		return mcp.NewToolResultError("label is required"), nil
	}

	if err := at.orchestratorAPI.StopService(label); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to stop service: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully stopped service '%s'", label)), nil
}

// HandleServiceRestart handles the service_restart tool call
func (at *APITools) HandleServiceRestart(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	label, err := req.RequireString("label")
	if err != nil {
		return mcp.NewToolResultError("label is required"), nil
	}

	if err := at.orchestratorAPI.RestartService(label); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to restart service: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully restarted service '%s'", label)), nil
}

// HandleServiceStatus handles the service_status tool call
func (at *APITools) HandleServiceStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	label, err := req.RequireString("label")
	if err != nil {
		return mcp.NewToolResultError("label is required"), nil
	}

	status, err := at.orchestratorAPI.GetServiceStatus(label)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get service status: %v", err)), nil
	}

	resultJSON, _ := json.MarshalIndent(status, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleClusterList handles the cluster_list tool call
func (at *APITools) HandleClusterList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	roleStr, err := req.RequireString("role")
	if err != nil {
		return mcp.NewToolResultError("role is required"), nil
	}

	role := api.ClusterRole(roleStr)
	clusters := at.orchestratorAPI.GetAvailableClusters(role)

	// Get active cluster
	activeName, _ := at.orchestratorAPI.GetActiveCluster(role)

	result := map[string]interface{}{
		"clusters": clusters,
		"active":   activeName,
		"total":    len(clusters),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleClusterSwitch handles the cluster_switch tool call
func (at *APITools) HandleClusterSwitch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	roleStr, err := req.RequireString("role")
	if err != nil {
		return mcp.NewToolResultError("role is required"), nil
	}

	clusterName, err := req.RequireString("cluster_name")
	if err != nil {
		return mcp.NewToolResultError("cluster_name is required"), nil
	}

	role := api.ClusterRole(roleStr)
	if err := at.orchestratorAPI.SwitchCluster(role, clusterName); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to switch cluster: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully switched %s cluster to '%s'", roleStr, clusterName)), nil
}

// HandleClusterActive handles the cluster_active tool call
func (at *APITools) HandleClusterActive(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	roleStr, err := req.RequireString("role")
	if err != nil {
		return mcp.NewToolResultError("role is required"), nil
	}

	role := api.ClusterRole(roleStr)
	activeName, exists := at.orchestratorAPI.GetActiveCluster(role)

	result := map[string]interface{}{
		"role":   roleStr,
		"active": activeName,
		"exists": exists,
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleMCPServerList handles the mcp_server_list tool call
func (at *APITools) HandleMCPServerList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	servers, err := at.mcpServiceAPI.ListServers(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list MCP servers: %v", err)), nil
	}

	result := map[string]interface{}{
		"servers": servers,
		"total":   len(servers),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleMCPServerInfo handles the mcp_server_info tool call
func (at *APITools) HandleMCPServerInfo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	label, err := req.RequireString("label")
	if err != nil {
		return mcp.NewToolResultError("label is required"), nil
	}

	info, err := at.mcpServiceAPI.GetServerInfo(ctx, label)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get MCP server info: %v", err)), nil
	}

	resultJSON, _ := json.MarshalIndent(info, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleMCPServerTools handles the mcp_server_tools tool call
func (at *APITools) HandleMCPServerTools(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	serverName, err := req.RequireString("server_name")
	if err != nil {
		return mcp.NewToolResultError("server_name is required"), nil
	}

	tools, err := at.mcpServiceAPI.GetTools(ctx, serverName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get MCP server tools: %v", err)), nil
	}

	result := map[string]interface{}{
		"server": serverName,
		"tools":  tools,
		"total":  len(tools),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleK8sConnectionList handles the k8s_connection_list tool call
func (at *APITools) HandleK8sConnectionList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	connections, err := at.k8sServiceAPI.ListConnections(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list K8s connections: %v", err)), nil
	}

	result := map[string]interface{}{
		"connections": connections,
		"total":       len(connections),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleK8sConnectionInfo handles the k8s_connection_info tool call
func (at *APITools) HandleK8sConnectionInfo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	label, err := req.RequireString("label")
	if err != nil {
		return mcp.NewToolResultError("label is required"), nil
	}

	info, err := at.k8sServiceAPI.GetConnectionInfo(ctx, label)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get K8s connection info: %v", err)), nil
	}

	resultJSON, _ := json.MarshalIndent(info, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleK8sConnectionByContext handles the k8s_connection_by_context tool call
func (at *APITools) HandleK8sConnectionByContext(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	contextName, err := req.RequireString("context")
	if err != nil {
		return mcp.NewToolResultError("context is required"), nil
	}

	info, err := at.k8sServiceAPI.GetConnectionByContext(ctx, contextName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to find K8s connection: %v", err)), nil
	}

	resultJSON, _ := json.MarshalIndent(info, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandlePortForwardList handles the portforward_list tool call
func (at *APITools) HandlePortForwardList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	forwards, err := at.portForwardServiceAPI.ListForwards(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list port forwards: %v", err)), nil
	}

	result := map[string]interface{}{
		"port_forwards": forwards,
		"total":         len(forwards),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandlePortForwardInfo handles the portforward_info tool call
func (at *APITools) HandlePortForwardInfo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	label, err := req.RequireString("label")
	if err != nil {
		return mcp.NewToolResultError("label is required"), nil
	}

	info, err := at.portForwardServiceAPI.GetForwardInfo(ctx, label)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get port forward info: %v", err)), nil
	}

	resultJSON, _ := json.MarshalIndent(info, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// Configuration handler functions

// HandleConfigGet handles the config_get tool call
func (at *APITools) HandleConfigGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := at.configServiceAPI.GetConfig(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get configuration: %v", err)), nil
	}

	resultJSON, _ := json.MarshalIndent(cfg, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleConfigGetClusters handles the config_get_clusters tool call
func (at *APITools) HandleConfigGetClusters(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	clusters, err := at.configServiceAPI.GetClusters(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get clusters: %v", err)), nil
	}

	result := map[string]interface{}{
		"clusters": clusters,
		"total":    len(clusters),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleConfigGetActiveClusters handles the config_get_active_clusters tool call
func (at *APITools) HandleConfigGetActiveClusters(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	activeClusters, err := at.configServiceAPI.GetActiveClusters(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get active clusters: %v", err)), nil
	}

	resultJSON, _ := json.MarshalIndent(activeClusters, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleConfigGetMCPServers handles the config_get_mcp_servers tool call
func (at *APITools) HandleConfigGetMCPServers(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	servers, err := at.configServiceAPI.GetMCPServers(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get MCP servers: %v", err)), nil
	}

	result := map[string]interface{}{
		"servers": servers,
		"total":   len(servers),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleConfigGetPortForwards handles the config_get_port_forwards tool call
func (at *APITools) HandleConfigGetPortForwards(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	portForwards, err := at.configServiceAPI.GetPortForwards(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get port forwards: %v", err)), nil
	}

	result := map[string]interface{}{
		"port_forwards": portForwards,
		"total":         len(portForwards),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleConfigGetWorkflows handles the config_get_workflows tool call
func (at *APITools) HandleConfigGetWorkflows(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflows, err := at.configServiceAPI.GetWorkflows(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get workflows: %v", err)), nil
	}

	result := map[string]interface{}{
		"workflows": workflows,
		"total":     len(workflows),
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleConfigGetAggregator handles the config_get_aggregator tool call
func (at *APITools) HandleConfigGetAggregator(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	aggregator, err := at.configServiceAPI.GetAggregatorConfig(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get aggregator config: %v", err)), nil
	}

	resultJSON, _ := json.MarshalIndent(aggregator, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleConfigGetGlobalSettings handles the config_get_global_settings tool call
func (at *APITools) HandleConfigGetGlobalSettings(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	settings, err := at.configServiceAPI.GetGlobalSettings(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get global settings: %v", err)), nil
	}

	resultJSON, _ := json.MarshalIndent(settings, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(resultJSON)),
		},
	}, nil
}

// HandleConfigUpdateMCPServer handles the config_update_mcp_server tool call
func (at *APITools) HandleConfigUpdateMCPServer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Type assert Arguments to map[string]interface{}
	argsMap, ok := req.Params.Arguments.(map[string]interface{})
	if !ok || argsMap == nil {
		return mcp.NewToolResultError("invalid arguments"), nil
	}

	serverData, ok := argsMap["server"]
	if !ok {
		return mcp.NewToolResultError("server is required"), nil
	}

	// Convert the server data to config.MCPServerDefinition
	serverBytes, err := json.Marshal(serverData)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal server data: %v", err)), nil
	}

	var server config.MCPServerDefinition
	if err := json.Unmarshal(serverBytes, &server); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse server definition: %v", err)), nil
	}

	if err := at.configServiceAPI.UpdateMCPServer(ctx, server); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update MCP server: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully updated MCP server '%s'", server.Name)), nil
}

// HandleConfigUpdatePortForward handles the config_update_port_forward tool call
func (at *APITools) HandleConfigUpdatePortForward(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Type assert Arguments to map[string]interface{}
	argsMap, ok := req.Params.Arguments.(map[string]interface{})
	if !ok || argsMap == nil {
		return mcp.NewToolResultError("invalid arguments"), nil
	}

	portForwardData, ok := argsMap["port_forward"]
	if !ok {
		return mcp.NewToolResultError("port_forward is required"), nil
	}

	// Convert the port forward data to config.PortForwardDefinition
	portForwardBytes, err := json.Marshal(portForwardData)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal port forward data: %v", err)), nil
	}

	var portForward config.PortForwardDefinition
	if err := json.Unmarshal(portForwardBytes, &portForward); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse port forward definition: %v", err)), nil
	}

	if err := at.configServiceAPI.UpdatePortForward(ctx, portForward); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update port forward: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully updated port forward '%s'", portForward.Name)), nil
}

// HandleConfigUpdateWorkflow handles the config_update_workflow tool call
func (at *APITools) HandleConfigUpdateWorkflow(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Type assert Arguments to map[string]interface{}
	argsMap, ok := req.Params.Arguments.(map[string]interface{})
	if !ok || argsMap == nil {
		return mcp.NewToolResultError("invalid arguments"), nil
	}

	workflowData, ok := argsMap["workflow"]
	if !ok {
		return mcp.NewToolResultError("workflow is required"), nil
	}

	// Convert the workflow data to config.WorkflowDefinition
	workflowBytes, err := json.Marshal(workflowData)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal workflow data: %v", err)), nil
	}

	var workflow config.WorkflowDefinition
	if err := json.Unmarshal(workflowBytes, &workflow); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse workflow definition: %v", err)), nil
	}

	if err := at.configServiceAPI.UpdateWorkflow(ctx, workflow); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update workflow: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully updated workflow '%s'", workflow.Name)), nil
}

// HandleConfigUpdateAggregator handles the config_update_aggregator tool call
func (at *APITools) HandleConfigUpdateAggregator(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Type assert Arguments to map[string]interface{}
	argsMap, ok := req.Params.Arguments.(map[string]interface{})
	if !ok || argsMap == nil {
		return mcp.NewToolResultError("invalid arguments"), nil
	}

	aggregatorData, ok := argsMap["aggregator"]
	if !ok {
		return mcp.NewToolResultError("aggregator is required"), nil
	}

	// Convert the aggregator data to config.AggregatorConfig
	aggregatorBytes, err := json.Marshal(aggregatorData)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal aggregator data: %v", err)), nil
	}

	var aggregator config.AggregatorConfig
	if err := json.Unmarshal(aggregatorBytes, &aggregator); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse aggregator config: %v", err)), nil
	}

	if err := at.configServiceAPI.UpdateAggregatorConfig(ctx, aggregator); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update aggregator config: %v", err)), nil
	}

	return mcp.NewToolResultText("Successfully updated aggregator configuration"), nil
}

// HandleConfigUpdateGlobalSettings handles the config_update_global_settings tool call
func (at *APITools) HandleConfigUpdateGlobalSettings(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Type assert Arguments to map[string]interface{}
	argsMap, ok := req.Params.Arguments.(map[string]interface{})
	if !ok || argsMap == nil {
		return mcp.NewToolResultError("invalid arguments"), nil
	}

	settingsData, ok := argsMap["settings"]
	if !ok {
		return mcp.NewToolResultError("settings is required"), nil
	}

	// Convert the settings data to config.GlobalSettings
	settingsBytes, err := json.Marshal(settingsData)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal settings data: %v", err)), nil
	}

	var settings config.GlobalSettings
	if err := json.Unmarshal(settingsBytes, &settings); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse global settings: %v", err)), nil
	}

	if err := at.configServiceAPI.UpdateGlobalSettings(ctx, settings); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update global settings: %v", err)), nil
	}

	return mcp.NewToolResultText("Successfully updated global settings"), nil
}

// HandleConfigDeleteMCPServer handles the config_delete_mcp_server tool call
func (at *APITools) HandleConfigDeleteMCPServer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}

	if err := at.configServiceAPI.DeleteMCPServer(ctx, name); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete MCP server: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully deleted MCP server '%s'", name)), nil
}

// HandleConfigDeletePortForward handles the config_delete_port_forward tool call
func (at *APITools) HandleConfigDeletePortForward(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}

	if err := at.configServiceAPI.DeletePortForward(ctx, name); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete port forward: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully deleted port forward '%s'", name)), nil
}

// HandleConfigDeleteWorkflow handles the config_delete_workflow tool call
func (at *APITools) HandleConfigDeleteWorkflow(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}

	if err := at.configServiceAPI.DeleteWorkflow(ctx, name); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete workflow: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully deleted workflow '%s'", name)), nil
}

// HandleConfigDeleteCluster handles the config_delete_cluster tool call
func (at *APITools) HandleConfigDeleteCluster(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError("name is required"), nil
	}

	if err := at.configServiceAPI.DeleteCluster(ctx, name); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete cluster: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully deleted cluster '%s'", name)), nil
}

// HandleConfigSave handles the config_save tool call
func (at *APITools) HandleConfigSave(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := at.configServiceAPI.SaveConfig(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to save configuration: %v", err)), nil
	}

	return mcp.NewToolResultText("Successfully saved configuration to file"), nil
}
