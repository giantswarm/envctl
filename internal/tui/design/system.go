package design

import (
	"github.com/charmbracelet/lipgloss"
)

// Design System Constants
// Following 4px base unit for consistent spacing
const (
	// Spacing units (based on 4px)
	SpaceNone = 0
	SpaceXS   = 1 // 4px
	SpaceSM   = 2 // 8px
	SpaceMD   = 3 // 12px
	SpaceLG   = 4 // 16px
	SpaceXL   = 6 // 24px
	SpaceXXL  = 8 // 32px

	// Border radius
	RadiusNone = 0
	RadiusSM   = 2
	RadiusMD   = 4
	RadiusLG   = 8

	// Component dimensions
	MinPanelHeight = 8
	MinPanelWidth  = 20
)

// Color Palette - Semantic colors with consistent light/dark mode support
var (
	// Brand Colors
	ColorPrimary = lipgloss.AdaptiveColor{
		Light: "#5A56E0",
		Dark:  "#7571F9",
	}
	ColorSecondary = lipgloss.AdaptiveColor{
		Light: "#6B7280",
		Dark:  "#9CA3AF",
	}

	// State Colors
	ColorSuccess = lipgloss.AdaptiveColor{
		Light: "#059669",
		Dark:  "#10B981",
	}
	ColorError = lipgloss.AdaptiveColor{
		Light: "#DC2626",
		Dark:  "#EF4444",
	}
	ColorWarning = lipgloss.AdaptiveColor{
		Light: "#D97706",
		Dark:  "#F59E0B",
	}
	ColorInfo = lipgloss.AdaptiveColor{
		Light: "#2563EB",
		Dark:  "#3B82F6",
	}

	// Neutral Colors
	ColorBackground = lipgloss.AdaptiveColor{
		Light: "#FFFFFF",
		Dark:  "#0F0F0F",
	}
	ColorSurface = lipgloss.AdaptiveColor{
		Light: "#F9FAFB",
		Dark:  "#1A1A1A",
	}
	ColorSurfaceAlt = lipgloss.AdaptiveColor{
		Light: "#F3F4F6",
		Dark:  "#262626",
	}
	ColorBorder = lipgloss.AdaptiveColor{
		Light: "#E5E7EB",
		Dark:  "#404040",
	}
	ColorBorderFocus = lipgloss.AdaptiveColor{
		Light: "#5A56E0",
		Dark:  "#7571F9",
	}

	// Text Colors
	ColorText = lipgloss.AdaptiveColor{
		Light: "#111827",
		Dark:  "#F9FAFB",
	}
	ColorTextSecondary = lipgloss.AdaptiveColor{
		Light: "#6B7280",
		Dark:  "#9CA3AF",
	}
	ColorTextTertiary = lipgloss.AdaptiveColor{
		Light: "#9CA3AF",
		Dark:  "#6B7280",
	}

	// Special Purpose Colors
	ColorHighlight = lipgloss.AdaptiveColor{
		Light: "#EEF2FF",
		Dark:  "#312E81",
	}
	ColorOverlay = lipgloss.AdaptiveColor{
		Light: "rgba(0, 0, 0, 0.5)",
		Dark:  "rgba(0, 0, 0, 0.7)",
	}
	ColorBackgroundOverlay = lipgloss.AdaptiveColor{
		Light: "#FFFFFF",
		Dark:  "#1E1E1E",
	}
	ColorBackgroundHighlight = lipgloss.AdaptiveColor{
		Light: "#E8F4FF",
		Dark:  "#2A3450",
	}
	ColorBackgroundSecondary = lipgloss.AdaptiveColor{
		Light: "#F8F8F8",
		Dark:  "#2A2A3A",
	}
	ColorTextMuted = lipgloss.AdaptiveColor{
		Light: "#9CA3AF",
		Dark:  "#6B7280",
	}
)

// Base Styles - Foundation for all components
var (
	// Text Styles
	TextStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	TextSecondaryStyle = lipgloss.NewStyle().
				Foreground(ColorTextSecondary)

	TextTertiaryStyle = lipgloss.NewStyle().
				Foreground(ColorTextTertiary)

	// State Text Styles
	TextSuccessStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess)

	TextErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	TextWarningStyle = lipgloss.NewStyle().
				Foreground(ColorWarning)

	TextInfoStyle = lipgloss.NewStyle().
			Foreground(ColorInfo)

	// Base Component Styles
	BaseStyle = lipgloss.NewStyle().
			Background(ColorBackground).
			Foreground(ColorText)

	SurfaceStyle = lipgloss.NewStyle().
			Background(ColorSurface).
			Foreground(ColorText).
			Padding(SpaceXS, SpaceSM)

	// Border Styles
	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder)

	BorderFocusStyle = lipgloss.NewStyle().
				Border(lipgloss.ThickBorder()).
				BorderForeground(ColorBorderFocus)
)

// Component Styles - Reusable component definitions
var (
	// Panel Styles
	PanelStyle = SurfaceStyle.Copy().
			Inherit(BorderStyle).
			Padding(SpaceSM-1, SpaceSM). // Account for border
			Margin(0)

	PanelFocusedStyle = PanelStyle.Copy().
				Inherit(BorderFocusStyle).
				Background(ColorHighlight)

	// Header Styles
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Background(ColorSurface).
			Foreground(ColorText).
			Padding(0, SpaceSM).
			Width(100) // Will be overridden

	// Status Bar Styles
	StatusBarStyle = lipgloss.NewStyle().
			Background(ColorSurfaceAlt).
			Foreground(ColorText).
			Padding(0, SpaceSM).
			Height(1)

	StatusBarSuccessStyle = StatusBarStyle.Copy().
				Background(ColorSuccess).
				Foreground(ColorBackground)

	StatusBarErrorStyle = StatusBarStyle.Copy().
				Background(ColorError).
				Foreground(ColorBackground)

	StatusBarWarningStyle = StatusBarStyle.Copy().
				Background(ColorWarning).
				Foreground(ColorBackground)

	StatusBarInfoStyle = StatusBarStyle.Copy().
				Background(ColorInfo).
				Foreground(ColorBackground)

	// List Item Styles
	ListItemStyle = lipgloss.NewStyle().
			PaddingLeft(SpaceSM)

	ListItemSelectedStyle = ListItemStyle.Copy().
				Foreground(ColorPrimary).
				Bold(true)

	ListItemDisabledStyle = ListItemStyle.Copy().
				Foreground(ColorTextTertiary)

	// Button Styles
	ButtonStyle = lipgloss.NewStyle().
			Padding(0, SpaceSM).
			Background(ColorPrimary).
			Foreground(ColorBackground).
			Bold(true)

	ButtonSecondaryStyle = ButtonStyle.Copy().
				Background(ColorSurfaceAlt).
				Foreground(ColorText).
				Bold(false)

	ButtonDisabledStyle = ButtonStyle.Copy().
				Background(ColorTextTertiary).
				Foreground(ColorSurfaceAlt)

	// Input Styles
	InputStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder).
			Padding(0, SpaceXS)

	InputFocusedStyle = InputStyle.Copy().
				BorderForeground(ColorBorderFocus)

	// Overlay Styles
	OverlayStyle = lipgloss.NewStyle().
			Background(ColorSurface).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(SpaceMD, SpaceLG).
			Margin(SpaceLG)

	// Title Styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorText).
			MarginBottom(SpaceXS)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorTextSecondary).
			MarginBottom(SpaceSM)
)

// Icon Styles - Consistent icon coloring
var (
	IconDefaultStyle = lipgloss.NewStyle().
				Foreground(ColorTextSecondary)

	IconSuccessStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess)

	IconErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	IconWarningStyle = lipgloss.NewStyle().
				Foreground(ColorWarning)

	IconInfoStyle = lipgloss.NewStyle().
			Foreground(ColorInfo)

	IconPrimaryStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary)
)

// Overlay styles
var (
	// Help overlay styles
	HelpTitleStyle = lipgloss.NewStyle().
			Bold(true).
			MarginBottom(1).
			Align(lipgloss.Center).
			Foreground(ColorText)

	CenteredOverlayContainerStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(ColorBorder).
					Background(ColorBackgroundOverlay).
					Foreground(ColorText).
					Padding(1, 2)

	// Log overlay styles
	LogOverlayStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Background(ColorBackgroundOverlay).
			Foreground(ColorText).
			Padding(1, 2)

	LogPanelTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Padding(0, 1).
				MarginBottom(1).
				Foreground(ColorText)

	// MCP Config overlay styles
	McpConfigOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder).
				Background(ColorBackgroundOverlay).
				Foreground(ColorText).
				Padding(1, 2)

	// Agent REPL styles
	AgentPromptStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true)

	DimStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)
)

// Log level styles
var (
	LogInfoStyle  = lipgloss.NewStyle().Foreground(ColorText)
	LogWarnStyle  = lipgloss.NewStyle().Foreground(ColorWarning)
	LogErrorStyle = lipgloss.NewStyle().Foreground(ColorError)
	LogDebugStyle = lipgloss.NewStyle().Foreground(ColorTextMuted).Italic(true)

	// Health related styles for the activity log
	LogHealthGoodStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	LogHealthWarnStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	LogHealthErrStyle  = lipgloss.NewStyle().Foreground(ColorError)
)

// Context pane styles
var (
	ContextPaneStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				Padding(0, 1).
				BorderForeground(ColorBorder).
				Background(ColorBackgroundSecondary).
				Foreground(ColorText)

	ActiveContextPaneStyle = ContextPaneStyle.Copy().
				BorderForeground(ColorPrimary).
				Background(ColorBackgroundHighlight)

	FocusedContextPaneStyle = ContextPaneStyle.Copy().
				Border(lipgloss.ThickBorder()).
				BorderForeground(ColorPrimary).
				Background(ColorBackgroundHighlight)

	FocusedAndActiveContextPaneStyle = ActiveContextPaneStyle.Copy().
						Border(lipgloss.ThickBorder()).
						BorderForeground(ColorPrimary).
						Background(ColorBackgroundHighlight)
)

// Health status styles
var (
	HealthLoadingStyle = lipgloss.NewStyle().Foreground(ColorText)
	HealthGoodStyle    = lipgloss.NewStyle().Foreground(ColorSuccess)
	HealthWarnStyle    = lipgloss.NewStyle().Foreground(ColorWarning)
	HealthErrorStyle   = lipgloss.NewStyle().Foreground(ColorError)
)

// Quit key style
var QuitKeyStyle = lipgloss.NewStyle().Foreground(ColorError).Bold(true)

// Helper Functions
func GetStateStyle(state string) lipgloss.Style {
	switch state {
	case "running", "connected", "healthy":
		return TextSuccessStyle
	case "failed", "error", "unhealthy":
		return TextErrorStyle
	case "starting", "connecting", "pending":
		return TextWarningStyle
	case "stopped", "disconnected":
		return TextSecondaryStyle
	default:
		return TextStyle
	}
}

func GetStateIconStyle(state string) lipgloss.Style {
	switch state {
	case "running", "connected", "healthy":
		return IconSuccessStyle
	case "failed", "error", "unhealthy":
		return IconErrorStyle
	case "starting", "connecting", "pending":
		return IconWarningStyle
	case "stopped", "disconnected":
		return IconDefaultStyle
	default:
		return IconDefaultStyle
	}
}

// Layout Helpers
func CenterHorizontal(width int, content string) string {
	contentWidth := lipgloss.Width(content)
	if contentWidth >= width {
		return content
	}
	padding := (width - contentWidth) / 2
	return lipgloss.NewStyle().
		PaddingLeft(padding).
		Width(width).
		Render(content)
}

func CenterVertical(height int, content string) string {
	contentHeight := lipgloss.Height(content)
	if contentHeight >= height {
		return content
	}
	padding := (height - contentHeight) / 2
	return lipgloss.NewStyle().
		PaddingTop(padding).
		Height(height).
		Render(content)
}

// Initialize sets up the design system
func Initialize(isDarkMode bool) {
	lipgloss.SetHasDarkBackground(isDarkMode)
}
