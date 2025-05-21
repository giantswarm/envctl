package controller

import (
	"envctl/internal/portforwarding"
	"envctl/internal/tui/model"
	"envctl/internal/utils"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// -----------------------------------------------------------------------------
// Port-forward setup helpers and handlers (previously portforward_handlers.go)
// -----------------------------------------------------------------------------

// SetupPortForwards configures the initial set of port-forwarding processes based on the cluster names.
// It populates the model's portForwards map and portForwardOrder slice.
func SetupPortForwards(m *model.Model, mcName, wcName string) {
	m.PortForwards = make(map[string]*model.PortForwardProcess)
	m.PortForwardOrder = nil

	// Context panes come first in navigation order.
	m.PortForwardOrder = append(m.PortForwardOrder, model.McPaneFocusKey)
	if wcName != "" {
		m.PortForwardOrder = append(m.PortForwardOrder, model.WcPaneFocusKey)
	}

	kubeConfigPath := "" // rely on default unless overridden later

	add := func(label, context, ns, svc, lp, rp, bind string, isWc bool) {
		cfg := portforwarding.PortForwardConfig{
			Label:          label,
			InstanceKey:    label,
			KubeContext:    context,
			Namespace:      ns,
			ServiceName:    svc,
			LocalPort:      lp,
			RemotePort:     rp,
			BindAddress:    bind,
			KubeConfigPath: kubeConfigPath,
		}
		m.PortForwardOrder = append(m.PortForwardOrder, label)
		m.PortForwards[label] = &model.PortForwardProcess{
			Label:     label,
			Config:    cfg,
			Active:    true,
			StatusMsg: "Awaiting Setup...",
		}
	}

	if mcName != "" {
		ctx := utils.BuildMcContext(mcName)
		add("Prometheus (MC)", ctx, "mimir", "service/mimir-query-frontend", "8080", "8080", "127.0.0.1", false)
		add("Grafana (MC)", ctx, "monitoring", "service/grafana", "3000", "3000", "127.0.0.1", false)
	}

	if wcName != "" {
		ctx := utils.BuildWcContext(mcName, wcName)
		add("Alloy Metrics (WC)", ctx, "kube-system", "service/alloy-metrics-cluster", "12345", "12345", "127.0.0.1", true)
	} else if mcName != "" {
		ctx := utils.BuildMcContext(mcName)
		add("Alloy Metrics (MC)", ctx, "kube-system", "service/alloy-metrics-cluster", "12345", "12345", "127.0.0.1", false)
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
