package kube

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"envctl/pkg/logging"

	"github.com/stretchr/testify/assert"
)

// TestDirectLogger_Write tests the directLogger's Write method.
func TestDirectLogger_Write(t *testing.T) {
	tests := []struct {
		name              string
		pfLabel           string
		input             string
		isError           bool
		expectedMsgSubstr string // Expected substring in the msg="..." part of the log
		expectLogOutput   bool
		expectedLevelStr  string
	}{
		{
			name:              "stdout common line",
			pfLabel:           "TestPF-Stdout",
			input:             "Handling connection for 8080\n",
			isError:           false,
			expectedMsgSubstr: "[PF_STDOUT_RAW] Handling connection for 8080",
			expectLogOutput:   true,
			expectedLevelStr:  "DEBUG",
		},
		{
			name:              "stdout 'Forwarding from' line",
			pfLabel:           "TestPF-StdoutForward",
			input:             "Forwarding from 127.0.0.1:8080 -> 8080\n",
			isError:           false,
			expectedMsgSubstr: "",
			expectLogOutput:   false, // directLogger should not log this specific line
			expectedLevelStr:  "",
		},
		{
			name:              "stderr line logged as DEBUG by directLogger",
			pfLabel:           "TestPF-Stderr",
			input:             "Some verbose output from stderr\n",
			isError:           true,
			expectedMsgSubstr: "[PF_STDERR_RAW] Some verbose output from stderr",
			expectLogOutput:   true,
			expectedLevelStr:  "DEBUG",
		},
		{
			name:              "stdout with client-go prefix",
			pfLabel:           "TestPF-ClientGo",
			input:             "I1234 12:34:56.789       1 client_go_thing.go:123] Actual message\n",
			isError:           false,
			expectedMsgSubstr: "[PF_STDOUT_RAW] Actual message", // Updated to reflect new cleaning logic
			expectLogOutput:   true,
			expectedLevelStr:  "DEBUG",
		},
		{
			name:              "empty input",
			pfLabel:           "TestPF-Empty",
			input:             "\n\n",
			isError:           false,
			expectedMsgSubstr: "",
			expectLogOutput:   false,
			expectedLevelStr:  "",
		},
		{
			name:    "multiple lines stdout",
			pfLabel: "TestPF-MultiStdout",
			input:   "Line one\nLine two\n",
			isError: false,
			// For multi-line, we'll check for each line's presence individually in the assertions.
			expectedMsgSubstr: "[PF_STDOUT_RAW] Line one", // We will also check for Line two
			expectLogOutput:   true,
			expectedLevelStr:  "DEBUG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuf bytes.Buffer
			// Stash and restore global logger. This is simplified.
			// Proper isolation would require more complex logger DI or pkg/logging enhancements.
			tempStdout := os.Stdout // Not a perfect stash, as original might not be os.Stdout
			// To properly test, we need to know what the global logger level *was*.
			// For now, assuming it was Info and we restore to that.
			originalGlobalLevel := logging.LevelInfo
			// It's better if pkg/logging itself has GetLevel() and GetOutput() if we are to manipulate globals.
			// Without them, this defer is best-effort.
			defer logging.InitForCLI(originalGlobalLevel, tempStdout)
			logging.InitForCLI(logging.LevelDebug, &logBuf)

			opsSubsystem := fmt.Sprintf("PortForward-%s-kube-ops", tt.pfLabel)
			writer := &directLogger{subsystem: opsSubsystem, isError: tt.isError}

			_, err := writer.Write([]byte(tt.input))
			assert.NoError(t, err)

			logOutput := logBuf.String()

			if tt.expectLogOutput {
				assert.Contains(t, logOutput, fmt.Sprintf("level=%s", tt.expectedLevelStr), "Log output mismatch for expected level")
				assert.Contains(t, logOutput, fmt.Sprintf("subsystem=%s", opsSubsystem), "Log output mismatch for expected subsystem")

				if tt.name == "multiple lines stdout" {
					assert.Contains(t, logOutput, "msg=\"[PF_STDOUT_RAW] Line one\"")
					assert.Contains(t, logOutput, "msg=\"[PF_STDOUT_RAW] Line two\"")
				} else {
					assert.Contains(t, logOutput, fmt.Sprintf("msg=\"%s\"", tt.expectedMsgSubstr), "Log output mismatch for expected message content")
				}
			} else {
				if tt.name == "stdout 'Forwarding from' line" {
					assert.NotContains(t, logOutput, "Forwarding from", "Should not log 'Forwarding from' line")
				} else if strings.TrimSpace(tt.input) != "" {
					if logOutput != "" {
						// Check if the specific subsystem for this test call appears. If it does, it means an unexpected log was made.
						// This is still a bit weak if other things log to the same buffer with different subsystems.
						assert.NotContains(t, logOutput, fmt.Sprintf("subsystem=%s", opsSubsystem), "Expected no specific log output for this directLogger call, but found its subsystem")
					}
				}
			}
		})
	}
}
