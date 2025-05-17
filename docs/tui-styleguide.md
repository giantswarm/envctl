# Envctl TUI Style Guide

## 1. Introduction

This style guide outlines the visual design principles, color palette, and styling conventions used in the `envctl` Terminal User Interface (TUI). The goal is to maintain a consistent, visually appealing, and user-friendly experience.

The TUI is built using Go with the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework and styled using [Lipgloss](https://github.com/charmbracelet/lipgloss). All styles are defined in `internal/tui/styles.go` unless otherwise noted.

A key principle is the use of **adaptive colors** to ensure readability and a good aesthetic in both light and dark terminal themes, with a primary focus on an excellent dark mode experience inspired by modern terminal applications.

## 2. Color Palette

The TUI primarily uses a dark theme aesthetic, drawing inspiration from themes like the one showcased in [this article](https://themarkokovacevic.com/posts/terminal-ui-with-bubbletea/).

### Primary & Accent Colors (Dark Theme Focus):
- **Backgrounds (General Panels/UI):** Dark greys, slightly off-black (e.g., `#1F1F1F`, `#2A2A2A`).
- **Text (General):** Light greys, off-whites for good contrast on dark backgrounds (e.g., `#FAFAFA`, `#E0E0E0`, `#C0C0C0`).
- **Accent/Focus:** Bright Magenta/Pink (e.g., `#FF00FF`, Lipgloss color `205` for spinner). This is used for focused panel borders.
- **Borders (Default):** Muted greys or blues (e.g., `#5C5C5C`).

### Status Colors:
These are primarily used for the status bar background and status-indicative elements like panel borders or specific text.
- **Success / Up:** Green (e.g., Background: `StatusBarSuccessBg` - Dark: `#059669`; Text: `StatusMessageSuccessStyle` - Dark: `#E0E0E0`, Health: `healthGoodStyle` - Dark: `#8AE234`).
- **Error / Failed:** Red (e.g., Background: `StatusBarErrorBg` - Dark: `#DC2626`; Text: `StatusMessageErrorStyle` - Dark: `#F0F0F0`, Health: `healthErrorStyle` - Dark: `#FF8787`).
- **Warning / Degraded:** Yellow/Amber (e.g., Background: `StatusBarWarningBg` - Dark: `#D97706`; Text: `StatusMessageWarningStyle` - Dark: `#111827` (dark text on yellow bg), Health: `healthWarnStyle` - Dark: `#FFB86C`).
- **Info / Connecting:** Blue (e.g., Background: `StatusBarInfoBg` - Dark: `#2563EB`; Text: `StatusMessageInfoStyle` - Dark: `#E0E0E0`).
- **Default / Neutral:** Dark Grey/Blue (e.g., Background: `StatusBarDefaultBg` - Dark: `#374151`).

All color definitions use `lipgloss.AdaptiveColor{Light: "...", Dark: "..."}`.

## 3. Layout Principles & Key Components

### 3.1. Header
- **Purpose:** Displays the application title and global key hints (Help, Navigate, Quit). Shows a spinner when global loading (`m.isLoading`) is active.
- **Style:** `headerStyle` (defined in `styles.go`). Features a subtle background distinct from the main content area. Spinner color is magenta, set in `model.go`.
- **Key Hints:** May include debug mode key hints if `m.debugMode` is true.

### 3.2. Status Bar
- **Purpose:** Provides a persistent overview of the application's overall status, active cluster context, and displays transient messages.
- **Structure (L-R):**
    - **Left:** Overall application status ("Up", "Connecting", "Degraded", "Failed") or loading spinner. Text styled with `StatusBarTextStyle`.
    - **Center:** Transient status messages (e.g., operation success/failure). Text styled with `StatusMessage*Style` (for foreground color) and centered in the available space.
    - **Right:** Active MC/WC information. Text styled with `StatusBarTextStyle`.
- **Background:** The entire status bar's background color dynamically changes based on the `OverallAppStatus` (Success = Green, Error = Red, etc.), using `StatusBar*Bg` adaptive colors.
- **Styles:** `StatusBarBaseStyle`, `StatusBarTextStyle`, `StatusMessage*Style`, `StatusBar*Bg` (all in `styles.go`).
- **Implementation:** `renderStatusBar` in `view_helpers.go`.

### 3.3. Main Content Panels
Consists of rows displaying information about:
    - Management Cluster (MC) & Workload Cluster (WC) Info
    - Port Forwarding Processes
    - MCP Server Proxies

- **General Panel Style:**
    - Rounded borders, consistent padding. Base style: `panelStyle`.
    - Default border color: Muted grey (`BorderForeground` in `panelStyle`).
    - Background: Generally subtle or transparent to blend with the terminal, making content and borders the primary visual differentiators. (e.g., `contextPaneStyle` uses `#1F1F1F` for dark mode).
- **Focus Indication:**
    - The currently focused panel (navigated via Tab/j/k) is highlighted with a **thick magenta border**.
    - Style: `focusedPanelStyle`. This is inherited by specific focused panel styles.
- **Status Indication (for Port Forwards, MCP Proxies):**
    - Panel borders change color to reflect the operational status of the process within (e.g., running, error, initializing).
    - Styles: `panelStatusRunningStyle` (Green border), `panelStatusErrorStyle` (Red border), `panelStatusInitializingStyle` (Blue border), etc.
    - Text within panels indicating status (e.g., "Status: Running") also uses specific colors defined in `statusMsg*Style`.
- **Content Titles:** Panel titles (e.g., "Prometheus (MC)") use `portTitleStyle`.
- **Health Info:** MC/WC health (node readiness) uses `health*Style` for text color (Good, Warn, Error, Loading).
- **Layout:** Panels are arranged in rows by `renderContextPanesRow`, `renderPortForwardingRow`, `renderMcpProxiesRow` in `view_helpers.go`.

### 3.4. Activity Log
- **Main View:** `renderCombinedLogPanel` in `view_helpers.go`. Displays a scrollable list of recent log messages. Title styled with `logPanelTitleStyle`. Log lines use `logLineStyle`.
- **Log Overlay (`L` key):**
    - A full-screen or large centered overlay for viewing more log history.
    - Container styled by `logOverlayStyle`. Uses `m.logViewport`.

### 3.5. Help Overlay (`h` key)
- **Purpose:** Displays keyboard shortcuts.
- **Implementation:** Uses `bubbles/help.Model`.
    - Key definitions and help text: `KeyMap` struct and `DefaultKeyMap()` in `keymap.go`.
    - `m.help.Styles` is customized in `InitialModel` (`model.go`) for text colors.
- **Appearance:**
    - Rendered by `m.help.View(m.keys)`.
    - A title "KEYBOARD SHORTCUTS" (styled by `helpTitleStyle`) is prepended.
    - The combined view is wrapped in `centeredOverlayContainerStyle` and placed in the center of the screen using `lipgloss.Place`.
    - The area around the help box is intended to be dimmed by `lipgloss.WithWhitespaceBackground` (terminal-dependent).

### 3.6. New Connection Input
- **Mode:** `ModeNewConnectionInput`.
- **Appearance:** Uses `bubbles/textinput.Model`. Rendered by `renderNewConnectionInputView` in `view_helpers.go`, which provides a clear prompt and a box around the input field.

## 4. Key Style Variables (from `styles.go`)

This is not exhaustive but lists some of the most important global style definitions:

- `appStyle`: Overall application container.
- `headerStyle`: Top header bar.
- `StatusBarBaseStyle`, `StatusBarTextStyle`, `StatusMessage*Style`, `StatusBar*Bg`: For the bottom status bar.
- `panelStyle`: Base for all content panels.
- `focusedPanelStyle`: Highlight for focused panels (magenta border).
- `panelStatus*Style`: Variants of `panelStyle` indicating operational status via border color.
- `statusMsg*Style`: Text colors for status messages within panels.
- `contextPaneStyle`, `activeContextPaneStyle`, `focusedContextPaneStyle`, `focusedAndActiveContextPaneStyle`: For MC/WC info panes.
- `health*Style`: Text colors for K8s node health status.
- `portTitleStyle`, `logPanelTitleStyle`: Titles for specific sections.
- `logOverlayStyle`, `centeredOverlayContainerStyle`, `helpTitleStyle`: For overlay views.

## 5. Typography & Spacing
- **Fonts:** Relies on the user's terminal font. Styles use `Bold(true)` where emphasis is needed.
- **Padding & Margins:** Defined within individual `lipgloss.Style` definitions (e.g., `Padding(0,1)`, `MarginBottom(1)`). Aim for clear separation without excessive whitespace. `StatusBarTextStyle` and `panelStyle` use `Padding(0,1)` for horizontal padding around text content.

By adhering to these guidelines, `envctl` aims for a TUI that is both functional and aesthetically pleasing. 