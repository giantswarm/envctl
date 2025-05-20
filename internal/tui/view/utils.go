package view

import (
	"strings"
)

// trimStatusMessage shortens long status strings for panel display.
func trimStatusMessage(status string) string {
	if strings.HasPrefix(status, "Running (PID:") {
		return "Running"
	}
	if strings.HasPrefix(status, "Forwarding from") {
		return "Forwarding"
	}
	if len(status) > 15 && (strings.Contains(status, "Error") || strings.Contains(status, "Failed")) {
		return status[:12] + "..."
	}
	return status
}
