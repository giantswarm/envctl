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
// Parameters: label, status message, detailed output log, isError flag, isReady flag.
type UpdateFunc func(label, status, outputLog string, isError, isReady bool)

// StartPortForwardsFunc is the type for the StartPortForwards function, for mocking.
// Note: This type matches the original StartPortForwards signature.
// The original StartPortForwards should be renamed (e.g., to defaultStartPortForwards)
// and this var should be initialized with it.
var StartPortForwards = defaultStartPortForwards

// defaultStartPortForwards is the actual implementation.
func defaultStartPortForwards(
	configs []PortForwardingConfig, 
	updateCb UpdateFunc,
	globalStopChan <-chan struct{},
	wg *sync.WaitGroup,
) map[string]chan struct{} {
	individualStopChans := make(map[string]chan struct{})

	if len(configs) == 0 {
		return individualStopChans
	}

	for _, pfCfg := range configs {
		wg.Add(1)
		config := pfCfg 
		individualStopChan := make(chan struct{})
		individualStopChans[config.Label] = individualStopChan

		go func() {
			defer wg.Done()

			if updateCb != nil {
				updateCb(config.Label, "Attempting to start...", "", false, false)
			}

			kubeUpdateCallback := func(status, outputLog string, isError, isReady bool) {
				if updateCb != nil { 
					updateCb(config.Label, status, outputLog, isError, isReady)
				}
			}

			portSpec := fmt.Sprintf("%s:%s", config.LocalPort, config.RemotePort)

			pfStopChan, initialStatus, initialErr := kube.StartPortForwardClientGo(
				config.KubeContext,
				config.Namespace,
				config.ServiceName,
				portSpec,
				config.Label,         
				kubeUpdateCallback, 
			)

			if initialErr != nil {
				if updateCb != nil {
					updateCb(config.Label, fmt.Sprintf("Failed to start: %v", initialErr), initialStatus, true, false)
				}
				return
			}
			if pfStopChan == nil && initialErr == nil {
				if updateCb != nil {
					updateCb(config.Label, "Setup returned no error but stop channel is nil.", initialStatus, true, false)
				}
				return
			}

			if updateCb != nil {
				updateCb(config.Label, "Port-forwarding setup initiated.", initialStatus, false, true)
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
			case <-globalStopChan: 
				if pfStopChan != nil {
					close(pfStopChan)
				}
				if updateCb != nil {
					updateCb(config.Label, "Stopping (global signal).", "", false, false)
				}
			}
		}()
	}
	return individualStopChans
}
