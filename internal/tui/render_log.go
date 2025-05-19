package tui

import (
	"encoding/json"
	"envctl/internal/mcpserver"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// renderLogOverlay (moved from view_helpers.go)
func renderLogOverlay(m model, width, height int) string {
    title := logPanelTitleStyle.Render(SafeIcon(IconScroll) + " Activity Log  (↑/↓ scroll  •  y copy  •  Esc close)")
    viewportView := m.logViewport.View()
    content := lipgloss.JoinVertical(lipgloss.Left, title, viewportView)
    return logOverlayStyle.Copy().
        Width(width - logOverlayStyle.GetHorizontalFrameSize()).
        Height(height - logOverlayStyle.GetVerticalFrameSize()).
        Render(content)
}

// renderCombinedLogPanel renders the activity log panel at bottom.
func renderCombinedLogPanel(m *model, availableWidth int, logSectionHeight int) string {
    if logSectionHeight <= 0 { return "" }

    border := panelStatusDefaultStyle.GetHorizontalFrameSize()
    innerWidth := availableWidth - border
    if innerWidth < 0 { innerWidth = 0 }

    titleView := logPanelTitleStyle.Render(SafeIcon(IconScroll) + "Combined Activity Log")
    viewportView := m.mainLogViewport.View()
    panelContent := lipgloss.JoinVertical(lipgloss.Left, titleView, viewportView)

    base := panelStatusDefaultStyle.Copy().
        Width(innerWidth).
        MaxHeight(0).
        BorderForeground(lipgloss.AdaptiveColor{Light: "#606060", Dark: "#A0A0A0"}).
        Background(lipgloss.AdaptiveColor{Light: "#F8F8F8", Dark: "#2A2A3A"})
    rendered := base.Render(panelContent)

    // ensure min size
    if h := lipgloss.Height(rendered); h < logSectionHeight {
        return lipgloss.NewStyle().Width(availableWidth).Height(logSectionHeight).Render(rendered)
    }
    return rendered
}

// prepareLogContent truncates long lines to avoid viewport wrapping.
func prepareLogContent(lines []string, maxWidth int) string {
    if maxWidth <= 0 { return strings.Join(lines, "\n") }
    out := make([]string, len(lines))
    for i, l := range lines {
        if runewidth.StringWidth(l) > maxWidth {
            truncated := runewidth.Truncate(l, maxWidth-1, "")
            out[i] = truncated + "…"
        } else { out[i] = l }
    }
    return strings.Join(out, "\n")
}

// generateMcpConfigJson & renderMcpConfigOverlay moved as-is.
func generateMcpConfigJson() string {
    type entry struct{ URL string `json:"url"` }
    servers := make(map[string]entry)
    for _, cfg := range mcpserver.PredefinedMcpServers {
        key := fmt.Sprintf("%s-mcp", cfg.Name)
        servers[key] = entry{URL: fmt.Sprintf("http://localhost:%d/sse", cfg.ProxyPort)}
    }
    root := map[string]interface{}{"mcpServers": servers}
    b, err := json.MarshalIndent(root, "", "  ")
    if err != nil { return "{}" }
    return string(b)
}

func renderMcpConfigOverlay(m model, width, height int) string {
    title := logPanelTitleStyle.Render(SafeIcon(IconGear) + " MCP Configuration  (↑/↓ scroll  •  y copy  •  Esc close)")
    viewportView := m.mcpConfigViewport.View()
    content := lipgloss.JoinVertical(lipgloss.Left, title, viewportView)
    return mcpConfigOverlayStyle.Copy().
        Width(width - mcpConfigOverlayStyle.GetHorizontalFrameSize()).
        Height(height - mcpConfigOverlayStyle.GetVerticalFrameSize()).
        Render(content)
} 