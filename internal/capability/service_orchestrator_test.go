package capability

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockOrchestratorToolCaller implements ToolCaller for testing
type mockOrchestratorToolCaller struct {
	calls         []orchestratorToolCall
	responses     map[string]map[string]interface{}
	errors        map[string]error
	shouldFail    bool
	expectedCalls []string
}

type orchestratorToolCall struct {
	toolName  string
	arguments map[string]interface{}
}

func newMockOrchestratorToolCaller() *mockOrchestratorToolCaller {
	return &mockOrchestratorToolCaller{
		calls:     make([]orchestratorToolCall, 0),
		responses: make(map[string]map[string]interface{}),
		errors:    make(map[string]error),
	}
}

func (m *mockOrchestratorToolCaller) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (map[string]interface{}, error) {
	m.calls = append(m.calls, orchestratorToolCall{
		toolName:  toolName,
		arguments: arguments,
	})

	if m.shouldFail {
		return nil, assert.AnError
	}

	if err, exists := m.errors[toolName]; exists {
		return nil, err
	}

	if response, exists := m.responses[toolName]; exists {
		return response, nil
	}

	// Default successful response
	return map[string]interface{}{
		"status":    "success",
		"serviceId": "mock-service-id-123",
	}, nil
}

func (m *mockOrchestratorToolCaller) setResponse(toolName string, response map[string]interface{}) {
	m.responses[toolName] = response
}

func (m *mockOrchestratorToolCaller) setError(toolName string, err error) {
	m.errors[toolName] = err
}

func (m *mockOrchestratorToolCaller) getCalls() []orchestratorToolCall {
	return m.calls
}

func (m *mockOrchestratorToolCaller) getCallsForTool(toolName string) []orchestratorToolCall {
	var result []orchestratorToolCall
	for _, call := range m.calls {
		if call.toolName == toolName {
			result = append(result, call)
		}
	}
	return result
}

func TestServiceOrchestrator_CreateService(t *testing.T) {
	// Set up test registry with a valid service capability
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_test_create": true,
			"x_test_delete": true,
		},
	}

	registry := NewServiceCapabilityRegistry("/nonexistent", toolChecker)

	// Register a test service capability
	testCapability := &ServiceCapabilityDefinition{
		Name:        "test_service",
		Type:        "test_service",
		Version:     "1.0.0",
		Description: "Test service capability",
		ServiceConfig: ServiceConfig{
			ServiceType:  "TestService",
			DefaultLabel: "test-{{ .name }}",
			LifecycleTools: LifecycleTools{
				Create: ToolCall{
					Tool:            "x_test_create",
					Arguments:       map[string]interface{}{"name": "{{ .name }}"},
					ResponseMapping: ResponseMapping{ServiceID: "$.serviceId"},
				},
				Delete: ToolCall{
					Tool:            "x_test_delete",
					Arguments:       map[string]interface{}{"id": "{{ .service_id }}"},
					ResponseMapping: ResponseMapping{},
				},
			},
			Timeout: TimeoutConfig{
				Create:      60 * time.Second,
				Delete:      30 * time.Second,
				HealthCheck: 10 * time.Second,
			},
		},
	}

	err := registry.RegisterDefinition(testCapability)
	require.NoError(t, err)

	// Create orchestrator with control loops disabled
	toolCaller := newMockOrchestratorToolCaller()
	config := DefaultServiceOrchestratorConfig()
	config.DisableControlLoops = true
	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)

	err = orchestrator.Start(context.Background())
	require.NoError(t, err)
	defer orchestrator.Stop(context.Background())

	// Test successful service creation
	req := CreateServiceRequest{
		CapabilityName: "test_service",
		Label:          "test-instance-1",
		Parameters: map[string]interface{}{
			"name": "test-service",
		},
	}

	info, err := orchestrator.CreateService(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "test-instance-1", info.Label)
	assert.Equal(t, "test_service", info.CapabilityName)
	assert.Equal(t, "TestService", info.CapabilityType)
	assert.NotEmpty(t, info.ServiceID)
	assert.Equal(t, ServiceStateRunning, info.State) // Should be Running after synchronous creation
	assert.Equal(t, req.Parameters, info.CreationParameters)

	// Verify service exists and is in expected state
	updatedInfo, err := orchestrator.GetService(info.ServiceID)
	require.NoError(t, err)
	assert.Equal(t, ServiceStateRunning, updatedInfo.State)
	assert.Equal(t, HealthStatusHealthy, updatedInfo.Health)
}

func TestServiceOrchestrator_CreateService_ValidationErrors(t *testing.T) {
	toolChecker := &mockServiceToolChecker{availableTools: map[string]bool{}}
	registry := NewServiceCapabilityRegistry("/nonexistent", toolChecker)
	toolCaller := newMockOrchestratorToolCaller()

	config := DefaultServiceOrchestratorConfig()
	config.DisableControlLoops = true
	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)
	err := orchestrator.Start(context.Background())
	require.NoError(t, err)
	defer orchestrator.Stop(context.Background())

	tests := []struct {
		name        string
		request     CreateServiceRequest
		expectedErr string
	}{
		{
			name: "missing capability name",
			request: CreateServiceRequest{
				Label:      "test",
				Parameters: map[string]interface{}{},
			},
			expectedErr: "capability name is required",
		},
		{
			name: "missing label",
			request: CreateServiceRequest{
				CapabilityName: "test",
				Parameters:     map[string]interface{}{},
			},
			expectedErr: "service label is required",
		},
		{
			name: "nonexistent capability",
			request: CreateServiceRequest{
				CapabilityName: "nonexistent",
				Label:          "test",
				Parameters:     map[string]interface{}{},
			},
			expectedErr: "capability nonexistent not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := orchestrator.CreateService(context.Background(), tt.request)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestServiceOrchestrator_CreateService_DuplicateLabel(t *testing.T) {
	// Set up registry with test capability
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_test_create": true,
			"x_test_delete": true,
		},
	}

	registry := NewServiceCapabilityRegistry("/nonexistent", toolChecker)
	testCapability := &ServiceCapabilityDefinition{
		Name:        "test_service",
		Type:        "test_service",
		Version:     "1.0.0",
		Description: "Test service capability",
		ServiceConfig: ServiceConfig{
			ServiceType:  "TestService",
			DefaultLabel: "test",
			LifecycleTools: LifecycleTools{
				Create: ToolCall{Tool: "x_test_create", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
				Delete: ToolCall{Tool: "x_test_delete", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
			},
			Timeout: TimeoutConfig{Create: 60 * time.Second, Delete: 30 * time.Second, HealthCheck: 10 * time.Second},
		},
	}

	err := registry.RegisterDefinition(testCapability)
	require.NoError(t, err)

	toolCaller := newMockOrchestratorToolCaller()
	config := DefaultServiceOrchestratorConfig()
	config.DisableControlLoops = true
	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)
	err = orchestrator.Start(context.Background())
	require.NoError(t, err)
	defer orchestrator.Stop(context.Background())

	// Create first service
	req := CreateServiceRequest{
		CapabilityName: "test_service",
		Label:          "duplicate-label",
		Parameters:     map[string]interface{}{},
	}

	_, err = orchestrator.CreateService(context.Background(), req)
	require.NoError(t, err)

	// Try to create second service with same label
	_, err = orchestrator.CreateService(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service with label duplicate-label already exists")
}

func TestServiceOrchestrator_DeleteService(t *testing.T) {
	// Set up registry and orchestrator
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_test_create": true,
			"x_test_delete": true,
		},
	}

	registry := NewServiceCapabilityRegistry("/nonexistent", toolChecker)
	testCapability := &ServiceCapabilityDefinition{
		Name:        "test_service",
		Type:        "test_service",
		Version:     "1.0.0",
		Description: "Test service capability",
		ServiceConfig: ServiceConfig{
			ServiceType:  "TestService",
			DefaultLabel: "test",
			LifecycleTools: LifecycleTools{
				Create: ToolCall{Tool: "x_test_create", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
				Delete: ToolCall{Tool: "x_test_delete", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
			},
			Timeout: TimeoutConfig{Create: 60 * time.Second, Delete: 30 * time.Second, HealthCheck: 10 * time.Second},
		},
	}

	err := registry.RegisterDefinition(testCapability)
	require.NoError(t, err)

	toolCaller := newMockOrchestratorToolCaller()
	config := DefaultServiceOrchestratorConfig()
	config.DisableControlLoops = true
	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = orchestrator.Start(ctx)
	require.NoError(t, err)
	defer orchestrator.Stop(context.Background())

	// Create a service first
	req := CreateServiceRequest{
		CapabilityName: "test_service",
		Label:          "delete-test",
		Parameters:     map[string]interface{}{},
	}

	info, err := orchestrator.CreateService(ctx, req)
	require.NoError(t, err)

	// Verify service was created
	assert.Equal(t, "delete-test", info.Label)
	assert.NotEmpty(t, info.ServiceID)

	// Verify service exists
	updatedInfo, err := orchestrator.GetService(info.ServiceID)
	require.NoError(t, err)
	assert.Equal(t, info.ServiceID, updatedInfo.ServiceID)

	// Delete the service
	err = orchestrator.DeleteService(ctx, info.ServiceID)
	require.NoError(t, err)

	// Verify service is deleted
	_, err = orchestrator.GetService(info.ServiceID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestServiceOrchestrator_GetServices(t *testing.T) {
	// Set up registry and orchestrator
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_test_create": true,
			"x_test_delete": true,
		},
	}

	registry := NewServiceCapabilityRegistry("/nonexistent", toolChecker)
	testCapability := &ServiceCapabilityDefinition{
		Name:        "test_service",
		Type:        "test_service",
		Version:     "1.0.0",
		Description: "Test service capability",
		ServiceConfig: ServiceConfig{
			ServiceType:  "TestService",
			DefaultLabel: "test",
			LifecycleTools: LifecycleTools{
				Create: ToolCall{Tool: "x_test_create", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
				Delete: ToolCall{Tool: "x_test_delete", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
			},
			Timeout: TimeoutConfig{Create: 60 * time.Second, Delete: 30 * time.Second, HealthCheck: 10 * time.Second},
		},
	}

	err := registry.RegisterDefinition(testCapability)
	require.NoError(t, err)

	toolCaller := newMockOrchestratorToolCaller()
	config := DefaultServiceOrchestratorConfig()
	config.DisableControlLoops = true
	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)
	err = orchestrator.Start(context.Background())
	require.NoError(t, err)
	defer orchestrator.Stop(context.Background())

	// Initially no services
	services := orchestrator.ListServices()
	assert.Empty(t, services)

	// Create multiple services
	labels := []string{"service-1", "service-2", "service-3"}
	serviceIDs := make([]string, len(labels))

	for i, label := range labels {
		req := CreateServiceRequest{
			CapabilityName: "test_service",
			Label:          label,
			Parameters:     map[string]interface{}{"index": i},
		}

		info, err := orchestrator.CreateService(context.Background(), req)
		require.NoError(t, err)
		serviceIDs[i] = info.ServiceID
	}

	// List all services
	services = orchestrator.ListServices()
	assert.Len(t, services, 3)

	// Verify each service can be retrieved individually
	for i, serviceID := range serviceIDs {
		info, err := orchestrator.GetService(serviceID)
		require.NoError(t, err)
		assert.Equal(t, labels[i], info.Label)
		assert.Equal(t, "test_service", info.CapabilityName)

		// Also test get by label
		infoByLabel, err := orchestrator.GetServiceByLabel(labels[i])
		require.NoError(t, err)
		assert.Equal(t, info.ServiceID, infoByLabel.ServiceID)
	}
}

func TestServiceOrchestrator_EventSubscription(t *testing.T) {
	// Set up registry and orchestrator
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_test_create": true,
			"x_test_delete": true,
		},
	}

	registry := NewServiceCapabilityRegistry("/nonexistent", toolChecker)
	testCapability := &ServiceCapabilityDefinition{
		Name:        "test_service",
		Type:        "test_service",
		Version:     "1.0.0",
		Description: "Test service capability",
		ServiceConfig: ServiceConfig{
			ServiceType:  "TestService",
			DefaultLabel: "test",
			LifecycleTools: LifecycleTools{
				Create: ToolCall{Tool: "x_test_create", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
				Delete: ToolCall{Tool: "x_test_delete", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
			},
			Timeout: TimeoutConfig{Create: 60 * time.Second, Delete: 30 * time.Second, HealthCheck: 10 * time.Second},
		},
	}

	err := registry.RegisterDefinition(testCapability)
	require.NoError(t, err)

	toolCaller := newMockOrchestratorToolCaller()
	config := DefaultServiceOrchestratorConfig()
	config.DisableControlLoops = true
	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)
	err = orchestrator.Start(context.Background())
	require.NoError(t, err)
	defer orchestrator.Stop(context.Background())

	// Subscribe to events
	eventChan := orchestrator.SubscribeToEvents()

	// Create a service and watch for events
	req := CreateServiceRequest{
		CapabilityName: "test_service",
		Label:          "event-test",
		Parameters:     map[string]interface{}{},
	}

	info, err := orchestrator.CreateService(context.Background(), req)
	require.NoError(t, err)

	// Verify the event subscription channel works
	assert.NotNil(t, eventChan, "Event channel should be available")

	// Verify service was created with synchronous behavior
	finalInfo, err := orchestrator.GetService(info.ServiceID)
	require.NoError(t, err)
	assert.Equal(t, ServiceStateRunning, finalInfo.State)
	assert.Equal(t, HealthStatusHealthy, finalInfo.Health)
}

func TestServiceOrchestrator_CapabilityAvailability(t *testing.T) {
	// Set up registry with unavailable tools
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_test_create": false, // Tool not available
			"x_test_delete": true,
		},
	}

	registry := NewServiceCapabilityRegistry("/nonexistent", toolChecker)
	testCapability := &ServiceCapabilityDefinition{
		Name:        "unavailable_service",
		Type:        "unavailable_service",
		Version:     "1.0.0",
		Description: "Service with unavailable tools",
		ServiceConfig: ServiceConfig{
			ServiceType:  "UnavailableService",
			DefaultLabel: "unavailable",
			LifecycleTools: LifecycleTools{
				Create: ToolCall{Tool: "x_test_create", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
				Delete: ToolCall{Tool: "x_test_delete", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
			},
			Timeout: TimeoutConfig{Create: 60 * time.Second, Delete: 30 * time.Second, HealthCheck: 10 * time.Second},
		},
	}

	err := registry.RegisterDefinition(testCapability)
	require.NoError(t, err)

	toolCaller := newMockOrchestratorToolCaller()
	config := DefaultServiceOrchestratorConfig()
	config.DisableControlLoops = true
	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)
	err = orchestrator.Start(context.Background())
	require.NoError(t, err)
	defer orchestrator.Stop(context.Background())

	// Try to create service with unavailable capability
	req := CreateServiceRequest{
		CapabilityName: "unavailable_service",
		Label:          "unavailable-test",
		Parameters:     map[string]interface{}{},
	}

	_, err = orchestrator.CreateService(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not available (missing required tools)")
}

func TestServiceOrchestrator_Stop(t *testing.T) {
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_test_create": true,
			"x_test_delete": true,
		},
	}

	registry := NewServiceCapabilityRegistry("/nonexistent", toolChecker)
	toolCaller := newMockOrchestratorToolCaller()
	config := DefaultServiceOrchestratorConfig()

	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orchestrator.Start(ctx)
	require.NoError(t, err)

	// Stop orchestrator immediately
	err = orchestrator.Stop(context.Background())
	require.NoError(t, err)

	// Verify stop completed cleanly
	assert.NotNil(t, orchestrator)
}

func TestServiceOrchestrator_ControlLoop(t *testing.T) {
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_test_create": true,
			"x_test_delete": true,
			"x_test_health": true,
		},
	}

	registry := NewServiceCapabilityRegistry("/nonexistent", toolChecker)

	// Register test capability with health check enabled
	testCapability := &ServiceCapabilityDefinition{
		Name:        "test_health_service",
		Type:        "test_health_service",
		Version:     "1.0.0",
		Description: "Test service with health check",
		ServiceConfig: ServiceConfig{
			ServiceType:  "TestHealthService",
			DefaultLabel: "test-health-{{ .name }}",
			LifecycleTools: LifecycleTools{
				Create:      ToolCall{Tool: "x_test_create", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
				Delete:      ToolCall{Tool: "x_test_delete", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
				HealthCheck: &ToolCall{Tool: "x_test_health", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
			},
			HealthCheck: HealthCheckConfig{
				Enabled:          true,
				Interval:         50 * time.Millisecond,
				FailureThreshold: 3,
				SuccessThreshold: 2,
			},
			Timeout: TimeoutConfig{
				Create:      60 * time.Second,
				Delete:      30 * time.Second,
				HealthCheck: 10 * time.Second,
			},
		},
	}

	err := registry.RegisterDefinition(testCapability)
	require.NoError(t, err)

	toolCaller := newMockOrchestratorToolCaller()
	config := DefaultServiceOrchestratorConfig()
	config.HealthCheckInterval = 0 // Disable automatic health checks for controlled testing
	config.DisableControlLoops = true

	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = orchestrator.Start(ctx)
	require.NoError(t, err)
	defer orchestrator.Stop(context.Background())

	// Create a service instance
	req := CreateServiceRequest{
		CapabilityName: "test_health_service",
		Label:          "health-test",
		Parameters:     map[string]interface{}{"name": "health-test"},
	}

	info, err := orchestrator.CreateService(ctx, req)
	require.NoError(t, err)

	// Verify service was created
	assert.Equal(t, "health-test", info.Label)
	assert.Equal(t, "test_health_service", info.CapabilityName)
	assert.NotEmpty(t, info.ServiceID)

	// Test control loop message sending
	select {
	case orchestrator.controlLoopChan <- controlLoopMessage{
		messageType: controlLoopHealthCheck,
		serviceID:   info.ServiceID,
	}:
		// Message sent successfully
	default:
		t.Fatal("Failed to send health check message to control loop")
	}

	// Verify control loop accepts messages
	assert.NotNil(t, orchestrator.controlLoopChan)
}

func TestServiceOrchestrator_HealthCheckDisabled(t *testing.T) {
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_test_create": true,
			"x_test_delete": true,
		},
	}

	registry := NewServiceCapabilityRegistry("/nonexistent", toolChecker)

	// Register test capability with health check disabled
	testCapability := &ServiceCapabilityDefinition{
		Name:        "test_no_health_service",
		Type:        "test_no_health_service",
		Version:     "1.0.0",
		Description: "Test service without health check",
		ServiceConfig: ServiceConfig{
			ServiceType:  "TestNoHealthService",
			DefaultLabel: "test-no-health",
			LifecycleTools: LifecycleTools{
				Create: ToolCall{Tool: "x_test_create", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
				Delete: ToolCall{Tool: "x_test_delete", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
			},
			HealthCheck: HealthCheckConfig{
				Enabled: false, // Health checks disabled
			},
			Timeout: TimeoutConfig{
				Create:      60 * time.Second,
				Delete:      30 * time.Second,
				HealthCheck: 10 * time.Second,
			},
		},
	}

	err := registry.RegisterDefinition(testCapability)
	require.NoError(t, err)

	toolCaller := newMockOrchestratorToolCaller()
	config := DefaultServiceOrchestratorConfig()
	config.HealthCheckInterval = 0 // Disable automatic health checks

	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = orchestrator.Start(ctx)
	require.NoError(t, err)
	defer orchestrator.Stop(context.Background())

	// Create a service instance
	req := CreateServiceRequest{
		CapabilityName: "test_no_health_service",
		Label:          "no-health-test",
		Parameters:     map[string]interface{}{},
	}

	info, err := orchestrator.CreateService(ctx, req)
	require.NoError(t, err)

	// Verify service was created
	assert.Equal(t, "no-health-test", info.Label)
	assert.Equal(t, "test_no_health_service", info.CapabilityName)
}

func TestServiceOrchestrator_ControlLoopMessages(t *testing.T) {
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_test_create": true,
			"x_test_delete": true,
		},
	}

	registry := NewServiceCapabilityRegistry("/nonexistent", toolChecker)
	toolCaller := newMockOrchestratorToolCaller()
	config := DefaultServiceOrchestratorConfig()
	config.HealthCheckInterval = 0 // Disable automatic health checks

	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := orchestrator.Start(ctx)
	require.NoError(t, err)
	defer orchestrator.Stop(context.Background())

	// Test control loop message sending
	select {
	case orchestrator.controlLoopChan <- controlLoopMessage{
		messageType: controlLoopHealthCheck,
		serviceID:   "non-existent-service",
	}:
		// Message sent successfully
	default:
		t.Fatal("Failed to send health check message")
	}

	select {
	case orchestrator.controlLoopChan <- controlLoopMessage{
		messageType: controlLoopServiceUpdate,
		serviceID:   "test-service",
	}:
		// Message sent successfully
	default:
		t.Fatal("Failed to send service update message")
	}

	// Verify control loop channel is working
	assert.NotNil(t, orchestrator.controlLoopChan)
}

func TestServiceOrchestrator_ConcurrentOperations(t *testing.T) {
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_test_create": true,
			"x_test_delete": true,
		},
	}

	registry := NewServiceCapabilityRegistry("/nonexistent", toolChecker)
	testCapability := &ServiceCapabilityDefinition{
		Name:        "concurrent_test_service",
		Type:        "concurrent_test_service",
		Version:     "1.0.0",
		Description: "Service for concurrent operation testing",
		ServiceConfig: ServiceConfig{
			ServiceType:  "ConcurrentTestService",
			DefaultLabel: "concurrent-test",
			LifecycleTools: LifecycleTools{
				Create: ToolCall{Tool: "x_test_create", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
				Delete: ToolCall{Tool: "x_test_delete", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
			},
			Timeout: TimeoutConfig{
				Create:      60 * time.Second,
				Delete:      30 * time.Second,
				HealthCheck: 10 * time.Second,
			},
		},
	}

	err := registry.RegisterDefinition(testCapability)
	require.NoError(t, err)

	toolCaller := newMockOrchestratorToolCaller()
	config := DefaultServiceOrchestratorConfig()
	config.MaxConcurrentOps = 5

	orchestrator := NewServiceOrchestrator(registry, toolCaller, config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = orchestrator.Start(ctx)
	require.NoError(t, err)
	defer orchestrator.Stop(context.Background())

	// Create multiple services concurrently
	numServices := 5
	var wg sync.WaitGroup
	serviceInfos := make([]*ServiceInstanceInfo, numServices)
	errors := make([]error, numServices)

	for i := 0; i < numServices; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			req := CreateServiceRequest{
				CapabilityName: "concurrent_test_service",
				Label:          fmt.Sprintf("concurrent-service-%d", index),
				Parameters:     map[string]interface{}{"index": index},
			}

			info, err := orchestrator.CreateService(ctx, req)
			serviceInfos[index] = info
			errors[index] = err
		}(i)
	}

	wg.Wait()

	// Verify all services were created successfully
	for i := 0; i < numServices; i++ {
		require.NoError(t, errors[i], "Service %d creation should succeed", i)
		require.NotNil(t, serviceInfos[i], "Service %d info should not be nil", i)
		assert.Equal(t, fmt.Sprintf("concurrent-service-%d", i), serviceInfos[i].Label)
	}

	// Verify all services exist
	allServices := orchestrator.ListServices()
	assert.Len(t, allServices, numServices)
}
