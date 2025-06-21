package capability

import (
	"testing"

	"envctl/internal/api"
)

func TestRegistry(t *testing.T) {
	t.Run("Register and Get", func(t *testing.T) {
		reg := NewRegistry()

		testCap := &api.Capability{
			ID:          "test-cap",
			Type:        "auth",
			Provider:    "test-provider",
			Name:        "Test Capability",
			Description: "A test capability",
			Features:    []string{"login", "refresh"},
		}

		// Register capability
		err := reg.Register(testCap)
		if err != nil {
			t.Fatalf("Failed to register capability: %v", err)
		}

		// Get capability
		retrieved, exists := reg.Get("test-cap")
		if !exists {
			t.Fatal("Capability not found after registration")
		}

		if retrieved.Name != testCap.Name {
			t.Errorf("Expected name %s, got %s", testCap.Name, retrieved.Name)
		}

		if retrieved.State != api.CapabilityStateActive {
			t.Errorf("Expected state %s, got %s", api.CapabilityStateActive, retrieved.State)
		}
	})

	t.Run("Register duplicate", func(t *testing.T) {
		reg := NewRegistry()

		testCap := &api.Capability{
			ID:       "test-cap-2",
			Type:     "auth",
			Provider: "test-provider",
			Name:     "Test Capability",
		}

		err := reg.Register(testCap)
		if err != nil {
			t.Fatalf("Failed to register capability: %v", err)
		}

		// Try to register again
		err = reg.Register(testCap)
		if err == nil {
			t.Fatal("Expected error when registering duplicate capability")
		}
	})

	t.Run("ListByType", func(t *testing.T) {
		reg := NewRegistry()

		// Register multiple capabilities
		authCap := &api.Capability{
			Type:     "auth",
			Provider: "auth-provider",
			Name:     "Auth Cap",
		}
		portForwardCap := &api.Capability{
			Type:     "port-forward",
			Provider: "pf-provider",
			Name:     "Port Forward Cap",
		}
		authCap2 := &api.Capability{
			Type:     "auth",
			Provider: "auth-provider-2",
			Name:     "Auth Cap 2",
		}

		reg.Register(authCap)
		reg.Register(portForwardCap)
		reg.Register(authCap2)

		// List by type
		authCaps := reg.ListByType("auth")
		if len(authCaps) != 2 {
			t.Errorf("Expected 2 auth capabilities, got %d", len(authCaps))
		}

		pfCaps := reg.ListByType("port-forward")
		if len(pfCaps) != 1 {
			t.Errorf("Expected 1 port forward capability, got %d", len(pfCaps))
		}
	})

	t.Run("ListByProvider", func(t *testing.T) {
		reg := NewRegistry()

		// Register capabilities from different providers
		cap1 := &api.Capability{
			Type:     "auth",
			Provider: "provider-a",
			Name:     "Cap 1",
		}
		cap2 := &api.Capability{
			Type:     "discovery",
			Provider: "provider-a",
			Name:     "Cap 2",
		}
		cap3 := &api.Capability{
			Type:     "auth",
			Provider: "provider-b",
			Name:     "Cap 3",
		}

		reg.Register(cap1)
		reg.Register(cap2)
		reg.Register(cap3)

		// List by provider
		providerACaps := reg.ListByProvider("provider-a")
		if len(providerACaps) != 2 {
			t.Errorf("Expected 2 capabilities from provider-a, got %d", len(providerACaps))
		}

		providerBCaps := reg.ListByProvider("provider-b")
		if len(providerBCaps) != 1 {
			t.Errorf("Expected 1 capability from provider-b, got %d", len(providerBCaps))
		}
	})

	t.Run("Unregister", func(t *testing.T) {
		reg := NewRegistry()

		testCap := &api.Capability{
			ID:       "test-cap-3",
			Type:     "auth",
			Provider: "test-provider",
			Name:     "Test Capability",
		}

		reg.Register(testCap)

		// Verify it exists
		_, exists := reg.Get("test-cap-3")
		if !exists {
			t.Fatal("Capability not found after registration")
		}

		// Unregister
		err := reg.Unregister("test-cap-3")
		if err != nil {
			t.Fatalf("Failed to unregister capability: %v", err)
		}

		// Verify it's gone
		_, exists = reg.Get("test-cap-3")
		if exists {
			t.Fatal("Capability still exists after unregistration")
		}

		// Verify it's removed from type index
		authCaps := reg.ListByType("auth")
		if len(authCaps) != 0 {
			t.Errorf("Expected 0 auth capabilities after unregister, got %d", len(authCaps))
		}
	})

	t.Run("Update", func(t *testing.T) {
		reg := NewRegistry()

		testCap := &api.Capability{
			ID:       "test-cap-4",
			Type:     "auth",
			Provider: "test-provider",
			Name:     "Test Capability",
		}

		reg.Register(testCap)

		// Update status
		err := reg.Update("test-cap-4", api.CapabilityStateUnhealthy, api.HealthUnhealthy, "Connection failed")
		if err != nil {
			t.Fatalf("Failed to update capability: %v", err)
		}

		// Verify update
		updated, _ := reg.Get("test-cap-4")
		if updated.State != api.CapabilityStateUnhealthy {
			t.Errorf("Expected state %s, got %s", api.CapabilityStateUnhealthy, updated.State)
		}
		if updated.Error != "Connection failed" {
			t.Errorf("Expected error 'Connection failed', got %s", updated.Error)
		}
	})

	t.Run("FindMatching", func(t *testing.T) {
		reg := NewRegistry()

		// Register capabilities with different features
		cap1 := &api.Capability{
			Type:     "auth",
			Provider: "provider-1",
			Name:     "Full Auth",
			Features: []string{"login", "refresh", "validate"},
		}
		cap2 := &api.Capability{
			Type:     "auth",
			Provider: "provider-2",
			Name:     "Basic Auth",
			Features: []string{"login"},
		}
		cap3 := &api.Capability{
			Type:     "auth",
			Provider: "provider-3",
			Name:     "Inactive Auth",
			Features: []string{"login", "refresh"},
		}

		reg.Register(cap1)
		reg.Register(cap2)
		reg.Register(cap3)

		// Make cap3 inactive
		reg.Update(cap3.ID, api.CapabilityStateInactive, api.HealthUnhealthy, "")

		// Find capabilities with login feature
		req := api.CapabilityRequest{
			Type:     "auth",
			Features: []string{"login"},
		}
		matches := reg.FindMatching(req)
		if len(matches) != 2 {
			t.Errorf("Expected 2 matches for login feature, got %d", len(matches))
		}

		// Find capabilities with login and refresh features
		req.Features = []string{"login", "refresh"}
		matches = reg.FindMatching(req)
		if len(matches) != 1 {
			t.Errorf("Expected 1 match for login+refresh features, got %d", len(matches))
		}
		if matches[0].Name != "Full Auth" {
			t.Errorf("Expected 'Full Auth', got %s", matches[0].Name)
		}
	})

	t.Run("Callbacks", func(t *testing.T) {
		reg := NewRegistry()

		var registeredCap *api.Capability
		var unregisteredID string
		var updatedCap *api.Capability

		// Add callbacks
		reg.OnRegister(func(cap *api.Capability) {
			registeredCap = cap
		})
		reg.OnUnregister(func(id string) {
			unregisteredID = id
		})
		reg.OnUpdate(func(cap *api.Capability) {
			updatedCap = cap
		})

		// Test registration callback
		testCap := &api.Capability{
			ID:       "test-cap-5",
			Type:     "auth",
			Provider: "test-provider",
			Name:     "Test Capability",
		}
		reg.Register(testCap)

		if registeredCap == nil || registeredCap.ID != "test-cap-5" {
			t.Error("OnRegister callback not called correctly")
		}

		// Test update callback
		reg.Update("test-cap-5", api.CapabilityStateUnhealthy, api.HealthUnhealthy, "")

		if updatedCap == nil || updatedCap.ID != "test-cap-5" {
			t.Error("OnUpdate callback not called correctly")
		}

		// Test unregister callback
		reg.Unregister("test-cap-5")

		if unregisteredID != "test-cap-5" {
			t.Error("OnUnregister callback not called correctly")
		}
	})

	t.Run("Auto-generate ID", func(t *testing.T) {
		reg := NewRegistry()

		testCap := &api.Capability{
			Type:     "auth",
			Provider: "test-provider",
			Name:     "Test Capability",
		}

		err := reg.Register(testCap)
		if err != nil {
			t.Fatalf("Failed to register capability: %v", err)
		}

		if testCap.ID == "" {
			t.Error("Expected auto-generated ID, got empty string")
		}
	})
}

func TestConcurrency(t *testing.T) {
	reg := NewRegistry()
	done := make(chan bool)

	// Start multiple goroutines registering capabilities
	for i := 0; i < 10; i++ {
		go func(id int) {
			testCap := &api.Capability{
				Type:     "auth",
				Provider: "test-provider",
				Name:     "Test Capability",
			}
			reg.Register(testCap)
			done <- true
		}(i)
	}

	// Start multiple goroutines listing capabilities
	for i := 0; i < 10; i++ {
		go func() {
			reg.ListAll()
			reg.ListByType("auth")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Verify some capabilities were registered
	all := reg.ListAll()
	if len(all) == 0 {
		t.Error("Expected some capabilities to be registered")
	}
}
