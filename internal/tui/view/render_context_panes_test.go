package view

import (
	"envctl/internal/api"
	"envctl/internal/tui/model"
	"testing"
)

func TestRenderMcPane_FindsConnectionWithNewLabel(t *testing.T) {
	// Create test model with new label format
	m := &model.Model{
		ManagementClusterName: "test",
		K8sConnections: map[string]*api.K8sConnectionInfo{
			"mc-test": {
				Label:      "mc-test",
				Context:    "teleport.giantswarm.io-test",
				IsMC:       true,
				State:      "running",
				Health:     "healthy",
				ReadyNodes: 3,
				TotalNodes: 3,
			},
		},
	}

	// Render MC pane - should find the connection
	result := renderMcPane(m, nil, 60)

	// Verify result contains expected content
	if len(result) == 0 {
		t.Error("Expected non-empty result from renderMcPane")
	}

	// Should contain MC name
	if !contains(result, "MC: test") {
		t.Error("Expected result to contain 'MC: test'")
	}

	// Should contain healthy node status
	if !contains(result, "3/3") {
		t.Error("Expected result to contain healthy node count '3/3'")
	}
}

func TestRenderWcPane_FindsConnectionWithNewLabel(t *testing.T) {
	// Create test model with new label format
	m := &model.Model{
		ManagementClusterName: "test",
		WorkloadClusterName:   "work",
		K8sConnections: map[string]*api.K8sConnectionInfo{
			"wc-work": {
				Label:      "wc-work",
				Context:    "teleport.giantswarm.io-test-work",
				IsMC:       false,
				State:      "running",
				Health:     "unhealthy",
				ReadyNodes: 1,
				TotalNodes: 2,
			},
		},
	}

	// Render WC pane - should find the connection
	result := renderWcPane(m, nil, 60)

	// Verify result contains expected content
	if len(result) == 0 {
		t.Error("Expected non-empty result from renderWcPane")
	}

	// Should contain WC name
	if !contains(result, "WC: work") {
		t.Error("Expected result to contain 'WC: work'")
	}

	// Should contain degraded node status
	if !contains(result, "1/2") {
		t.Error("Expected result to contain degraded node count '1/2'")
	}
}

func TestRenderContextPanesRow_BothClusters(t *testing.T) {
	// Create test model with both MC and WC
	m := &model.Model{
		ManagementClusterName: "test",
		WorkloadClusterName:   "work",
		K8sConnections: map[string]*api.K8sConnectionInfo{
			"mc-test": {
				Label:      "mc-test",
				Context:    "teleport.giantswarm.io-test",
				IsMC:       true,
				State:      "running",
				Health:     "healthy",
				ReadyNodes: 3,
				TotalNodes: 3,
			},
			"wc-work": {
				Label:      "wc-work",
				Context:    "teleport.giantswarm.io-test-work",
				IsMC:       false,
				State:      "running",
				Health:     "healthy",
				ReadyNodes: 2,
				TotalNodes: 2,
			},
		},
	}

	// Render both panes
	result := renderContextPanesRow(m, 120, 10)

	// Verify result contains both clusters
	if !contains(result, "MC: test") {
		t.Error("Expected result to contain 'MC: test'")
	}
	if !contains(result, "WC: work") {
		t.Error("Expected result to contain 'WC: work'")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) > len(substr) &&
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	if len(s) <= len(substr) {
		return false
	}
	for i := 1; i < len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
