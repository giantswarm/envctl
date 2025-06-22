package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAggregatorInfo(t *testing.T) {
	tests := []struct {
		name          string
		setupRegistry func(*mockServiceRegistryHandler)
		expectedInfo  *AggregatorInfo
		expectedError string
	}{
		{
			name: "successful aggregator info retrieval",
			setupRegistry: func(registry *mockServiceRegistryHandler) {
				// Setup aggregator service
				aggService := &mockServiceInfo{
					name:    "aggregator",
					svcType: TypeAggregator,
					state:   StateRunning,
					health:  HealthHealthy,
					data: map[string]interface{}{
						"endpoint":          "http://localhost:8080/sse",
						"port":              8080,
						"host":              "localhost",
						"servers_total":     3,
						"servers_connected": 2,
						"tools":             15,
						"resources":         10,
						"prompts":           5,
						"blocked_tools":     3,
						"yolo":              false,
					},
				}
				registry.addService(aggService)
			},
			expectedInfo: &AggregatorInfo{
				Endpoint:         "http://localhost:8080/sse",
				Port:             8080,
				Host:             "localhost",
				State:            "running",
				Health:           "healthy",
				ServersTotal:     3,
				ServersConnected: 2,
				ToolsCount:       15,
				ResourcesCount:   10,
				PromptsCount:     5,
				BlockedTools:     3,
				YoloMode:         false,
			},
		},
		{
			name: "no aggregator found",
			setupRegistry: func(registry *mockServiceRegistryHandler) {
				// Don't add any aggregator service
			},
			expectedError: "no MCP aggregator found",
		},
		{
			name: "aggregator without service data",
			setupRegistry: func(registry *mockServiceRegistryHandler) {
				// Create a service with no data
				aggService := &mockServiceInfo{
					name:    "aggregator",
					svcType: TypeAggregator,
					state:   StateRunning,
					health:  HealthHealthy,
					data:    nil,
				}
				registry.addService(aggService)
			},
			expectedInfo: &AggregatorInfo{
				State:  "running",
				Health: "healthy",
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
			api := NewAggregatorAPI()

			// Call GetAggregatorInfo
			ctx := context.Background()
			info, err := api.GetAggregatorInfo(ctx)

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

func TestGetAllTools(t *testing.T) {
	tests := []struct {
		name          string
		setupRegistry func(*mockServiceRegistryHandler)
		expectedError string
		expectedTools []MCPTool
	}{
		{
			name: "no aggregator found",
			setupRegistry: func(registry *mockServiceRegistryHandler) {
				// Don't add any aggregator service
			},
			expectedError: "no MCP aggregator found",
		},
		{
			name: "aggregator not running",
			setupRegistry: func(registry *mockServiceRegistryHandler) {
				aggService := &mockServiceInfo{
					name:    "aggregator",
					svcType: TypeAggregator,
					state:   StateStopped,
					health:  HealthUnknown,
				}
				registry.addService(aggService)
			},
			expectedError: "MCP aggregator is not running",
		},
		{
			name: "aggregator without port",
			setupRegistry: func(registry *mockServiceRegistryHandler) {
				aggService := &mockServiceInfo{
					name:    "aggregator",
					svcType: TypeAggregator,
					state:   StateRunning,
					health:  HealthHealthy,
					data:    map[string]interface{}{}, // No port in data
				}
				registry.addService(aggService)
			},
			expectedError: "MCP aggregator has no port configured",
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
			api := NewAggregatorAPI()

			// Call GetAllTools
			ctx := context.Background()
			tools, err := api.GetAllTools(ctx)

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
