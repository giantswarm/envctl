# TUI Implementation Details

This document provides technical details about the Terminal User Interface (TUI) implementation in `envctl` for developers who want to understand, maintain, or extend the codebase.

## Technology Stack

The TUI is built using:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea): TUI framework following the Model-View-Update (MVU) pattern
- [Lipgloss](https://github.com/charmbracelet/lipgloss): Styling library for terminal applications
- [Bubbles](https://github.com/charmbracelet/bubbles): Reusable components for Bubble Tea (viewport, textinput)

## Code Organization

The TUI code is organized in the `internal/tui` package with these key files:

- `model.go`: Core model definition with `Init()`, `Update()`, and `View()` methods
- `view_helpers.go`: Helper functions for rendering UI components
- `handlers.go`: Event handlers for key presses and messages
- `styles.go`: UI styling definitions
- `portforward_handlers.go`: Port forwarding logic
- `connection_flow.go`: Cluster connection flow management
- `message_types.go`: Custom message type definitions

## Model Structure

The `model` struct in `model.go` contains all state data:

```go
type model struct {
    // Cluster Information
    managementCluster  string
    workloadCluster    string
    kubeContext        string
    currentKubeContext string

    // Health Information
    MCHealth clusterHealthInfo
    WCHealth clusterHealthInfo

    // Port Forwarding
    portForwards     map[string]*portForwardProcess
    portForwardOrder []string
    focusedPanelKey  string

    // UI State & Output
    combinedOutput []string
    quitting       bool
    ready          bool
    width          int
    height         int
    debugMode      bool
    colorMode      string
    helpVisible    bool
    logOverlayVisible bool
    logViewport       viewport.Model
    mainLogViewport   viewport.Model

    // New Connection Input State
    isConnectingNew    bool
    newConnectionInput textinput.Model
    currentInputStep   newInputStep
    stashedMcName      string
    clusterInfo        *utils.ClusterInfo

    // Message Channel
    TUIChannel chan tea.Msg
}
```

## The MVU Pattern

The implementation follows Elm's MVU (Model-View-Update) architecture as implemented by Bubble Tea:

1. **Model**: Holds the application state
2. **Update**: Handles messages and updates the model
3. **View**: Renders the current model to the terminal

### The Update Loop

The update loop is implemented in the `Update()` method of `model.go`. It handles various message types:

```go
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Handle key presses
    case tea.WindowSizeMsg:
        // Handle window resizes
    case portForwardStatusUpdateMsg:
        // Handle port forward updates
    // Other message types...
    }
    // Always return a command to read from the channel
    return m, channelReaderCmd(m.TUIChannel)
}
```

### The View Rendering

The `View()` method in `model.go` renders the entire UI based on the current model state:

```go
func (m model) View() string {
    // Check for special states (quitting, not ready)
    // Calculate layout dimensions
    // Render each UI section
    // Combine sections and apply final styling
    // Handle overlay rendering (help, logs)
    return finalView
}
```

## Asynchronous Communication

A key feature is the asynchronous message handling system that allows background operations to update the UI:

1. The model has a `TUIChannel chan tea.Msg` field
2. Background goroutines send messages to this channel
3. The `channelReaderCmd` continuously reads from this channel:

```go
func channelReaderCmd(ch chan tea.Msg) tea.Cmd {
    return func() tea.Msg {
        return <-ch
    }
}
```

4. Messages are processed by the `Update()` method

## Port Forwarding Implementation

Port forwarding is implemented with these key components:

### Port Forward Process

Each port forward is represented by a `portForwardProcess` struct:

```go
type portForwardProcess struct {
    label        string
    serviceName  string
    namespace    string
    port         string
    context      string
    cmd          *exec.Cmd
    output       []string
    statusMsg    string
    running      bool
    // Other fields...
}
```

### Starting Port Forwards

Port forwards are started using the `startPortForward` function, which:
1. Creates a new process with the appropriate kubectl command
2. Sets up pipes for stdout/stderr
3. Starts the process in a goroutine
4. Monitors its output and status
5. Sends status updates to the TUI channel

### Restarting Port Forwards

Port forwards can be restarted using the 'r' key when a panel is focused:

```go
if key == "r" {
    if pf, exists := m.portForwards[m.focusedPanelKey]; exists {
        // Stop existing process
        // Start a new one
        // Update model
    }
}
```

## Layout & Styling

The layout is calculated dynamically based on terminal dimensions:

### Vertical Layout

The screen is divided vertically into sections with allocated heights:
1. Header (fixed height)
2. Cluster info row (~20% of remaining height)
3. Port forwarding row (~30% of remaining height)
4. Log panel (remaining space)

### Horizontal Layout

Each row is further divided horizontally:
- Cluster info: MC and WC panes (50% each)
- Port forwarding: 3 equal columns

### Styling

All styling is defined in `styles.go` using Lipgloss:

```go
var (
    // Base styles
    appStyle = lipgloss.NewStyle().Margin(0, 0)
    headerStyle = lipgloss.NewStyle().Bold(true).Foreground(...)
    
    // Panel styles
    panelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
    
    // Status-specific styles
    panelStatusRunningStyle = panelStyle.Copy().Background(...)
    // More styles...
)
```

## Responsive Design Features

The UI adapts to different terminal sizes:

1. **Minimal Mode**: For very small terminals, only shows header
2. **Compact Mode**: For small terminals, shows header and cluster info
3. **Full Mode**: For normal terminals, shows all components
4. **Log Overlay**: When terminal too small for log panel (or 'L' key pressed)

The implementation uses these strategies:

```go
// Handle extremely small windows
if totalAvailableHeight < 5 || contentWidth < 20 {
    return renderHeader(m, contentWidth)
}

// Handle small windows
if totalAvailableHeight < 15 {
    // Render minimal UI...
}

// Full UI for normal windows...
```

## Dark Mode Implementation

Dark mode is implemented using Lipgloss's adaptive colors:

```go
Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"})
```

Toggling dark mode is done by:

```go
isDark := lipgloss.HasDarkBackground()
lipgloss.SetHasDarkBackground(!isDark)
```

## Mouse Support

Mouse events are handled for scrolling viewports:

```go
case tea.MouseMsg:
    if m.logOverlayVisible {
        m.logViewport, cmd = m.logViewport.Update(msg)
    } else {
        m.mainLogViewport, cmd = m.mainLogViewport.Update(msg)
    }
```

## Error Handling Strategy

Errors are handled through:
1. Adding error messages to the combined output log
2. Visual styling (red panels for port forward errors)
3. Status message displays in the port forward panels

## Testing

When testing UI changes:
1. Ensure the layout works at various terminal dimensions
2. Test edge cases (empty clusters, long names, errors)
3. Check both light and dark modes
4. Verify keyboard navigation works as expected

## Common Pitfalls and Solutions

### Height Calculation Issues

When working with Lipgloss layouts, the height calculation is crucial:
- Use `lipgloss.Height()` to measure rendered components
- Account for borders and padding in calculations
- When in doubt, add debug measurements to inspect actual vs. expected heights

### Width Alignment Problems

For width alignment:
- Account for frame size with `GetHorizontalFrameSize()`
- Use consistent widths for all rows
- Distribute space evenly between panels

### Performance Concerns

For performance:
- Limit the size of log buffers
- Avoid expensive rendering in hot loops
- Use focused updates when possible

## Customization Points

To customize the TUI:

### Styling Changes

Modify `styles.go` to change:
- Colors and themes
- Border styles
- Text formatting

### Layout Modification

Edit `View()` in `model.go` to change:
- Section layouts and proportions
- Component organization
- Addition of new panels

### Adding Features

To add new features:
1. Add new fields to the `model` struct
2. Create message types for async operations
3. Add handlers in the `Update()` method
4. Update the `View()` method to render new components 