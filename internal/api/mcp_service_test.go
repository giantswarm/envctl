package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMCPServiceAPI(t *testing.T) {
	api := NewMCPServiceAPI()

	if api == nil {
		t.Error("Expected NewMCPServiceAPI to return non-nil API")
	}
}

func TestGetMCPServerInfo(t *testing.T) {
	// Setup mock registry
	registry := newMockServiceRegistryHandler()
	RegisterServiceRegistry(registry)
	defer func() {
		RegisterServiceRegistry(nil)
	}()

	api := NewMCPServiceAPI()

	// Test service not found
	_, err := api.GetServerInfo(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent service")
	}

	// Create a mock MCP server service
	mockSvc := &mockServiceInfo{
		label:   "test-mcp",
		svcType: TypeMCPServer,
		state:   StateRunning,
		health:  HealthHealthy,
		data: map[string]interface{}{
			"name":    "test-mcp",
			"icon":    "🔧",
			"enabled": true,
		},
	}

	registry.addService(mockSvc)

	// Test successful retrieval
	info, err := api.GetServerInfo(context.Background(), "test-mcp")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info.Label != "test-mcp" {
		t.Errorf("Expected label 'test-mcp', got %s", info.Label)
	}

	if info.State != "running" {
		t.Errorf("Expected state 'running', got %s", info.State)
	}

	if info.Health != "healthy" {
		t.Errorf("Expected health 'healthy', got %s", info.Health)
	}

	if info.Name != "test-mcp" {
		t.Errorf("Expected name 'test-mcp', got %s", info.Name)
	}

	if info.Icon != "🔧" {
		t.Errorf("Expected icon '🔧', got %s", info.Icon)
	}

	if !info.Enabled {
		t.Error("Expected enabled to be true")
	}
}

func TestGetMCPServerInfoWithError(t *testing.T) {
	// Setup mock registry
	registry := newMockServiceRegistryHandler()
	RegisterServiceRegistry(registry)
	defer func() {
		RegisterServiceRegistry(nil)
	}()

	api := NewMCPServiceAPI()

	// Create a mock service with error
	testErr := errors.New("mcp server failed")
	mockSvc := &mockServiceInfo{
		label:   "error-mcp",
		svcType: TypeMCPServer,
		state:   StateError,
		health:  HealthUnhealthy,
		lastErr: testErr,
		data: map[string]interface{}{
			"name": "error-mcp",
		},
	}

	registry.addService(mockSvc)

	info, err := api.GetServerInfo(context.Background(), "error-mcp")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info.Error != testErr.Error() {
		t.Errorf("Expected error message '%s', got '%s'", testErr.Error(), info.Error)
	}
}

func TestGetMCPServerInfoWrongType(t *testing.T) {
	// Setup mock registry
	registry := newMockServiceRegistryHandler()
	RegisterServiceRegistry(registry)
	defer func() {
		RegisterServiceRegistry(nil)
	}()

	api := NewMCPServiceAPI()

	// Create a mock service of wrong type
	mockSvc := &mockServiceInfo{
		label:   "wrong-type",
		svcType: TypePortForward,
		state:   StateRunning,
		health:  HealthHealthy,
	}

	registry.addService(mockSvc)

	_, err := api.GetServerInfo(context.Background(), "wrong-type")
	if err == nil {
		t.Error("Expected error for wrong service type")
	}

	if err.Error() != "service wrong-type is not an MCP server" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

func TestListMCPServers(t *testing.T) {
	// Setup mock registry
	registry := newMockServiceRegistryHandler()
	RegisterServiceRegistry(registry)
	defer func() {
		RegisterServiceRegistry(nil)
	}()

	api := NewMCPServiceAPI()

	// Create multiple mock MCP server services
	mockSvc1 := &mockServiceInfo{
		label:   "mcp-1",
		svcType: TypeMCPServer,
		state:   StateRunning,
		health:  HealthHealthy,
		data: map[string]interface{}{
			"name":    "mcp-1",
			"command": "npx mcp-server-1",
		},
	}

	mockSvc2 := &mockServiceInfo{
		label:   "mcp-2",
		svcType: TypeMCPServer,
		state:   StateStarting,
		health:  HealthUnknown,
		data: map[string]interface{}{
			"name":    "mcp-2",
			"command": "npx mcp-server-2",
		},
	}

	// Add a non-MCP-server service (should be filtered out)
	mockSvc3 := &mockServiceInfo{
		label:   "port-forward",
		svcType: TypePortForward,
		state:   StateRunning,
		health:  HealthHealthy,
	}

	registry.addService(mockSvc1)
	registry.addService(mockSvc2)
	registry.addService(mockSvc3)

	mcpServers, err := api.ListServers(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(mcpServers) != 2 {
		t.Errorf("Expected 2 MCP servers, got %d", len(mcpServers))
	}

	// Check that we got the right services
	labels := make(map[string]bool)
	for _, mcp := range mcpServers {
		labels[mcp.Label] = true
	}

	if !labels["mcp-1"] {
		t.Error("Expected mcp-1 in MCP servers")
	}

	if !labels["mcp-2"] {
		t.Error("Expected mcp-2 in MCP servers")
	}

	if labels["port-forward"] {
		t.Error("Did not expect port-forward in MCP servers")
	}
}

func TestMCPServerInfo_Defaults(t *testing.T) {
	// Setup mock registry
	registry := newMockServiceRegistryHandler()
	RegisterServiceRegistry(registry)
	defer func() {
		RegisterServiceRegistry(nil)
	}()

	api := NewMCPServiceAPI()

	// Create a mock service with minimal data
	mockSvc := &mockServiceInfo{
		label:   "minimal-mcp",
		svcType: TypeMCPServer,
		state:   StateRunning,
		health:  HealthHealthy,
		data: map[string]interface{}{
			// Only required fields
			"name": "minimal-mcp",
		},
	}

	registry.addService(mockSvc)

	info, err := api.GetServerInfo(context.Background(), "minimal-mcp")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check default values
	if info.Icon != "" {
		t.Errorf("Expected empty icon, got %s", info.Icon)
	}

	if info.Enabled {
		t.Error("Expected enabled to be false by default")
	}
}

func TestGetTools(t *testing.T) {
	tests := []struct {
		name          string
		serverName    string
		setupRegistry func(*mockServiceRegistryHandler)
		expectedError string
		expectedTools []MCPTool
	}{
		{
			name:       "server not found",
			serverName: "test-server",
			setupRegistry: func(registry *mockServiceRegistryHandler) {
				// Don't add any service
			},
			expectedError: "MCP server test-server not found",
		},
		{
			name:       "server not running",
			serverName: "test-server",
			setupRegistry: func(registry *mockServiceRegistryHandler) {
				mockSvc := &mockServiceInfo{
					label:   "test-server",
					svcType: TypeMCPServer,
					state:   StateStopped,
					health:  HealthUnknown,
				}
				registry.addService(mockSvc)
			},
			expectedError: "MCP server test-server is not running (state: stopped)",
		},
		{
			name:       "server does not provide client access",
			serverName: "test-server",
			setupRegistry: func(registry *mockServiceRegistryHandler) {
				mockSvc := &mockServiceInfo{
					label:   "test-server",
					svcType: TypeMCPServer,
					state:   StateRunning,
					health:  HealthHealthy,
				}
				registry.addService(mockSvc)
			},
			expectedError: "MCP server test-server does not have a registered handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock registry
			mockRegistry := newMockServiceRegistryHandler()

			// Setup registry
			tt.setupRegistry(mockRegistry)

			// Register the mock handler
			RegisterServiceRegistry(mockRegistry)
			defer func() {
				RegisterServiceRegistry(nil)
			}()

			// Create API
			api := NewMCPServiceAPI()

			// Call GetTools
			ctx := context.Background()
			tools, err := api.GetTools(ctx, tt.serverName)

			// Check results
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedTools, tools)
			}
		})
	}
}

func TestGetServerInfo(t *testing.T) {
	tests := []struct {
		name          string
		label         string
		setupRegistry func(*mockServiceRegistryHandler)
		expectedInfo  *MCPServerInfo
		expectedError string
	}{
		{
			name:  "server not found",
			label: "test-server",
			setupRegistry: func(registry *mockServiceRegistryHandler) {
				// Don't add any service
			},
			expectedError: "MCP server test-server not found",
		},
		{
			name:  "not an MCP server",
			label: "test-server",
			setupRegistry: func(registry *mockServiceRegistryHandler) {
				mockSvc := &mockServiceInfo{
					label:   "test-server",
					svcType: ServiceType("other"),
					state:   StateRunning,
					health:  HealthHealthy,
				}
				registry.addService(mockSvc)
			},
			expectedError: "service test-server is not an MCP server",
		},
		{
			name:  "successful retrieval",
			label: "test-server",
			setupRegistry: func(registry *mockServiceRegistryHandler) {
				mockSvc := &mockServiceInfo{
					label:   "test-server",
					svcType: TypeMCPServer,
					state:   StateRunning,
					health:  HealthHealthy,
					data: map[string]interface{}{
						"name":    "Test Server",
						"icon":    "🔧",
						"enabled": true,
					},
				}
				registry.addService(mockSvc)
			},
			expectedInfo: &MCPServerInfo{
				Label:   "test-server",
				Name:    "Test Server",
				State:   "running",
				Health:  "healthy",
				Icon:    "🔧",
				Enabled: true,
			},
		},
		{
			name:  "server with error",
			label: "test-server",
			setupRegistry: func(registry *mockServiceRegistryHandler) {
				mockSvc := &mockServiceInfo{
					label:   "test-server",
					svcType: TypeMCPServer,
					state:   StateError,
					health:  HealthUnhealthy,
					lastErr: errors.New("connection failed"),
					data: map[string]interface{}{
						"name": "Test Server",
					},
				}
				registry.addService(mockSvc)
			},
			expectedInfo: &MCPServerInfo{
				Label:  "test-server",
				Name:   "Test Server",
				State:  "error",
				Health: "unhealthy",
				Error:  "connection failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock registry
			mockRegistry := newMockServiceRegistryHandler()

			// Setup registry
			tt.setupRegistry(mockRegistry)

			// Register the mock handler
			RegisterServiceRegistry(mockRegistry)
			defer func() {
				RegisterServiceRegistry(nil)
			}()

			// Create API
			api := NewMCPServiceAPI()

			// Call GetServerInfo
			ctx := context.Background()
			info, err := api.GetServerInfo(ctx, tt.label)

			// Check results
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedInfo, info)
			}
		})
	}
}

func TestListServers(t *testing.T) {
	tests := []struct {
		name          string
		setupRegistry func(*mockServiceRegistryHandler)
		expectedCount int
		expectedError string
	}{
		{
			name: "multiple MCP servers",
			setupRegistry: func(registry *mockServiceRegistryHandler) {
				// Add MCP servers
				mockSvc1 := &mockServiceInfo{
					label:   "mcp-1",
					svcType: TypeMCPServer,
					state:   StateRunning,
					health:  HealthHealthy,
					data: map[string]interface{}{
						"name": "Server 1",
					},
				}
				mockSvc2 := &mockServiceInfo{
					label:   "mcp-2",
					svcType: TypeMCPServer,
					state:   StateStarting,
					health:  HealthUnknown,
					data: map[string]interface{}{
						"name": "Server 2",
					},
				}
				// Add non-MCP server (should be filtered)
				mockSvc3 := &mockServiceInfo{
					label:   "other",
					svcType: TypePortForward,
					state:   StateRunning,
					health:  HealthHealthy,
				}
				registry.addService(mockSvc1)
				registry.addService(mockSvc2)
				registry.addService(mockSvc3)
			},
			expectedCount: 2,
		},
		{
			name: "no MCP servers",
			setupRegistry: func(registry *mockServiceRegistryHandler) {
				// Only add non-MCP services
				mockSvc := &mockServiceInfo{
					label:   "other",
					svcType: TypePortForward,
					state:   StateRunning,
					health:  HealthHealthy,
				}
				registry.addService(mockSvc)
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock registry
			mockRegistry := newMockServiceRegistryHandler()

			// Setup registry
			tt.setupRegistry(mockRegistry)

			// Register the mock handler
			RegisterServiceRegistry(mockRegistry)
			defer func() {
				RegisterServiceRegistry(nil)
			}()

			// Create API
			api := NewMCPServiceAPI()

			// Call ListServers
			ctx := context.Background()
			servers, err := api.ListServers(ctx)

			// Check results
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, len(servers))
			}
		})
	}
}
