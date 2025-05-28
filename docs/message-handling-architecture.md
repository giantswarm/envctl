# Message Handling Architecture

This document describes the message handling architecture implemented in envctl, which provides a simple and effective system for managing service state updates and event processing.

## Overview

The message handling architecture in envctl follows a straightforward callback-based approach with channel-based event forwarding. The system consists of three main components:

1. **Service State Callbacks** - Services notify state changes via callbacks
2. **Orchestrator Event Forwarding** - The orchestrator forwards events to the API layer
3. **TUI Event Subscription** - The TUI subscribes to state changes via channels

## Architecture Components

### 1. Service State Management

Each service implements the `Service` interface which includes state change notification:

```go
type Service interface {
    // ... other methods ...
    
    // State change notifications
    SetStateChangeCallback(callback StateChangeCallback)
}

type StateChangeCallback func(label string, oldState, newState ServiceState, health HealthStatus, err error)
```

Services use the `BaseService` implementation which provides thread-safe state management:

- `UpdateState()` - Updates service state and triggers callbacks
- `UpdateHealth()` - Updates only health status
- `UpdateError()` - Updates error state

### 2. Service States and Health

The system defines clear service states and health statuses:

```go
// Service states
type ServiceState string

const (
    StateUnknown  ServiceState = "Unknown"
    StateStarting ServiceState = "Starting"
    StateRunning  ServiceState = "Running"
    StateStopping ServiceState = "Stopping"
    StateStopped  ServiceState = "Stopped"
    StateFailed   ServiceState = "Failed"
    StateRetrying ServiceState = "Retrying"
)

// Health statuses
type HealthStatus string

const (
    HealthUnknown   HealthStatus = "Unknown"
    HealthHealthy   HealthStatus = "Healthy"
    HealthUnhealthy HealthStatus = "Unhealthy"
    HealthChecking  HealthStatus = "Checking"
)
```

### 3. Event Flow

The event flow follows this path:

1. **Service State Change**: A service updates its state using `UpdateState()`
2. **Callback Invocation**: The registered callback is invoked with state details
3. **Orchestrator Handling**: The orchestrator receives the callback and forwards it
4. **API Event Channel**: Events are sent through the API's event channel
5. **TUI Reception**: The TUI receives events and updates the display

### 4. API Layer Events

The API layer defines a simple event structure:

```go
type ServiceStateChangedEvent struct {
    Label    string
    OldState string
    NewState string
    Health   string
    Error    error
}
```

The orchestrator API provides subscription:

```go
type OrchestratorAPI interface {
    // ... other methods ...
    
    // Service state monitoring
    SubscribeToStateChanges() <-chan ServiceStateChangedEvent
}
```

## Implementation Details

### Service Implementation

Services embed `BaseService` for common functionality:

```go
type PortForwardService struct {
    *services.BaseService
    // ... service-specific fields
}

// State updates trigger callbacks
func (s *PortForwardService) handleStatusUpdate(status string) {
    switch status {
    case "Initializing":
        s.UpdateState(services.StateStarting, services.HealthUnknown, nil)
    case "ForwardingActive":
        s.UpdateState(services.StateRunning, services.HealthHealthy, nil)
    case "Failed":
        s.UpdateState(services.StateFailed, services.HealthUnhealthy, fmt.Errorf("port forward failed"))
    }
}
```

### TUI Integration

The TUI subscribes to state changes and processes them:

```go
// In model initialization
func (m *Model) ListenForStateChanges() tea.Cmd {
    return func() tea.Msg {
        event, ok := <-m.StateChangeEvents
        if !ok {
            return nil
        }
        return event
    }
}

// In update handler
func handleServiceStateChange(m *model.Model, event api.ServiceStateChangedEvent) tea.Cmd {
    // Log the state change
    logMsg := fmt.Sprintf("[%s] %s: %s → %s",
        time.Now().Format("15:04:05"),
        event.Label,
        event.OldState,
        event.NewState,
    )
    
    m.ActivityLog = append(m.ActivityLog, logMsg)
    // Refresh service data
    return refreshServiceData(m)
}
```

### Thread Safety

The architecture ensures thread safety through:

- Mutex protection in `BaseService` for state updates
- Buffered channels for event delivery
- Callback execution outside of locks to prevent deadlocks

## Message Types

The TUI uses several message types for service-related updates:

- `ServiceStartedMsg` - Service successfully started
- `ServiceStoppedMsg` - Service successfully stopped
- `ServiceRestartedMsg` - Service successfully restarted
- `ServiceErrorMsg` - Service encountered an error
- `ServiceStateChangedEvent` - Generic state change event

## Best Practices

### 1. State Updates

- Always use the provided update methods (`UpdateState`, `UpdateHealth`, `UpdateError`)
- Include meaningful error messages when services fail
- Update state before performing long-running operations

### 2. Callback Implementation

- Keep callbacks lightweight and non-blocking
- Avoid performing heavy operations in callbacks
- Use channels or commands for async operations

### 3. Error Handling

- Propagate errors through the state callback mechanism
- Include context in error messages
- Use appropriate health statuses with errors

## Example Usage

### Starting a Service

```go
// Service implementation
func (s *K8sConnectionService) Start(ctx context.Context) error {
    s.UpdateState(services.StateStarting, services.HealthUnknown, nil)
    
    // Perform startup operations
    health, err := s.CheckHealth(ctx)
    if err != nil {
        s.UpdateState(services.StateFailed, health, err)
        return err
    }
    
    s.UpdateState(services.StateRunning, health, nil)
    return nil
}
```

### Monitoring State Changes

```go
// Orchestrator sets up callbacks
service.SetStateChangeCallback(func(label string, oldState, newState ServiceState, health HealthStatus, err error) {
    // Forward to API layer
    api.forwardStateChange(label, oldState, newState, health, err)
})
```

## Limitations and Future Considerations

The current implementation is intentionally simple and has some limitations:

- No event persistence or replay capability
- Limited event metadata (no correlation tracking)
- No built-in retry or circuit breaker patterns
- Events can be dropped if channels are full

These limitations are acceptable for the current use case but could be addressed in future iterations if needed.

## Conclusion

The message handling architecture in envctl provides a clean, simple approach to service state management. By using callbacks and channels, it maintains loose coupling between components while ensuring reliable state propagation from services to the UI. 