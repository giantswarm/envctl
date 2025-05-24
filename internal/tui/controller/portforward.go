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
	// m.PortForwardingConfig is now []config.PortForwardDefinition
	for _, pfCfg := range m.PortForwardingConfig { // pfCfg is config.PortForwardDefinition
		if !pfCfg.Enabled { // Only setup enabled port forwards
			continue
		}
		// Use pfCfg.Name as the key and label
		m.PortForwardOrder = append(m.PortForwardOrder, pfCfg.Name)
		m.PortForwards[pfCfg.Name] = &model.PortForwardProcess{
			Label:     pfCfg.Name, // Use Name as the display Label in the TUI panel
			Config:    pfCfg,      // Store the full config.PortForwardDefinition
			Active:    true,       // Initially active, ServiceManager will start it
			StatusMsg: "Awaiting Setup...",
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
