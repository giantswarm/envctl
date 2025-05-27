package kube

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// LoginToKubeCluster executes `tsh kube login <clusterName>` to authenticate with a Teleport Kubernetes cluster.
// It captures and returns the standard output and standard error from the command.
// Note: This function currently passes os.Stdin to the command, which might cause issues
// if `tsh` prompts for interactive input (e.g., 2FA) in a non-interactive environment like the TUI.
// - clusterName: The name of the Teleport Kubernetes cluster to log into.
// Returns the stdout string, stderr string, and an error if the command execution fails.
var LoginToKubeCluster = func(clusterName string) (stdout string, stderr string, err error) {
	cmd := exec.Command("tsh", "kube", "login", clusterName)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Stdin might still be needed if tsh prompts for anything (e.g., 2FA),
	// but for non-interactive TUI, this might be an issue if it hangs.
	// For now, keep os.Stdin, but this could be a point of failure if tsh blocks.
	// Consider if tsh login can be made fully non-interactive or if a timeout is needed.
	cmd.Stdin = os.Stdin

	runErr := cmd.Run()

	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	if runErr != nil {
		// Include tsh's stderr in the error message for better diagnostics
		return stdoutStr, stderrStr, fmt.Errorf("failed to execute 'tsh kube login %s': %w. Stderr: %s", clusterName, runErr, stderrStr)
	}
	return stdoutStr, stderrStr, nil
}

// GetClusterInfo executes the `tsh kube ls` command and parses its output to populate a ClusterInfo struct.
// The parsing logic attempts to distinguish between management clusters (e.g., "ceres") and
// workload clusters (e.g., "ceres-bobcat") based on naming conventions (presence of a hyphen).
// It returns a pointer to the populated ClusterInfo struct and an error if `tsh kube ls` fails or parsing encounters issues.
var GetClusterInfo = func() (*ClusterInfo, error) {
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

	// Skip header lines of `tsh kube ls` output. Typically, the first 2 lines are headers.
	if len(lines) < 3 { // Expect at least headers + one data line for any meaningful output.
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

		// Heuristic to differentiate MCs and WCs: WCs usually contain a hyphen (e.g., "mc-name-wc-shortname").
		// This assumes a naming convention where the MC name is the part before the first hyphen,
		// and the WC short name is the part after.
		if strings.Contains(fullClusterName, "-") {
			// Potential Workload Cluster (e.g., "mcname-wcshortname")
			nameParts := strings.SplitN(fullClusterName, "-", 2)
			if len(nameParts) == 2 {
				mcName := nameParts[0]
				wcShortName := nameParts[1]
				info.WorkloadClusters[mcName] = append(info.WorkloadClusters[mcName], wcShortName)
			}
		} else {
			// Assumed to be a Management Cluster if no hyphen is present in the relevant part of the name.
			info.ManagementClusters = append(info.ManagementClusters, fullClusterName)
		}
	}

	// This step ensures that any MC inferred from a WC's name (e.g., "mcName" from "mcName-wcShortName")
	// is also included in the ManagementClusters list, even if it wasn't listed as a standalone MC entry by `tsh kube ls`.
	// This can happen if only workload clusters under a specific MC are available/listed.
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
