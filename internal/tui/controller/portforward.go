package controller

import (
	// "envctl/internal/portforwarding" // Likely no longer needed if commands are removed
	"envctl/internal/tui/model"
	// "fmt" // May not be needed
	// "time" // May not be needed
	// tea "github.com/charmbracelet/bubbletea" // May not be needed
)

// -----------------------------------------------------------------------------
// Port-forward setup helpers and handlers (previously portforward_handlers.go)
// -----------------------------------------------------------------------------

// SetupPortForwards configures the model's PortForwards map and PortForwardOrder slice
// based on the PortForwardingConfig which is determined externally and passed into the model.
// This function is KEPT as it populates the model for TUI display purposes.
func SetupPortForwards(m *model.Model, mcName, wcName string) {
	m.PortForwards = make(map[string]*model.PortForwardProcess)
	m.PortForwardOrder = nil

	// Context panes come first in navigation order.
	m.PortForwardOrder = append(m.PortForwardOrder, model.McPaneFocusKey)
	if wcName != "" {
		m.PortForwardOrder = append(m.PortForwardOrder, model.WcPaneFocusKey)
	}

	// Iterate over the centrally defined port forward configurations stored in the model
	for _, pfCfg := range m.PortForwardingConfig {
		// The InstanceKey from PortForwardingConfig should be used as the key
		// Ensure Label and InstanceKey are consistent if they need to be.
		// For now, assuming pfCfg.Label is suitable as the key and for display.
		m.PortForwardOrder = append(m.PortForwardOrder, pfCfg.Label)
		m.PortForwards[pfCfg.Label] = &model.PortForwardProcess{
			Label:     pfCfg.Label,
			Config:    pfCfg, // Store the full config from PortForwardingConfig
			Active:    true,  // Assume all available PFs are initially active
			StatusMsg: "Awaiting Setup...", // Initial status, will be updated by ServiceManager
		}
	}
}

// -----------------------------------------------------------------------------
// tea.Msg handlers ----------------------------------------------------------------

// REMOVED: handlePortForwardSetupResultMsg
// REMOVED: handlePortForwardCoreUpdateMsg

// -----------------------------------------------------------------------------
// Commands ----------------------------------------------------------------------

// REMOVED: GetInitialPortForwardCmds
// REMOVED: createRestartPortForwardCmd
