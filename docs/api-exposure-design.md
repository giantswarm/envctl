# API Exposure Design for TUI

## Current State Analysis

### Existing Architecture

The current architecture has several key components:

1. **Event Bus System**: A sophisticated publish/subscribe system for decoupled communication
2. **Service Manager**: Manages lifecycle of services (port forwards, MCP servers, K8s connections)
3. **Orchestrator**: Higher-level component managing dependencies and health monitoring
4. **Reporting System**: Uses reporters (TUI/Console) to communicate state changes
5. **State Store**: Centralized state management for all services

### Current Issues

1. **Service Manager Overload**: The K8s health check was added to the service manager, diluting its core responsibility
2. **Limited API Exposure**: No clean way to expose service-specific functionality (like MCP tools) to the TUI
3. **Tight Coupling**: TUI directly accesses internal structures rather than through well-defined APIs
4. **Testing Challenges**: Direct dependencies make unit testing difficult

## Proposed Design: Application API Layer

### Core Concept

Introduce an **Application API** layer that acts as a facade between the TUI and the underlying services. This API would:

1. Expose domain-specific functionality through well-defined interfaces
2. Use the existing event bus for asynchronous communication
3. Maintain clear separation of concerns
4. Enable easy testing through interface mocking

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                          TUI Layer                          │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │   Model     │  │  Controller  │  │      View       │  │
│  └─────────────┘  └──────────────┘  └─────────────────┘  │
└────────────────────────────┬────────────────────────────────┘
                             │ Uses
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                    Application API Layer                     │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │   MCP API    │  │   Kube API   │  │ PortForward API │  │
│  └──────────────┘  └──────────────┘  └─────────────────┘  │
└────────────────────────────┬────────────────────────────────┘
                             │ Delegates to
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                     Core Services Layer                      │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │ MCP Package  │  │ Kube Package │  │  PF Package     │  │
│  └──────────────┘  └──────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                    Infrastructure Layer                      │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │  Event Bus   │  │ State Store  │  │  Orchestrator   │  │
│  └──────────────┘  └──────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### API Interface Definitions

#### MCP API

```go
// MCPServerAPI provides access to MCP server functionality
type MCPServerAPI interface {
    // GetTools returns the list of tools exposed by an MCP server
    GetTools(ctx context.Context, serverName string) ([]MCPTool, error)
    
    // GetToolDetails returns detailed information about a specific tool
    GetToolDetails(ctx context.Context, serverName string, toolName string) (*MCPToolDetails, error)
    
    // ExecuteTool executes a tool and returns the result
    ExecuteTool(ctx context.Context, serverName string, toolName string, params map[string]interface{}) (interface{}, error)
    
    // GetServerStatus returns the current status of an MCP server
    GetServerStatus(serverName string) (*MCPServerStatus, error)
    
    // SubscribeToToolUpdates subscribes to tool list changes
    SubscribeToToolUpdates(serverName string) <-chan MCPToolUpdateEvent
}

// MCPTool represents a tool exposed by an MCP server
type MCPTool struct {
    Name        string
    Description string
    InputSchema json.RawMessage
}

// MCPToolDetails includes full details about a tool
type MCPToolDetails struct {
    MCPTool
    Examples    []MCPToolExample
    LastUpdated time.Time
}

// MCPServerStatus represents the status of an MCP server
type MCPServerStatus struct {
    Name       string
    State      reporting.ServiceState
    ToolCount  int
    LastCheck  time.Time
    Error      error
}
```

#### Kubernetes API

```go
// KubernetesAPI provides Kubernetes-specific functionality
type KubernetesAPI interface {
    // GetClusterHealth returns health information for a cluster
    GetClusterHealth(ctx context.Context, contextName string) (*ClusterHealth, error)
    
    // GetNamespaces returns list of namespaces
    GetNamespaces(ctx context.Context, contextName string) ([]string, error)
    
    // GetResources returns resources in a namespace
    GetResources(ctx context.Context, contextName string, namespace string, resourceType string) ([]Resource, error)
    
    // SubscribeToHealthUpdates subscribes to cluster health changes
    SubscribeToHealthUpdates(contextName string) <-chan ClusterHealthEvent
}
```

#### Port Forward API

```go
// PortForwardAPI provides port forwarding functionality
type PortForwardAPI interface {
    // GetActiveForwards returns all active port forwards
    GetActiveForwards() []PortForwardInfo
    
    // GetForwardMetrics returns metrics for a port forward
    GetForwardMetrics(name string) (*PortForwardMetrics, error)
    
    // TestConnection tests if a port forward is working
    TestConnection(ctx context.Context, name string) error
}
```

### Implementation Strategy

#### 1. API Provider Pattern

```go
// APIProvider aggregates all API interfaces
type APIProvider struct {
    MCP         MCPServerAPI
    Kubernetes  KubernetesAPI
    PortForward PortForwardAPI
}

// NewAPIProvider creates a new API provider with all implementations
func NewAPIProvider(
    eventBus reporting.EventBus,
    stateStore reporting.StateStore,
    kubeMgr kube.Manager,
) *APIProvider {
    return &APIProvider{
        MCP:         NewMCPServerAPI(eventBus, stateStore),
        Kubernetes:  NewKubernetesAPI(eventBus, stateStore, kubeMgr),
        PortForward: NewPortForwardAPI(eventBus, stateStore),
    }
}
```

#### 2. Event-Driven Updates

APIs use the event bus for real-time updates:

```go
// Example implementation in MCP API
func (m *mcpServerAPI) SubscribeToToolUpdates(serverName string) <-chan MCPToolUpdateEvent {
    ch := make(chan MCPToolUpdateEvent, 10)
    
    // Subscribe to relevant events
    filter := reporting.CombineFilters(
        reporting.FilterByType(reporting.EventTypeServiceRunning),
        reporting.FilterBySource(serverName),
    )
    
    subscription := m.eventBus.Subscribe(filter, func(event reporting.Event) {
        if serviceEvent, ok := event.(*reporting.ServiceStateEvent); ok {
            // Check if tools need to be refreshed
            if serviceEvent.NewState == reporting.StateRunning {
                go m.refreshToolsAsync(serverName, ch)
            }
        }
    })
    
    // Clean up on channel close
    go func() {
        <-ch
        m.eventBus.Unsubscribe(subscription)
    }()
    
    return ch
}
```

#### 3. Caching Layer

Implement caching to reduce redundant API calls:

```go
type mcpServerAPI struct {
    eventBus   reporting.EventBus
    stateStore reporting.StateStore
    toolCache  *cache.Cache // TTL-based cache for tool lists
    mu         sync.RWMutex
}

func (m *mcpServerAPI) GetTools(ctx context.Context, serverName string) ([]MCPTool, error) {
    // Check cache first
    if cached, found := m.toolCache.Get(serverName); found {
        return cached.([]MCPTool), nil
    }
    
    // Fetch from MCP server
    tools, err := m.fetchToolsFromServer(ctx, serverName)
    if err != nil {
        return nil, err
    }
    
    // Cache the result
    m.toolCache.Set(serverName, tools, 5*time.Minute)
    
    return tools, nil
}
```

### Integration with TUI

#### 1. Model Enhancement

```go
// Add to Model struct
type Model struct {
    // ... existing fields ...
    
    // API Provider
    APIs *APIProvider
    
    // Cached data from APIs
    MCPTools      map[string][]MCPTool
    ClusterHealth map[string]*ClusterHealth
}
```

#### 2. Controller Commands

```go
// New command to fetch MCP tools
func FetchMCPToolsCmd(apis *APIProvider, serverName string) tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        
        tools, err := apis.MCP.GetTools(ctx, serverName)
        if err != nil {
            return MCPToolsErrorMsg{ServerName: serverName, Error: err}
        }
        
        return MCPToolsLoadedMsg{
            ServerName: serverName,
            Tools:      tools,
        }
    }
}
```

#### 3. View Integration

```go
// New view component for MCP tools
func RenderMCPToolsPanel(m *Model) string {
    var b strings.Builder
    
    for serverName, tools := range m.MCPTools {
        b.WriteString(fmt.Sprintf("📦 %s (%d tools)\n", serverName, len(tools)))
        
        for _, tool := range tools {
            b.WriteString(fmt.Sprintf("  • %s - %s\n", tool.Name, tool.Description))
        }
    }
    
    return b.String()
}
```

## Benefits of This Approach

### 1. **Separation of Concerns**
- Service Manager remains focused on lifecycle management
- Domain-specific logic lives in appropriate packages
- TUI only knows about high-level APIs

### 2. **Testability**
- APIs are interfaces, easily mockable
- Unit tests can focus on specific functionality
- Integration tests can use real implementations

### 3. **Extensibility**
- New APIs can be added without modifying existing code
- Features can be added to APIs independently
- Third-party plugins could implement API interfaces

### 4. **Performance**
- Caching reduces redundant operations
- Event-driven updates minimize polling
- Async operations keep UI responsive

### 5. **Developer Experience**
- Clear, documented APIs
- Type-safe interfaces
- Consistent patterns across different domains

## Implementation Plan

### Phase 1: Foundation (Week 1)
1. Create API interface definitions
2. Implement APIProvider pattern
3. Add basic MCP API implementation
4. Write comprehensive tests

### Phase 2: MCP Tools Feature (Week 2)
1. Implement GetTools and GetToolDetails
2. Add caching layer
3. Create TUI commands and messages
4. Add tools panel to TUI view

### Phase 3: Migration (Week 3)
1. Move K8s health check from ServiceManager to KubernetesAPI
2. Refactor existing TUI code to use APIs
3. Update documentation
4. Performance testing

### Phase 4: Enhancement (Week 4)
1. Add remaining API methods
2. Implement subscription mechanisms
3. Add metrics and monitoring
4. Create developer documentation

## Risks and Mitigations

### Risk 1: Breaking Changes
**Mitigation**: Implement APIs alongside existing code, migrate gradually

### Risk 2: Performance Overhead
**Mitigation**: Use caching, event-driven updates, and connection pooling

### Risk 3: Complexity Increase
**Mitigation**: Keep APIs simple, well-documented, with clear examples

## Alternative Approaches Considered

### 1. Direct Package Access
**Pros**: Simple, no abstraction layer
**Cons**: Tight coupling, hard to test, no clear API boundaries

### 2. GraphQL API
**Pros**: Flexible querying, single endpoint
**Cons**: Overhead for internal use, complexity for simple operations

### 3. gRPC Services
**Pros**: Type-safe, efficient, could enable remote access
**Cons**: Overhead for in-process communication, additional complexity

## Conclusion

The Application API layer provides a clean, testable, and extensible way to expose functionality to the TUI. It maintains separation of concerns while leveraging the existing event bus infrastructure for real-time updates. This approach scales well as new features are added and makes the codebase more maintainable. 