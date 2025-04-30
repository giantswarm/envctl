package utils

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// ClusterInfo holds parsed information about clusters from 'tsh kube ls'.
type ClusterInfo struct {
	ManagementClusters []string
	WorkloadClusters   map[string][]string // Key: Management Cluster, Value: List of short Workload Cluster names
}

// GetClusterInfo runs 'tsh kube ls' and parses the output to categorize clusters.
func GetClusterInfo() (*ClusterInfo, error) {
	cmd := exec.Command("tsh", "kube", "ls")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to execute 'tsh kube ls': %w\nStderr: %s", err, stderr.String())
	}

	lines := strings.Split(out.String(), "\n")
	info := &ClusterInfo{
		ManagementClusters: []string{},
		WorkloadClusters:   make(map[string][]string),
	}

	// Skip header lines
	if len(lines) < 3 {
		return info, nil // Return empty info if no data rows
	}

	for _, line := range lines[2:] { // Start from the third data line
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		fullClusterName := strings.TrimSuffix(parts[0], "*")

		if strings.Contains(fullClusterName, "-") {
			// Potential Workload Cluster
			nameParts := strings.SplitN(fullClusterName, "-", 2)
			if len(nameParts) == 2 {
				mcName := nameParts[0]
				wcShortName := nameParts[1]
				info.WorkloadClusters[mcName] = append(info.WorkloadClusters[mcName], wcShortName)
			}
		} else {
			// Management Cluster
			info.ManagementClusters = append(info.ManagementClusters, fullClusterName)
		}
	}

	// Ensure all management clusters mentioned in workload clusters map exist in the MC list
	// (This handles cases where a WC exists but its corresponding MC isn't listed standalone)
	// This is less likely with tsh but good practice.
	existingMCs := make(map[string]bool)
	for _, mc := range info.ManagementClusters {
		existingMCs[mc] = true
	}
	for mcName := range info.WorkloadClusters {
		if !existingMCs[mcName] {
			info.ManagementClusters = append(info.ManagementClusters, mcName)
		}
	}

	return info, nil
}
