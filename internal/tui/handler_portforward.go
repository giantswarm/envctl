package tui

import (
	"envctl/internal/portforwarding"
	"envctl/internal/utils"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// -----------------------------------------------------------------------------
// Port-forward setup helpers and handlers (previously portforward_handlers.go)
// -----------------------------------------------------------------------------

// setupPortForwards initializes or re-initializes the port-forward configurations
// for the current MC/WC combination.
func setupPortForwards(m *model, mcName, wcName string) {
    m.portForwards = make(map[string]*portForwardProcess)
    m.portForwardOrder = nil

    // Context panes come first in navigation order.
    m.portForwardOrder = append(m.portForwardOrder, mcPaneFocusKey)
    if wcName != "" {
        m.portForwardOrder = append(m.portForwardOrder, wcPaneFocusKey)
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
        m.portForwardOrder = append(m.portForwardOrder, label)
        m.portForwards[label] = &portForwardProcess{
            label:     label,
            config:    cfg,
            active:    true,
            statusMsg: "Awaiting Setup...",
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

func handlePortForwardSetupResultMsg(m model, msg portForwardSetupResultMsg) (model, tea.Cmd) {
    m.isLoading = false
    var status tea.Cmd

    if pf, ok := m.portForwards[msg.InstanceKey]; ok {
        if msg.Err != nil {
            pf.err = msg.Err
            pf.statusMsg = fmt.Sprintf("Setup Failed: %v", msg.Err)
            pf.active = false
            status = m.setStatusMessage(fmt.Sprintf("[%s] PF Setup Failed", msg.InstanceKey), StatusBarError, 5*time.Second)
        } else {
            pf.stopChan = msg.StopChan
            pf.cmd = msg.Cmd
            pf.pid = msg.InitialPID
            pf.statusMsg = "Initializing..."
            pf.running = true
            status = m.setStatusMessage(fmt.Sprintf("[%s] PF setup initiated.", msg.InstanceKey), StatusBarInfo, 3*time.Second)
        }
    }

    return m, status
}

func handlePortForwardCoreUpdateMsg(m model, msg portForwardCoreUpdateMsg) (model, tea.Cmd) {
    up := msg.update
    if pf, ok := m.portForwards[up.InstanceKey]; ok {
        pf.statusMsg = up.StatusMsg
        pf.err = up.Error
        pf.pid = up.PID
        pf.running = up.Running

        if up.OutputLog != "" {
            pf.output = append(pf.output, up.OutputLog)
            if len(pf.output) > maxPanelLogLines {
                pf.output = pf.output[len(pf.output)-maxPanelLogLines:]
            }
            m.LogInfo("[%s] %s", up.InstanceKey, up.OutputLog)
        }
        if up.Error != nil {
            pf.active = false
        }
        if !up.Running {
            pf.active = false
        }
    }
    return m, nil
}

// -----------------------------------------------------------------------------
// Commands ----------------------------------------------------------------------

func getInitialPortForwardCmds(m *model) []tea.Cmd {
    var out []tea.Cmd
    for _, label := range m.portForwardOrder {
        pf, ok := m.portForwards[label]
        if !ok || !pf.active {
            continue
        }
        cfg := pf.config
        out = append(out, func() tea.Msg {
            cb := func(update portforwarding.PortForwardProcessUpdate) {
                m.TUIChannel <- portForwardCoreUpdateMsg{update: update}
            }
            cmd, stop, err := portforwarding.StartAndManageIndividualPortForward(cfg, cb)
            pid := 0
            if cmd != nil && cmd.Process != nil {
                pid = cmd.Process.Pid
            }
            return portForwardSetupResultMsg{InstanceKey: cfg.InstanceKey, Cmd: cmd, StopChan: stop, InitialPID: pid, Err: err}
        })
    }
    return out
}

func createRestartPortForwardCmd(m *model, pf *portForwardProcess) tea.Cmd {
    if pf == nil {
        return nil
    }
    safeCloseChan(pf.stopChan)
    pf.stopChan = nil
    pf.statusMsg = "Restarting..."
    pf.output = nil
    pf.err = nil
    pf.running = false
    pf.pid = 0

    cfg := pf.config
    return func() tea.Msg {
        cb := func(update portforwarding.PortForwardProcessUpdate) {
            m.TUIChannel <- portForwardCoreUpdateMsg{update: update}
        }
        cmd, stop, err := portforwarding.StartAndManageIndividualPortForward(cfg, cb)
        pid := 0
        if cmd != nil && cmd.Process != nil {
            pid = cmd.Process.Pid
        }
        return portForwardSetupResultMsg{InstanceKey: cfg.InstanceKey, Cmd: cmd, StopChan: stop, InitialPID: pid, Err: err}
    }
} 