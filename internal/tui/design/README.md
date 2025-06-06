# TUI Design System

This package provides a comprehensive design system for the envctl TUI, ensuring consistent styling and improved user experience across all UI components.

## Overview

The design system follows these principles:
- **Consistency**: All colors, spacing, and components follow unified standards
- **Accessibility**: Adaptive colors for both light and dark terminal themes
- **Modularity**: Reusable components that can be composed together
- **Simplicity**: Clean, minimal interface focused on functionality

## Color Palette

### Semantic Colors

```go
// State Colors
ColorSuccess  // Green - indicates successful operations
ColorError    // Red - indicates errors or failures
ColorWarning  // Yellow/Amber - indicates warnings or pending states
ColorInfo     // Blue - informational messages

// UI Colors
ColorPrimary   // Brand color for primary actions
ColorSecondary // Secondary UI elements
ColorBackground // Main background
ColorSurface    // Panel/card backgrounds
ColorBorder     // Default borders
```

All colors are adaptive and automatically adjust for light/dark terminals.

## Spacing System

Based on a 4px unit for consistent spacing:

```go
SpaceXS  = 1 // 4px
SpaceSM  = 2 // 8px
SpaceMD  = 3 // 12px
SpaceLG  = 4 // 16px
SpaceXL  = 6 // 24px
SpaceXXL = 8 // 32px
```

## Typography

Text styles with semantic meaning:

```go
TextStyle          // Default text
TextSecondaryStyle // Secondary/muted text
TextTertiaryStyle  // Tertiary/disabled text
TitleStyle         // Bold titles
SubtitleStyle      // Subtitles
```

## Components

### Panel
Reusable bordered container with optional title and status:

```go
panel := components.NewPanel("Title").
    WithContent("Panel content").
    WithDimensions(width, height).
    WithType(components.PanelTypeSuccess).
    SetFocused(true)
```

### Status Indicator
Consistent status display with icon and text:

```go
status := components.NewStatusIndicator(components.StatusTypeRunning).
    WithText("Custom Status")
```

### Header
Application header with optional spinner:

```go
header := components.NewHeader("App Title").
    WithSubtitle("Subtitle").
    WithSpinner(spinnerView)
```

### Status Bar
Bottom status bar with message support:

```go
statusBar := components.NewStatusBar(width).
    WithMessage("Operation complete", model.StatusBarSuccess)
```

## Icons

The design system includes a comprehensive set of icons:

```go
IconCheck      // Success/completion
IconCross      // Error/failure
IconWarning    // Warning/attention
IconHourglass  // Loading/waiting
IconKubernetes // Kubernetes resources
IconGear       // Settings/configuration
IconLink       // Connections/links
// ... and more
```

Use `SafeIcon()` to ensure proper spacing:

```go
text := design.SafeIcon(design.IconCheck) + "Operation complete"
```

## Usage Examples

### Creating a Dashboard Panel

```go
// Create a panel with dynamic status
panelType := components.PanelTypeDefault
if service.IsRunning() {
    panelType = components.PanelTypeSuccess
} else if service.HasError() {
    panelType = components.PanelTypeError
}

panel := components.NewPanel("Service Status").
    WithContent(buildServiceContent()).
    WithDimensions(50, 20).
    WithType(panelType).
    WithIcon(design.IconGear)
```

### Building Consistent Lists

```go
var lines []string
for _, item := range items {
    // Use consistent selection indicator
    line := "  " // Default indent
    if item.IsSelected {
        line = design.ListItemSelectedStyle.Render("▶ ")
    }
    
    // Add status indicator
    status := components.NewStatusIndicator(
        components.StatusFromString(item.State),
    ).IconOnly()
    
    line += fmt.Sprintf("%s %s", item.Name, status.Render())
    lines = append(lines, line)
}
```

### Layout Management

```go
layout := components.NewLayout(terminalWidth, terminalHeight)

// Split screen 40/60
topHeight, bottomHeight := layout.SplitHorizontal(0.4)

// Split bottom section 30/70
leftWidth, rightWidth := layout.SplitVertical(0.3)

// Join components
dashboard := components.JoinVertical(
    header,
    topPanel,
    components.JoinHorizontal(1, leftPanel, rightPanel),
    statusBar,
)
```

## Best Practices

1. **Always use semantic colors** - Don't hardcode color values
2. **Use consistent spacing** - Stick to the spacing units
3. **Leverage components** - Don't recreate existing patterns
4. **Handle focus states** - Use `SetFocused()` for keyboard navigation
5. **Adaptive design** - Test in both light and dark terminals
6. **Consistent icons** - Use the provided icon set with `SafeIcon()`

## Migration Guide

To migrate existing code to the design system:

1. Replace `color.*` imports with `design.*`
2. Replace hardcoded colors with semantic colors
3. Use components instead of manual styling
4. Update icon references to use `design.Icon*`
5. Use `design.SafeIcon()` instead of `view.SafeIcon()` 