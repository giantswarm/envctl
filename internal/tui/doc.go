// Package tui provides the Terminal User Interface for envctl.
//
// This package implements an interactive terminal interface using the Bubble Tea
// framework, providing users with real-time monitoring and control of their
// Giant Swarm cluster connections, port forwards, and MCP servers.
//
// # Architecture
//
// The TUI follows a Model-View-Controller (MVC) pattern:
//
//   - Model: Maintains application state and business logic
//   - View: Renders the UI components and handles layout
//   - Controller: Manages user input and orchestrates updates
//
// # Core Components
//
// Model (internal/tui/model/):
//   - Maintains the current state of all services
//   - Interfaces with the orchestrator for service management
//   - Handles state updates from various sources
//
// View (internal/tui/view/):
//   - Renders panels for different service types
//   - Manages layout and styling
//   - Provides overlays for help, logs, and configuration
//
// Controller (internal/tui/controller/):
//   - Processes keyboard input
//   - Dispatches commands to appropriate handlers
//   - Manages the Bubble Tea program lifecycle
//
// # Key Features
//
// Service Monitoring:
//   - Real-time status updates for all services
//   - Health indicators with color coding
//   - Dependency visualization
//   - Error state reporting
//
// Interactive Controls:
//   - Start/stop/restart services
//   - Switch Kubernetes contexts
//   - View logs and configuration
//   - Navigate between service panels
//
// Visual Design:
//   - Responsive layout that adapts to terminal size
//   - Color-coded status indicators
//   - Dark/light theme support
//   - Consistent styling across components
//
// # Message Flow
//
// The TUI uses a message-based architecture for updates:
//
//  1. External events (service state changes, health updates) are sent via channels
//  2. The controller converts these to Bubble Tea messages
//  3. The model processes messages and updates state
//  4. The view renders the updated state
//
// # Keyboard Navigation
//
// The TUI supports intuitive keyboard shortcuts:
//
//   - Tab/Shift+Tab: Navigate between panels
//   - Arrow keys: Alternative navigation
//   - Enter/Space: Activate focused element
//   - r: Restart selected service
//   - s: Switch Kubernetes context
//   - h: Show help overlay
//   - L: Show log viewer
//   - q/Ctrl+C: Quit application
//
// # Panel Types
//
// The TUI displays different panel types for each service category:
//
// K8s Connection Panel:
//   - Shows cluster name and context
//   - Displays connection health
//   - Indicates if it's MC or WC
//
// Port Forward Panel:
//   - Shows service name and namespace
//   - Displays local and remote ports
//   - Indicates target context
//   - Shows connection status
//
// MCP Server Panel:
//   - Shows server name and type
//   - Displays proxy port
//   - Lists available tools (when healthy)
//   - Shows dependencies
//
// # Design System
//
// The TUI uses a comprehensive design system (internal/tui/design/):
//
//   - Semantic color palette with adaptive light/dark support
//   - Consistent spacing based on 4px units
//   - Reusable component library (internal/tui/components/)
//   - Unified icon set with proper spacing
//   - Typography scales and text styles
//
// Key design principles:
//   - Consistency across all UI elements
//   - Accessibility with high contrast colors
//   - Modular components for maintainability
//   - Clean, minimal interface design
//
// # State Management
//
// The TUI maintains synchronized state with the orchestrator:
//
//   - Uses StateStore as single source of truth
//   - Subscribes to state change events
//   - Reconciles UI state on startup
//   - Handles out-of-order updates
//
// # Error Handling
//
// The TUI provides user-friendly error handling:
//
//   - Visual error indicators on affected services
//   - Detailed error messages in status bar
//   - Log viewer for debugging
//   - Graceful degradation for non-critical errors
//
// # Usage Example
//
//	// Create and run the TUI program
//	p, err := controller.NewProgram(
//	    mcName,
//	    wcName,
//	    initialContext,
//	    debugMode,
//	    config,
//	    logChannel,
//	)
//	if err != nil {
//	    return err
//	}
//
//	// Run the TUI (blocks until user quits)
//	if _, err := p.Run(); err != nil {
//	    return err
//	}
package tui
