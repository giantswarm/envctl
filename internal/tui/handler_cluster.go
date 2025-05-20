package tui

import (
	"envctl/internal/utils"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/client-go/tools/clientcmd"
)

// handleKubeContextResultMsg updates the model with the detected current kube-context.
func handleKubeContextResultMsg(m model, msg kubeContextResultMsg) model {
    m.LogDebug("----------------------------------------------------------------")
    m.LogDebug("[handleKubeContextResultMsg] ENTERED. Msg context: '%s', Msg err: %v", msg.context, msg.err)
    m.LogDebug("----------------------------------------------------------------")

    if msg.err != nil {
        m.currentKubeContext = "Error fetching context"
        m.LogError("Error getting current Kubernetes context: %s", msg.err.Error())
        m.LogDebug("Context fetch error details: %v", msg.err)
    } else {
        previousContext := m.currentKubeContext
        m.currentKubeContext = msg.context
        
        m.LogInfo("Current Kubernetes context: %s", msg.context)
        if previousContext != msg.context && previousContext != "" {
            m.LogDebug("Context changed: %s -> %s", previousContext, msg.context)
        }
        
        m.LogDebug("Context update: currentKubeContext=%s, MCName=%s, WCName=%s", 
            msg.context, m.managementClusterName, m.workloadClusterName)
            
        if config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load(); err == nil {
            m.LogDebug("All available contexts in kubeconfig:")
            for ctx := range config.Contexts {
                if ctx == msg.context {
                    m.LogDebug("  - %s (CURRENT)", ctx)
                } else {
                    m.LogDebug("  - %s", ctx)
                }
            }
        }
    }
    return m
}

// handleRequestClusterHealthUpdate schedules node-status fetches for MC and WC clusters.
func handleRequestClusterHealthUpdate(m model) (model, tea.Cmd) {
    var cmds []tea.Cmd
    m.LogInfo("Requesting cluster health updates at %s", time.Now().Format("15:04:05"))
    m.LogDebug("Health check cycle starting: MCName=%s, WCName=%s, CurrentContext=%s", 
        m.managementClusterName, m.workloadClusterName, m.currentKubeContext)

    if m.managementClusterName != "" {
        m.MCHealth.IsLoading = true
        mcTargetContext := utils.BuildMcContext(m.managementClusterName)
        m.LogDebug("Scheduling MC health check: cluster=%s, targetCtx=%s, lastUpdated=%v", 
            m.managementClusterName, mcTargetContext, m.MCHealth.LastUpdated)
        cmds = append(cmds, fetchNodeStatusCmd(mcTargetContext, true, m.managementClusterName))
    } else {
        m.LogDebug("SKIPPED MC health check: No management cluster configured")
    }
    
    if m.workloadClusterName != "" && m.managementClusterName != "" {
        m.WCHealth.IsLoading = true
        wcTargetContext := utils.BuildWcContext(m.managementClusterName, m.workloadClusterName)
        m.LogDebug("Scheduling WC health check: cluster=%s, targetCtx=%s, lastUpdated=%v", 
            m.workloadClusterName, wcTargetContext, m.WCHealth.LastUpdated)
        cmds = append(cmds, fetchNodeStatusCmd(wcTargetContext, false, m.workloadClusterName))
    } else {
        m.LogDebug("SKIPPED WC health check: No workload cluster (and/or MC) configured")
    }
    
    if m.MCHealth.IsLoading || m.WCHealth.IsLoading {
        m.isLoading = true
        m.LogDebug("Set loading state to true for health check cycle")
    }

    cmds = append(cmds, tea.Tick(healthUpdateInterval, func(t time.Time) tea.Msg { return requestClusterHealthUpdate{} }))
    return m, tea.Batch(cmds...)
}

// handleNodeStatusMsg applies the node-status result to the appropriate health struct.
func handleNodeStatusMsg(m model, msg nodeStatusMsg) model {
    var target *clusterHealthInfo
    var name string
    
    m.LogDebug("Received health update for cluster=%s, isMC=%v", msg.clusterShortName, msg.forMC)

    if msg.forMC && msg.clusterShortName == m.managementClusterName {
        target, name = &m.MCHealth, m.managementClusterName
        m.LogDebug("Processing health for management cluster: %s", name)
    } else if !msg.forMC && msg.clusterShortName == m.workloadClusterName && m.managementClusterName != "" { 
        target, name = &m.WCHealth, m.workloadClusterName
        m.LogDebug("Processing health for workload cluster: %s (MC: %s)", name, m.managementClusterName)
    } else {
        m.LogWarn("[HEALTH STALE/MISMATCH] Received status for '%s' (isMC:%v) current MCName:'%s' WCName:'%s'. Discarding.", 
            msg.clusterShortName, msg.forMC, m.managementClusterName, m.workloadClusterName)
        m.LogDebug("Cluster mismatch details: msgCluster=%s, isMC=%v, activeModel[MCName=%s, WCName=%s]", 
            msg.clusterShortName, msg.forMC, m.managementClusterName, m.workloadClusterName)
        return m
    }

    if msg.debugInfo != "" {
        m.LogDebug("Health check debug info for %s:\n%s", name, msg.debugInfo)
    }

    m.LogDebug("Updating health state for %s", name)
    target.IsLoading = false
    target.LastUpdated = time.Now()
    
    if msg.err != nil {
        target.StatusError = msg.err
        target.ReadyNodes = 0
        target.TotalNodes = 0
        m.LogError("[HEALTH %s] Error: %s", name, msg.err.Error())
        m.LogDebug("Health check failed for %s: %v", name, msg.err)
    } else {
        prevReady := target.ReadyNodes
        prevTotal := target.TotalNodes
        
        target.StatusError = nil
        target.ReadyNodes = msg.readyNodes
        target.TotalNodes = msg.totalNodes
        
        m.LogInfo("[HEALTH %s] Nodes: %d/%d", name, msg.readyNodes, msg.totalNodes)
        
        if prevReady != msg.readyNodes || prevTotal != msg.totalNodes {
            m.LogDebug("Node count changed for %s: %d/%d -> %d/%d", 
                name, prevReady, prevTotal, msg.readyNodes, msg.totalNodes)
        }
    }
    
    if !m.MCHealth.IsLoading && (m.workloadClusterName == "" || !m.WCHealth.IsLoading) {
        m.isLoading = false
        m.LogDebug("All cluster health checks completed, loading state cleared")
    }
    
    m.LogDebug("Health update complete for %s", name)
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
    
    m.LogDebug("[handleKubeContextSwitchedMsg] Received: TargetContext='%s', err=%v", msg.TargetContext, msg.err)
    m.LogDebug("[handleKubeContextSwitchedMsg] Full DebugInfo from msg: %s", msg.DebugInfo)
    
    if msg.err != nil {
        m.LogError("Failed to switch Kubernetes context to '%s': %s", msg.TargetContext, msg.err.Error())
        m.LogDebug("Context switch error details: target=%s, error=%v", msg.TargetContext, msg.err)
        status = m.setStatusMessage(fmt.Sprintf("Failed to switch context: %s", msg.TargetContext), StatusBarError, 5*time.Second)
    } else {
        m.LogInfo("Successfully switched Kubernetes context. Target was: %s", msg.TargetContext)
        m.LogDebug("Context switch successful: %s", msg.TargetContext)
        status = m.setStatusMessage(fmt.Sprintf("Context switched to: %s", msg.TargetContext), StatusBarSuccess, 3*time.Second)

        // The model's managementClusterName and workloadClusterName are NOT changed by a manual switch.
        // Only the currentKubeContext is updated by fetching it again.
        cmdToGetCurrentCtx := getCurrentKubeContextCmd()
        m.LogDebug("[handleKubeContextSwitchedMsg] Dispatching getCurrentKubeContextCmd(). Command: %T", cmdToGetCurrentCtx)
        if cmdToGetCurrentCtx == nil {
             m.LogWarn("[handleKubeContextSwitchedMsg] getCurrentKubeContextCmd() returned nil!")
        }
        cmds = append(cmds, cmdToGetCurrentCtx) 
    }
    return m, tea.Batch(append(cmds, status)...)
} 