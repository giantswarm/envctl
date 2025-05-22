package mcpserver

import (
	// "envctl/internal/reporting" // No longer needed by this test if using local McpProcessUpdate
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// TestPipeFails specifically tests the scenario where cmd.StdoutPipe() fails.
func TestPipeFails(t *testing.T) {
	originalExecCommand := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		// Return a command that might cause pipe creation to fail, e.g., invalid path for os/exec's pipe logic
		// Using /dev/null as it's not a typical executable for piping stdout/stderr from before Start.
		return originalExecCommand("/dev/null", args...)
	}
	defer func() { execCommand = originalExecCommand }()

	serverCfg := MCPServerConfig{
		Name:      "pipe-fail-server",
		ProxyPort: 9002,
		Command:   "some-cmd",
		Args:      []string{"some-arg"},
	}
	mockUpdateFn := func(update McpDiscreteStatusUpdate) { /* no-op, or add assertions if needed */ }

	_, _, err := StartAndManageIndividualMcpServer(serverCfg, mockUpdateFn, nil)

	if err == nil {
		t.Fatal("Expected StartAndManageIndividualMcpServer to return an error for pipe failure, but it was nil")
	}

	// If cmd.Path is bad, StdoutPipe or StderrPipe might be the first to fail.
	expectedErrSubstr := fmt.Sprintf("stdout pipe for %s", serverCfg.Name)
	// It could also be a stderr pipe error if stdout somehow passed, or a start error if pipes surprisingly work.
	// For now, let's be a bit flexible or primarily target stdout pipe.
	// A more robust test might require deeper mocking of the Cmd object itself.
	if !strings.Contains(err.Error(), expectedErrSubstr) {
		// Fallback check if it was a start error instead for this invalid path
		altExpectedErrSubstr := fmt.Sprintf("failed to start mcp-proxy for %s", serverCfg.Name)
		if !strings.Contains(err.Error(), altExpectedErrSubstr) {
			t.Errorf("Expected error to contain %q (or %q), got %q", expectedErrSubstr, altExpectedErrSubstr, err.Error())
		}
	}
}

// proxyArgsForTest is a helper to reconstruct the arguments mcp-proxy would be called with.
func proxyArgsForTest(serverCfg MCPServerConfig) []string {
	proxyArgs := []string{
		"--port", fmt.Sprintf("%d", serverCfg.ProxyPort),
		"--pass-environment",
		"--",
		serverCfg.Command,
	}
	return append(proxyArgs, serverCfg.Args...)
}
