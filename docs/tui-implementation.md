# TUI Implementation Details

This document provides technical details about the Terminal User Interface (TUI) implementation in `envctl` for developers who want to understand, maintain, or extend the codebase.

## Technology Stack

The TUI is built using:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea): TUI framework following the Model-View-Update (MVU) pattern
- [Lipgloss](https://github.com/charmbracelet/lipgloss): Styling library for terminal applications
- [Bubbles](https://github.com/charmbracelet/bubbles): Reusable components for Bubble Tea (viewport, textinput)

## Code Organization

The TUI code is organized in the `internal/tui` package, which is further divided into `model`, `view`, and `controller` sub-packages:

- **`internal/tui/model/`**:
    - `types.go`: Defines the core `Model` struct, other data-holding structs (e.g., `ClusterHealthInfo`, `PortForwardProcess`, `McpServerProcess`), and UI-specific enums (e.g., `AppMode`, `MessageType`).
    - `messages.go`: Contains all custom `tea.Msg` types for TUI internal communication.
    - `init.go`: Provides `InitialModel()` for default model state.
    - `export.go`: Defines other model-related types like `InputStep`.
- **`internal/tui/view/`**:
    - `render.go`: Contains the main `Render(m *model.Model)` function.
    - `styles.go`: Centralizes all `lipgloss` UI styling definitions.
    - Component-specific rendering files (e.g., `context.go`, `portforward.go`, `mcp.go`, `log.go`, `status.go`, `misc.go`, `icons.go`) for different UI parts.
- **`internal/tui/controller/`**:
    - `app.go`: Defines `AppModel`, the top-level `tea.Model` for Bubble Tea. Its `Init()`, `Update()`, and `View()` methods orchestrate the application.
    - `update.go`: Houses `mainControllerDispatch()`, the central message routing logic.
    - `commands.go`: Functions that generate `tea.Cmd` for asynchronous operations.
    - Handler files for specific concerns (e.g., `keyglobal.go`, `keyinput.go` for key presses; `connection.go`, `cluster.go`, `portforward.go`, `mcpserver.go` for domain logic).
    - `logger.go`: TUI-specific logging utilities.
    - `program.go`: `NewProgram()` to create the `tea.Program` instance.

## Model Structure

The `model.Model` struct in `internal/tui/model/types.go` contains all state data. Its structure is (simplified for brevity, see `types.go` for full details):

```go
type Model struct {
    // --- Cluster Information ---
    ManagementClusterName    string
    WorkloadClusterName      string
    CurrentKubeContext       string
    MCHealth                 ClusterHealthInfo
    WCHealth                 ClusterHealthInfo

    // --- Port Forwarding & MCP ---
    PortForwards             map[string]*PortForwardProcess
    PortForwardOrder         []string
    McpServers               map[string]*McpServerProcess
    McpProxyOrder            []string
    FocusedPanelKey          string // Tracks which UI panel has focus

    // --- UI State & Output ---
    ActivityLog              []string
    Width, Height            int
    DebugMode                bool
    ColorMode                string // e.g., "Dark", "Light"
    CurrentAppMode           AppMode    // e.g., ModeMainDashboard, ModeHelpOverlay
    LogViewport              viewport.Model
    MainLogViewport          viewport.Model
    McpConfigViewport        viewport.Model
    StatusBarMessage         string
    StatusBarMessageType     MessageType
    IsLoading                bool
    Spinner                  spinner.Model

    // --- New Connection Input State ---
    NewConnectionInput       textinput.Model
    CurrentInputStep         InputStep
    StashedMcName            string
    ClusterInfo              *utils.ClusterInfo // For autocomplete

    // --- Infrastructure & Services ---
    Keys                     KeyMap // Keybindings
    Help                     help.Model
    TUIChannel               chan tea.Msg // For async messages from goroutines
    Services                 service.Services // Access to backend services
    DependencyGraph          *dependency.Graph
    // ... other fields for specific UI components and states
}
```

## The MVU Pattern

The implementation follows Elm's MVU (Model-View-Update) architecture as implemented by Bubble Tea, mapped to an MVC structure:

1.  **Model** (`internal/tui/model/`): Holds the application state (`model.Model`).
2.  **Update** (`internal/tui/controller/`): The `controller.AppModel.Update()` method delegates to `controller.mainControllerDispatch()` which handles messages and updates the `model.Model`.
3.  **View** (`internal/tui/view/`): The `controller.AppModel.View()` method calls `view.Render()` with the current `model.Model` to render the UI to the terminal.

### The Update Loop

The main update loop is initiated by `controller.AppModel.Update()`, which calls `controller.mainControllerDispatch()` (in `internal/tui/controller/update.go`). This function contains the primary switch statement for handling various `tea.Msg` types:

```go
// In internal/tui/controller/update.go (simplified)
func mainControllerDispatch(m *model.Model, msg tea.Msg) (*model.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Delegate to key handlers like handleKeyMsgGlobal or handleKeyMsgInputMode
    case tea.WindowSizeMsg:
        // Delegate to handleWindowSizeMsg
    case model.PortForwardCoreUpdateMsg: // Note: Messages are now in model package
        // Delegate to handlePortForwardCoreUpdateMsg
    // Other message types...
    }
    // ...
    return m, tea.Batch(cmds...)
}
```
The `controller.AppModel` then returns the updated model and commands to Bubble Tea.

### The View Rendering

The `controller.AppModel.View()` method (in `internal/tui/controller/app.go`) calls `view.Render()` (in `internal/tui/view/render.go`) with the current `model.Model`. `view.Render()` then constructs the UI:

```go
// In internal/tui/view/render.go (simplified)
func Render(m *model.Model) string {
    // Check CurrentAppMode (e.g., ModeQuitting, ModeMainDashboard, ModeHelpOverlay)
    // Calculate layout dimensions based on m.Width, m.Height
    // Call other rendering functions in the view package (e.g., renderHeader, renderContextPanesRow)
    // Combine rendered parts and apply final styling using lipgloss
    // Handle overlay rendering (help, logs, MCP config)
    return finalViewString
}
```

## Asynchronous Communication

A key feature is the asynchronous message handling system:

1.  The `model.Model` has a `TUIChannel chan tea.Msg` field.
2.  Background goroutines (often launched by `tea.Cmd`s created in `internal/tui/controller/commands.go`) send messages (defined in `internal/tui/model/messages.go`) to this channel.
3.  A `tea.Cmd` (often `channelReaderCmd` returned by `model.Model.Init()`) continuously reads from this channel:

```go
// In internal/tui/model/init.go (conceptual)
func channelReaderCmd(ch chan tea.Msg) tea.Cmd {
    return func() tea.Msg {
        return <-ch // Read a message from the channel
    }
}
```
This `tea.Msg` is then processed by the `controller.mainControllerDispatch()` in the update loop.

## Port Forwarding Implementation

Port forwarding is implemented with these key components:

### Port Forward Process

Each port forward is represented by a `model.PortForwardProcess` struct (in `internal/tui/model/types.go`):

```go
// In internal/tui/model/types.go
type PortForwardProcess struct {
    Label       string
    Command     string // For display/debug, not direct execution by TUI
    LocalPort   int
    RemotePort  int
    TargetHost  string
    ContextName string
    Pid         int
    CmdInstance *exec.Cmd // If TUI were to manage it directly, now handled by service.PF
    StopChan    chan struct{}
    Log         []string
    Active      bool // User-configured to be active
    Running     bool // Actual running state
    StatusMsg   string
    Err         error
    Config      portforwarding.PortForwardConfig // Holds the actual config
}
```

### Starting Port Forwards

Port forwards are started by `tea.Cmd`s generated in `internal/tui/controller/commands.go` (e.g., `GetInitialPortForwardCmds`, `createRestartPortForwardCmd`). These commands typically:
1.  Use the `service.PortForwardingService` (accessed via `m.Services.PF`) to initiate the port forwarding.
2.  The service handles the `exec.Cmd` and manages stdout/stderr.
3.  The service uses a callback (provided by the controller command) to send `model.PortForwardCoreUpdateMsg` messages back to the `m.TUIChannel` with status, logs, and errors.
4.  An initial `model.PortForwardSetupResultMsg` is sent upon attempting to start.

### Restarting Port Forwards

Port forwards can be restarted using the 'r' key. This is handled in `internal/tui/controller/keyglobal.go`:
- It finds the focused `model.PortForwardProcess`.
- If found, it closes its existing `StopChan` (signaling the service to stop it).
- It then returns a `tea.Cmd` (created by `createRestartPortForwardCmd`) to start a new instance via the `service.PortForwardingService`.

## Layout & Styling

The layout is calculated dynamically based on terminal dimensions within functions in the `internal/tui/view/` package.
For a detailed breakdown of colors, component styles, and visual guidelines, please refer to the [TUI Style Guide](./tui-styleguide.md). Styles are defined in `internal/tui/view/styles.go`.

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

All styling is defined in `internal/tui/view/styles.go` using Lipgloss:

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

Dark mode is implemented using Lipgloss's adaptive colors, defined in `internal/tui/view/styles.go`.
Toggling dark mode (handled in `internal/tui/controller/keyglobal.go` or similar after MVC refactor, by updating a field in `model.Model` and letting Lipgloss handle re-render with adaptive styles) involves:
```go
// This is a conceptual representation. The actual toggle changes a model field,
// and lipgloss.SetHasDarkBackground is typically called once at startup.
// The styles themselves are adaptive.
// lipgloss.SetHasDarkBackground(!lipgloss.HasDarkBackground())
// m.colorMode = ... // Update model if it tracks this explicitly
```
The `lipgloss.SetHasDarkBackground()` is usually called once at application startup (e.g., in `main.go` or `controller/program.go`). The styles in `view/styles.go` use `lipgloss.AdaptiveColor` to react to this global setting.

## Mouse Support

Mouse events are handled in `controller.mainControllerDispatch()` (in `internal/tui/controller/update.go`) for scrolling viewports:

```go
// In internal/tui/controller/update.go
case tea.MouseMsg:
    if m.CurrentAppMode == model.ModeLogOverlay {
        m.LogViewport, cmd = m.LogViewport.Update(msg)
    } else if m.CurrentAppMode == model.ModeMcpConfigOverlay {
        m.McpConfigViewport, cmd = m.McpConfigViewport.Update(msg)
    } else { // Assuming main dashboard view
        m.MainLogViewport, cmd = m.MainLogViewport.Update(msg)
    }
    cmds = append(cmds, cmd)
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

Modify `internal/tui/view/styles.go` to change:
- Colors and themes
- Border styles
- Text formatting

### Layout Modification

Edit rendering functions within `internal/tui/view/` (primarily `render.go` and its helpers like `context.go`, `portforward.go`, etc.) to change:
- Section layouts and proportions
- Component organization
- Addition of new panels

### Adding Features

To add new features:
1.  Add new fields to the `model.Model` struct in `internal/tui/model/types.go`.
2.  Create new `tea.Msg` types in `internal/tui/model/messages.go` if new asynchronous operations or events are needed.
3.  Add handlers for these messages or new key presses in relevant files within `internal/tui/controller/` (e.g., `update.go`, `keyglobal.go`, specific logic files). These handlers will update the model.
4.  Update rendering functions in `internal/tui/view/` to display the new components or state.
5.  If new asynchronous operations are needed, create new command-generating functions in `internal/tui/controller/commands.go`.

## Interfacing with Core Logic Packages (e.g., `internal/mcpserver`)

To maintain a clean separation of concerns and ensure the TUI package (`internal/tui`) does not become overly entangled with business logic or process management details, `envctl` adopts a pattern where core functionalities are housed in separate, agnostic internal packages. The `internal/mcpserver` package, responsible for managing Management Cluster Proxy (MCP) servers, is a prime example.

**Key Principles for TUI Interaction:**

1.  **Core Package Agnosticism:** The core package (e.g., `mcpserver`) is designed to be unaware of the TUI. It does not use `tea.Msg` types or have any direct dependency on Bubble Tea. Its functions for starting and managing processes use generic callbacks for reporting updates.

2.  **TUI as a Consumer:** The TUI logic, primarily within `internal/tui/commands.go`, acts as a consumer of the core package.
    *   It reads configurations (like `mcpserver.PredefinedMcpServers`) from the core package to understand what entities (e.g., MCP servers) are available or need to be managed.
    *   For each entity that requires an asynchronous operation (like starting an MCP server), the TUI creates a `tea.Cmd`.

3.  **`tea.Cmd` Wraps Core Logic:** The `tea.Cmd` generated by the TUI (e.g., in `internal/tui/controller/commands.go` for an MCP server via `StartMcpProxiesCmd`) will typically:
    *   Call a function from the core package that performs the actual work (e.g., `m.Services.Proxy.Start` which wraps `mcpserver.StartAndManageIndividualMcpServer`).
    *   Provide a TUI-specific implementation of the generic callback function required by the core package. For instance, it provides an `mcpserver.McpUpdateFunc` that takes the generic `mcpserver.McpProcessUpdate` and translates it into a TUI-specific `tea.Msg` (like `model.McpServerStatusUpdateMsg`). This message is then sent to the TUI's main event channel (`m.TUIChannel`).
    *   Return an initial `tea.Msg` (e.g., `model.McpServerSetupCompletedMsg`) to the TUI model. This message signals that the asynchronous operation has been initiated and can include essential items like a `stopChan` for the process or any immediate errors encountered during the setup phase.
4.  **State Management in TUI Model:**
    *   The TUI model (`model.Model` in `internal/tui/model/types.go`) receives the initial setup messages and subsequent update messages.
    *   It uses these messages (handled by the controller) to update its internal state regarding each managed process (e.g., storing PIDs, stop channels, status messages, logs for each MCP server in `m.McpServers`).
    *   The `view.Render()` method then uses this state to render the UI components for each managed process.

**Example Flow for Starting an MCP Server in TUI Mode:**

1.  The controller decides to start MCP servers (e.g., during `AppModel.Init()` or in response to a message) and generates commands using `controller.StartMcpProxiesCmd`.
2.  `controller.StartMcpProxiesCmd` (in `internal/tui/controller/commands.go`):
    *   Iterates configurations like `mcpserver.PredefinedMcpServers`.
    *   For each `serverCfg`, it returns a `tea.Cmd` (let's call it `proxyStartTeaCmd`).
3.  When Bubble Tea executes `proxyStartTeaCmd`:
    *   A TUI-specific `mcpserver.McpUpdateFunc` (named `tuiUpdateFn` inside the command) is defined. This function will convert `mcpserver.McpProcessUpdate` into `model.McpServerStatusUpdateMsg` and send it to `m.TUIChannel` (where `m` is the model instance captured by the command's closure).
    *   `m.Services.Proxy.Start(capturedServerCfg, tuiUpdateFn)` is called (which internally might call something like `mcpserver.StartAndManageIndividualMcpServer`).
    *   `proxyStartTeaCmd` immediately returns a `model.McpServerSetupCompletedMsg` with the `pid`, `stopChan`, and `initialError`.
4.  The TUI's `controller.mainControllerDispatch()` handles `model.McpServerSetupCompletedMsg`:
    *   Updates the state in the `model.Model` for that specific MCP server.
5.  As the MCP server runs, the core service calls `tuiUpdateFn` with logs and status changes.
6.  `tuiUpdateFn` sends `model.McpServerStatusUpdateMsg` to `m.TUIChannel`.
7.  The TUI's `controller.mainControllerDispatch()` handles these `model.McpServerStatusUpdateMsg` messages:
    *   Further updates the state in the `model.Model` for the specific MCP server.

This pattern ensures that the TUI remains responsive, handles asynchronous operations in a way that fits the Bubble Tea architecture, and keeps the core process management logic decoupled and reusable by other parts of the application (like the non-TUI mode).

By using this approach, the `internal/tui/{controller, model, view}` packages focus on their respective MVC roles, while `internal/mcpserver` (and other services) focus on their specific domain logic, promoting modularity and maintainability. 