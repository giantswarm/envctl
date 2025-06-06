# TUI Implementation Details

This document provides technical details about the Terminal User Interface (TUI) implementation in `envctl` for developers who want to understand, maintain, or extend the codebase.

## Technology Stack

The TUI is built using:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea): TUI framework following the Model-View-Update (MVU) pattern
- [Lipgloss](https://github.com/charmbracelet/lipgloss): Styling library for terminal applications
- [Bubbles](https://github.com/charmbracelet/bubbles): Reusable components for Bubble Tea (viewport, textinput, spinner, help)

## Architecture Overview

The TUI has been refactored to follow a clean service-based architecture with clear separation of concerns:

1. **Service Layer** (`internal/services/`): Defines core service interfaces and implementations
2. **Orchestrator** (`internal/orchestrator/`): Manages service lifecycle and dependencies
3. **API Layer** (`internal/api/`): Provides clean interfaces for service interaction
4. **TUI Layer** (`internal/tui/`): Implements the terminal interface using MVC pattern

## Code Organization

### Service Layer (`internal/services/`)

The service layer provides the foundation for all manageable components:

- **Base Types**: `interfaces.go`, `base.go` - Core service interfaces and base implementations
- **Registry**: `registry.go` - Thread-safe service registry
- **Service Implementations**:
  - `k8s/`: Kubernetes connection services
  - `portforward/`: Port forwarding services
  - `mcpserver/`: MCP server services

### Orchestrator (`internal/orchestrator/`)

The orchestrator manages service lifecycle and dependencies:

- **Core**: `orchestrator.go` - Main orchestrator implementation
- **Dependencies**: `orchestrator_deps.go` - Dependency graph management
- **Health**: `orchestrator_health.go` - Health monitoring and recovery

### API Layer (`internal/api/`)

Provides clean interfaces for service interaction:

- **Interfaces**: 
  - `orchestrator.go`: `OrchestratorAPI` for service lifecycle management
  - `k8s_service.go`: `K8sServiceAPI` for Kubernetes connections
  - `portforward_service.go`: `PortForwardServiceAPI` for port forwards
  - `mcp_service.go`: `MCPServiceAPI` for MCP servers
- **Types**: `types.go` - Common data structures

### TUI Layer (`internal/tui/`)

The TUI follows a Model-View-Controller (MVC) pattern:

- **Model** (`internal/tui/model/`):
  - `types.go`: Core `Model` struct and enums
  - `messages.go`: All `tea.Msg` types for internal communication
  - `init.go`: Model initialization
- **View** (`internal/tui/view/`):
  - `render.go`: Main render function and mode switching
  - `render_dashboard.go`: Main dashboard layout
  - `render_context_panes.go`: K8s connection panels
  - `render_port_forwarding.go`: Port forward panels
  - `render_mcp_proxies.go`: MCP server panels
  - `render_overlays.go`: Help, log, and config overlays
  - `colors.go`, `icons.go`: Visual styling elements
- **Controller** (`internal/tui/controller/`):
  - `app.go`: Main `AppModel` implementing `tea.Model`
  - `update.go`: Central message handling and dispatch
  - `helpers.go`: Utility functions
  - `program.go`: Program initialization

## Model Structure

The `model.Model` struct has been simplified to work with the service architecture:

```go
type Model struct {
    // Terminal dimensions
    Width, Height int
    
    // Application state
    CurrentAppMode  AppMode
    FocusedPanelKey string
    
    // Service Architecture Components
    Orchestrator    *orchestrator.Orchestrator
    OrchestratorAPI api.OrchestratorAPI
    MCPServiceAPI   api.MCPServiceAPI
    PortForwardAPI  api.PortForwardServiceAPI
    K8sServiceAPI   api.K8sServiceAPI
    
    // Cached service data for display
    K8sConnections map[string]*api.K8sConnectionInfo
    PortForwards   map[string]*api.PortForwardServiceInfo
    MCPServers     map[string]*api.MCPServerInfo
    MCPTools       map[string][]api.MCPTool
    
    // Service ordering for display
    K8sConnectionOrder []string
    PortForwardOrder   []string
    MCPServerOrder     []string
    
    // UI Components
    LogViewport      viewport.Model
    MainLogViewport  viewport.Model
    McpConfigViewport viewport.Model
    McpToolsViewport  viewport.Model
    Spinner          spinner.Model
    Help             help.Model
    
    // Event channels
    TUIChannel        chan tea.Msg
    StateChangeEvents <-chan api.ServiceStateChangedEvent
    LogChannel        <-chan logging.LogEntry
}
```

## The MVU Pattern with Service Architecture

The implementation follows Bubble Tea's MVU pattern integrated with the service architecture:

1. **Model**: Holds UI state and cached service data
2. **Update**: Processes messages and interacts with service APIs
3. **View**: Renders the current state

### Update Loop

The main update loop in `controller.Update()` handles:

1. **Tea Messages**: Window resize, key presses, etc.
2. **Service Events**: State changes from the orchestrator
3. **Log Events**: New log entries from services
4. **UI Commands**: Service start/stop/restart requests

```go
func Update(msg tea.Msg, m *model.Model) (*model.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Handle keyboard input
    case api.ServiceStateChangedEvent:
        // Handle service state changes
    case model.NewLogEntryMsg:
        // Handle new log entries
    case model.ServiceStartedMsg:
        // Handle service started confirmation
    // ... other message types
    }
}
```

### Service Interaction

The TUI interacts with services through the API layer:

```go
// Starting a service
func (m *Model) StartService(label string) tea.Cmd {
    return func() tea.Msg {
        ctx := context.Background()
        if err := m.OrchestratorAPI.StartService(ctx, label); err != nil {
            return ServiceErrorMsg{Label: label, Err: err}
        }
        return ServiceStartedMsg{Label: label}
    }
}

// Refreshing service data
func (m *Model) RefreshServiceData() error {
    k8sConns, _ := m.K8sServiceAPI.ListConnections(ctx)
    portForwards, _ := m.PortForwardAPI.ListForwards(ctx)
    mcpServers, _ := m.MCPServiceAPI.ListServers(ctx)
    // Update model's cached data
}
```

## Event System

The TUI uses multiple event channels for real-time updates:

1. **State Change Events**: Service state transitions
2. **Log Events**: Service logs and activity
3. **TUI Channel**: Internal UI events

Events are processed continuously:

```go
// Listen for state changes
func (m *Model) ListenForStateChanges() tea.Cmd {
    return func() tea.Msg {
        event := <-m.StateChangeEvents
        return event
    }
}

// Listen for logs
func (m *Model) ListenForLogs() tea.Cmd {
    return func() tea.Msg {
        entry := <-m.LogChannel
        return model.NewLogEntryMsg{Entry: entry}
    }
}
```

## Service Management

Services are managed through the orchestrator with dependency awareness:

### Service Types

1. **K8s Connections**: Foundation services with no dependencies
2. **Port Forwards**: Depend on K8s connections
3. **MCP Servers**: May depend on port forwards

### Dependency Handling

The orchestrator automatically:
- Starts services in dependency order
- Stops dependent services when dependencies fail
- Restarts dependent services when dependencies recover

## Layout & Rendering

The view layer has been reorganized for better maintainability:

### Main Dashboard (`render_dashboard.go`)

Orchestrates the overall layout:
1. Header with navigation hints
2. Service panels in rows
3. Activity log
4. Status bar

### Component Rendering

Each service type has dedicated rendering:
- `render_context_panes.go`: K8s connection panels
- `render_port_forwarding.go`: Port forward panels
- `render_mcp_proxies.go`: MCP server panels

### Responsive Design

The layout adapts to terminal size:
```go
// Calculate available space
contentHeight := m.Height - headerHeight - statusBarHeight

// Distribute space among components
k8sRowHeight := int(float64(contentHeight) * 0.25)
portForwardRowHeight := int(float64(contentHeight) * 0.30)
mcpRowHeight := int(float64(contentHeight) * 0.25)
logHeight := contentHeight - k8sRowHeight - portForwardRowHeight - mcpRowHeight
```

## Styling System

Styling has been centralized in the `internal/tui/design/` package:

- Consistent color palette across the application
- Theme support (dark/light modes)
- Semantic color usage (success, warning, error)
- Accessibility considerations

The TUI view layer uses these styles through simple imports:
```go
import "envctl/internal/tui/design"

// Using predefined styles
design.PanelFocusedStyle.Render(content)
design.TextErrorStyle.Render(errorMsg)
```

## Testing Approach

The refactored architecture enables better testing:

1. **Service Layer**: Unit tests for individual services
2. **Orchestrator**: Integration tests for service coordination
3. **API Layer**: Mock implementations for testing
4. **TUI Layer**: Golden file tests for view rendering

## Common Development Tasks

### Adding a New Service Type

1. Implement the `services.Service` interface
2. Add service-specific API interface in `internal/api/`
3. Register with orchestrator
4. Add rendering logic in `internal/tui/view/`

### Modifying the UI

1. Update model types if new state is needed
2. Add message types for new interactions
3. Update controller to handle new messages
4. Modify view rendering functions

### Adding New Keyboard Shortcuts

1. Add key binding to `KeyMap` in model
2. Handle key in `handleMainDashboardKeys()`
3. Update help overlay content

## Performance Considerations

The refactored architecture improves performance:

1. **Cached Data**: Service data is cached and refreshed periodically
2. **Selective Updates**: Only affected components re-render
3. **Efficient Event Handling**: Channels prevent blocking
4. **Resource Management**: Services handle their own resources

## Migration from Previous Architecture

Key changes from the previous implementation:

1. **Service Abstraction**: Direct process management replaced with service interfaces
2. **Centralized Orchestration**: Dependency management moved to orchestrator
3. **API Layer**: Clean interfaces replace direct service access
4. **Simplified Model**: UI state separated from service management
5. **Event-Driven Updates**: Polling replaced with event subscriptions 