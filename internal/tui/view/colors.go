package view

import "github.com/charmbracelet/lipgloss"

// Define colors
var (
	Primary = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
	Success = lipgloss.AdaptiveColor{Light: "#05A167", Dark: "#05D176"}
	Error   = lipgloss.AdaptiveColor{Light: "#E06A56", Dark: "#F97171"}
	Warning = lipgloss.AdaptiveColor{Light: "#E0A956", Dark: "#F9C171"}
	Info    = lipgloss.AdaptiveColor{Light: "#5A9FE0", Dark: "#71B7F9"}
	Subtle  = lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"}
	Border  = lipgloss.AdaptiveColor{Light: "#D1D1D1", Dark: "#3C3C3C"}
)

// Define styles
var (
	SuccessStyle = lipgloss.NewStyle().Foreground(Success)
	ErrorStyle   = lipgloss.NewStyle().Foreground(Error)
	WarningStyle = lipgloss.NewStyle().Foreground(Warning)
	InfoStyle    = lipgloss.NewStyle().Foreground(Info)
	SubtleStyle  = lipgloss.NewStyle().Foreground(Subtle)
)
