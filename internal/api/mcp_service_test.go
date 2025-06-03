package api

import (
	"context"
	"envctl/internal/services"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockServiceRegistry is a mock implementation of ServiceRegistry
type MockServiceRegistry struct {
	mock.Mock
}

func (m *MockServiceRegistry) Register(service services.Service) error {
	args := m.Called(service)
	return args.Error(0)
}

func (m *MockServiceRegistry) Unregister(label string) error {
	args := m.Called(label)
	return args.Error(0)
}

func (m *MockServiceRegistry) Get(label string) (services.Service, bool) {
	args := m.Called(label)
	if args.Get(0) == nil {
		return nil, args.Bool(1)
	}
	return args.Get(0).(services.Service), args.Bool(1)
}

func (m *MockServiceRegistry) GetAll() []services.Service {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]services.Service)
}

func (m *MockServiceRegistry) GetByType(serviceType services.ServiceType) []services.Service {
	args := m.Called(serviceType)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]services.Service)
}

// MockService is a mock implementation of Service
type MockService struct {
	mock.Mock
}

func (m *MockService) GetLabel() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockService) GetType() services.ServiceType {
	args := m.Called()
	return args.Get(0).(services.ServiceType)
}

func (m *MockService) GetState() services.ServiceState {
	args := m.Called()
	return args.Get(0).(services.ServiceState)
}

func (m *MockService) GetHealth() services.HealthStatus {
	args := m.Called()
	return args.Get(0).(services.HealthStatus)
}

func (m *MockService) GetLastError() error {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Error(0)
}

func (m *MockService) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockService) Stop(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockService) Restart(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockService) GetDependencies() []string {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]string)
}

func (m *MockService) SetStateChangeCallback(callback services.StateChangeCallback) {
	m.Called(callback)
}

// MockServiceDataProvider is a mock implementation of ServiceDataProvider
type MockServiceDataProvider struct {
	MockService
}

func (m *MockServiceDataProvider) GetServiceData() map[string]interface{} {
	args := m.Called()
	return args.Get(0).(map[string]interface{})
}

func TestNewMCPServiceAPI(t *testing.T) {
	registry := newMockRegistry()
	api := NewMCPServiceAPI(registry)

	if api == nil {
		t.Error("Expected NewMCPServiceAPI to return non-nil API")
	}
}

func TestGetMCPServerInfo(t *testing.T) {
	registry := newMockRegistry()
	api := NewMCPServiceAPI(registry)

	// Test service not found
	_, err := api.GetServerInfo(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent service")
	}

	// Create a mock MCP server service
	mockSvc := &mockService{
		label:       "test-mcp",
		serviceType: services.TypeMCPServer,
		state:       services.StateRunning,
		health:      services.HealthHealthy,
		serviceData: map[string]interface{}{
			"name":    "test-mcp",
			"icon":    "🔧",
			"enabled": true,
		},
	}

	registry.Register(mockSvc)

	// Test successful retrieval
	info, err := api.GetServerInfo(context.Background(), "test-mcp")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info.Label != "test-mcp" {
		t.Errorf("Expected label 'test-mcp', got %s", info.Label)
	}

	if info.State != "Running" {
		t.Errorf("Expected state 'Running', got %s", info.State)
	}

	if info.Health != "Healthy" {
		t.Errorf("Expected health 'Healthy', got %s", info.Health)
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
	registry := newMockRegistry()
	api := NewMCPServiceAPI(registry)

	// Create a mock service with error
	testErr := errors.New("mcp server failed")
	mockSvc := &mockService{
		label:       "error-mcp",
		serviceType: services.TypeMCPServer,
		state:       services.StateFailed,
		health:      services.HealthUnhealthy,
		lastError:   testErr,
		serviceData: map[string]interface{}{
			"name": "error-mcp",
		},
	}

	registry.Register(mockSvc)

	info, err := api.GetServerInfo(context.Background(), "error-mcp")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info.Error != testErr.Error() {
		t.Errorf("Expected error message '%s', got '%s'", testErr.Error(), info.Error)
	}
}

func TestGetMCPServerInfoWrongType(t *testing.T) {
	registry := newMockRegistry()
	api := NewMCPServiceAPI(registry)

	// Create a mock service of wrong type
	mockSvc := &mockService{
		label:       "wrong-type",
		serviceType: services.TypePortForward,
		state:       services.StateRunning,
		health:      services.HealthHealthy,
	}

	registry.Register(mockSvc)

	_, err := api.GetServerInfo(context.Background(), "wrong-type")
	if err == nil {
		t.Error("Expected error for wrong service type")
	}

	if err.Error() != "service wrong-type is not an MCP server" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

func TestListMCPServers(t *testing.T) {
	registry := newMockRegistry()
	api := NewMCPServiceAPI(registry)

	// Create multiple mock MCP server services
	mockSvc1 := &mockService{
		label:       "mcp-1",
		serviceType: services.TypeMCPServer,
		state:       services.StateRunning,
		health:      services.HealthHealthy,
		serviceData: map[string]interface{}{
			"name":    "mcp-1",
			"command": "npx mcp-server-1",
		},
	}

	mockSvc2 := &mockService{
		label:       "mcp-2",
		serviceType: services.TypeMCPServer,
		state:       services.StateStarting,
		health:      services.HealthChecking,
		serviceData: map[string]interface{}{
			"name":    "mcp-2",
			"command": "npx mcp-server-2",
		},
	}

	// Add a non-MCP-server service (should be filtered out)
	mockSvc3 := &mockService{
		label:       "port-forward",
		serviceType: services.TypePortForward,
		state:       services.StateRunning,
		health:      services.HealthHealthy,
	}

	registry.Register(mockSvc1)
	registry.Register(mockSvc2)
	registry.Register(mockSvc3)

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
	registry := newMockRegistry()
	api := NewMCPServiceAPI(registry)

	// Create a mock service with minimal data
	mockSvc := &mockService{
		label:       "minimal-mcp",
		serviceType: services.TypeMCPServer,
		state:       services.StateRunning,
		health:      services.HealthHealthy,
		serviceData: map[string]interface{}{
			// Only required fields
			"name": "minimal-mcp",
		},
	}

	registry.Register(mockSvc)

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
		setupMocks    func(*MockServiceRegistry, *MockServiceDataProvider)
		expectedError string
		expectedTools []MCPTool
	}{
		{
			name:       "server not found",
			serverName: "test-server",
			setupMocks: func(registry *MockServiceRegistry, service *MockServiceDataProvider) {
				registry.On("Get", "test-server").Return(nil, false)
			},
			expectedError: "MCP server test-server not found",
		},
		{
			name:       "server not running",
			serverName: "test-server",
			setupMocks: func(registry *MockServiceRegistry, service *MockServiceDataProvider) {
				registry.On("Get", "test-server").Return(service, true)
				service.On("GetType").Return(services.TypeMCPServer)
				service.On("GetState").Return(services.StateStopped)
			},
			expectedError: "MCP server test-server is not running (state: Stopped)",
		},
		{
			name:       "server does not provide client access",
			serverName: "test-server",
			setupMocks: func(registry *MockServiceRegistry, service *MockServiceDataProvider) {
				registry.On("Get", "test-server").Return(service, true)
				service.On("GetType").Return(services.TypeMCPServer)
				service.On("GetState").Return(services.StateRunning)
			},
			expectedError: "MCP server test-server does not provide client access",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockRegistry := new(MockServiceRegistry)
			mockService := new(MockServiceDataProvider)

			// Setup mocks
			tt.setupMocks(mockRegistry, mockService)

			// Create API
			api := NewMCPServiceAPI(mockRegistry)

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

			// Verify mocks
			mockRegistry.AssertExpectations(t)
			mockService.AssertExpectations(t)
		})
	}
}

func TestGetServerInfo(t *testing.T) {
	tests := []struct {
		name          string
		label         string
		setupMocks    func(*MockServiceRegistry, *MockServiceDataProvider)
		expectedInfo  *MCPServerInfo
		expectedError string
	}{
		{
			name:  "server not found",
			label: "test-server",
			setupMocks: func(registry *MockServiceRegistry, service *MockServiceDataProvider) {
				registry.On("Get", "test-server").Return(nil, false)
			},
			expectedError: "MCP server test-server not found",
		},
		{
			name:  "not an MCP server",
			label: "test-server",
			setupMocks: func(registry *MockServiceRegistry, service *MockServiceDataProvider) {
				registry.On("Get", "test-server").Return(service, true)
				service.On("GetType").Return(services.ServiceType("other"))
			},
			expectedError: "service test-server is not an MCP server",
		},
		{
			name:  "successful retrieval",
			label: "test-server",
			setupMocks: func(registry *MockServiceRegistry, service *MockServiceDataProvider) {
				registry.On("Get", "test-server").Return(service, true)
				service.On("GetType").Return(services.TypeMCPServer)
				service.On("GetLabel").Return("test-server")
				service.On("GetState").Return(services.StateRunning)
				service.On("GetHealth").Return(services.HealthHealthy)
				service.On("GetLastError").Return(nil)
				service.On("GetServiceData").Return(map[string]interface{}{
					"name":    "Test Server",
					"icon":    "🔧",
					"enabled": true,
				})
			},
			expectedInfo: &MCPServerInfo{
				Label:   "test-server",
				Name:    "Test Server",
				State:   "Running",
				Health:  "Healthy",
				Icon:    "🔧",
				Enabled: true,
			},
		},
		{
			name:  "server with error",
			label: "test-server",
			setupMocks: func(registry *MockServiceRegistry, service *MockServiceDataProvider) {
				registry.On("Get", "test-server").Return(service, true)
				service.On("GetType").Return(services.TypeMCPServer)
				service.On("GetLabel").Return("test-server")
				service.On("GetState").Return(services.StateFailed)
				service.On("GetHealth").Return(services.HealthUnhealthy)
				service.On("GetLastError").Return(errors.New("connection failed"))
				service.On("GetServiceData").Return(map[string]interface{}{
					"name": "Test Server",
				})
			},
			expectedInfo: &MCPServerInfo{
				Label:  "test-server",
				Name:   "Test Server",
				State:  "Failed",
				Health: "Unhealthy",
				Error:  "connection failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockRegistry := new(MockServiceRegistry)
			mockService := new(MockServiceDataProvider)

			// Setup mocks
			tt.setupMocks(mockRegistry, mockService)

			// Create API
			api := NewMCPServiceAPI(mockRegistry)

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

			// Verify mocks
			mockRegistry.AssertExpectations(t)
			mockService.AssertExpectations(t)
		})
	}
}

func TestListServers(t *testing.T) {
	// Create mocks
	mockRegistry := new(MockServiceRegistry)
	mockService1 := new(MockServiceDataProvider)
	mockService2 := new(MockServiceDataProvider)

	// Setup mocks
	mockRegistry.On("GetByType", services.TypeMCPServer).Return([]services.Service{
		mockService1,
		mockService2,
	})

	// Setup registry Get calls for GetServerInfo
	mockRegistry.On("Get", "server1").Return(mockService1, true)
	mockRegistry.On("Get", "server2").Return(mockService2, true)

	// Setup service 1
	mockService1.On("GetLabel").Return("server1")
	mockService1.On("GetType").Return(services.TypeMCPServer)
	mockService1.On("GetState").Return(services.StateRunning)
	mockService1.On("GetHealth").Return(services.HealthHealthy)
	mockService1.On("GetLastError").Return(nil)
	mockService1.On("GetServiceData").Return(map[string]interface{}{
		"name": "Server 1",
	})

	// Setup service 2
	mockService2.On("GetLabel").Return("server2")
	mockService2.On("GetType").Return(services.TypeMCPServer)
	mockService2.On("GetState").Return(services.StateStopped)
	mockService2.On("GetHealth").Return(services.HealthUnknown)
	mockService2.On("GetLastError").Return(nil)
	mockService2.On("GetServiceData").Return(map[string]interface{}{
		"name": "Server 2",
	})

	// Create API
	api := NewMCPServiceAPI(mockRegistry)

	// Call ListServers
	ctx := context.Background()
	servers, err := api.ListServers(ctx)

	// Check results
	assert.NoError(t, err)
	assert.Len(t, servers, 2)
	assert.Equal(t, "server1", servers[0].Label)
	assert.Equal(t, "Server 1", servers[0].Name)
	assert.Equal(t, "Running", servers[0].State)
	assert.Equal(t, "server2", servers[1].Label)
	assert.Equal(t, "Server 2", servers[1].Name)
	assert.Equal(t, "Stopped", servers[1].State)

	// Verify mocks
	mockRegistry.AssertExpectations(t)
	mockService1.AssertExpectations(t)
	mockService2.AssertExpectations(t)
}
