# Service State Transitions Design

This document describes the unified design pattern for handling service state transitions in envctl.

## Overview

All services in envctl follow a consistent pattern for reporting state transitions to the TUI. This design prioritizes simplicity, debuggability, and consistency across different service types.

## Design Principles

1. **Direct Callback Pattern**: Services use direct callbacks instead of channels for state updates
2. **Common Status Types**: All services use common status constants and mapping functions
3. **Synchronous Updates**: State updates are processed synchronously for easier debugging
4. **Consistent Flow**: All services follow the same state transition flow

## Architecture

### 1. Status Update Flow

```
External System → Service Callback → Status Mapping → State Update → TUI
```

- **External System**: The underlying implementation (kubectl, process, container)
- **Service Callback**: Direct callback function that receives status updates
- **Status Mapping**: Common functions to map status strings to ServiceState/HealthStatus
- **State Update**: BaseService.UpdateState() triggers registered callbacks
- **TUI**: Receives events through orchestrator → API → channel

### 2. Common Types

All services use these common types from `internal/services/status.go`:

```go
// Common status update structure
type StatusUpdate struct {
    Label    string
    Status   string
    Detail   string
    IsError  bool
    IsReady  bool
    Error    error
}

// Common status constants
const (
    StatusInitializing = "Initializing"
    StatusStarting     = "Starting"
    StatusRunning      = "Running"
    StatusActive       = "Active"
    StatusStopping     = "Stopping"
    StatusStopped      = "Stopped"
    StatusFailed       = "Failed"
    StatusUnhealthy    = "Unhealthy"
)
```

### 3. Service Implementation Pattern

Each service follows this pattern:

```go
func (s *MyService) Start(ctx context.Context) error {
    s.UpdateState(services.StateStarting, services.HealthUnknown, nil)

    // Create update callback
    updateFn := func(/* service-specific params */) {
        // Convert to common status
        status := mapToCommonStatus(specificStatus)
        
        // Create status update
        update := services.StatusUpdate{
            Label:   label,
            Status:  status,
            IsError: hasError,
            IsReady: isReady,
            Error:   err,
        }
        
        // Map to state and health
        newState := services.MapStatusToState(update.Status)
        newHealth := services.MapStatusToHealth(update.Status, update.IsError)
        
        // Update service state
        s.UpdateState(newState, newHealth, update.Error)
    }

    // Start the underlying implementation with callback
    return startImplementation(updateFn)
}
```

## Service-Specific Implementations

### Port Forward Services

Port forwards receive updates from the Kubernetes port forwarding system:

```go
updateFn := func(label string, detail PortForwardStatusDetail, isReady bool, err error) {
    // Map PortForwardStatusDetail to common status
    // Update service state
}
```

### MCP Server Services

MCP servers receive updates from process/container management:

```go
updateFn := func(update McpDiscreteStatusUpdate) {
    // Map process/container status to common status
    // Update service state
}
```

### K8s Connection Services

K8s connections monitor cluster health directly:

```go
// No external callback - monitors health internally
go s.monitorConnection(ctx)
```

## Benefits

1. **Simplicity**: Direct callbacks are easier to understand than channel-based communication
2. **Debuggability**: Synchronous flow makes it easy to trace state transitions
3. **Consistency**: All services follow the same pattern
4. **Maintainability**: Common code reduces duplication

## Debugging State Transitions

To debug state transitions:

1. Enable debug logging to see status updates
2. Set breakpoints in the service's update callback
3. Trace through the common mapping functions
4. Follow the state change through BaseService.UpdateState()

## Adding New Service Types

When adding a new service type:

1. Implement the Service interface
2. Use BaseService for common functionality
3. Create an update callback that maps to common status types
4. Use the common mapping functions
5. Follow the established pattern

## Best Practices for Service Implementation

### 1. Minimize Shared State

Most service fields fall into these categories:

- **Immutable Configuration**: Set at construction, never changes (no protection needed)
- **Write-Once Runtime State**: Set during Start(), cleared during Stop() (minimal locking)
- **Lifecycle State**: Managed by BaseService (already thread-safe)

Example structure:
```go
type MyService struct {
    *services.BaseService
    
    // Immutable - no protection needed
    config MyConfig
    
    // Write-once - minimal mutex usage
    mu       sync.RWMutex
    stopChan chan struct{}
    resource *SomeResource
}
```

### 2. Lock Granularity

Only lock when accessing mutable shared state:

```go
func (s *MyService) Start(ctx context.Context) error {
    // No lock needed for state checks - BaseService is thread-safe
    if s.GetState() == services.StateRunning {
        return nil
    }
    
    // Do initialization work without holding locks
    resource, stopChan, err := initializeResource()
    if err != nil {
        return err
    }
    
    // Only lock when storing the results
    s.mu.Lock()
    s.resource = resource
    s.stopChan = stopChan
    s.mu.Unlock()
    
    return nil
}
```

### 3. Alternative Concurrency Patterns

Consider these simpler alternatives to mutexes:

#### Atomic Values
For simple fields that are set once:
```go
type MyService struct {
    *services.BaseService
    stopChan atomic.Pointer[chan struct{}]
}

// Set
ch := make(chan struct{})
s.stopChan.Store(&ch)

// Get
if ch := s.stopChan.Load(); ch != nil {
    close(*ch)
}
```

#### Message Passing
For complex state updates, consider channels:
```go
type MyService struct {
    *services.BaseService
    commands chan command
}

type command struct {
    op     string
    result chan error
}

// Background goroutine processes commands
func (s *MyService) run() {
    for cmd := range s.commands {
        switch cmd.op {
        case "stop":
            // Handle stop
            cmd.result <- nil
        }
    }
}
```

### 4. Copy on Read

When exposing internal state, copy values while holding the lock briefly:
```go
func (s *MyService) GetServiceData() map[string]interface{} {
    // Copy values out
    s.mu.RLock()
    resourceID := s.resource.ID
    resourceName := s.resource.Name
    s.mu.RUnlock()
    
    // Build response without holding lock
    return map[string]interface{}{
        "id":   resourceID,
        "name": resourceName,
    }
}
```

## Troubleshooting

### Mutex Locking Issues

One common issue that can prevent state transitions from being reported is improper mutex usage:

**Problem**: Holding a service mutex for the entire duration of long-running operations can block state updates from being processed.

**Example of problematic code**:
```go
func (s *Service) Start(ctx context.Context) error {
    s.mu.Lock()
    defer s.mu.Unlock()  // Holds mutex for entire method!
    
    // Long running operations...
    // Callbacks from goroutines can't update state
}
```

**Solution**: Only hold mutexes when accessing shared state:
```go
func (s *Service) Start(ctx context.Context) error {
    // Check state with minimal locking
    s.mu.Lock()
    if s.GetState() == services.StateRunning {
        s.mu.Unlock()
        return nil
    }
    s.mu.Unlock()
    
    // Long running operations without holding mutex
    // State updates from callbacks work properly
}
```

### Stop Method State Checks

Another issue is preventing state transitions in Stop() methods:

**Problem**:
```go
func (s *Service) Stop(ctx context.Context) error {
    if s.GetState() != services.StateRunning {
        return nil  // Prevents transition to Stopped!
    }
}
```

**Solution**: Always allow state transitions:
```go
func (s *Service) Stop(ctx context.Context) error {
    // Always transition to stopping state
    s.UpdateState(services.StateStopping, s.GetHealth(), nil)
    // ... stop logic
}
```

## Migration Notes

The following changes were made to unify the design:

- **Port Forwards**: Removed internal statusChan and monitorStatus goroutine
- **MCP Servers**: Updated to use common status types and mapping functions
- **All Services**: Now use consistent status constants and mapping logic 