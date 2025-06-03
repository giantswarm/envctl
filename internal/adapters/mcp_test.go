package adapters

import (
	"context"
	"envctl/internal/aggregator"
	"envctl/internal/api"
	"envctl/internal/services"
	"testing"
)

// mockMCPServiceAPI implements a mock MCPServiceAPI for testing
type mockMCPServiceAPI struct{}

func (m *mockMCPServiceAPI) GetServerInfo(ctx context.Context, label string) (*api.MCPServerInfo, error) {
	return &api.MCPServerInfo{}, nil
}

func (m *mockMCPServiceAPI) ListServers(ctx context.Context) ([]*api.MCPServerInfo, error) {
	return []*api.MCPServerInfo{}, nil
}

func (m *mockMCPServiceAPI) GetTools(ctx context.Context, serverName string) ([]api.MCPTool, error) {
	return []api.MCPTool{}, nil
}

// mockService implements a basic service for testing
type mockService struct {
	label       string
	serviceType services.ServiceType
	state       services.ServiceState
	health      services.HealthStatus
	lastError   error
}

func (m *mockService) GetLabel() string                                       { return m.label }
func (m *mockService) GetType() services.ServiceType                          { return m.serviceType }
func (m *mockService) GetState() services.ServiceState                        { return m.state }
func (m *mockService) GetHealth() services.HealthStatus                       { return m.health }
func (m *mockService) GetDependencies() []string                              { return []string{} }
func (m *mockService) Start(ctx context.Context) error                        { return nil }
func (m *mockService) Stop(ctx context.Context) error                         { return nil }
func (m *mockService) Restart(ctx context.Context) error                      { return nil }
func (m *mockService) GetLastError() error                                    { return m.lastError }
func (m *mockService) SetStateChangeCallback(cb services.StateChangeCallback) {}

// mockMCPService implements an MCP service with GetMCPClient
type mockMCPService struct {
	mockService
	mcpClient interface{}
}

func (m *mockMCPService) GetMCPClient() interface{} {
	return m.mcpClient
}

// mockServiceRegistry implements a basic service registry for testing
type mockServiceRegistry struct {
	services map[string]services.Service
}

func newMockServiceRegistry() *mockServiceRegistry {
	return &mockServiceRegistry{
		services: make(map[string]services.Service),
	}
}

func (r *mockServiceRegistry) Register(service services.Service) error {
	r.services[service.GetLabel()] = service
	return nil
}

func (r *mockServiceRegistry) Unregister(label string) error {
	delete(r.services, label)
	return nil
}

func (r *mockServiceRegistry) Get(label string) (services.Service, bool) {
	service, exists := r.services[label]
	return service, exists
}

func (r *mockServiceRegistry) GetAll() []services.Service {
	result := make([]services.Service, 0, len(r.services))
	for _, service := range r.services {
		result = append(result, service)
	}
	return result
}

func (r *mockServiceRegistry) GetByType(serviceType services.ServiceType) []services.Service {
	result := make([]services.Service, 0)
	for _, service := range r.services {
		if service.GetType() == serviceType {
			result = append(result, service)
		}
	}
	return result
}

func TestNewMCPServiceAdapter(t *testing.T) {
	mockAPI := &mockMCPServiceAPI{}
	mockRegistry := newMockServiceRegistry()

	adapter := NewMCPServiceAdapter(mockAPI, mockRegistry)

	if adapter == nil {
		t.Fatal("NewMCPServiceAdapter returned nil")
	}

	if adapter.api == nil {
		t.Error("adapter.api should not be nil")
	}

	if adapter.registry == nil {
		t.Error("adapter.registry should not be nil")
	}
}

func TestMCPServiceAdapter_GetAllMCPServices(t *testing.T) {
	mockAPI := &mockMCPServiceAPI{}
	mockRegistry := newMockServiceRegistry()

	// Register some test services
	mcpService1 := &mockService{
		label:       "mcp-server-1",
		serviceType: services.TypeMCPServer,
		state:       services.StateRunning,
		health:      services.HealthHealthy,
	}

	mcpService2 := &mockService{
		label:       "mcp-server-2",
		serviceType: services.TypeMCPServer,
		state:       services.StateStopped,
		health:      services.HealthUnknown,
	}

	nonMCPService := &mockService{
		label:       "port-forward-1",
		serviceType: services.TypePortForward,
		state:       services.StateRunning,
		health:      services.HealthHealthy,
	}

	mockRegistry.Register(mcpService1)
	mockRegistry.Register(mcpService2)
	mockRegistry.Register(nonMCPService)

	adapter := NewMCPServiceAdapter(mockAPI, mockRegistry)

	// Get all MCP services
	mcpServices := adapter.GetAllMCPServices()

	// Should only return MCP server services
	if len(mcpServices) != 2 {
		t.Errorf("Expected 2 MCP services, got %d", len(mcpServices))
	}

	// Verify service details
	expectedServices := map[string]aggregator.MCPServiceInfo{
		"mcp-server-1": {
			Name:   "mcp-server-1",
			State:  "Running",
			Health: "Healthy",
		},
		"mcp-server-2": {
			Name:   "mcp-server-2",
			State:  "Stopped",
			Health: "Unknown",
		},
	}

	for _, service := range mcpServices {
		expected, exists := expectedServices[service.Name]
		if !exists {
			t.Errorf("Unexpected service: %s", service.Name)
			continue
		}

		if service.State != expected.State {
			t.Errorf("Service %s: expected state %s, got %s", service.Name, expected.State, service.State)
		}

		if service.Health != expected.Health {
			t.Errorf("Service %s: expected health %s, got %s", service.Name, expected.Health, service.Health)
		}
	}
}

func TestMCPServiceAdapter_GetMCPClient(t *testing.T) {
	mockAPI := &mockMCPServiceAPI{}
	mockRegistry := newMockServiceRegistry()

	// Create a mock MCP client
	mockClient := struct{ name string }{name: "test-client"}

	testCases := []struct {
		name           string
		serviceName    string
		serviceToAdd   services.Service
		expectedClient interface{}
		expectNil      bool
	}{
		{
			name:        "existing MCP service with client",
			serviceName: "mcp-server-with-client",
			serviceToAdd: &mockMCPService{
				mockService: mockService{
					label:       "mcp-server-with-client",
					serviceType: services.TypeMCPServer,
				},
				mcpClient: mockClient,
			},
			expectedClient: mockClient,
			expectNil:      false,
		},
		{
			name:        "existing MCP service without client interface",
			serviceName: "mcp-server-no-interface",
			serviceToAdd: &mockService{
				label:       "mcp-server-no-interface",
				serviceType: services.TypeMCPServer,
			},
			expectedClient: nil,
			expectNil:      true,
		},
		{
			name:        "non-MCP service",
			serviceName: "port-forward-service",
			serviceToAdd: &mockService{
				label:       "port-forward-service",
				serviceType: services.TypePortForward,
			},
			expectedClient: nil,
			expectNil:      true,
		},
		{
			name:           "non-existent service",
			serviceName:    "non-existent",
			serviceToAdd:   nil,
			expectedClient: nil,
			expectNil:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear registry
			mockRegistry.services = make(map[string]services.Service)

			// Add test service if provided
			if tc.serviceToAdd != nil {
				mockRegistry.Register(tc.serviceToAdd)
			}

			adapter := NewMCPServiceAdapter(mockAPI, mockRegistry)

			// Get MCP client
			client := adapter.GetMCPClient(tc.serviceName)

			if tc.expectNil && client != nil {
				t.Errorf("Expected nil client, got %v", client)
			}

			if !tc.expectNil && client != tc.expectedClient {
				t.Errorf("Expected client %v, got %v", tc.expectedClient, client)
			}
		})
	}
}

func TestMCPServiceAdapter_EmptyRegistry(t *testing.T) {
	mockAPI := &mockMCPServiceAPI{}
	mockRegistry := newMockServiceRegistry()

	adapter := NewMCPServiceAdapter(mockAPI, mockRegistry)

	// Should return empty list
	services := adapter.GetAllMCPServices()
	if len(services) != 0 {
		t.Errorf("Expected 0 services, got %d", len(services))
	}

	// Should return nil for any client request
	client := adapter.GetMCPClient("any-service")
	if client != nil {
		t.Errorf("Expected nil client, got %v", client)
	}
}
