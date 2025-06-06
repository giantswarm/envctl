# Envctl TUI Style Guide

## 1. Introduction

This style guide outlines the visual design principles, color palette, and styling conventions used in the `envctl` Terminal User Interface (TUI). The goal is to maintain a consistent, visually appealing, and user-friendly experience.

The TUI is built using Go with the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework and styled using [Lipgloss](https://github.com/charmbracelet/lipgloss). All styles are now centralized in the `internal/tui/design/` package for consistency across the application.

A key principle is the use of **adaptive colors** to ensure readability and a good aesthetic in both light and dark terminal themes, with a primary focus on an excellent dark mode experience inspired by modern terminal applications.

## 2. Design System Architecture

The styling system has been refactored into a centralized design package (`internal/tui/design/`) that provides:

- Consistent color definitions across all UI components
- Adaptive colors that automatically adjust for light/dark modes
- Semantic naming for easy understanding and maintenance
- Reusable style compositions
- Component library with pre-built UI elements

### Importing Styles

All TUI components import styles from the centralized package:

```go
import "envctl/internal/tui/design"

// Using predefined styles
content := design.FocusedStyle.Render("Focused Panel")
errorMsg := design.ErrorStyle.Render("Error occurred")
```

## 3. Color Palette

The TUI uses adaptive colors that automatically adjust based on the terminal's background (light or dark mode).

### Primary Colors

- **Text (General):** 
  - Light mode: `#111827` (Dark Gray)
  - Dark mode: `#F9FAFB` (Light Gray)
- **Backgrounds:**
  - Light mode: `#FFFFFF` to `#F9FAFB` (Light grays)
  - Dark mode: `#0F0F0F` to `#1A1A1A` (Dark grays)
- **Focus/Accent:** 
  - Light mode: `#5A56E0` (Purple)
  - Dark mode: `#7571F9` (Bright Purple)

### Status Colors

Status colors are used consistently throughout the UI for panels, text, and status bar:

#### Success/Running (Green)
- **Color:** Light: `#059669`, Dark: `#10B981`
- **Usage:** Running services, healthy states, success messages

#### Error/Failed (Red)
- **Color:** Light: `#DC2626`, Dark: `#EF4444`
- **Usage:** Failed services, errors, critical alerts

#### Warning/Degraded (Yellow/Amber)
- **Color:** Light: `#D97706`, Dark: `#F59E0B`
- **Usage:** Warning states, degraded performance, caution messages

#### Info/Initializing (Blue)
- **Color:** Light: `#2563EB`, Dark: `#3B82F6`
- **Usage:** Information messages, initializing states

### Health Status Colors

Used specifically for Kubernetes node health indicators and service health:
- Uses the same Success/Error/Warning colors for consistency
- Loading states use standard text color

### Log Level Colors

For activity log entries:
- **Info:** Standard text color
- **Warning:** Warning color
- **Error:** Error color
- **Debug:** Muted text color with italic style

## 4. Component Library

The design system includes pre-built components for consistent UI:

### 4.1. Panel Component
```go
panel := components.NewPanel().
    WithTitle("Service Status").
    WithType(components.PanelTypeDefault).
    WithFocused(true)
```

### 4.2. Status Indicator
```go
status := components.NewStatusIndicator("running").
    WithIcon(true).
    WithLabel("Service Running")
```

### 4.3. Header Component
```go
header := components.NewHeader("Envctl Dashboard").
    WithSubtitle("Managing 5 services").
    WithSpinner(spinner)
```

### 4.4. Status Bar
```go
statusBar := components.NewStatusBar().
    WithMessage("Ready", components.StatusBarTypeDefault).
    WithRightText("Press ? for help")
```

### 4.5. Layout Manager
```go
layout := components.NewLayout(width, height).
    SplitVertical(0.3).
    SplitHorizontal(0.5)
```

## 5. Typography & Text Styles

### Base Text Styles
- **Primary:** `design.TextStyle`
- **Secondary:** `design.TextSecondaryStyle`
- **Tertiary:** `design.TextTertiaryStyle`

### State Text Styles
- **Success:** `design.TextSuccessStyle`
- **Error:** `design.TextErrorStyle`
- **Warning:** `design.TextWarningStyle`
- **Info:** `design.TextInfoStyle`

## 6. Layout & Spacing

The design system uses a 4px base unit for consistent spacing:

### Spacing Constants
- `design.SpaceXS` - 4px
- `design.SpaceSM` - 8px
- `design.SpaceMD` - 12px
- `design.SpaceLG` - 16px
- `design.SpaceXL` - 24px
- `design.SpaceXXL` - 32px

### Component Dimensions
- Minimum panel height: 8 lines
- Minimum panel width: 20 characters

## 7. Dark Mode Implementation

Dark mode is handled automatically through Lipgloss's adaptive color system:

```go
// Initialize design system (typically in main.go)
design.Initialize(isDarkMode)

// All styles automatically adapt
style := design.PanelStyle // Uses appropriate colors for current mode
```

The system detects the terminal background and adjusts all adaptive colors accordingly.

## 8. Best Practices

### When Adding New Styles

1. **Define in `internal/tui/design/system.go`**: All styles should be centralized
2. **Use Adaptive Colors**: Always use `lipgloss.AdaptiveColor` for theme support
3. **Follow Naming Conventions**: Use descriptive names with proper prefixes
4. **Consider Component Reuse**: Use the component library for common UI patterns

### Color Selection Guidelines

1. **Contrast**: Ensure sufficient contrast between text and background
2. **Consistency**: Use semantic colors (success=green, error=red, etc.)
3. **Accessibility**: Test in both light and dark modes
4. **Subtlety**: Use muted backgrounds, vibrant accents

### Using Components

1. **Prefer Components**: Use pre-built components over custom implementations
2. **Consistent Patterns**: Follow established patterns for similar UI elements
3. **Composition**: Build complex UIs by composing simple components
4. **State Management**: Let components handle their own visual states

## 9. Style Composition Example

```go
// Creating a custom style by composing existing ones
MyCustomStyle := design.PanelStyle.Copy().
    Inherit(design.PanelFocusedStyle).
    Background(design.ColorHighlight).
    Foreground(design.ColorText)

// Using components
panel := components.NewPanel().
    WithTitle("Custom Panel").
    WithContent(MyCustomStyle.Render("Content")).
    WithFocused(true)
```

## 10. Migration Notes

When migrating from inline styles to the design system:

1. Replace local style definitions with imports from `internal/tui/design`
2. Update style references: `color.PanelStyle` → `design.PanelStyle`
3. Replace custom implementations with components from `internal/tui/components`
4. Remove duplicate color and style definitions
5. Test thoroughly to ensure visual consistency

By adhering to these guidelines and using the centralized design system, `envctl` maintains a consistent, professional, and accessible terminal user interface. 