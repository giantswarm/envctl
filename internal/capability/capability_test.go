package capability

import (
	"testing"

	"envctl/internal/api"
)

func TestCapabilityRegistry(t *testing.T) {
	registry := NewRegistry()

	// Test registration
	cap := &api.Capability{
		ID:       "test-cap",
		Name:     "Test Capability",
		Type:     "database",
		Provider: "test-provider",
		Features: []string{"read", "write"},
		State:    api.CapabilityStateRegistering,
		Health:   api.HealthUnknown,
	}

	err := registry.Register(cap)
	if err != nil {
		t.Fatalf("Failed to register capability: %v", err)
	}

	// Test retrieval
	retrieved, exists := registry.Get(cap.ID)
	if !exists {
		t.Fatal("Capability should exist after registration")
	}

	if retrieved.ID != cap.ID {
		t.Errorf("Expected ID %s, got %s", cap.ID, retrieved.ID)
	}

	// Test status update
	err = registry.Update(cap.ID, api.CapabilityStateActive, api.HealthHealthy, "")
	if err != nil {
		t.Fatalf("Failed to update capability status: %v", err)
	}

	// Verify update
	retrieved, _ = registry.Get(cap.ID)
	if retrieved.State != api.CapabilityStateActive {
		t.Errorf("Expected state %s, got %s", api.CapabilityStateActive, retrieved.State)
	}
}

func TestCapabilityResolver(t *testing.T) {
	registry := NewRegistry()

	// Register a capability
	cap := &api.Capability{
		ID:       "test-cap",
		Name:     "Test Capability",
		Type:     "database",
		Provider: "test-provider",
		Features: []string{"read", "write"},
		State:    api.CapabilityStateActive,
		Health:   api.HealthHealthy,
	}

	err := registry.Register(cap)
	if err != nil {
		t.Fatalf("Failed to register capability: %v", err)
	}

	// Test capability matching
	req := api.CapabilityRequest{
		Type:     "database",
		Features: []string{"read"},
		Config:   map[string]interface{}{"host": "localhost"},
	}

	matching := registry.FindMatching(req)
	if len(matching) != 1 {
		t.Errorf("Expected 1 matching capability, got %d", len(matching))
	}

	if len(matching) > 0 && matching[0].ID != cap.ID {
		t.Errorf("Expected matching capability %s, got %s", cap.ID, matching[0].ID)
	}
}

func TestCapabilityMatching(t *testing.T) {
	registry := NewRegistry()

	// Register capabilities with different features
	cap1 := &api.Capability{
		Type:     "port-forward",
		Provider: "kube-pf",
		Name:     "Kubernetes Port Forward",
		Features: []string{"tcp", "http", "grpc"},
		State:    api.CapabilityStateActive,
		Health:   api.HealthHealthy,
	}

	cap2 := &api.Capability{
		Type:     "port-forward",
		Provider: "ssh-pf",
		Name:     "SSH Port Forward",
		Features: []string{"tcp"},
		State:    api.CapabilityStateActive,
		Health:   api.HealthHealthy,
	}

	cap3 := &api.Capability{
		Type:     "port-forward",
		Provider: "teleport-pf",
		Name:     "Teleport Port Forward",
		Features: []string{"tcp", "http"},
		State:    api.CapabilityStateActive,
		Health:   api.HealthHealthy,
	}

	// Register all capabilities
	registry.Register(cap1)
	registry.Register(cap2)
	registry.Register(cap3)

	// Set the third capability to unhealthy state after registration
	registry.Update(cap3.ID, api.CapabilityStateUnhealthy, api.HealthUnhealthy, "unhealthy")

	// Test matching with required features
	request := api.CapabilityRequest{
		Type:     "port-forward",
		Features: []string{"tcp", "http"},
	}

	matches := registry.FindMatching(request)
	// Should return capabilities with all required features
	if len(matches) < 1 {
		t.Errorf("Expected at least 1 matching capability, got %d", len(matches))
	}

	// Test matching with single feature
	request.Features = []string{"tcp"}
	matches = registry.FindMatching(request)
	if len(matches) < 2 {
		t.Errorf("Expected at least 2 matching capabilities, got %d", len(matches))
	}
}
