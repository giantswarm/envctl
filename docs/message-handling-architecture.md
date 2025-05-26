# Message Handling and Reporting Architecture

This document describes the comprehensive message handling and reporting architecture implemented in envctl, which provides a robust, scalable, and maintainable system for managing service state updates, health monitoring, and event processing.

## Overview

The message handling architecture consists of four main components:

1. **Correlation Tracking System** - Traces related messages and cascading effects
2. **Centralized State Store** - Single source of truth for all service states
3. **Structured Event System** - Semantic events with publish/subscribe functionality
4. **Performance Optimization** - Monitoring, batching, and memory management

## Architecture Components

### 1. Correlation Tracking System

The correlation tracking system enables tracing of related messages and cascading effects throughout the application.

#### Key Features

- **Correlation IDs**: Unique identifiers that link related operations
- **Causality Tracking**: Records what triggered each operation
- **Parent-Child Relationships**: Tracks hierarchical relationships between operations
- **Helper Functions**: Convenient methods for creating correlated updates

#### Usage Example

```go
// Create a correlated update
update := NewManagedServiceUpdate(ServiceTypePortForward, "test-pf", StateStarting)
update = update.WithCause("user_action").WithCorrelation("corr-123", "user_restart", "")

// Report the update
reporter.Report(update)
```

#### Data Structures

```go
type ManagedServiceUpdate struct {
    SourceType    ServiceType
    SourceLabel   string
    State         ServiceState
    CorrelationID string  // Links related operations
    CausedBy      string  // What triggered this update
    ParentID      string  // Parent operation ID
    // ... other fields
}
```

### 2. Centralized State Store

The state store provides a single source of truth for all service states, eliminating duplication and ensuring consistency.

#### Key Features

- **Thread-Safe Operations**: Concurrent access with proper locking
- **State Change Subscriptions**: Reactive updates when states change
- **Comprehensive Metrics**: Performance and usage tracking
- **Query Capabilities**: Filter services by type, state, or other criteria

#### Usage Example

```go
// Create state store
stateStore := NewStateStore()

// Set service state
update := NewManagedServiceUpdate(ServiceTypePortForward, "test-pf", StateRunning)
stateChanged, err := stateStore.SetServiceState(update)

// Get service state
snapshot, exists := stateStore.GetServiceState("test-pf")
if exists {
    fmt.Printf("Service %s is %s\n", snapshot.Label, snapshot.State)
}

// Subscribe to state changes
subscription := stateStore.Subscribe("test-pf")
go func() {
    for event := range subscription.Channel {
        fmt.Printf("State changed: %s -> %s\n", event.OldState, event.NewState)
    }
}()
```

#### Data Structures

```go
type ServiceStateSnapshot struct {
    Label         string
    SourceType    ServiceType
    State         ServiceState
    IsReady       bool
    ErrorDetail   error
    ProxyPort     int
    PID           int
    LastUpdated   time.Time
    CorrelationID string
    CausedBy      string
    ParentID      string
}

type StateChangeEvent struct {
    Label    string
    OldState ServiceState
    NewState ServiceState
    Snapshot ServiceStateSnapshot
}
```

### 3. Structured Event System

The event system replaces raw messages with semantic events, providing better clarity and enabling sophisticated event handling.

#### Event Hierarchy

```go
type Event interface {
    Type() EventType
    Source() string
    Timestamp() time.Time
    Severity() EventSeverity
    CorrelationID() string
    CausedBy() string
    ParentID() string
    Metadata() map[string]interface{}
    String() string
}
```

#### Event Types

1. **ServiceStateEvent**: Service lifecycle changes
2. **HealthEvent**: Cluster health status updates
3. **DependencyEvent**: Cascade start/stop operations
4. **UserActionEvent**: User-initiated actions
5. **SystemEvent**: System-level operations

#### Event Bus

The event bus provides publish/subscribe functionality with advanced filtering capabilities.

```go
// Create event bus
eventBus := NewEventBus()

// Subscribe with filter
filter := FilterByType(EventTypeServiceRunning, EventTypeServiceFailed)
subscription := eventBus.Subscribe(filter, func(event Event) {
    fmt.Printf("Received event: %s\n", event.String())
})

// Publish event
event := NewServiceStateEvent(ServiceTypePortForward, "test-pf", StateStarting, StateRunning)
eventBus.Publish(event)

// Cleanup
eventBus.Unsubscribe(subscription)
```

#### Event Filters

The system provides composable filters for sophisticated event handling:

```go
// Filter by event type
typeFilter := FilterByType(EventTypeServiceRunning, EventTypeServiceFailed)

// Filter by source
sourceFilter := FilterBySource("test-pf", "test-mcp")

// Filter by severity
severityFilter := FilterBySeverity(SeverityError)

// Combine filters with AND logic
combinedFilter := CombineFilters(typeFilter, sourceFilter)

// Combine filters with OR logic
anyFilter := AnyFilter(typeFilter, severityFilter)
```

### 4. Performance Optimization

The architecture includes several performance optimization features:

#### Performance Monitoring

```go
// Create performance monitor
monitor := NewPerformanceMonitor(eventBus, stateStore)
monitor.Start(context.Background(), 1*time.Second)

// Get metrics
metrics := monitor.GetMetrics()
fmt.Printf("Events per second: %.2f\n", metrics.EventsPerSecond)
fmt.Printf("Performance score: %.1f\n", metrics.PerformanceScore)
```

#### Event Batching

```go
// Create batch processor
batchProcessor := NewEventBatchProcessor(eventBus, 50, 10*time.Millisecond)
batchProcessor.Start()

// Queue events for batching
batchProcessor.QueueEvent(event)
```

#### Optimized Event Bus

```go
// Create optimized event bus with all features
config := DefaultOptimizedEventBusConfig()
optimizedBus := NewOptimizedEventBus(config)

// Use like regular event bus but with optimizations
optimizedBus.Publish(event)

// Get performance metrics
if metrics := optimizedBus.GetPerformanceMetrics(); metrics != nil {
    fmt.Printf("System healthy: %t\n", metrics.SystemHealthy)
}
```

#### Object Pooling

```go
// Create event pool manager
poolManager := NewEventPoolManager()

// Get pooled event
event := poolManager.GetServiceStateEvent()
// ... use event ...
poolManager.PutServiceStateEvent(event) // Return to pool
```

## Integration with Existing Components

### ServiceReporter Interface

The architecture maintains backwards compatibility through the enhanced `ServiceReporter` interface:

```go
type ServiceReporter interface {
    Report(update ManagedServiceUpdate)
    ReportHealth(update HealthStatusUpdate)
    GetStateStore() StateStore  // New method for direct state access
}
```

### TUIReporter

The TUI reporter now uses the centralized state store and configurable buffering:

```go
// Create TUI reporter with custom configuration
config := TUIReporterConfig{
    BufferSize:     2000,
    BufferStrategy: NewPriorityBufferStrategy(),
    StateStore:     stateStore, // Use shared state store
}
reporter := NewTUIReporterWithConfig(updateChan, config)
```

### EventBusAdapter

For backwards compatibility, the `EventBusAdapter` converts between the old and new systems:

```go
// Create adapter
adapter := NewEventBusAdapter(eventBus, stateStore)

// Use as ServiceReporter
var reporter ServiceReporter = adapter
reporter.Report(update) // Automatically converts to events
```

## Configuration

### Buffer Strategies

Configure how the system handles buffer overflow:

```go
// Drop messages when buffer is full
dropStrategy := &SimpleBufferStrategy{Action: BufferActionDrop}

// Block until space is available
blockStrategy := &SimpleBufferStrategy{Action: BufferActionBlock}

// Evict oldest messages to make room
evictStrategy := &SimpleBufferStrategy{Action: BufferActionEvictOldest}

// Priority-based strategy
priorityStrategy := NewPriorityBufferStrategy()
```

### Event Bus Configuration

```go
config := OptimizedEventBusConfig{
    EnableBatching:     true,
    EnableMonitoring:   true,
    BatchSize:          50,
    BatchFlushTime:     10 * time.Millisecond,
    MonitoringInterval: 1 * time.Second,
    StateStore:         stateStore,
}
```

## Best Practices

### 1. Correlation Tracking

- Always use correlation IDs for related operations
- Set meaningful `CausedBy` values for debugging
- Use parent IDs for hierarchical operations

### 2. Event Handling

- Use specific event filters to reduce processing overhead
- Handle events asynchronously when possible
- Implement proper error handling in event handlers

### 3. Performance

- Use batching for high-volume event scenarios
- Monitor performance metrics regularly
- Use object pooling for frequently created events

### 4. State Management

- Always use the centralized state store as the source of truth
- Subscribe to state changes for reactive updates
- Clear unused state to prevent memory leaks

## Migration Guide

### From Raw Messages to Structured Events

**Before:**
```go
// Old way with raw messages
reporter.Report(ManagedServiceUpdate{
    SourceType:  ServiceTypePortForward,
    SourceLabel: "test-pf",
    State:       StateRunning,
})
```

**After:**
```go
// New way with structured events
event := NewServiceStateEvent(ServiceTypePortForward, "test-pf", StateStarting, StateRunning)
event.WithCorrelation("corr-123", "user_action", "")
eventBus.Publish(event)
```

### From Local State to Centralized Store

**Before:**
```go
// Old way with local state tracking
type ServiceManager struct {
    serviceStates map[string]ServiceState
    mu           sync.Mutex
}
```

**After:**
```go
// New way with centralized state store
type ServiceManager struct {
    reporter ServiceReporter // Contains state store
}

func (sm *ServiceManager) getServiceState(label string) ServiceState {
    snapshot, exists := sm.reporter.GetStateStore().GetServiceState(label)
    if exists {
        return snapshot.State
    }
    return StateUnknown
}
```

## Testing

The architecture includes comprehensive testing utilities:

### Integration Tests

```go
func TestCompleteEventFlow(t *testing.T) {
    eventBus := NewEventBus()
    stateStore := NewStateStore()
    adapter := NewEventBusAdapter(eventBus, stateStore)
    
    // Test complete flow from updates to events
    // ... test implementation
}
```

### Performance Tests

```go
func TestPerformanceUnderLoad(t *testing.T) {
    // Test system performance under high load
    // ... test implementation
}
```

### Error Recovery Tests

```go
func TestErrorRecovery(t *testing.T) {
    // Test system behavior under error conditions
    // ... test implementation
}
```

## Metrics and Monitoring

### Event Bus Metrics

- `EventsPublished`: Total events published
- `EventsDelivered`: Total events delivered to subscribers
- `EventsDropped`: Total events dropped due to buffer overflow
- `ActiveSubscriptions`: Current number of active subscriptions
- `AverageDeliveryTime`: Average time to deliver events

### State Store Metrics

- `TotalServices`: Number of services tracked
- `StateChanges`: Total state changes recorded
- `SubscriptionEvents`: Events sent to subscriptions
- `SubscriptionDrops`: Events dropped from subscription buffers

### Performance Metrics

- `EventsPerSecond`: Current event processing rate
- `PerformanceScore`: Overall system performance (0-100)
- `SystemHealthy`: Boolean indicating system health
- `MemoryUsageBytes`: Estimated memory usage

## Troubleshooting

### Common Issues

1. **High Event Drop Rate**
   - Increase buffer sizes
   - Use priority-based buffer strategies
   - Optimize event handlers

2. **Poor Performance Score**
   - Check for slow event handlers
   - Reduce subscription count
   - Enable batching

3. **Memory Usage Growth**
   - Clear unused state regularly
   - Use object pooling
   - Monitor subscription cleanup

### Debug Tools

```go
// Enable debug logging
logging.SetLevel(logging.LevelDebug)

// Monitor performance
monitor := NewPerformanceMonitor(eventBus, stateStore)
monitor.Start(context.Background(), 1*time.Second)

// Check metrics
metrics := eventBus.GetMetrics()
fmt.Printf("Debug info: %+v\n", metrics)
```

## Future Enhancements

The architecture is designed to be extensible. Potential future enhancements include:

1. **Persistent Event Store**: Store events for replay and analysis
2. **Event Sourcing**: Rebuild state from event history
3. **Distributed Events**: Support for multi-instance deployments
4. **Advanced Analytics**: Machine learning for performance optimization
5. **Custom Event Types**: Plugin system for domain-specific events

## Conclusion

The message handling and reporting architecture provides a robust foundation for envctl's communication needs. It offers:

- **Scalability**: Handles high event volumes efficiently
- **Maintainability**: Clear separation of concerns and well-defined interfaces
- **Observability**: Comprehensive metrics and correlation tracking
- **Flexibility**: Configurable behavior and extensible design
- **Reliability**: Error recovery and graceful degradation

This architecture ensures that envctl can handle complex service orchestration scenarios while maintaining performance and reliability. 