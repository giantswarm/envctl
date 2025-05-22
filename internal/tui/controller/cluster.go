package controller

import (
	"envctl/internal/k8smanager"
	"envctl/internal/tui/model"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// HealthUpdateInterval defines how often cluster health is polled.
	HealthUpdateInterval       = 15 * time.Second    // Exported
	clusterControllerSubsystem = "ClusterController" // Define a subsystem for this controller's logs
)

// handleKubeContextResultMsg updates the model with the detected current kube-context.
func handleKubeContextResultMsg(m *model.Model, msg model.KubeContextResultMsg) (*model.Model, tea.Cmd) {
	LogDebug(m, clusterControllerSubsystem, "----------------------------------------------------------------")
	LogDebug(m, clusterControllerSubsystem, "[handleKubeContextResultMsg] ENTERED. Msg context: '%s', Msg err: %v", msg.Context, msg.Err)
	LogDebug(m, clusterControllerSubsystem, "----------------------------------------------------------------")

	if msg.Err != nil {
		LogDebug(m, clusterControllerSubsystem, "Error getting current kube context: %v", msg.Err)
		m.CurrentKubeContext = "Error fetching context"
		LogError(clusterControllerSubsystem, msg.Err, "Error getting current Kubernetes context: %s", msg.Err.Error())
		return m, m.SetStatusMessage("Failed to get current context", model.StatusBarError, 5*time.Second)
	}
	m.CurrentKubeContext = msg.Context
	LogDebug(m, clusterControllerSubsystem, "Successfully got current kube context: %s", msg.Context)

	if config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load(); err == nil {
		LogDebug(m, clusterControllerSubsystem, "All available contexts in kubeconfig:")
		for ctx := range config.Contexts {
			if ctx == msg.Context {
				LogDebug(m, clusterControllerSubsystem, "  - %s (CURRENT)", ctx)
			} else {
				LogDebug(m, clusterControllerSubsystem, "  - %s", ctx)
			}
		}
	}

	if m.CurrentAppMode == model.ModeInitializing || m.MCHealth.LastUpdated.IsZero() {
		LogInfo(clusterControllerSubsystem, "Initial KubeContext received (%s), triggering initial health checks.", m.CurrentKubeContext)
		var cmds []tea.Cmd
		if m.KubeMgr == nil {
			LogInfo(clusterControllerSubsystem, "KubeManager not available for initial health check.")
		} else {
			if m.ManagementClusterName != "" {
				mcTargetContext := m.KubeMgr.BuildMcContextName(m.ManagementClusterName)
				cmds = append(cmds, FetchNodeStatusCmd(m.KubeMgr, mcTargetContext, true, m.ManagementClusterName))
			}
			if m.WorkloadClusterName != "" && m.ManagementClusterName != "" {
				wcTargetContext := m.KubeMgr.BuildWcContextName(m.ManagementClusterName, m.WorkloadClusterName)
				cmds = append(cmds, FetchNodeStatusCmd(m.KubeMgr, wcTargetContext, false, m.WorkloadClusterName))
			}
		}
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// handleRequestClusterHealthUpdate schedules node-status fetches for MC and WC clusters.
func handleRequestClusterHealthUpdate(m *model.Model) (*model.Model, tea.Cmd) {
	var cmds []tea.Cmd
	LogInfo(clusterControllerSubsystem, "Requesting cluster health updates at %s", time.Now().Format("15:04:05"))
	LogDebug(m, clusterControllerSubsystem, "Health check cycle starting: MCName=%s, WCName=%s, CurrentContext=%s",
		m.ManagementClusterName, m.WorkloadClusterName, m.CurrentKubeContext)

	if m.KubeMgr == nil {
		LogInfo(clusterControllerSubsystem, "KubeManager not available for health update.")
		return m, nil
	}

	if m.ManagementClusterName != "" {
		m.MCHealth.IsLoading = true
		mcTargetContext := m.KubeMgr.BuildMcContextName(m.ManagementClusterName)
		LogDebug(m, clusterControllerSubsystem, "Scheduling MC health check: cluster=%s, targetCtx=%s, lastUpdated=%v",
			m.ManagementClusterName, mcTargetContext, m.MCHealth.LastUpdated)
		cmds = append(cmds, FetchNodeStatusCmd(m.KubeMgr, mcTargetContext, true, m.ManagementClusterName))
	} else {
		LogDebug(m, clusterControllerSubsystem, "SKIPPED MC health check: No management cluster configured")
	}

	if m.WorkloadClusterName != "" && m.ManagementClusterName != "" {
		m.WCHealth.IsLoading = true
		wcTargetContext := m.KubeMgr.BuildWcContextName(m.ManagementClusterName, m.WorkloadClusterName)
		LogDebug(m, clusterControllerSubsystem, "Scheduling WC health check: cluster=%s, targetCtx=%s, lastUpdated=%v",
			m.WorkloadClusterName, wcTargetContext, m.WCHealth.LastUpdated)
		cmds = append(cmds, FetchNodeStatusCmd(m.KubeMgr, wcTargetContext, false, m.WorkloadClusterName))
	} else {
		LogDebug(m, clusterControllerSubsystem, "SKIPPED WC health check: No workload cluster (and/or MC) configured")
	}

	if m.MCHealth.IsLoading || m.WCHealth.IsLoading {
		m.IsLoading = true
		LogDebug(m, clusterControllerSubsystem, "Set loading state to true for health check cycle")
	}

	cmds = append(cmds, tea.Tick(HealthUpdateInterval, func(t time.Time) tea.Msg { return model.RequestClusterHealthUpdate{} }))
	return m, tea.Batch(cmds...)
}

// handleNodeStatusMsg applies the node-status result to the appropriate health struct.
func handleNodeStatusMsg(m *model.Model, msg model.NodeStatusMsg) (*model.Model, tea.Cmd) {
	LogDebug(m, clusterControllerSubsystem, "Handling NodeStatusMsg for %s (isMC: %v): Ready=%d, Total=%d, Err=%v", msg.ClusterShortName, msg.ForMC, msg.ReadyNodes, msg.TotalNodes, msg.Err)
	if msg.DebugInfo != "" {
		LogDebug(m, clusterControllerSubsystem, "NodeStatusMsg Full DebugInfo for %s available (enable verbose debug if needed).", msg.ClusterShortName)
	}

	if msg.ForMC {
		m.MCHealth.IsLoading = false
		m.MCHealth.ReadyNodes = msg.ReadyNodes
		m.MCHealth.TotalNodes = msg.TotalNodes
		m.MCHealth.StatusError = msg.Err
		m.MCHealth.LastUpdated = time.Now()
		m.MCHealth.DebugLog = msg.DebugInfo
	} else {
		m.WCHealth.IsLoading = false
		m.WCHealth.ReadyNodes = msg.ReadyNodes
		m.WCHealth.TotalNodes = msg.TotalNodes
		m.WCHealth.StatusError = msg.Err
		m.WCHealth.LastUpdated = time.Now()
		m.WCHealth.DebugLog = msg.DebugInfo
	}

	var cmds []tea.Cmd
	if msg.Err == nil {
		LogInfo(clusterControllerSubsystem, "[HEALTH %s] Nodes: %d/%d", msg.ClusterShortName, msg.ReadyNodes, msg.TotalNodes)
	} else {
		LogError(clusterControllerSubsystem, msg.Err, "[HEALTH %s] Error fetching node status", msg.ClusterShortName)
	}

	if !m.MCHealth.IsLoading && (m.WorkloadClusterName == "" || !m.WCHealth.IsLoading) {
		m.IsLoading = false
		LogDebug(m, clusterControllerSubsystem, "All cluster health checks completed, loading state cleared")
	}

	LogDebug(m, clusterControllerSubsystem, "Health update complete for %s", msg.ClusterShortName)
	return m, tea.Batch(cmds...)
}

// handleClusterListResultMsg stores fetched cluster lists for autocomplete.
func handleClusterListResultMsg(m *model.Model, msg model.ClusterListResultMsg) *model.Model {
	LogDebug(m, clusterControllerSubsystem, "Handling ClusterListResultMsg, Error: %v", msg.Err)
	if msg.Err != nil {
		LogError(clusterControllerSubsystem, msg.Err, "Failed to fetch cluster list: %v", msg.Err)
		m.ClusterInfo = &k8smanager.ClusterList{}
		return m
	}
	m.ClusterInfo = msg.Info
	LogDebug(m, clusterControllerSubsystem, "Updated m.ClusterInfo. MCs: %d", len(m.ClusterInfo.ManagementClusters))
	return m
}

// handleKubeContextSwitchedMsg processes the result of performSwitchKubeContextCmd.
func handleKubeContextSwitchedMsg(m *model.Model, msg model.KubeContextSwitchedMsg) (*model.Model, tea.Cmd) {
	LogDebug(m, clusterControllerSubsystem, "Handling KubeContextSwitchedMsg. Target: %s, Err: %v. Debug: %s", msg.TargetContext, msg.Err, msg.DebugInfo)

	if msg.Err != nil {
		LogError(clusterControllerSubsystem, msg.Err, "Failed to switch kube context to %s: %v", msg.TargetContext, msg.Err)
		return m, m.SetStatusMessage(fmt.Sprintf("Ctx switch to %s failed!", msg.TargetContext), model.StatusBarError, 5*time.Second)
	}
	m.CurrentKubeContext = msg.TargetContext
	LogInfo(clusterControllerSubsystem, "Context successfully switched to %s", msg.TargetContext)
	statusCmd := m.SetStatusMessage(fmt.Sprintf("Context switched to %s", msg.TargetContext), model.StatusBarSuccess, 3*time.Second)

	return m, statusCmd
}
