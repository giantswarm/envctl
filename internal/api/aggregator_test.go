package api

import (
	"context"
	"envctl/internal/services"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetAggregatorInfo(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*MockServiceRegistry, *MockServiceDataProvider)
		expectedInfo  *AggregatorInfo
		expectedError string
	}{
		{
			name: "successful aggregator info retrieval",
			setupMocks: func(registry *MockServiceRegistry, service *MockServiceDataProvider) {
				// Setup aggregator service
				registry.On("GetByType", services.ServiceType("Aggregator")).Return([]services.Service{service})
				service.On("GetHealth").Return(services.HealthHealthy)
				service.On("GetServiceData").Return(map[string]interface{}{
					"endpoint":          "http://localhost:8080/sse",
					"port":              8080,
					"host":              "localhost",
					"servers_total":     3,
					"servers_connected": 2,
					"tools":             15,
					"resources":         10,
					"prompts":           5,
				})
			},
			expectedInfo: &AggregatorInfo{
				Endpoint:         "http://localhost:8080/sse",
				Port:             8080,
				Host:             "localhost",
				Health:           "Healthy",
				ServersTotal:     3,
				ServersConnected: 2,
				ToolsCount:       15,
				ResourcesCount:   10,
				PromptsCount:     5,
			},
		},
		{
			name: "no aggregator found",
			setupMocks: func(registry *MockServiceRegistry, service *MockServiceDataProvider) {
				registry.On("GetByType", services.ServiceType("Aggregator")).Return([]services.Service{})
			},
			expectedError: "no MCP aggregator found",
		},
		{
			name: "aggregator without service data",
			setupMocks: func(registry *MockServiceRegistry, service *MockServiceDataProvider) {
				// Create a mock service that doesn't implement ServiceDataProvider
				basicService := new(MockService)
				basicService.On("GetHealth").Return(services.HealthHealthy)
				registry.On("GetByType", services.ServiceType("Aggregator")).Return([]services.Service{basicService})
			},
			expectedInfo: &AggregatorInfo{
				Health: "Healthy",
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
			api := NewAggregatorAPI(mockRegistry)

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

			// Verify mocks
			mockRegistry.AssertExpectations(t)
			mockService.AssertExpectations(t)
		})
	}
}

func TestGetAllTools(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*MockServiceRegistry, *MockServiceDataProvider)
		expectedError string
		expectedTools []MCPTool
	}{
		{
			name: "no aggregator found",
			setupMocks: func(registry *MockServiceRegistry, service *MockServiceDataProvider) {
				registry.On("GetByType", services.ServiceType("Aggregator")).Return([]services.Service{})
			},
			expectedError: "no MCP aggregator found",
		},
		{
			name: "aggregator not running",
			setupMocks: func(registry *MockServiceRegistry, service *MockServiceDataProvider) {
				registry.On("GetByType", services.ServiceType("Aggregator")).Return([]services.Service{service})
				service.On("GetState").Return(services.StateStopped)
			},
			expectedError: "MCP aggregator is not running",
		},
		{
			name: "aggregator without port",
			setupMocks: func(registry *MockServiceRegistry, service *MockServiceDataProvider) {
				registry.On("GetByType", services.ServiceType("Aggregator")).Return([]services.Service{service})
				service.On("GetState").Return(services.StateRunning)
				service.On("GetServiceData").Return(map[string]interface{}{})
			},
			expectedError: "MCP aggregator has no port configured",
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
			api := NewAggregatorAPI(mockRegistry)

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

			// Verify mocks
			mockRegistry.AssertExpectations(t)
			mockService.AssertExpectations(t)
		})
	}
}
