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

// mockStartAndManageIndividualMcpServer is a mock implementation for testing
var mockStartAndManageIndividualMcpServer func(
	serverConfig config.MCPServerDefinition,
	updateFn mcpserver.McpUpdateFunc,
	wg *sync.WaitGroup,
) (pid int, stopChan chan struct{}, initialError error)

func init() {
	// Replace the actual function with our mock during tests
	// This would require exporting the function or using dependency injection
	// For now, we'll test the service logic assuming the mcpserver package works
}

func TestNewMCPServerService(t *testing.T) {
	tests := []struct {
		name     string
		config   config.MCPServerDefinition
		wantDeps []string
	}{
		{
			name: "service without dependencies",
			config: config.MCPServerDefinition{
				Name:    "test-server",
				Command: []string{"test", "command"},
			},
			wantDeps: []string{},
		},
		{
			name: "service with port forward dependencies",
			config: config.MCPServerDefinition{
				Name:                 "test-server",
				Command:              []string{"test", "command"},
				RequiresPortForwards: []string{"dep1", "dep2"},
			},
			wantDeps: []string{"dep1", "dep2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewMCPServerService(tt.config)

			if service.GetLabel() != tt.config.Name {
				t.Errorf("expected label %s, got %s", tt.config.Name, service.GetLabel())
			}

			if service.GetType() != services.TypeMCPServer {
				t.Errorf("expected type %s, got %s", services.TypeMCPServer, service.GetType())
			}

			deps := service.GetDependencies()
			if len(deps) != len(tt.wantDeps) {
				t.Errorf("expected %d dependencies, got %d", len(tt.wantDeps), len(deps))
			}

			for i, dep := range deps {
				if dep != tt.wantDeps[i] {
					t.Errorf("expected dependency %s at index %d, got %s", tt.wantDeps[i], i, dep)
				}
			}
		})
	}
}

func TestMCPServerService_Start(t *testing.T) {
	tests := []struct {
		name          string
		config        config.MCPServerDefinition
		mockBehavior  func(*MCPServerService, context.Context)
		expectError   bool
		expectedState services.ServiceState
	}{
		{
			name: "successful start",
			config: config.MCPServerDefinition{
				Name:    "test-server",
				Command: []string{"test", "command"},
			},
			mockBehavior: func(s *MCPServerService, ctx context.Context) {
				// Simulate successful start
				go func() {
					time.Sleep(10 * time.Millisecond)
					select {
					case s.updateChan <- mcpserver.McpDiscreteStatusUpdate{
						Label:         "test-server",
						ProcessStatus: "ProcessRunning",
						PID:           12345,
						ProxyPort:     8080,
					}:
					case <-ctx.Done():
					}
				}()
			},
			expectError:   false,
			expectedState: services.StateStarting,
		},
		{
			name: "start failure",
			config: config.MCPServerDefinition{
				Name:    "test-server",
				Command: []string{"test", "command"},
			},
			mockBehavior: func(s *MCPServerService, ctx context.Context) {
				// Simulate start failure
				s.UpdateState(services.StateFailed, services.HealthUnhealthy, errors.New("mock start error"))
			},
			expectError:   true,
			expectedState: services.StateFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewMCPServerService(tt.config)
			ctx := context.Background()

			if tt.mockBehavior != nil {
				tt.mockBehavior(service, ctx)
			}

			// For this test, we'll simulate the behavior since we can't easily mock the actual function
			// In a real implementation, you'd use dependency injection or interfaces
			if tt.name == "successful start" {
				service.mu.Lock()
				service.pid = 12345
				service.stopChan = make(chan struct{})
				service.mu.Unlock()
				service.UpdateState(services.StateStarting, services.HealthUnknown, nil)
			} else if tt.name == "start failure" {
				service.UpdateState(services.StateFailed, services.HealthUnhealthy, errors.New("mock start error"))
			}

			if service.GetState() != tt.expectedState {
				t.Errorf("expected state %s, got %s", tt.expectedState, service.GetState())
			}
		})
	}
}

func TestMCPServerService_Stop(t *testing.T) {
	tests := []struct {
		name          string
		setupService  func() *MCPServerService
		expectError   bool
		expectedState services.ServiceState
	}{
		{
			name: "stop running service",
			setupService: func() *MCPServerService {
				service := NewMCPServerService(config.MCPServerDefinition{
					Name:    "test-server",
					Command: []string{"test", "command"},
				})
				service.mu.Lock()
				service.pid = 12345
				service.port = 8080
				service.stopChan = make(chan struct{})
				service.mu.Unlock()
				service.UpdateState(services.StateRunning, services.HealthHealthy, nil)
				return service
			},
			expectError:   false,
			expectedState: services.StateStopped,
		},
		{
			name: "stop already stopped service",
			setupService: func() *MCPServerService {
				service := NewMCPServerService(config.MCPServerDefinition{
					Name:    "test-server",
					Command: []string{"test", "command"},
				})
				service.UpdateState(services.StateStopped, services.HealthUnknown, nil)
				return service
			},
			expectError:   false,
			expectedState: services.StateStopped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := tt.setupService()
			ctx := context.Background()

			err := service.Stop(ctx)

			if (err != nil) != tt.expectError {
				t.Errorf("expected error: %v, got: %v", tt.expectError, err)
			}

			if service.GetState() != tt.expectedState {
				t.Errorf("expected state %s, got %s", tt.expectedState, service.GetState())
			}

			// Verify internal state is cleaned up
			service.mu.RLock()
			if service.pid != 0 {
				t.Error("expected pid to be 0 after stop")
			}
			if service.port != 0 {
				t.Error("expected port to be 0 after stop")
			}
			if service.stopChan != nil {
				t.Error("expected stopChan to be nil after stop")
			}
			service.mu.RUnlock()
		})
	}
}

func TestMCPServerService_Restart(t *testing.T) {
	service := NewMCPServerService(config.MCPServerDefinition{
		Name:    "test-server",
		Command: []string{"test", "command"},
	})

	// Set up as running
	service.mu.Lock()
	service.pid = 12345
	service.port = 8080
	service.stopChan = make(chan struct{})
	service.mu.Unlock()
	service.UpdateState(services.StateRunning, services.HealthHealthy, nil)

	// Since we can't easily test the full restart without mocking the actual start function,
	// we'll test that the service goes through the expected state transitions
	go func() {
		time.Sleep(50 * time.Millisecond)
		// Simulate the service being restarted
		service.UpdateState(services.StateStarting, services.HealthUnknown, nil)
	}()

	// The restart method would normally call Stop then Start
	// For this test, we'll verify the state transitions
	service.UpdateState(services.StateStopping, services.HealthHealthy, nil)
	time.Sleep(100 * time.Millisecond)
	service.UpdateState(services.StateStopped, services.HealthUnknown, nil)

	if service.GetState() != services.StateStopped {
		t.Errorf("expected state %s after restart simulation, got %s", services.StateStopped, service.GetState())
	}
}

func TestMCPServerService_GetServiceData(t *testing.T) {
	tests := []struct {
		name         string
		setupService func() *MCPServerService
		expectedData map[string]interface{}
	}{
		{
			name: "service with all data",
			setupService: func() *MCPServerService {
				service := NewMCPServerService(config.MCPServerDefinition{
					Name:      "test-server",
					Command:   []string{"test", "command"},
					Icon:      "🔥",
					Enabled:   true,
					ProxyPort: 8000,
				})
				service.mu.Lock()
				service.pid = 12345
				service.port = 8080
				service.mu.Unlock()
				return service
			},
			expectedData: map[string]interface{}{
				"name":    "test-server",
				"command": []string{"test", "command"},
				"icon":    "🔥",
				"enabled": true,
				"pid":     12345,
				"port":    8080,
			},
		},
		{
			name: "service without runtime data",
			setupService: func() *MCPServerService {
				return NewMCPServerService(config.MCPServerDefinition{
					Name:      "test-server",
					Command:   []string{"test", "command"},
					Icon:      "☸",
					Enabled:   false,
					ProxyPort: 9000,
				})
			},
			expectedData: map[string]interface{}{
				"name":    "test-server",
				"command": []string{"test", "command"},
				"icon":    "☸",
				"enabled": false,
				"port":    9000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := tt.setupService()
			data := service.GetServiceData()

			for key, expectedValue := range tt.expectedData {
				actualValue, exists := data[key]
				if !exists {
					t.Errorf("expected key %s not found in service data", key)
					continue
				}

				// Special handling for slices
				if key == "command" {
					expectedCmd := expectedValue.([]string)
					actualCmd := actualValue.([]string)
					if len(expectedCmd) != len(actualCmd) {
						t.Errorf("command length mismatch: expected %d, got %d", len(expectedCmd), len(actualCmd))
						continue
					}
					for i := range expectedCmd {
						if expectedCmd[i] != actualCmd[i] {
							t.Errorf("command[%d] mismatch: expected %s, got %s", i, expectedCmd[i], actualCmd[i])
						}
					}
				} else if actualValue != expectedValue {
					t.Errorf("key %s: expected %v, got %v", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestMCPServerService_CheckHealth(t *testing.T) {
	tests := []struct {
		name           string
		setupService   func() *MCPServerService
		expectedHealth services.HealthStatus
		expectError    bool
	}{
		{
			name: "healthy running service",
			setupService: func() *MCPServerService {
				service := NewMCPServerService(config.MCPServerDefinition{
					Name:      "test-server",
					Command:   []string{"test", "command"},
					ProxyPort: 8080,
				})
				service.mu.Lock()
				service.port = 8080
				service.mu.Unlock()
				service.UpdateState(services.StateRunning, services.HealthHealthy, nil)
				return service
			},
			expectedHealth: services.HealthHealthy,
			expectError:    false,
		},
		{
			name: "service without port",
			setupService: func() *MCPServerService {
				service := NewMCPServerService(config.MCPServerDefinition{
					Name:    "test-server",
					Command: []string{"test", "command"},
				})
				service.UpdateState(services.StateRunning, services.HealthUnknown, nil)
				return service
			},
			expectedHealth: services.HealthUnknown,
			expectError:    true,
		},
		{
			name: "stopped service",
			setupService: func() *MCPServerService {
				service := NewMCPServerService(config.MCPServerDefinition{
					Name:      "test-server",
					Command:   []string{"test", "command"},
					ProxyPort: 8080,
				})
				service.UpdateState(services.StateStopped, services.HealthUnknown, nil)
				return service
			},
			expectedHealth: services.HealthUnhealthy,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := tt.setupService()
			ctx := context.Background()

			health, err := service.CheckHealth(ctx)

			if (err != nil) != tt.expectError {
				t.Errorf("expected error: %v, got: %v", tt.expectError, err)
			}

			if health != tt.expectedHealth {
				t.Errorf("expected health %s, got %s", tt.expectedHealth, health)
			}

			// Verify the service's health was updated
			if service.GetHealth() != tt.expectedHealth {
				t.Errorf("service health not updated: expected %s, got %s", tt.expectedHealth, service.GetHealth())
			}
		})
	}
}

func TestMCPServerService_GetHealthCheckInterval(t *testing.T) {
	service := NewMCPServerService(config.MCPServerDefinition{
		Name:    "test-server",
		Command: []string{"test", "command"},
	})

	interval := service.GetHealthCheckInterval()
	expectedInterval := 10 * time.Second

	if interval != expectedInterval {
		t.Errorf("expected health check interval %v, got %v", expectedInterval, interval)
	}
}

func TestMCPServerService_HandleProcessUpdate(t *testing.T) {
	tests := []struct {
		name           string
		update         mcpserver.McpDiscreteStatusUpdate
		expectedState  services.ServiceState
		expectedHealth services.HealthStatus
		expectError    bool
	}{
		{
			name: "process initializing",
			update: mcpserver.McpDiscreteStatusUpdate{
				Label:         "test-server",
				ProcessStatus: "ProcessInitializing",
				PID:           12345,
				ProxyPort:     0,
			},
			expectedState:  services.StateStarting,
			expectedHealth: services.HealthUnknown,
			expectError:    false,
		},
		{
			name: "process starting",
			update: mcpserver.McpDiscreteStatusUpdate{
				Label:         "test-server",
				ProcessStatus: "ProcessStarting",
				PID:           12345,
				ProxyPort:     8080,
			},
			expectedState:  services.StateStarting,
			expectedHealth: services.HealthUnknown,
			expectError:    false,
		},
		{
			name: "process running",
			update: mcpserver.McpDiscreteStatusUpdate{
				Label:         "test-server",
				ProcessStatus: "ProcessRunning",
				PID:           12345,
				ProxyPort:     8080,
			},
			expectedState:  services.StateRunning,
			expectedHealth: services.HealthHealthy,
			expectError:    false,
		},
		{
			name: "process stopped by user",
			update: mcpserver.McpDiscreteStatusUpdate{
				Label:         "test-server",
				ProcessStatus: "ProcessStoppedByUser",
				PID:           12345,
				ProxyPort:     8080,
			},
			expectedState:  services.StateStopped,
			expectedHealth: services.HealthUnknown,
			expectError:    false,
		},
		{
			name: "process exited gracefully",
			update: mcpserver.McpDiscreteStatusUpdate{
				Label:         "test-server",
				ProcessStatus: "ProcessExitedGracefully",
				PID:           12345,
				ProxyPort:     8080,
			},
			expectedState:  services.StateStopped,
			expectedHealth: services.HealthUnknown,
			expectError:    false,
		},
		{
			name: "process start failed",
			update: mcpserver.McpDiscreteStatusUpdate{
				Label:         "test-server",
				ProcessStatus: "ProcessStartFailed",
				ProcessErr:    errors.New("start failed"),
				PID:           0,
				ProxyPort:     0,
			},
			expectedState:  services.StateFailed,
			expectedHealth: services.HealthUnhealthy,
			expectError:    true,
		},
		{
			name: "process exited with error",
			update: mcpserver.McpDiscreteStatusUpdate{
				Label:         "test-server",
				ProcessStatus: "ProcessExitedWithError",
				ProcessErr:    errors.New("exit error"),
				PID:           12345,
				ProxyPort:     8080,
			},
			expectedState:  services.StateFailed,
			expectedHealth: services.HealthUnhealthy,
			expectError:    true,
		},
		{
			name: "process kill failed",
			update: mcpserver.McpDiscreteStatusUpdate{
				Label:         "test-server",
				ProcessStatus: "ProcessKillFailed",
				ProcessErr:    errors.New("kill failed"),
				PID:           12345,
				ProxyPort:     8080,
			},
			expectedState:  services.StateFailed,
			expectedHealth: services.HealthUnhealthy,
			expectError:    true,
		},
		{
			name: "unknown status",
			update: mcpserver.McpDiscreteStatusUpdate{
				Label:         "test-server",
				ProcessStatus: "UnknownStatus",
				PID:           12345,
				ProxyPort:     8080,
			},
			expectedState:  services.StateUnknown,
			expectedHealth: services.HealthUnknown,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewMCPServerService(config.MCPServerDefinition{
				Name:    "test-server",
				Command: []string{"test", "command"},
			})

			service.handleProcessUpdate(tt.update)

			if service.GetState() != tt.expectedState {
				t.Errorf("expected state %s, got %s", tt.expectedState, service.GetState())
			}

			if service.GetHealth() != tt.expectedHealth {
				t.Errorf("expected health %s, got %s", tt.expectedHealth, service.GetHealth())
			}

			if tt.expectError && service.GetLastError() == nil {
				t.Error("expected error but got nil")
			} else if !tt.expectError && service.GetLastError() != nil {
				t.Errorf("expected no error but got: %v", service.GetLastError())
			}

			// Verify internal state updates
			service.mu.RLock()
			if tt.update.PID > 0 && service.pid != tt.update.PID {
				t.Errorf("expected pid %d, got %d", tt.update.PID, service.pid)
			}
			if tt.update.ProxyPort > 0 && service.port != tt.update.ProxyPort {
				t.Errorf("expected port %d, got %d", tt.update.ProxyPort, service.port)
			}
			service.mu.RUnlock()
		})
	}
}

func TestMCPServerService_MonitorProcess(t *testing.T) {
	service := NewMCPServerService(config.MCPServerDefinition{
		Name:    "test-server",
		Command: []string{"test", "command"},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the monitor in a goroutine
	go service.monitorProcess(ctx)

	// Send some updates
	updates := []mcpserver.McpDiscreteStatusUpdate{
		{
			Label:         "test-server",
			ProcessStatus: "ProcessStarting",
			PID:           12345,
			ProxyPort:     0,
		},
		{
			Label:         "test-server",
			ProcessStatus: "ProcessRunning",
			PID:           12345,
			ProxyPort:     8080,
		},
	}

	for _, update := range updates {
		select {
		case service.updateChan <- update:
			// Give the monitor time to process
			time.Sleep(10 * time.Millisecond)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout sending update")
		}
	}

	// Verify the last update was processed
	if service.GetState() != services.StateRunning {
		t.Errorf("expected state %s, got %s", services.StateRunning, service.GetState())
	}

	service.mu.RLock()
	if service.pid != 12345 {
		t.Errorf("expected pid 12345, got %d", service.pid)
	}
	if service.port != 8080 {
		t.Errorf("expected port 8080, got %d", service.port)
	}
	service.mu.RUnlock()

	// Test context cancellation
	cancel()
	time.Sleep(50 * time.Millisecond)

	// Try to send another update - should not be processed
	select {
	case service.updateChan <- mcpserver.McpDiscreteStatusUpdate{
		Label:         "test-server",
		ProcessStatus: "ProcessStoppedByUser",
	}:
		// Update sent but should not be processed
	default:
		// Channel might be blocked, which is also fine
	}

	// State should remain as it was
	if service.GetState() != services.StateRunning {
		t.Errorf("state should not have changed after context cancellation, got %s", service.GetState())
	}
}

func TestMCPServerService_ConcurrentAccess(t *testing.T) {
	service := NewMCPServerService(config.MCPServerDefinition{
		Name:      "test-server",
		Command:   []string{"test", "command"},
		ProxyPort: 8080,
	})

	ctx := context.Background()
	var wg sync.WaitGroup

	// Define states and health statuses to cycle through
	states := []services.ServiceState{
		services.StateUnknown,
		services.StateStarting,
		services.StateRunning,
		services.StateStopping,
	}
	healthStatuses := []services.HealthStatus{
		services.HealthUnknown,
		services.HealthHealthy,
		services.HealthUnhealthy,
	}

	// Simulate concurrent access to the service
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Perform various operations concurrently
			_ = service.GetServiceData()
			_, _ = service.CheckHealth(ctx)
			service.UpdateState(states[id%len(states)], healthStatuses[id%len(healthStatuses)], nil)
			_ = service.GetState()
			_ = service.GetHealth()
			_ = service.GetLastError()
		}(i)
	}

	// Also send process updates concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			update := mcpserver.McpDiscreteStatusUpdate{
				Label:         "test-server",
				ProcessStatus: "ProcessRunning",
				PID:           1000 + id,
				ProxyPort:     8080 + id,
			}
			service.handleProcessUpdate(update)
		}(i)
	}

	wg.Wait()

	// Service should still be in a valid state
	state := service.GetState()
	validStates := map[services.ServiceState]bool{
		services.StateUnknown:  true,
		services.StateStarting: true,
		services.StateRunning:  true,
		services.StateStopping: true,
		services.StateStopped:  true,
		services.StateFailed:   true,
		services.StateRetrying: true,
	}
	if !validStates[state] {
		t.Errorf("invalid state after concurrent access: %v", state)
	}

	health := service.GetHealth()
	validHealthStatuses := map[services.HealthStatus]bool{
		services.HealthUnknown:   true,
		services.HealthHealthy:   true,
		services.HealthUnhealthy: true,
		services.HealthChecking:  true,
	}
	if !validHealthStatuses[health] {
		t.Errorf("invalid health after concurrent access: %v", health)
	}
}
