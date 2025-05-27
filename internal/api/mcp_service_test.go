package api

import (
	"context"
	"envctl/internal/services"
	"errors"
	"testing"
)

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
			"port":    8080,
			"pid":     12345,
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

	if info.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", info.Port)
	}

	if info.PID != 12345 {
		t.Errorf("Expected PID 12345, got %d", info.PID)
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
			"port":    8080,
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
			"port":    8081,
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
	if info.Port != 0 {
		t.Errorf("Expected port 0, got %d", info.Port)
	}

	if info.PID != 0 {
		t.Errorf("Expected PID 0, got %d", info.PID)
	}

	if info.Icon != "" {
		t.Errorf("Expected empty icon, got %s", info.Icon)
	}

	if info.Enabled {
		t.Error("Expected enabled to be false by default")
	}
}
