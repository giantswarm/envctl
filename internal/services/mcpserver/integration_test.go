package mcpserver

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/mcpserver"
	"envctl/internal/services"
	"errors"
	"sync"
	"testing"
	"time"
)

// mockStarter implements mcpServerStarter for testing
type mockStarter struct {
	startFunc func(config.MCPServerDefinition, mcpserver.McpUpdateFunc, *sync.WaitGroup) (int, chan struct{}, error)
}

func (m *mockStarter) StartAndManageIndividualMcpServer(
	serverConfig config.MCPServerDefinition,
	updateFn mcpserver.McpUpdateFunc,
	wg *sync.WaitGroup,
) (pid int, stopChan chan struct{}, initialError error) {
	if m.startFunc != nil {
		return m.startFunc(serverConfig, updateFn, wg)
	}
	return 0, nil, errors.New("mock not configured")
}

// TestMCPServerService_StartIntegration tests the Start method with a mock implementation
func TestMCPServerService_StartIntegration(t *testing.T) {
	tests := []struct {
		name           string
		config         config.MCPServerDefinition
		mockStart      func(config.MCPServerDefinition, mcpserver.McpUpdateFunc, *sync.WaitGroup) (int, chan struct{}, error)
		expectError    bool
		expectedState  services.ServiceState
		expectedHealth services.HealthStatus
	}{
		{
			name: "successful start with process updates",
			config: config.MCPServerDefinition{
				Name:    "test-server",
				Command: []string{"test", "command"},
			},
			mockStart: func(cfg config.MCPServerDefinition, updateFn mcpserver.McpUpdateFunc, wg *sync.WaitGroup) (int, chan struct{}, error) {
				// Simulate successful start
				stopChan := make(chan struct{})

				// Send process updates in a goroutine
				go func() {
					time.Sleep(10 * time.Millisecond)
					updateFn(mcpserver.McpDiscreteStatusUpdate{
						Label:         cfg.Name,
						ProcessStatus: "ProcessStarting",
						PID:           12345,
						ProxyPort:     0,
					})
					time.Sleep(10 * time.Millisecond)
					updateFn(mcpserver.McpDiscreteStatusUpdate{
						Label:         cfg.Name,
						ProcessStatus: "ProcessRunning",
						PID:           12345,
						ProxyPort:     8080,
					})
				}()

				return 12345, stopChan, nil
			},
			expectError:    false,
			expectedState:  services.StateRunning,
			expectedHealth: services.HealthHealthy,
		},
		{
			name: "start failure",
			config: config.MCPServerDefinition{
				Name:    "test-server",
				Command: []string{"test", "command"},
			},
			mockStart: func(cfg config.MCPServerDefinition, updateFn mcpserver.McpUpdateFunc, wg *sync.WaitGroup) (int, chan struct{}, error) {
				return 0, nil, errors.New("failed to start process")
			},
			expectError:    true,
			expectedState:  services.StateFailed,
			expectedHealth: services.HealthUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewMCPServerService(tt.config)

			// Replace the starter with our mock
			service.starter = &mockStarter{
				startFunc: tt.mockStart,
			}

			ctx := context.Background()

			// Start monitoring process updates
			go service.monitorProcess(ctx)

			err := service.Start(ctx)

			if (err != nil) != tt.expectError {
				t.Errorf("expected error: %v, got: %v", tt.expectError, err)
			}

			// Give time for state updates to propagate
			time.Sleep(50 * time.Millisecond)

			if service.GetState() != tt.expectedState {
				t.Errorf("expected state %s, got %s", tt.expectedState, service.GetState())
			}

			if service.GetHealth() != tt.expectedHealth {
				t.Errorf("expected health %s, got %s", tt.expectedHealth, service.GetHealth())
			}
		})
	}
}

// TestMCPServerService_RestartIntegration tests the full Restart method
func TestMCPServerService_RestartIntegration(t *testing.T) {
	// Mock the start function
	mockStartCalls := 0
	mockStart := func(cfg config.MCPServerDefinition, updateFn mcpserver.McpUpdateFunc, wg *sync.WaitGroup) (int, chan struct{}, error) {
		mockStartCalls++
		stopChan := make(chan struct{})

		// Simulate process starting
		go func() {
			updateFn(mcpserver.McpDiscreteStatusUpdate{
				Label:         cfg.Name,
				ProcessStatus: "ProcessRunning",
				PID:           12345 + mockStartCalls,
				ProxyPort:     8080,
			})
		}()

		return 12345 + mockStartCalls, stopChan, nil
	}

	service := NewMCPServerService(config.MCPServerDefinition{
		Name:    "test-server",
		Command: []string{"test", "command"},
	})

	// Replace the starter with our mock
	service.starter = &mockStarter{
		startFunc: mockStart,
	}

	ctx := context.Background()

	// Start monitoring
	go service.monitorProcess(ctx)

	// Start the service first
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	// Wait for it to be running
	time.Sleep(50 * time.Millisecond)

	if service.GetState() != services.StateRunning {
		t.Fatalf("service not running before restart, state: %s", service.GetState())
	}

	// Now restart
	err = service.Restart(ctx)
	if err != nil {
		t.Errorf("restart failed: %v", err)
	}

	// Verify the service went through stop and start again
	if mockStartCalls != 2 {
		t.Errorf("expected 2 start calls (initial + restart), got %d", mockStartCalls)
	}

	// The service should be running again
	time.Sleep(50 * time.Millisecond)
	if service.GetState() != services.StateRunning {
		t.Errorf("service not running after restart, state: %s", service.GetState())
	}
}

// TestMCPServerService_RestartWithStopError tests restart when stop fails
func TestMCPServerService_RestartWithStopError(t *testing.T) {
	service := NewMCPServerService(config.MCPServerDefinition{
		Name:    "test-server",
		Command: []string{"test", "command"},
	})

	// Create a context that will be cancelled to simulate stop error
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Set up the service as running
	service.mu.Lock()
	service.pid = 12345
	service.port = 8080
	service.stopChan = make(chan struct{})
	service.mu.Unlock()
	service.UpdateState(services.StateRunning, services.HealthHealthy, nil)

	// Wait for context to timeout
	time.Sleep(5 * time.Millisecond)

	// Try to restart with the cancelled context
	err := service.Restart(ctx)
	if err == nil {
		t.Error("expected error from restart with cancelled context, got nil")
	}

	// The error should be context related
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context deadline exceeded error, got: %v", err)
	}
}
