package aggregator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrefixToolName_NewNamingConvention(t *testing.T) {
	// Create a test aggregator server with config
	server := &AggregatorServer{
		config: AggregatorConfig{
			EnvctlPrefix: "x",
		},
	}

	tests := []struct {
		name         string
		provider     string
		toolName     string
		expectedName string
		description  string
	}{
		// Management tools that should get core_ prefix
		{
			name:         "orchestrator service management",
			provider:     "orchestrator",
			toolName:     "service_start",
			expectedName: "core_service_start",
			description:  "service management tools get core_ prefix",
		},
		{
			name:         "orchestrator service status",
			provider:     "orchestrator",
			toolName:     "service_status",
			expectedName: "core_service_status",
			description:  "service status tools get core_ prefix",
		},
		{
			name:         "workflow management",
			provider:     "workflow",
			toolName:     "workflow_list",
			expectedName: "core_workflow_list",
			description:  "workflow management tools get core_ prefix",
		},
		{
			name:         "capability management",
			provider:     "capability",
			toolName:     "capability_info",
			expectedName: "core_capability_info",
			description:  "capability management tools get core_ prefix",
		},
		{
			name:         "config management",
			provider:     "config",
			toolName:     "config_get",
			expectedName: "core_config_get",
			description:  "config management tools get core_ prefix",
		},
		{
			name:         "mcp management",
			provider:     "mcp",
			toolName:     "mcp_server_list",
			expectedName: "core_mcp_server_list",
			description:  "MCP management tools get core_ prefix",
		},
		{
			name:         "cluster management",
			provider:     "orchestrator",
			toolName:     "cluster_list",
			expectedName: "core_cluster_list",
			description:  "cluster management tools get core_ prefix",
		},
		// Execution tools transformation
		{
			name:         "action to workflow transformation",
			provider:     "workflow",
			toolName:     "action_deploy_app",
			expectedName: "workflow_deploy_app",
			description:  "action_* tools become workflow_* tools",
		},
		{
			name:         "action execution tool",
			provider:     "workflow",
			toolName:     "action_backup",
			expectedName: "workflow_backup",
			description:  "action tools are transformed to workflow execution tools",
		},
		// API tools stay unchanged
		{
			name:         "api capability operation",
			provider:     "capability",
			toolName:     "api_auth_login",
			expectedName: "api_auth_login",
			description:  "api_* tools stay unchanged",
		},
		{
			name:         "api cluster connect",
			provider:     "capability",
			toolName:     "api_cluster_connect",
			expectedName: "api_cluster_connect",
			description:  "api_* capability operations remain unchanged",
		},
		// External MCP server tools get external prefix
		{
			name:         "external mcp tool",
			provider:     "external",
			toolName:     "kubectl_get",
			expectedName: "x_kubectl_get",
			description:  "external tools get configurable prefix",
		},
		{
			name:         "custom capability operation",
			provider:     "external",
			toolName:     "prometheus_query",
			expectedName: "x_prometheus_query",
			description:  "external capability tools get external prefix",
		},
		// Legacy tools (will be removed but need to work for now)
		{
			name:         "legacy portforward management",
			provider:     "portforward",
			toolName:     "portforward_list",
			expectedName: "core_portforward_list",
			description:  "legacy portforward tools get core_ prefix",
		},
		{
			name:         "legacy k8s management",
			provider:     "k8s",
			toolName:     "k8s_connection_list",
			expectedName: "core_k8s_connection_list",
			description:  "legacy K8s tools get core_ prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.prefixToolName(tt.provider, tt.toolName)
			assert.Equal(t, tt.expectedName, result, tt.description)
		})
	}
}

func TestPrefixToolName_EdgeCases(t *testing.T) {
	server := &AggregatorServer{
		config: AggregatorConfig{
			EnvctlPrefix: "envctl",
		},
	}

	tests := []struct {
		name         string
		provider     string
		toolName     string
		expectedName string
		description  string
	}{
		{
			name:         "custom envctl prefix",
			provider:     "external",
			toolName:     "custom_tool",
			expectedName: "envctl_custom_tool",
			description:  "external tools use configurable prefix",
		},
		{
			name:         "empty tool name",
			provider:     "orchestrator",
			toolName:     "",
			expectedName: "envctl_",
			description:  "empty tool name gets external prefix",
		},
		{
			name:         "partial match not service",
			provider:     "external",
			toolName:     "something_service_related",
			expectedName: "envctl_something_service_related",
			description:  "partial matches don't trigger management prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.prefixToolName(tt.provider, tt.toolName)
			assert.Equal(t, tt.expectedName, result, tt.description)
		})
	}
}
