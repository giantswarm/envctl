package view

import (
	"envctl/internal/tui/model"
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func renderStatusBar(m *model.Model, width int) string {
	overallStatus, _ := calculateOverallStatus(m)
	var bg lipgloss.AdaptiveColor
	switch overallStatus {
	case model.AppStatusUp:
		bg = StatusBarSuccessBg
	case model.AppStatusConnecting:
		bg = StatusBarInfoBg
	case model.AppStatusDegraded:
		bg = StatusBarWarningBg
	case model.AppStatusFailed:
		bg = StatusBarErrorBg
	default:
		bg = StatusBarDefaultBg
	}

	leftW := int(float64(width) * 0.25)
	rightW := int(float64(width) * 0.35)
	centerW := width - leftW - rightW
	if centerW < 0 {
		centerW = 0
	}

	// left
	var leftStr string
	if m.IsLoading {
		leftStr = lipgloss.NewStyle().Background(bg).Width(leftW).Render(m.Spinner.View())
	} else {
		icon := ""
		switch overallStatus {
		case model.AppStatusUp:
			icon = SafeIcon(IconCheck)
		case model.AppStatusConnecting:
			icon = SafeIcon(IconHourglass)
		case model.AppStatusDegraded:
			icon = SafeIcon(IconWarning)
		case model.AppStatusFailed:
			icon = SafeIcon(IconCross)
		default:
			icon = SafeIcon(IconInfo)
		}
		leftStr = StatusBarTextStyle.Copy().Background(bg).Width(leftW).Render(icon + overallStatus.String())
	}

	// right
	mcDisplay := m.ManagementClusterName
	if mcDisplay == "" {
		mcDisplay = "N/A"
	}
	mcWc := fmt.Sprintf("%s MC: %s", SafeIcon(IconKubernetes), mcDisplay)

	if m.WorkloadClusterName != "" {
		wcDisplay := m.WorkloadClusterName
		mcWc += fmt.Sprintf(" / %s WC: %s", SafeIcon(IconKubernetes), wcDisplay)
	}
	rightStr := StatusBarTextStyle.Copy().Background(bg).Width(rightW).Align(lipgloss.Right).Render(mcWc)

	// center transient
	var centerStr string
	if m.StatusBarMessage != "" {
		var msgStyle lipgloss.Style
		var icon string
		switch m.StatusBarMessageType {
		case model.StatusBarSuccess:
			msgStyle = StatusMessageSuccessStyle.Copy()
			icon = SafeIcon(IconSparkles)
		case model.StatusBarError:
			msgStyle = StatusMessageErrorStyle.Copy()
			icon = SafeIcon(IconCross)
		case model.StatusBarWarning:
			msgStyle = StatusMessageWarningStyle.Copy()
			icon = SafeIcon(IconLightbulb)
		default:
			msgStyle = StatusMessageInfoStyle.Copy()
			icon = SafeIcon(IconInfo)
		}
		centerStr = msgStyle.Background(bg).Width(centerW).Align(lipgloss.Center).Render(icon + m.StatusBarMessage)
	} else {
		centerStr = lipgloss.NewStyle().Background(bg).Width(centerW).Render("")
	}

	final := lipgloss.JoinHorizontal(lipgloss.Bottom, leftStr, centerStr, rightStr)
	return StatusBarBaseStyle.Copy().Width(width).Render(final)
}
