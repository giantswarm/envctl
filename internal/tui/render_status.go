package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func renderStatusBar(m model, width int) string {
    overallStatus, _ := m.calculateOverallStatus()
    var bg lipgloss.AdaptiveColor
    switch overallStatus {
    case AppStatusUp:
        bg = StatusBarSuccessBg
    case AppStatusConnecting:
        bg = StatusBarInfoBg
    case AppStatusDegraded:
        bg = StatusBarWarningBg
    case AppStatusFailed:
        bg = StatusBarErrorBg
    default:
        bg = StatusBarDefaultBg
    }

    leftW := int(float64(width) * 0.25)
    rightW := int(float64(width) * 0.35)
    centerW := width - leftW - rightW
    if centerW < 0 { centerW = 0 }

    // left
    var leftStr string
    if m.isLoading {
        leftStr = lipgloss.NewStyle().Background(bg).Width(leftW).Render(m.spinner.View())
    } else {
        icon := ""
        switch overallStatus {
        case AppStatusUp:
            icon = SafeIcon(IconCheck)
        case AppStatusConnecting:
            icon = SafeIcon(IconHourglass)
        case AppStatusDegraded:
            icon = SafeIcon(IconWarning)
        case AppStatusFailed:
            icon = SafeIcon(IconCross)
        default:
            icon = SafeIcon(IconInfo)
        }
        leftStr = StatusBarTextStyle.Copy().Background(bg).Width(leftW).Render(icon + overallStatus.String())
    }

    // right
    mcDisplay := m.managementClusterName
    if mcDisplay == "" { mcDisplay = "N/A" }
    mcWc := fmt.Sprintf("%s MC: %s", SafeIcon(IconKubernetes), mcDisplay)

    if m.workloadClusterName != "" {
        wcDisplay := m.workloadClusterName
        mcWc += fmt.Sprintf(" / %s WC: %s", SafeIcon(IconKubernetes), wcDisplay)
    }
    rightStr := StatusBarTextStyle.Copy().Background(bg).Width(rightW).Align(lipgloss.Right).Render(mcWc)

    // center transient
    var centerStr string
    if m.statusBarMessage != "" {
        var msgStyle lipgloss.Style
        var icon string
        switch m.statusBarMessageType {
        case StatusBarSuccess:
            msgStyle = StatusMessageSuccessStyle.Copy()
            icon = SafeIcon(IconSparkles)
        case StatusBarError:
            msgStyle = StatusMessageErrorStyle.Copy()
            icon = SafeIcon(IconCross)
        case StatusBarWarning:
            msgStyle = StatusMessageWarningStyle.Copy()
            icon = SafeIcon(IconLightbulb)
        default:
            msgStyle = StatusMessageInfoStyle.Copy()
            icon = SafeIcon(IconInfo)
        }
        centerStr = msgStyle.Background(bg).Width(centerW).Align(lipgloss.Center).Render(icon + m.statusBarMessage)
    } else {
        centerStr = lipgloss.NewStyle().Background(bg).Width(centerW).Render("")
    }

    final := lipgloss.JoinHorizontal(lipgloss.Bottom, leftStr, centerStr, rightStr)
    return StatusBarBaseStyle.Copy().Width(width).Render(final)
} 