package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// handleKubeContextResultMsg updates the model with the detected current kube-context.
func handleKubeContextResultMsg(m model, msg kubeContextResultMsg) model {
    if msg.err != nil {
        m.currentKubeContext = "Error fetching context"
        m.LogError("Error getting current Kubernetes context: %s", msg.err.Error())
    } else {
        m.currentKubeContext = msg.context
        m.LogInfo("Current Kubernetes context: %s", msg.context)
    }
    return m
}

// handleRequestClusterHealthUpdate schedules node-status fetches for MC and WC clusters.
func handleRequestClusterHealthUpdate(m model) (model, tea.Cmd) {
    var cmds []tea.Cmd
    m.LogInfo("Requesting cluster health updates at %s", time.Now().Format("15:04:05"))

    if m.managementCluster != "" {
        m.MCHealth.IsLoading = true
        if id := m.getManagementClusterContextIdentifier(); id != "" {
            m.LogDebug("Scheduling MC health fetch for id=%s", id)
            cmds = append(cmds, fetchNodeStatusCmd(id, true, m.managementCluster))
        }
    }
    if m.workloadCluster != "" {
        m.WCHealth.IsLoading = true
        if id := m.getWorkloadClusterContextIdentifier(); id != "" {
            m.LogDebug("Scheduling WC health fetch for id=%s", id)
            cmds = append(cmds, fetchNodeStatusCmd(id, false, m.workloadCluster))
        }
    }
    if m.MCHealth.IsLoading || m.WCHealth.IsLoading {
        m.isLoading = true
    }

    cmds = append(cmds, tea.Tick(healthUpdateInterval, func(t time.Time) tea.Msg { return requestClusterHealthUpdate{} }))
    return m, tea.Batch(cmds...)
}

// handleNodeStatusMsg applies the node-status result to the appropriate health struct.
func handleNodeStatusMsg(m model, msg nodeStatusMsg) model {
    var target *clusterHealthInfo
    var name string

    if msg.forMC && msg.clusterShortName == m.managementCluster {
        target, name = &m.MCHealth, m.managementCluster
    } else if !msg.forMC && msg.clusterShortName == m.workloadCluster {
        target, name = &m.WCHealth, m.workloadCluster
    } else {
        m.appendLogLine(fmt.Sprintf("[HEALTH STALE/MISMATCH] Received status for '%s' (isMC:%v) current MC:'%s' WC:'%s'. Discarding.", msg.clusterShortName, msg.forMC, m.managementCluster, m.workloadCluster))
        return m
    }

    if m.debugMode && msg.debugInfo != "" {
        m.LogDebug("%s", msg.debugInfo)
    }

    target.IsLoading = false
    target.LastUpdated = time.Now()
    if msg.err != nil {
        target.StatusError = msg.err
        target.ReadyNodes = 0
        target.TotalNodes = 0
        m.appendLogLine(fmt.Sprintf("[HEALTH %s] Error: %s", name, msg.err.Error()))
    } else {
        target.StatusError = nil
        target.ReadyNodes = msg.readyNodes
        target.TotalNodes = msg.totalNodes
        m.appendLogLine(fmt.Sprintf("[HEALTH %s] Nodes: %d/%d", name, msg.readyNodes, msg.totalNodes))
    }
    if !m.MCHealth.IsLoading && (m.workloadCluster == "" || !m.WCHealth.IsLoading) {
        m.isLoading = false
    }
    return m
}

// handleClusterListResultMsg stores fetched cluster lists for autocomplete.
func handleClusterListResultMsg(m model, msg clusterListResultMsg) model {
    if msg.err != nil {
        m.LogError("Failed to fetch cluster list: %v", msg.err)
    } else {
        m.clusterInfo = msg.info
    }
    return m
}

// handleKubeContextSwitchedMsg processes the result of performSwitchKubeContextCmd.
func handleKubeContextSwitchedMsg(m model, msg kubeContextSwitchedMsg) (model, tea.Cmd) {
    var cmds []tea.Cmd
    var status tea.Cmd
    if msg.err != nil {
        m.LogError("Failed to switch Kubernetes context to '%s': %s", msg.TargetContext, msg.err.Error())
        status = m.setStatusMessage(fmt.Sprintf("Failed to switch context: %s", msg.TargetContext), StatusBarError, 5*time.Second)
    } else {
        m.LogInfo("Successfully switched Kubernetes context. Target was: %s", msg.TargetContext)
        status = m.setStatusMessage(fmt.Sprintf("Context switched to: %s", msg.TargetContext), StatusBarSuccess, 3*time.Second)

        cmds = append(cmds, getCurrentKubeContextCmd())
        if m.managementCluster != "" {
            m.MCHealth.IsLoading = true
            if id := m.getManagementClusterContextIdentifier(); id != "" {
                cmds = append(cmds, fetchNodeStatusCmd(id, true, m.managementCluster))
            }
        }
        if m.workloadCluster != "" {
            m.WCHealth.IsLoading = true
            if id := m.getWorkloadClusterContextIdentifier(); id != "" {
                cmds = append(cmds, fetchNodeStatusCmd(id, false, m.workloadCluster))
            }
        }
    }
    return m, tea.Batch(append(cmds, status)...)
} 