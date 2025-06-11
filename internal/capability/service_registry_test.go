package capability

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockToolChecker implements ToolAvailabilityChecker for testing
type mockServiceToolChecker struct {
	availableTools map[string]bool
}

func (m *mockServiceToolChecker) IsToolAvailable(toolName string) bool {
	return m.availableTools[toolName]
}

func (m *mockServiceToolChecker) GetAvailableTools() []string {
	tools := make([]string, 0, len(m.availableTools))
	for tool, available := range m.availableTools {
		if available {
			tools = append(tools, tool)
		}
	}
	return tools
}

func TestServiceCapabilityRegistry_LoadServiceDefinitions(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "service_registry_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test service capability definition
	testDef := `
name: test_service_provider
type: test_service_provider
version: "1.0.0"
description: "Test service capability"

serviceConfig:
  serviceType: "TestService"
  defaultLabel: "test-{{ .name }}"
  dependencies: []
  
  lifecycleTools:
    create:
      tool: "x_test_create"
      arguments:
        name: "{{ .name }}"
      responseMapping:
        serviceId: "$.id"
    
    delete:
      tool: "x_test_delete"
      arguments:
        id: "{{ .service_id }}"
      responseMapping:
        status: "$.status"
  
  healthCheck:
    enabled: true
    interval: "30s"
    failureThreshold: 3
    successThreshold: 1
  
  timeout:
    create: "60s"
    delete: "30s"
    healthCheck: "10s"
  
  createParameters:
    name:
      toolParameter: "name"
      required: true

operations:
  create_service:
    description: "Create test service"
    requires:
      - x_orchestrator_create
    workflow:
      name: test_workflow
      steps:
        - id: test_step
          tool: x_orchestrator_create
          args: {}

metadata:
  provider: "test"
`

	// Write test file
	testFile := filepath.Join(tempDir, "service_test.yaml")
	err = os.WriteFile(testFile, []byte(testDef), 0644)
	require.NoError(t, err)

	// Create tool checker with available tools
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_test_create":         true,
			"x_test_delete":         true,
			"x_orchestrator_create": true,
		},
	}

	// Create registry and load definitions
	registry := NewServiceCapabilityRegistry(tempDir, toolChecker)
	err = registry.LoadServiceDefinitions()
	require.NoError(t, err)

	// Verify definition was loaded
	def, exists := registry.GetServiceCapabilityDefinition("test_service_provider")
	assert.True(t, exists)
	assert.Equal(t, "test_service_provider", def.Name)
	assert.Equal(t, "TestService", def.ServiceConfig.ServiceType)

	// Verify availability
	assert.True(t, registry.IsServiceCapabilityAvailable("test_service_provider"))
}

func TestServiceCapabilityRegistry_ValidationErrors(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "service_registry_validation_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create invalid service capability definition (missing required fields)
	invalidDef := `
name: invalid_service
# Missing type, version, description, serviceConfig
operations: {}
`

	// Write invalid test file
	testFile := filepath.Join(tempDir, "service_invalid.yaml")
	err = os.WriteFile(testFile, []byte(invalidDef), 0644)
	require.NoError(t, err)

	// Create registry and load definitions
	toolChecker := &mockServiceToolChecker{availableTools: map[string]bool{}}
	registry := NewServiceCapabilityRegistry(tempDir, toolChecker)

	// Load should succeed but skip invalid files
	err = registry.LoadServiceDefinitions()
	require.NoError(t, err)

	// Verify invalid definition was not loaded
	_, exists := registry.GetServiceCapabilityDefinition("invalid_service")
	assert.False(t, exists)

	// Verify registry is empty
	capabilities := registry.ListServiceCapabilities()
	assert.Empty(t, capabilities)
}

func TestServiceCapabilityRegistry_ToolAvailability(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "service_registry_tools_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create service capability with multiple tools
	testDef := `
name: multi_tool_service
type: multi_tool_service
version: "1.0.0"
description: "Service with multiple tools"

serviceConfig:
  serviceType: "MultiToolService"
  defaultLabel: "multi-{{ .name }}"
  
  lifecycleTools:
    create:
      tool: "x_create_tool"
      arguments: {}
      responseMapping: {}
    
    delete:
      tool: "x_delete_tool"
      arguments: {}
      responseMapping: {}
      
    healthCheck:
      tool: "x_health_tool"
      arguments: {}
      responseMapping: {}
      
    status:
      tool: "x_status_tool"
      arguments: {}
      responseMapping: {}
  
  timeout:
    create: "60s"
    delete: "30s"
    healthCheck: "10s"

operations:
  test_op:
    description: "Test operation"
    requires:
      - x_required_tool
    workflow:
      name: test
      steps: []

metadata: {}
`

	testFile := filepath.Join(tempDir, "service_multi_tool.yaml")
	err = os.WriteFile(testFile, []byte(testDef), 0644)
	require.NoError(t, err)

	tests := []struct {
		name              string
		availableTools    map[string]bool
		expectedAvailable bool
	}{
		{
			name: "all tools available",
			availableTools: map[string]bool{
				"x_create_tool":   true,
				"x_delete_tool":   true,
				"x_health_tool":   true,
				"x_status_tool":   true,
				"x_required_tool": true,
			},
			expectedAvailable: true,
		},
		{
			name: "missing lifecycle tool",
			availableTools: map[string]bool{
				"x_create_tool":   true,
				"x_delete_tool":   false, // Missing
				"x_health_tool":   true,
				"x_status_tool":   true,
				"x_required_tool": true,
			},
			expectedAvailable: false,
		},
		{
			name: "missing operation tool",
			availableTools: map[string]bool{
				"x_create_tool":   true,
				"x_delete_tool":   true,
				"x_health_tool":   true,
				"x_status_tool":   true,
				"x_required_tool": false, // Missing
			},
			expectedAvailable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolChecker := &mockServiceToolChecker{availableTools: tt.availableTools}
			registry := NewServiceCapabilityRegistry(tempDir, toolChecker)

			err = registry.LoadServiceDefinitions()
			require.NoError(t, err)

			available := registry.IsServiceCapabilityAvailable("multi_tool_service")
			assert.Equal(t, tt.expectedAvailable, available)

			// Test capability info
			capabilities := registry.ListServiceCapabilities()
			require.Len(t, capabilities, 1)

			info := capabilities[0]
			assert.Equal(t, tt.expectedAvailable, info.Available)
			assert.Equal(t, tt.availableTools["x_create_tool"], info.CreateToolAvailable)
			assert.Equal(t, tt.availableTools["x_delete_tool"], info.DeleteToolAvailable)
			assert.Equal(t, tt.availableTools["x_health_tool"], info.HealthCheckToolAvailable)
			assert.Equal(t, tt.availableTools["x_status_tool"], info.StatusToolAvailable)
		})
	}
}

func TestServiceCapabilityRegistry_ProgrammaticRegistration(t *testing.T) {
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_test_create": true,
			"x_test_delete": true,
		},
	}

	registry := NewServiceCapabilityRegistry("/nonexistent", toolChecker)

	// Create valid service capability definition
	def := &ServiceCapabilityDefinition{
		Name:        "programmatic_service",
		Type:        "programmatic_service",
		Version:     "1.0.0",
		Description: "Programmatically registered service",
		ServiceConfig: ServiceConfig{
			ServiceType:  "ProgrammaticService",
			DefaultLabel: "prog-{{ .name }}",
			LifecycleTools: LifecycleTools{
				Create: ToolCall{
					Tool:            "x_test_create",
					Arguments:       map[string]interface{}{},
					ResponseMapping: ResponseMapping{},
				},
				Delete: ToolCall{
					Tool:            "x_test_delete",
					Arguments:       map[string]interface{}{},
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

	// Test registration
	err := registry.RegisterDefinition(def)
	assert.NoError(t, err)

	// Verify registration
	retrievedDef, exists := registry.GetServiceCapabilityDefinition("programmatic_service")
	assert.True(t, exists)
	assert.Equal(t, def.Name, retrievedDef.Name)

	// Test duplicate registration
	err = registry.RegisterDefinition(def)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")

	// Test unregistration
	err = registry.UnregisterDefinition("programmatic_service")
	assert.NoError(t, err)

	// Verify unregistration
	_, exists = registry.GetServiceCapabilityDefinition("programmatic_service")
	assert.False(t, exists)

	// Test unregistering non-existent
	err = registry.UnregisterDefinition("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestServiceCapabilityRegistry_RefreshAvailability(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "service_registry_refresh_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test definition
	testDef := `
name: refresh_test_service
type: refresh_test_service
version: "1.0.0"
description: "Service for refresh testing"

serviceConfig:
  serviceType: "RefreshTestService"
  defaultLabel: "refresh-test"
  
  lifecycleTools:
    create:
      tool: "x_dynamic_tool"
      arguments: {}
      responseMapping: {}
    delete:
      tool: "x_static_tool"
      arguments: {}
      responseMapping: {}
  
  timeout:
    create: "60s"
    delete: "30s"
    healthCheck: "10s"

operations: {}
metadata: {}
`

	testFile := filepath.Join(tempDir, "service_refresh_test.yaml")
	err = os.WriteFile(testFile, []byte(testDef), 0644)
	require.NoError(t, err)

	// Create tool checker with dynamic availability
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_static_tool":  true,
			"x_dynamic_tool": false, // Initially unavailable
		},
	}

	registry := NewServiceCapabilityRegistry(tempDir, toolChecker)
	err = registry.LoadServiceDefinitions()
	require.NoError(t, err)

	// Initially should be unavailable
	assert.False(t, registry.IsServiceCapabilityAvailable("refresh_test_service"))

	// Make tool available
	toolChecker.availableTools["x_dynamic_tool"] = true

	// Refresh availability
	registry.RefreshAvailability()

	// Should now be available
	assert.True(t, registry.IsServiceCapabilityAvailable("refresh_test_service"))
}

func TestServiceCapabilityRegistry_Callbacks(t *testing.T) {
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_test_tool": true,
		},
	}

	registry := NewServiceCapabilityRegistry("/nonexistent", toolChecker)

	// Test callbacks
	var registered *ServiceCapabilityDefinition
	var unregistered string

	registry.OnRegister(func(def *ServiceCapabilityDefinition) {
		registered = def
	})

	registry.OnUnregister(func(name string) {
		unregistered = name
	})

	// Create and register definition
	def := &ServiceCapabilityDefinition{
		Name:        "callback_test",
		Type:        "callback_test",
		Version:     "1.0.0",
		Description: "Callback test service",
		ServiceConfig: ServiceConfig{
			ServiceType:  "CallbackTest",
			DefaultLabel: "callback-test",
			LifecycleTools: LifecycleTools{
				Create: ToolCall{
					Tool:            "x_test_tool",
					Arguments:       map[string]interface{}{},
					ResponseMapping: ResponseMapping{},
				},
				Delete: ToolCall{
					Tool:            "x_test_tool",
					Arguments:       map[string]interface{}{},
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

	// Register - should trigger callback
	err := registry.RegisterDefinition(def)
	require.NoError(t, err)
	assert.NotNil(t, registered)
	assert.Equal(t, "callback_test", registered.Name)

	// Unregister - should trigger callback
	err = registry.UnregisterDefinition("callback_test")
	require.NoError(t, err)
	assert.Equal(t, "callback_test", unregistered)
}

func TestServiceCapabilityRegistry_FileDiscovery(t *testing.T) {
	// Create temporary directory structure
	tempDir, err := os.MkdirTemp("", "service_registry_discovery_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create subdirectory
	examplesDir := filepath.Join(tempDir, "examples")
	err = os.MkdirAll(examplesDir, 0755)
	require.NoError(t, err)

	// Create various files
	files := map[string]string{
		"service_test1.yaml": `name: service1
type: service1
version: "1.0.0"
description: "Service 1"
serviceConfig:
  serviceType: "Service1"
  defaultLabel: "service1"
  lifecycleTools:
    create:
      tool: "x_tool"
      arguments: {}
      responseMapping: {}
    delete:
      tool: "x_tool"
      arguments: {}
      responseMapping: {}
  timeout:
    create: "60s"
    delete: "30s"
    healthCheck: "10s"
operations: {}
metadata: {}`,

		"regular_capability.yaml": `name: regular
type: regular
version: "1.0.0"
description: "Regular capability"
operations:
  test:
    description: "Test"
    requires: []
    workflow:
      name: test
      steps: []
metadata: {}`,

		filepath.Join("examples", "service_test2.yaml"): `name: service2
type: service2
version: "1.0.0"
description: "Service 2"
serviceConfig:
  serviceType: "Service2"
  defaultLabel: "service2"
  lifecycleTools:
    create:
      tool: "x_tool"
      arguments: {}
      responseMapping: {}
    delete:
      tool: "x_tool"
      arguments: {}
      responseMapping: {}
  timeout:
    create: "60s"
    delete: "30s"
    healthCheck: "10s"
operations: {}
metadata: {}`,
	}

	// Write files
	for path, content := range files {
		fullPath := filepath.Join(tempDir, path)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Create registry
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_tool": true,
		},
	}

	registry := NewServiceCapabilityRegistry(tempDir, toolChecker)
	err = registry.LoadServiceDefinitions()
	require.NoError(t, err)

	// Should find both service capabilities but not regular capability
	capabilities := registry.ListServiceCapabilities()
	assert.Len(t, capabilities, 2)

	names := make([]string, len(capabilities))
	for i, cap := range capabilities {
		names[i] = cap.Name
	}

	assert.Contains(t, names, "service1")
	assert.Contains(t, names, "service2")
	assert.NotContains(t, names, "regular")
}

func TestServiceCapabilityRegistry_ListOperations(t *testing.T) {
	toolChecker := &mockServiceToolChecker{
		availableTools: map[string]bool{
			"x_available_tool":   true,
			"x_unavailable_tool": false,
		},
	}

	registry := NewServiceCapabilityRegistry("/nonexistent", toolChecker)

	// Register capabilities with different availability
	availableDef := &ServiceCapabilityDefinition{
		Name:        "available_service",
		Type:        "available_service",
		Version:     "1.0.0",
		Description: "Available service",
		ServiceConfig: ServiceConfig{
			ServiceType:  "AvailableService",
			DefaultLabel: "available",
			LifecycleTools: LifecycleTools{
				Create: ToolCall{Tool: "x_available_tool", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
				Delete: ToolCall{Tool: "x_available_tool", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
			},
			Timeout: TimeoutConfig{Create: 60 * time.Second, Delete: 30 * time.Second, HealthCheck: 10 * time.Second},
		},
	}

	unavailableDef := &ServiceCapabilityDefinition{
		Name:        "unavailable_service",
		Type:        "unavailable_service",
		Version:     "1.0.0",
		Description: "Unavailable service",
		ServiceConfig: ServiceConfig{
			ServiceType:  "UnavailableService",
			DefaultLabel: "unavailable",
			LifecycleTools: LifecycleTools{
				Create: ToolCall{Tool: "x_unavailable_tool", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
				Delete: ToolCall{Tool: "x_unavailable_tool", Arguments: map[string]interface{}{}, ResponseMapping: ResponseMapping{}},
			},
			Timeout: TimeoutConfig{Create: 60 * time.Second, Delete: 30 * time.Second, HealthCheck: 10 * time.Second},
		},
	}

	err := registry.RegisterDefinition(availableDef)
	require.NoError(t, err)
	err = registry.RegisterDefinition(unavailableDef)
	require.NoError(t, err)

	// Test list all
	all := registry.ListServiceCapabilities()
	assert.Len(t, all, 2)

	// Test list available only
	available := registry.ListAvailableServiceCapabilities()
	assert.Len(t, available, 1)
	assert.Equal(t, "available_service", available[0].Name)
	assert.True(t, available[0].Available)
}
