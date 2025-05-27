package view

import (
	"envctl/internal/color"
	"envctl/internal/tui/model"
	"fmt"
)

// RenderV2 renders the UI for ModelV2
func RenderV2(m *model.ModelV2) string {
	switch m.CurrentAppMode {
	case model.ModeQuitting:
		return color.StatusStyle.Render(m.QuittingMessage)
	case model.ModeInitializing:
		if m.Width == 0 || m.Height == 0 {
			return color.StatusStyle.Render("Initializing... (waiting for window size)")
		}
		return color.StatusStyle.Render(fmt.Sprintf("%s Initializing...", m.Spinner.View()))
	case model.ModeNewConnectionInput:
		return renderNewConnectionInputViewV2(m, m.Width)
	case model.ModeMainDashboard:
		return renderMainDashboardV2(m)
	case model.ModeHelpOverlay:
		return renderHelpOverlayV2(m)
	case model.ModeLogOverlay:
		return renderLogOverlayV2(m)
	case model.ModeMcpConfigOverlay:
		return renderMcpConfigOverlayV2(m)
	case model.ModeMcpToolsOverlay:
		return renderMcpToolsOverlayV2(m)
	default:
		return color.StatusStyle.Render(fmt.Sprintf("Unhandled application mode: %s", m.CurrentAppMode.String()))
	}
}
