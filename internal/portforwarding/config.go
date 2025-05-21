package portforwarding

import (
	"envctl/internal/kube"
	"envctl/internal/utils"
	"fmt"
	"sync"
)

// GetPortForwardConfig generates the default list of port forwarding configurations
// based on the management cluster and workload cluster arguments.
func GetPortForwardConfig(mcShortName, workloadClusterArg string) []PortForwardingConfig {
	configs := make([]PortForwardingConfig, 0)
	mcKubeContext := utils.BuildMcContext(mcShortName)

	// Determine alloyMetricsTargetContext internally
	var alloyMetricsTargetContext string
	if workloadClusterArg != "" && mcShortName != "" { // WC is specified
		alloyMetricsTargetContext = utils.BuildWcContext(mcShortName, workloadClusterArg)
	} else if mcShortName != "" { // Only MC specified
		alloyMetricsTargetContext = utils.BuildMcContext(mcShortName)
	}

	if mcShortName != "" {
		configs = append(configs, PortForwardingConfig{
			Label:       "Prometheus (MC)",
			InstanceKey: "Prometheus (MC)",
			ServiceName: "service/mimir-query-frontend",
			Namespace:   "mimir",
			LocalPort:   "8080",
			RemotePort:  "8080",
			KubeContext: mcKubeContext,
			BindAddress: "127.0.0.1",
		})
		configs = append(configs, PortForwardingConfig{
			Label:       "Grafana (MC)",
			InstanceKey: "Grafana (MC)",
			ServiceName: "service/grafana",
			Namespace:   "monitoring",
			LocalPort:   "3000",
			RemotePort:  "3000",
			KubeContext: mcKubeContext,
			BindAddress: "127.0.0.1",
		})
	}

	alloyLabel := "Alloy Metrics"
	// Use workloadClusterArg to determine if it's a WC or MC context for Alloy label
	if workloadClusterArg != "" && mcShortName != "" {
		alloyLabel += " (WC)"
	} else if mcShortName != "" {
		alloyLabel += " (MC)"
	}

	if alloyMetricsTargetContext != "" {
		configs = append(configs, PortForwardingConfig{
			Label:       alloyLabel,
			InstanceKey: alloyLabel,
			ServiceName: "service/alloy-metrics-cluster",
			Namespace:   "kube-system",
			LocalPort:   "12345",
			RemotePort:  "12345",
			KubeContext: alloyMetricsTargetContext,
			BindAddress: "127.0.0.1",
		})
	}
	return configs
}

// UpdateFunc defines the callback for port forwarding status updates.
// Reverted to old signature: label, status, outputLog, isError, isReady bool
type UpdateFunc func(label, status, outputLog string, isError, isReady bool)

// StartPortForwardsFunc is the type for the StartPortForwards function, for mocking.
var StartPortForwards = StartPortForwardings // Points to the exported function

// DefaultStartPortForwards is the actual implementation, now exported.
func StartPortForwardings(
	configs []PortForwardingConfig,
	updateCb UpdateFunc, // Now old signature
	wg *sync.WaitGroup,
) map[string]chan struct{} {

	if updateCb != nil {
		// Log entry using old signature
		updateCb("PortForwardingSys", "TRACE", fmt.Sprintf(">>> ENTERED REAL DefaultStartPortForwards. Configs: %d", len(configs)), false, false)
	}

	individualStopChans := make(map[string]chan struct{})

	if len(configs) == 0 {
		if updateCb != nil {
			updateCb("PortForwardingSys", "INFO_defaultStartPortForwards", "No port forward configs to process.", false, false)
		}
		return individualStopChans
	}

	for _, pfCfg := range configs {
		if updateCb != nil {
			updateCb(pfCfg.Label, "DEBUG_PF_LOOP", fmt.Sprintf("Looping for: %s", pfCfg.Label), false, false)
		}
		wg.Add(1)
		config := pfCfg
		individualStopChan := make(chan struct{})
		individualStopChans[config.Label] = individualStopChan

		go func() {
			defer wg.Done()

			if updateCb != nil {
				updateCb(config.Label, "Attempting to start...", "", false, false)
			}

			if updateCb != nil {
				updateCb(config.Label, "DEBUG_PF_GOROUTINE", "Active, pre-portspec", false, false)
			}

			// kubeUpdateCallback now also uses the old signature for kube.SendUpdateFunc
			kubeUpdateCallback := func(status, outputLog string, isError, isReady bool) {
				if updateCb != nil {
					updateCb(config.Label, status, outputLog, isError, isReady)
				}
			}

			portSpec := fmt.Sprintf("%s:%s", config.LocalPort, config.RemotePort)

			if updateCb != nil {
				updateCb(config.Label, "", fmt.Sprintf("DEBUG_PF_CONFIG: PRE-CALL to kube.StartPortForwardClientGo for %s", config.Label), false, false)
			}

			pfStopChan, initialStatus, initialErr := kube.StartPortForwardClientGo(
				config.KubeContext,
				config.Namespace,
				config.ServiceName,
				portSpec,
				config.Label,
				kubeUpdateCallback,
			)

			if updateCb != nil {
				debugMsg := fmt.Sprintf("POST-CALL kube.StartPortForwardClientGo: initialStatus='%s', pfStopChan_is_nil=%t", initialStatus, pfStopChan == nil)
				updateCb(config.Label, "", debugMsg, initialErr != nil, false)
			}

			if initialErr != nil {
				if updateCb != nil {
					updateCb(config.Label, fmt.Sprintf("Failed to start: %v", initialErr), initialStatus, true, false)
				}
				return
			}
			if pfStopChan == nil {
				if updateCb != nil {
					updateCb(config.Label, "Critical setup error: stop channel is nil despite no initial error.", initialStatus, true, false)
				}
				return
			}

			if updateCb != nil {
				updateCb(config.Label, "Port-forwarding setup process initiated.", initialStatus, false, false)
			}

			select {
			case <-pfStopChan:
				if updateCb != nil {
					updateCb(config.Label, "Stopped (internal signal).", "", false, false)
				}
			case <-individualStopChan:
				if pfStopChan != nil {
					close(pfStopChan)
				}
				if updateCb != nil {
					updateCb(config.Label, "Stopped (caller signal).", "", false, false)
				}
			}
		}()
	}
	return individualStopChans
}
