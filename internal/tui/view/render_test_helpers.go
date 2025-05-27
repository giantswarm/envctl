package view

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"envctl/internal/reporting"
)

// updateGoldenFiles is a flag to indicate that golden files should be updated.
var updateGoldenFiles = flag.Bool("update", false, "Update golden files")

// mockKubeManager is a simple mock that doesn't use k8smanager types
type mockKubeManager struct{}

func (m *mockKubeManager) Login(clusterName string) (stdout string, stderr string, err error) {
	return "", "", nil
}
func (m *mockKubeManager) ListClusters() (interface{}, error) {
	return nil, nil
}
func (m *mockKubeManager) GetCurrentContext() (string, error)           { return "test-context", nil }
func (m *mockKubeManager) SwitchContext(targetContextName string) error { return nil }
func (m *mockKubeManager) GetAvailableContexts() ([]string, error) {
	return []string{"test-context"}, nil
}
func (m *mockKubeManager) BuildMcContextName(mcShortName string) string {
	return "teleport.giantswarm.io-" + mcShortName
}
func (m *mockKubeManager) BuildWcContextName(mcShortName, wcShortName string) string {
	return "teleport.giantswarm.io-" + mcShortName + "-" + wcShortName
}
func (m *mockKubeManager) StripTeleportPrefix(contextName string) string {
	return contextName
}
func (m *mockKubeManager) HasTeleportPrefix(contextName string) bool {
	return strings.HasPrefix(contextName, "teleport.giantswarm.io-")
}
func (m *mockKubeManager) GetClusterNodeHealth(ctx context.Context, kubeContextName string) (interface{}, error) {
	// Return a simple struct with the expected fields
	return struct {
		ReadyNodes int
		TotalNodes int
		Error      error
	}{ReadyNodes: 1, TotalNodes: 1, Error: nil}, nil
}

func (m *mockKubeManager) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
	return "mockProvider", nil
}

func (m *mockKubeManager) SetReporter(reporter reporting.ServiceReporter) {
	// Mock implementation
}

// checkGoldenFile compares the actual output with the golden file.
// If -update flag is set, it updates the golden file.
func checkGoldenFile(t *testing.T, goldenFile string, actualOutput string) {
	t.Helper()

	// Normalize line endings to LF for consistent comparisons
	actualOutputNormalized := strings.ReplaceAll(actualOutput, "\r\n", "\n")

	if *updateGoldenFiles {
		// Ensure the testdata directory exists
		if err := os.MkdirAll(filepath.Dir(goldenFile), 0755); err != nil {
			t.Fatalf("Failed to create testdata directory: %v", err)
		}
		if err := ioutil.WriteFile(goldenFile, []byte(actualOutputNormalized), 0644); err != nil {
			t.Fatalf("Failed to update golden file %s: %v", goldenFile, err)
		}
		t.Logf("Updated golden file: %s", goldenFile)
		return
	}

	expectedOutputBytes, err := ioutil.ReadFile(goldenFile)
	if err != nil {
		// If file doesn't exist, and not in update mode, create it so the user can inspect.
		if errOsStat := os.MkdirAll(filepath.Dir(goldenFile), 0755); errOsStat != nil {
			t.Fatalf("Failed to create testdata directory for new golden file: %v", errOsStat)
		}
		if errWrite := ioutil.WriteFile(goldenFile, []byte(actualOutputNormalized), 0644); errWrite != nil {
			t.Fatalf("Failed to write initial golden file %s: %v", goldenFile, errWrite)
		}
		t.Errorf("Golden file %s did not exist. Created it with current output. Verify and re-run. Or use -update flag.", goldenFile)
		return
	}

	expectedOutputNormalized := strings.ReplaceAll(string(expectedOutputBytes), "\r\n", "\n")

	if actualOutputNormalized != expectedOutputNormalized {
		var diffDetails strings.Builder
		diffDetails.WriteString(fmt.Sprintf("Output does not match golden file %s.\n", goldenFile))

		expectedLines := strings.Split(expectedOutputNormalized, "\n")
		actualLines := strings.Split(actualOutputNormalized, "\n")

		maxLines := len(expectedLines)
		if len(actualLines) > maxLines {
			maxLines = len(actualLines)
		}

		diffDetails.WriteString("Line-by-line differences:\n")
		foundDiff := false
		for i := 0; i < maxLines; i++ {
			var eLine, aLine string
			lineDiff := false
			if i < len(expectedLines) {
				eLine = expectedLines[i]
			} else {
				eLine = "<no line>"
				lineDiff = true
			}
			if i < len(actualLines) {
				aLine = actualLines[i]
			} else {
				aLine = "<no line>"
				lineDiff = true
			}

			if !lineDiff && eLine != aLine {
				lineDiff = true
			}

			if lineDiff {
				foundDiff = true
				diffDetails.WriteString(fmt.Sprintf("Line %d:\n", i+1))
				diffDetails.WriteString(fmt.Sprintf("  Expected: %s\n", strconv.Quote(eLine)))
				diffDetails.WriteString(fmt.Sprintf("  Actual:   %s\n", strconv.Quote(aLine)))
			}
		}

		if !foundDiff {
			diffDetails.WriteString("No line-by-line differences found, but content differs (possibly trailing whitespace or normalization issue).\n")
		}

		diffDetails.WriteString("\nFull Expected Output:\n")
		diffDetails.WriteString(expectedOutputNormalized)
		diffDetails.WriteString("\nFull Actual Output:\n")
		diffDetails.WriteString(actualOutputNormalized)

		t.Error(diffDetails.String())
	}
}
