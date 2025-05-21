package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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
