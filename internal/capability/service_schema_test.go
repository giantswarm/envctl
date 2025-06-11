package capability

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestServiceCapabilityDefinitionValidation(t *testing.T) {
	tests := []struct {
		name        string
		definition  ServiceCapabilityDefinition
		expectError bool
		errorFields []string
	}{
		{
			name: "valid service capability definition",
			definition: ServiceCapabilityDefinition{
				Name:        "test_service",
				Type:        "test_type",
				Version:     "1.0.0",
				Description: "Test service capability",
				ServiceConfig: ServiceConfig{
					ServiceType:  "TestService",
					DefaultLabel: "test-{{ .param }}",
					LifecycleTools: LifecycleTools{
						Create: ToolCall{
							Tool:      "x_test_create",
							Arguments: map[string]interface{}{"param": "{{ .param }}"},
							ResponseMapping: ResponseMapping{
								ServiceID: "$.id",
							},
						},
						Delete: ToolCall{
							Tool:            "x_test_delete",
							Arguments:       map[string]interface{}{"id": "{{ .service_id }}"},
							ResponseMapping: ResponseMapping{},
						},
					},
					HealthCheck: HealthCheckConfig{
						Enabled:          true,
						Interval:         30 * time.Second,
						FailureThreshold: 3,
						SuccessThreshold: 1,
					},
					Timeout: TimeoutConfig{
						Create:      60 * time.Second,
						Delete:      30 * time.Second,
						HealthCheck: 10 * time.Second,
					},
					CreateParameters: map[string]ParameterMapping{
						"param": {
							ToolParameter: "param",
							Required:      true,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "missing required fields",
			definition: ServiceCapabilityDefinition{
				// Missing name, type, version, description
				ServiceConfig: ServiceConfig{
					// Missing serviceType, defaultLabel
					LifecycleTools: LifecycleTools{
						// Missing create and delete tools
					},
				},
			},
			expectError: true,
			errorFields: []string{"name", "type", "version", "description", "serviceConfig.serviceType", "serviceConfig.defaultLabel"},
		},
		{
			name: "invalid tool names",
			definition: ServiceCapabilityDefinition{
				Name:        "test_service",
				Type:        "test_type",
				Version:     "1.0.0",
				Description: "Test service capability",
				ServiceConfig: ServiceConfig{
					ServiceType:  "TestService",
					DefaultLabel: "test-label",
					LifecycleTools: LifecycleTools{
						Create: ToolCall{
							Tool:            "invalid_tool_name", // Should start with x_
							Arguments:       map[string]interface{}{},
							ResponseMapping: ResponseMapping{},
						},
						Delete: ToolCall{
							Tool:            "another_invalid_tool", // Should start with x_
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
			},
			expectError: true,
			errorFields: []string{"serviceConfig.lifecycleTools.create.tool", "serviceConfig.lifecycleTools.delete.tool"},
		},
		{
			name: "invalid timeout values",
			definition: ServiceCapabilityDefinition{
				Name:        "test_service",
				Type:        "test_type",
				Version:     "1.0.0",
				Description: "Test service capability",
				ServiceConfig: ServiceConfig{
					ServiceType:  "TestService",
					DefaultLabel: "test-label",
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
						Create:      -1 * time.Second, // Invalid negative timeout
						Delete:      0,                // Invalid zero timeout
						HealthCheck: 15 * time.Minute, // Exceeds maximum
					},
				},
			},
			expectError: true,
			errorFields: []string{"serviceConfig.timeout.create", "serviceConfig.timeout.delete", "serviceConfig.timeout.healthCheck"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateServiceCapabilityDefinition(&tt.definition)

			if tt.expectError {
				require.Error(t, err)

				// Check that expected error fields are present
				for _, field := range tt.errorFields {
					assert.Contains(t, err.Error(), field, "Expected error to mention field %s", field)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestServiceInstanceState(t *testing.T) {
	state := NewServiceInstanceState()

	// Test creating instances
	instance1 := state.CreateInstance("id1", "label1", "cap1", "type1", map[string]interface{}{"param": "value"})
	assert.Equal(t, "id1", instance1.ID)
	assert.Equal(t, "label1", instance1.Label)
	assert.Equal(t, "cap1", instance1.CapabilityName)
	assert.Equal(t, "type1", instance1.CapabilityType)

	// Test retrieving instances
	retrieved, exists := state.GetInstance("id1")
	assert.True(t, exists)
	assert.Equal(t, instance1, retrieved)

	retrievedByLabel, exists := state.GetInstanceByLabel("label1")
	assert.True(t, exists)
	assert.Equal(t, instance1, retrievedByLabel)

	// Test non-existent instance
	_, exists = state.GetInstance("nonexistent")
	assert.False(t, exists)

	// Test updating instance state
	state.UpdateInstanceState("id1", "Running", "Healthy", nil)
	updated, _ := state.GetInstance("id1")
	assert.Equal(t, "Running", string(updated.State))
	assert.Equal(t, "Healthy", string(updated.Health))
	assert.Empty(t, updated.LastError)

	// Test listing instances
	instances := state.ListInstances()
	assert.Len(t, instances, 1)
	assert.Equal(t, instance1, instances[0])

	// Test deleting instances
	state.DeleteInstance("id1")
	_, exists = state.GetInstance("id1")
	assert.False(t, exists)

	instances = state.ListInstances()
	assert.Len(t, instances, 0)
}

func TestParseServiceCapabilityFromYAML(t *testing.T) {
	yamlContent := `
name: test_service_provider
type: test_service_provider
version: "1.0.0"
description: "Test service capability for unit testing"

serviceConfig:
  serviceType: "TestService"
  defaultLabel: "test-{{ .name }}"
  dependencies: []
  
  lifecycleTools:
    create:
      tool: "x_test_create"
      arguments:
        name: "{{ .name }}"
        port: "{{ .port }}"
      responseMapping:
        serviceId: "$.id"
        status: "$.status"
    
    delete:
      tool: "x_test_delete"
      arguments:
        id: "{{ .service_id }}"
      responseMapping:
        status: "$.status"
        
    healthCheck:
      tool: "x_test_health"
      arguments:
        id: "{{ .service_id }}"
      responseMapping:
        health: "$.healthy"
  
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
    port:
      toolParameter: "port"
      required: true

operations:
  create_service:
    description: "Create a new test service instance"
    parameters:
      name:
        type: string
        required: true
        description: "Service name"
      port:
        type: number
        required: true
        description: "Service port"
    requires:
      - x_service_orchestrator_create_service
    workflow:
      name: create_test_service
      description: "Create test service"
      steps:
        - id: create_service
          tool: x_service_orchestrator_create_service
          args:
            capability_name: "test_service_provider"
            label: "test-{{ .name }}"
            parameters:
              name: "{{ .name }}"
              port: "{{ .port }}"

metadata:
  provider: "test"
  category: "testing"
  icon: "🧪"
`

	var def ServiceCapabilityDefinition
	err := yaml.Unmarshal([]byte(yamlContent), &def)
	require.NoError(t, err)

	// Validate the parsed definition
	err = ValidateServiceCapabilityDefinition(&def)
	assert.NoError(t, err)

	// Check parsed values
	assert.Equal(t, "test_service_provider", def.Name)
	assert.Equal(t, "test_service_provider", def.Type)
	assert.Equal(t, "1.0.0", def.Version)
	assert.Equal(t, "TestService", def.ServiceConfig.ServiceType)
	assert.Equal(t, "test-{{ .name }}", def.ServiceConfig.DefaultLabel)

	// Check lifecycle tools
	assert.Equal(t, "x_test_create", def.ServiceConfig.LifecycleTools.Create.Tool)
	assert.Equal(t, "x_test_delete", def.ServiceConfig.LifecycleTools.Delete.Tool)
	assert.NotNil(t, def.ServiceConfig.LifecycleTools.HealthCheck)
	assert.Equal(t, "x_test_health", def.ServiceConfig.LifecycleTools.HealthCheck.Tool)

	// Check health check config
	assert.True(t, def.ServiceConfig.HealthCheck.Enabled)
	assert.Equal(t, 30*time.Second, def.ServiceConfig.HealthCheck.Interval)
	assert.Equal(t, 3, def.ServiceConfig.HealthCheck.FailureThreshold)
	assert.Equal(t, 1, def.ServiceConfig.HealthCheck.SuccessThreshold)

	// Check timeout config
	assert.Equal(t, 60*time.Second, def.ServiceConfig.Timeout.Create)
	assert.Equal(t, 30*time.Second, def.ServiceConfig.Timeout.Delete)
	assert.Equal(t, 10*time.Second, def.ServiceConfig.Timeout.HealthCheck)

	// Check create parameters
	assert.Len(t, def.ServiceConfig.CreateParameters, 2)
	assert.True(t, def.ServiceConfig.CreateParameters["name"].Required)
	assert.True(t, def.ServiceConfig.CreateParameters["port"].Required)

	// Check operations
	assert.Len(t, def.Operations, 1)
	createOp := def.Operations["create_service"]
	assert.Equal(t, "Create a new test service instance", createOp.Description)
}

func TestHealthCheckConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      HealthCheckConfig
		expectError bool
	}{
		{
			name: "valid enabled health check",
			config: HealthCheckConfig{
				Enabled:          true,
				Interval:         30 * time.Second,
				FailureThreshold: 3,
				SuccessThreshold: 1,
			},
			expectError: false,
		},
		{
			name: "valid disabled health check",
			config: HealthCheckConfig{
				Enabled: false,
				// Other fields don't matter when disabled
			},
			expectError: false,
		},
		{
			name: "invalid enabled health check - negative interval",
			config: HealthCheckConfig{
				Enabled:          true,
				Interval:         -1 * time.Second,
				FailureThreshold: 3,
				SuccessThreshold: 1,
			},
			expectError: true,
		},
		{
			name: "invalid enabled health check - zero thresholds",
			config: HealthCheckConfig{
				Enabled:          true,
				Interval:         30 * time.Second,
				FailureThreshold: 0,
				SuccessThreshold: 0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHealthCheckConfig(&tt.config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTimeoutConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      TimeoutConfig
		expectError bool
	}{
		{
			name: "valid timeout config",
			config: TimeoutConfig{
				Create:      60 * time.Second,
				Delete:      30 * time.Second,
				HealthCheck: 10 * time.Second,
			},
			expectError: false,
		},
		{
			name: "invalid timeout config - negative values",
			config: TimeoutConfig{
				Create:      -1 * time.Second,
				Delete:      -1 * time.Second,
				HealthCheck: -1 * time.Second,
			},
			expectError: true,
		},
		{
			name: "invalid timeout config - zero values",
			config: TimeoutConfig{
				Create:      0,
				Delete:      0,
				HealthCheck: 0,
			},
			expectError: true,
		},
		{
			name: "invalid timeout config - exceeds maximum",
			config: TimeoutConfig{
				Create:      15 * time.Minute,
				Delete:      15 * time.Minute,
				HealthCheck: 15 * time.Minute,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTimeoutConfig(&tt.config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
