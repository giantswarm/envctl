# Terminal User Interface (TUI) Documentation

The `envctl` TUI is a polished, interactive terminal interface built with the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework from Charmbracelet. It provides a dashboard-like experience for monitoring and managing connections to Giant Swarm Kubernetes clusters.

![TUI Overview](images/tui-overview.png)

## Overview

The TUI is designed to provide real-time feedback about:
- Cluster connection status (Management and Workload Clusters)
- Node health for connected clusters
- Active port forwarding processes
- Operation logs and events

It also enables interactive operations like:
- Navigating between panels with keyboard shortcuts
- Starting new connections to different clusters
- Restarting port forwards
- Switching Kubernetes contexts
- Viewing detailed logs

## Screenshots

### Main Interface
![Main TUI Interface](images/tui-overview.png)

The main interface provides a complete view of:
- Header with navigation hints
- Management and Workload Cluster status panes
- Port forwarding panels for each service
- Activity log with real-time updates

### Help Overlay
![Help Overlay](images/tui-help-overlay.png)

The help overlay displays all available keyboard shortcuts, organized by category.

### Log Overlay
![Log Overlay](images/tui-log-overlay.png)

The log overlay provides expanded view of logs, particularly useful for:
- Debugging connection issues
- Monitoring port forward activity
- Viewing detailed cluster information

### Dark Mode
![Dark Mode](images/tui-dark-mode.png)

Dark mode provides optimal visibility in low-light environments and reduced eye strain.

## Architecture

The TUI follows the Model-View-Update (MVU) architecture pattern as implemented by Bubble Tea:

### Model

The core state is maintained in the `model` struct (in `internal/tui/model.go`), which contains:

- **Cluster Information**: Management/workload cluster names and contexts
- **Health Information**: Node status for each cluster
- **Port Forwarding State**: Active processes, their status, and output
- **UI State**: Window dimensions, viewport positions, focus state, etc.

### View

The `View()` method (in `model.go`) renders the current state as a string for display:

- It constructs the layout by dividing the screen into sections
- Each section is rendered by helper functions in `view_helpers.go`
- The overall app has a consistent color scheme and styling

### Update

The `Update()` method (in `model.go`) handles all events and messages:

- Key presses are routed to handlers in `handlers.go`
- Asynchronous messages (like port-forward status updates) are handled by specific functions
- Window size changes trigger layout recalculations
- All state changes happen through this method

## Component Structure

The TUI is composed of several distinct sections:

1. **Header**: Displays the application title, keyboard hints, and optional debug info
2. **Cluster Information Panes**: Shows MC and WC connection details and node health
3. **Port Forwarding Panels**: Displays active port forwards with status indicators
4. **Activity Log**: Shows a scrollable log of recent operations and events

## Message System

The TUI uses an asynchronous message system to handle events from port forwarding, Kubernetes operations, and UI interactions:

- Custom message types (like `portForwardStatusUpdateMsg`) defined throughout the codebase
- A channel-based approach with `TUIChannel` to safely receive messages from background goroutines
- The `channelReaderCmd` function ensures continuous processing of these messages

## Styling

All UI styling is centralized in `styles.go` using the [Lipgloss](https://github.com/charmbracelet/lipgloss) library:

- Adaptive colors that work in both light and dark modes
- Consistent borders, padding, and margins
- Status-specific styling (e.g., error states in red, success in green)

## Key Features

### Responsive Layout

- Automatically adapts to terminal window size changes
- Degrades gracefully to simpler layouts for small terminals
- Maintains full-width consistency for all panels

### Keyboard Navigation

- Tab/Shift+Tab to navigate between panels
- ESC to exit overlays
- Shortcut keys for common operations

### Port Forward Management

- Monitor active port forwards (Prometheus, Grafana, Alloy Metrics) with status indicators.
- Prometheus (MC) and Grafana (MC) are standard and always use the Management Cluster context.
- Alloy Metrics port-forwarding follows this logic:
  - If both a Management Cluster and a Workload Cluster are configured, Alloy Metrics connects to the Workload Cluster.
  - If only a Management Cluster is configured, Alloy Metrics connects to that Management Cluster.
- Restart individual port forwards when needed using the 'r' key with the panel focused.

### Dark Mode Support

- Complete dark mode support with 'D' key toggle
- Adaptive colors for all UI elements
- Proper contrast in both modes for readability

### Focus System

- Focused panels have distinct visual styling
- Focus can be moved between all interactive elements
- Current focus affects context-specific operations (e.g., 'r' to restart focused port forward)

### Overlays

- Help overlay ('h') displays all keyboard shortcuts
- Log overlay ('L') for expanded log viewing when screen space is limited

## Implementation Details

### Port Forwarding Management

Port forwards are managed by:
- `portforward_handlers.go`: Logic for setting up, monitoring, and restarting port forwards, with specific behavior for each service:
  - Prometheus (MC) and Grafana (MC) always connect to the Management Cluster
  - Alloy Metrics connects to the Workload Cluster if one is specified, otherwise it connects to the Management Cluster
- `portForwardProcess` struct: Tracks process state (including which cluster it targets), output, and errors for each port-forward.
- Status update messages: Keep the UI in sync with actual process status for all services.

### Context Switching

The TUI handles context switching through:
- `connection_flow.go`: Functions to manage the connection flow
- `handlers.go`: Event handlers for keyboard shortcuts
- Asynchronous operations to update the UI as contexts change

### Viewport Management

Scrollable log views are implemented using Bubble Tea's viewport component:
- `mainLogViewport`: For the in-line log panel
- `logViewport`: For the expandable log overlay
- Mouse wheel scrolling support in both viewports

### Debug Features

The TUI includes debugging capabilities:
- Toggle debug mode with 'z' key
- View raw dimensions and layout calculations
- Detect and display color profile information

## File Structure

- `model.go`: Core model definition, Update() and View() methods
- `view_helpers.go`: Helper functions for rendering UI components
- `handlers.go`: Key press and event handlers
- `styles.go`: UI styling definitions
- `portforward_handlers.go`: Port forwarding logic
- `connection_flow.go`: Cluster connection handling
- `message_types.go`: Custom message type definitions

## Design Decisions

### Use of Bubble Tea & Lipgloss

These libraries were chosen for:
- Strong typing and Go-native approach
- Excellent terminal compatibility
- Component composition model
- Elegant handling of async events

### Separate UI and Logic Components

The codebase separates:
- UI rendering (view helpers)
- State management (model)
- Business logic (handlers)

This separation enables easier testing and maintenance.

### Asynchronous Communication

The TUI uses channels for non-blocking operations to:
- Keep the UI responsive while long-running operations execute
- Allow real-time updates of port forward status
- Support health checking in the background

## Troubleshooting

### Common Issues

- **Layout Issues**: If panels appear misaligned, it may be due to terminal font settings or Unicode rendering
- **Color Problems**: Some terminals may not support all colors; use 'D' to toggle between modes
- **Performance**: Large log output can impact performance; consider increasing buffer size if needed

### Debugging

1. Enable debug mode with 'z' key
2. Check terminal dimensions and layout calculations
3. Review logs for any error messages

## Future Enhancements

- Clickable UI elements for easier navigation
- Additional panel types for other service statuses
- Draggable/resizable panels
- Configuration options for colors and layout
- Search functionality in logs 