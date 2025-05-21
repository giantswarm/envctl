package portforwarding

import "envctl/internal/utils"

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
