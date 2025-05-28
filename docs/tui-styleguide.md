# Envctl TUI Style Guide

## 1. Introduction

This style guide outlines the visual design principles, color palette, and styling conventions used in the `envctl` Terminal User Interface (TUI). The goal is to maintain a consistent, visually appealing, and user-friendly experience.

The TUI is built using Go with the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework and styled using [Lipgloss](https://github.com/charmbracelet/lipgloss). All styles are now centralized in the `internal/color/` package for consistency across the application.

A key principle is the use of **adaptive colors** to ensure readability and a good aesthetic in both light and dark terminal themes, with a primary focus on an excellent dark mode experience inspired by modern terminal applications.

## 2. Color System Architecture

The styling system has been refactored into a centralized color package (`internal/color/`) that provides:

- Consistent color definitions across all UI components
- Adaptive colors that automatically adjust for light/dark modes
- Semantic naming for easy understanding and maintenance
- Reusable style compositions

### Importing Styles

All TUI components import styles from the centralized package:

```go
import "envctl/internal/color"

// Using predefined styles
content := color.FocusedStyle.Render("Focused Panel")
errorMsg := color.ErrorStyle.Render("Error occurred")
```

## 3. Color Palette

The TUI uses adaptive colors that automatically adjust based on the terminal's background (light or dark mode).

### Primary Colors

- **Text (General):** 
  - Light mode: `#000000` (Black)
  - Dark mode: `#FFFFFF` (White)
- **Backgrounds:**
  - Light mode: `#F8F8F8` to `#FFFFFF` (Light grays)
  - Dark mode: `#1E1E1E` to `#2A2A3A` (Dark grays)
- **Focus/Accent:** 
  - Light mode: `#0000CC` (Blue)
  - Dark mode: `#58A6FF` (Bright blue)

### Status Colors

Status colors are used consistently throughout the UI for panels, text, and status bar:

#### Success/Running (Green)
- **Panel Border:** Light: `#307030`, Dark: `#60A060`
- **Panel Background:** Light: `#D4EFDF`, Dark: `#1A3A1A`
- **Text:** Light: `#004400`, Dark: `#8AE234`
- **Status Bar:** Light: `#10B981`, Dark: `#059669`

#### Error/Failed (Red)
- **Panel Border:** Light: `#A04040`, Dark: `#B07070`
- **Panel Background:** Light: `#FADBD8`, Dark: `#4D2A2A`
- **Text:** Light: `#880000`, Dark: `#FF8787`
- **Status Bar:** Light: `#EF4444`, Dark: `#DC2626`

#### Warning/Degraded (Yellow/Amber)
- **Panel Border:** Light: `#A07030`, Dark: `#B0A070`
- **Panel Background:** Light: `#FCF3CF`, Dark: `#4D4D2A`
- **Text:** Light: `#553300`, Dark: `#FFB86C`
- **Status Bar:** Light: `#F59E0B`, Dark: `#D97706`

#### Info/Initializing (Blue)
- **Panel Border:** Light: `#5060A0`, Dark: `#6A78AD`
- **Panel Background:** Light: `#E0E8FF`, Dark: `#2A384D`
- **Text:** Light: `#000080`, Dark: `#82B0FF`
- **Status Bar:** Light: `#3B82F6`, Dark: `#2563EB`

### Health Status Colors

Used specifically for Kubernetes node health indicators:
- **Healthy:** Light: `#005500`, Dark: `#90FF90`
- **Warning:** Light: `#703000`, Dark: `#FFFF00`
- **Error:** Light: `#990000`, Dark: `#FF9090`
- **Loading:** Light: `#303030`, Dark: `#F0F0F0`

### Log Level Colors

For activity log entries:
- **Info:** Light: `#000000`, Dark: `#E0E0E0`
- **Warning:** Light: `#A07000`, Dark: `#FFD066`
- **Error:** Light: `#B30000`, Dark: `#FF6B6B`
- **Debug:** Light: `#606060`, Dark: `#909090` (Italic)

## 4. Component Styles

### 4.1. Header
- **Style:** `color.HeaderStyle`
- **Features:** Bold text, adaptive background (`#D0D0D0`/`#303030`), horizontal padding
- **Purpose:** Application title and global navigation hints

### 4.2. Panels

#### Base Panel
- **Style:** `color.PanelStyle`
- **Features:** Rounded borders, minimal padding (0, 1)
- **Usage:** Foundation for all service panels

#### Focused Panel
- **Style:** `color.FocusedPanelStyle`
- **Features:** Thick border with bright blue color
- **Border:** Light: `#0000CC`, Dark: `#58A6FF`

#### Status-Based Panel Variants
Panels change appearance based on service state:
- **Default:** `color.PanelStatusDefaultStyle`
- **Initializing:** `color.PanelStatusInitializingStyle` (Blue tint)
- **Running:** `color.PanelStatusRunningStyle` (Green tint)
- **Error:** `color.PanelStatusErrorStyle` (Red tint)
- **Exited:** `color.PanelStatusExitedStyle` (Yellow tint)

### 4.3. Context Panes (K8s Connections)
- **Base:** `color.ContextPaneStyle`
- **Active:** `color.ActiveContextPaneStyle` (Blue border)
- **Focused:** `color.FocusedContextPaneStyle`
- **Focused & Active:** `color.FocusedAndActiveContextPaneStyle`

### 4.4. Status Bar
- **Base Style:** `color.StatusBarBaseStyle`
- **Text Style:** `color.StatusBarTextStyle`
- **Background Colors:**
  - Default: `color.StatusBarDefaultBg`
  - Success: `color.StatusBarSuccessBg`
  - Error: `color.StatusBarErrorBg`
  - Warning: `color.StatusBarWarningBg`
  - Info: `color.StatusBarInfoBg`

### 4.5. Overlays

#### Help Overlay
- **Container:** `color.CenteredOverlayContainerStyle`
- **Title:** `color.HelpTitleStyle`
- **Background:** `color.HelpOverlayBgColor`

#### Log Overlay
- **Style:** `color.LogOverlayStyle`
- **Features:** Rounded border, adaptive background, padding

#### MCP Config Overlay
- **Style:** `color.McpConfigOverlayStyle`
- **Features:** Similar to log overlay with code-friendly styling

## 5. Typography & Text Styles

### Titles
- **Port Forward Titles:** `color.PortTitleStyle` (Bold, high contrast)
- **Log Panel Title:** `color.LogPanelTitleStyle` (Bold with padding)

### Status Messages
- **Initializing:** `color.StatusMsgInitializingStyle`
- **Running:** `color.StatusMsgRunningStyle`
- **Error:** `color.StatusMsgErrorStyle`
- **Exited:** `color.StatusMsgExitedStyle`

### Special Text
- **Error Messages:** `color.ErrorStyle`
- **Quit Confirmation:** `color.QuitKeyStyle`

## 6. Layout & Spacing

### Padding Conventions
- **Panels:** `Padding(0, 1)` - Minimal vertical, standard horizontal
- **Header:** `Padding(0, 2)` - Extra horizontal padding
- **Overlays:** `Padding(1, 2)` - Standard padding for readability

### Margins
- **App Container:** `Margin(0, 0)` - Full terminal width
- **Header:** `MarginBottom(0)` - No gap between header and content
- **Titles:** `MarginBottom(1)` - Single line spacing

## 7. Dark Mode Implementation

Dark mode is handled automatically through Lipgloss's adaptive color system:

```go
// Initialize color system (typically in main.go)
color.Initialize(isDarkMode)

// All styles automatically adapt
style := color.PanelStyle // Uses appropriate colors for current mode
```

The system detects the terminal background and adjusts all adaptive colors accordingly.

## 8. Best Practices

### When Adding New Styles

1. **Define in `internal/color/color.go`**: All styles should be centralized
2. **Use Adaptive Colors**: Always use `lipgloss.AdaptiveColor` for theme support
3. **Follow Naming Conventions**: Use descriptive names ending with `Style`
4. **Consider Inheritance**: Build on existing styles using `.Copy()` and `.Inherit()`

### Color Selection Guidelines

1. **Contrast**: Ensure sufficient contrast between text and background
2. **Consistency**: Use semantic colors (success=green, error=red, etc.)
3. **Accessibility**: Test in both light and dark modes
4. **Subtlety**: Use muted backgrounds, vibrant borders/text

### Testing Styles

1. **Both Modes**: Always test in light and dark terminals
2. **Different Terminals**: Test in various terminal emulators
3. **Color Profiles**: Verify with different color profile settings
4. **Focus States**: Ensure focused elements are clearly distinguishable

## 9. Style Composition Example

```go
// Creating a new style by composing existing ones
MyCustomStyle := color.PanelStyle.Copy().
    Inherit(color.FocusedPanelStyle).
    Background(lipgloss.AdaptiveColor{Light: "#F0F0F0", Dark: "#2A2A2A"}).
    Foreground(color.ErrorStyle.GetForeground())
```

## 10. Migration Notes

When migrating from inline styles to the centralized system:

1. Replace local style definitions with imports from `internal/color`
2. Update style references: `panelStyle` → `color.PanelStyle`
3. Remove duplicate color definitions
4. Test thoroughly to ensure visual consistency

By adhering to these guidelines and using the centralized color system, `envctl` maintains a consistent, professional, and accessible terminal user interface. 