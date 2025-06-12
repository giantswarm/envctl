package capability

import (
	"context"
	"testing"
	"time"

	"envctl/internal/api"

	"github.com/mark3labs/mcp-go/mcp"
)

// testToolChecker is a mock implementation of ToolAvailabilityChecker for testing
type testToolChecker struct {
	availableTools map[string]bool
}

func (m *testToolChecker) IsToolAvailable(toolName string) bool {
	return m.availableTools[toolName]
}

func (m *testToolChecker) GetAvailableTools() []string {
	tools := make([]string, 0, len(m.availableTools))
	for tool, available := range m.availableTools {
		if available {
			tools = append(tools, tool)
		}
	}
	return tools
}

func TestServiceOrchestratorAPIAdapter_ListServiceCapabilities(t *testing.T) {
	// Create a mock tool checker
	toolChecker := &testToolChecker{
		availableTools: map[string]bool{
			"k8s_create_connection": true,
			"k8s_delete_connection": true,
			"k8s_check_health":      true,
		},
	}

	// Set up aggregator tool caller (using mock)
	mockAggregator := &mockAggregatorClient{
		callResponses:  make(map[string]*mcp.CallToolResult),
		availableTools: make(map[string]bool),
	}
	toolCaller := NewAggregatorToolCaller(mockAggregator)

	// Create registry and orchestrator with proper parameters
	registry := NewServiceCapabilityRegistry("test_definitions", toolChecker)
	config := ServiceOrchestratorConfig{
		HealthCheckInterval:  30 * time.Second,
		DefaultCreateTimeout: 60 * time.Second,
		DefaultDeleteTimeout: 30 * time.Second,
		MaxConcurrentOps:     10,
	}
	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)

	// Create the API adapter
	adapter := NewServiceOrchestratorAPIAdapter(orchestrator, registry)

	// Test listing capabilities (should be empty without loaded definitions)
	capabilities := adapter.ListServiceCapabilities()
	if len(capabilities) != 0 {
		t.Errorf("Expected empty capabilities list, got %d capabilities", len(capabilities))
	}
}

func TestServiceOrchestratorAPIAdapter_GetTools(t *testing.T) {
	// Create minimal setup
	toolChecker := &testToolChecker{availableTools: map[string]bool{}}
	registry := NewServiceCapabilityRegistry("test_definitions", toolChecker)

	// Set up mock tool caller and config
	mockAggregator := &mockAggregatorClient{
		callResponses:  make(map[string]*mcp.CallToolResult),
		availableTools: make(map[string]bool),
	}
	toolCaller := NewAggregatorToolCaller(mockAggregator)
	config := DefaultServiceOrchestratorConfig()
	config.DisableControlLoops = true // For testing
	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)

	adapter := NewServiceOrchestratorAPIAdapter(orchestrator, registry)

	// Test GetTools
	tools := adapter.GetTools()
	expectedTools := []string{
		"service_capability_list",
		"service_capability_info",
		"service_capability_check",
		"service_create",
		"service_delete",
		"service_get",
		"service_get_by_label",
		"service_list",
	}

	if len(tools) != len(expectedTools) {
		t.Errorf("Expected %d tools, got %d", len(expectedTools), len(tools))
	}

	// Check that all expected tools are present
	toolMap := make(map[string]bool)
	for _, tool := range tools {
		toolMap[tool.Name] = true
	}

	for _, expectedTool := range expectedTools {
		if !toolMap[expectedTool] {
			t.Errorf("Expected tool %s not found", expectedTool)
		}
	}
}

func TestServiceOrchestratorAPIAdapter_ExecuteTool(t *testing.T) {
	// Create minimal setup
	toolChecker := &testToolChecker{availableTools: map[string]bool{}}
	registry := NewServiceCapabilityRegistry("test_definitions", toolChecker)

	// Set up mock tool caller and config
	mockAggregator := &mockAggregatorClient{
		callResponses:  make(map[string]*mcp.CallToolResult),
		availableTools: make(map[string]bool),
	}
	toolCaller := NewAggregatorToolCaller(mockAggregator)
	config := DefaultServiceOrchestratorConfig()
	config.DisableControlLoops = true // For testing
	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)

	adapter := NewServiceOrchestratorAPIAdapter(orchestrator, registry)

	// Test service_capability_list tool
	result, err := adapter.ExecuteTool(context.Background(), "service_capability_list", map[string]interface{}{})
	if err != nil {
		t.Errorf("service_capability_list tool failed: %v", err)
	}

	if result.IsError {
		t.Error("service_capability_list returned error result")
	}

	if len(result.Content) != 1 {
		t.Errorf("Expected 1 content item, got %d", len(result.Content))
	}

	// Test service_list tool
	result, err = adapter.ExecuteTool(context.Background(), "service_list", map[string]interface{}{})
	if err != nil {
		t.Errorf("service_list tool failed: %v", err)
	}

	if result.IsError {
		t.Error("service_list returned error result")
	}

	// Test unknown tool
	_, err = adapter.ExecuteTool(context.Background(), "unknown_tool", map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for unknown tool")
	}
}

func TestServiceOrchestratorAPIAdapter_IsServiceCapabilityAvailable(t *testing.T) {
	// Create minimal setup
	toolChecker := &testToolChecker{availableTools: map[string]bool{}}
	registry := NewServiceCapabilityRegistry("test_definitions", toolChecker)

	// Set up mock tool caller and config
	mockAggregator := &mockAggregatorClient{
		callResponses:  make(map[string]*mcp.CallToolResult),
		availableTools: make(map[string]bool),
	}
	toolCaller := NewAggregatorToolCaller(mockAggregator)
	config := DefaultServiceOrchestratorConfig()
	config.DisableControlLoops = true // For testing
	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)

	adapter := NewServiceOrchestratorAPIAdapter(orchestrator, registry)

	// Test non-existent capability
	available := adapter.IsServiceCapabilityAvailable("non-existent")
	if available {
		t.Error("Expected non-existent capability to be unavailable")
	}
}

func TestServiceOrchestratorAPIAdapter_Register(t *testing.T) {
	// Create minimal setup
	toolChecker := &testToolChecker{availableTools: map[string]bool{}}
	registry := NewServiceCapabilityRegistry("test_definitions", toolChecker)

	// Set up mock tool caller and config
	mockAggregator := &mockAggregatorClient{
		callResponses:  make(map[string]*mcp.CallToolResult),
		availableTools: make(map[string]bool),
	}
	toolCaller := NewAggregatorToolCaller(mockAggregator)
	config := DefaultServiceOrchestratorConfig()
	config.DisableControlLoops = true // For testing
	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)

	adapter := NewServiceOrchestratorAPIAdapter(orchestrator, registry)

	// Test register (should not panic)
	adapter.Register()

	// Verify it was registered
	handler := api.GetServiceOrchestrator()
	if handler == nil {
		t.Error("Expected service orchestrator handler to be registered")
	}
}

func TestServiceOrchestratorAPIAdapter_WithNilOrchestrator(t *testing.T) {
	// Test adapter with nil orchestrator
	adapter := NewServiceOrchestratorAPIAdapter(nil, nil)

	// Test ListServiceCapabilities with nil registry
	capabilities := adapter.ListServiceCapabilities()
	if len(capabilities) != 0 {
		t.Errorf("Expected empty capabilities list with nil registry, got %d capabilities", len(capabilities))
	}

	// Test IsServiceCapabilityAvailable with nil registry
	available := adapter.IsServiceCapabilityAvailable("test")
	if available {
		t.Error("Expected capability to be unavailable with nil registry")
	}

	// Test CreateService with nil orchestrator
	_, err := adapter.CreateService(context.Background(), api.CreateServiceRequest{
		CapabilityName: "test",
		Label:          "test",
		Parameters:     map[string]interface{}{},
	})
	if err == nil {
		t.Error("Expected error with nil orchestrator")
	}

	// Test DeleteService with nil orchestrator
	err = adapter.DeleteService(context.Background(), "test")
	if err == nil {
		t.Error("Expected error with nil orchestrator")
	}

	// Test GetService with nil orchestrator
	_, err = adapter.GetService("test")
	if err == nil {
		t.Error("Expected error with nil orchestrator")
	}

	// Test GetServiceByLabel with nil orchestrator
	_, err = adapter.GetServiceByLabel("test")
	if err == nil {
		t.Error("Expected error with nil orchestrator")
	}

	// Test ListServices with nil orchestrator
	services := adapter.ListServices()
	if len(services) != 0 {
		t.Errorf("Expected empty services list with nil orchestrator, got %d services", len(services))
	}

	// Test SubscribeToServiceEvents with nil orchestrator
	eventChan := adapter.SubscribeToServiceEvents()
	select {
	case _, ok := <-eventChan:
		if ok {
			t.Error("Expected closed channel with nil orchestrator")
		}
	default:
		t.Error("Expected channel to be closed immediately with nil orchestrator")
	}
}
