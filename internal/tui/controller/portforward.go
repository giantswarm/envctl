package controller

import (
	"envctl/internal/portforwarding"
	"envctl/internal/tui/model"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// -----------------------------------------------------------------------------
// Port-forward setup helpers and handlers (previously portforward_handlers.go)
// -----------------------------------------------------------------------------

// SetupPortForwards configures the model's PortForwards map and PortForwardOrder slice
// based on the PortForwardingConfig which is determined externally and passed into the model.
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
			StatusMsg: "Awaiting Setup...",
		}
	}
}

// -----------------------------------------------------------------------------
// tea.Msg handlers ----------------------------------------------------------------

func handlePortForwardSetupResultMsg(m *model.Model, msg model.PortForwardSetupResultMsg) (*model.Model, tea.Cmd) {
	m.IsLoading = false
	var statusCmd tea.Cmd

	if pf, ok := m.PortForwards[msg.InstanceKey]; ok {
		if msg.Err != nil {
			pf.Err = msg.Err
			pf.StatusMsg = fmt.Sprintf("Setup Failed: %v", msg.Err)
			pf.Active = false
			statusCmd = m.SetStatusMessage(fmt.Sprintf("[%s] PF Setup Failed", msg.InstanceKey), model.StatusBarError, 5*time.Second)
		} else {
			pf.StopChan = msg.StopChan
			pf.StatusMsg = "Initializing..."
			pf.Running = true
			statusCmd = m.SetStatusMessage(fmt.Sprintf("[%s] PF setup initiated.", msg.InstanceKey), model.StatusBarInfo, 3*time.Second)
		}
	}

	return m, statusCmd
}

func handlePortForwardCoreUpdateMsg(m *model.Model, msg model.PortForwardCoreUpdateMsg) (*model.Model, tea.Cmd) {
	up := msg.Update
	if pf, ok := m.PortForwards[up.InstanceKey]; ok {
		pf.StatusMsg = up.StatusMsg
		pf.Err = up.Error
		pf.Running = up.Running

		if up.OutputLog != "" {
			pf.Log = append(pf.Log, up.OutputLog)
			if len(pf.Log) > model.MaxPanelLogLines {
				pf.Log = pf.Log[len(pf.Log)-model.MaxPanelLogLines:]
			}
			LogInfo(m, "[%s] %s", up.InstanceKey, up.OutputLog)
		}
		if up.Error != nil {
			pf.Active = false
		}
		if !up.Running {
			pf.Active = false
		}
	}
	return m, nil
}

// -----------------------------------------------------------------------------
// Commands ----------------------------------------------------------------------

// GetInitialPortForwardCmds generates the initial set of commands to start all configured port-forwards.
func GetInitialPortForwardCmds(m *model.Model) []tea.Cmd {
	var out []tea.Cmd
	for _, label := range m.PortForwardOrder {
		pf, ok := m.PortForwards[label]
		if !ok || !pf.Active {
			continue
		}
		cfg := pf.Config
		out = append(out, func() tea.Msg {
			cb := func(update portforwarding.PortForwardProcessUpdate) {
				m.TUIChannel <- model.PortForwardCoreUpdateMsg{Update: update}
			}
			stop, err := m.Services.PF.Start(cfg, cb)
			return model.PortForwardSetupResultMsg{InstanceKey: cfg.InstanceKey, StopChan: stop, Err: err}
		})
	}
	return out
}

func createRestartPortForwardCmd(m *model.Model, pf *model.PortForwardProcess) tea.Cmd {
	if pf == nil {
		return nil
	}
	safeCloseChan(pf.StopChan)
	pf.StopChan = nil
	pf.StatusMsg = "Restarting..."
	pf.Log = nil
	pf.Err = nil
	pf.Running = false

	cfg := pf.Config
	return func() tea.Msg {
		cb := func(update portforwarding.PortForwardProcessUpdate) {
			m.TUIChannel <- model.PortForwardCoreUpdateMsg{Update: update}
		}
		stop, err := m.Services.PF.Start(cfg, cb)
		return model.PortForwardSetupResultMsg{InstanceKey: cfg.InstanceKey, StopChan: stop, Err: err}
	}
}
