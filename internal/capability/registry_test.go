package capability

import (
	"testing"
)

func TestRegistry(t *testing.T) {
	t.Run("Register and Get", func(t *testing.T) {
		reg := NewRegistry()

		cap := &Capability{
			ID:          "test-cap-1",
			Type:        CapabilityTypeAuth,
			Provider:    "test-provider",
			Name:        "Test Capability",
			Description: "A test capability",
			Features:    []string{"login", "refresh"},
		}

		// Register capability
		err := reg.Register(cap)
		if err != nil {
			t.Fatalf("Failed to register capability: %v", err)
		}

		// Get capability
		retrieved, exists := reg.Get("test-cap-1")
		if !exists {
			t.Fatal("Capability not found after registration")
		}

		if retrieved.Name != cap.Name {
			t.Errorf("Expected name %s, got %s", cap.Name, retrieved.Name)
		}

		if retrieved.Status.State != CapabilityStateActive {
			t.Errorf("Expected state %s, got %s", CapabilityStateActive, retrieved.Status.State)
		}
	})

	t.Run("Register duplicate", func(t *testing.T) {
		reg := NewRegistry()

		cap := &Capability{
			ID:       "test-cap-2",
			Type:     CapabilityTypeAuth,
			Provider: "test-provider",
			Name:     "Test Capability",
		}

		err := reg.Register(cap)
		if err != nil {
			t.Fatalf("Failed to register capability: %v", err)
		}

		// Try to register again
		err = reg.Register(cap)
		if err == nil {
			t.Fatal("Expected error when registering duplicate capability")
		}
	})

	t.Run("ListByType", func(t *testing.T) {
		reg := NewRegistry()

		// Register multiple capabilities
		authCap := &Capability{
			Type:     CapabilityTypeAuth,
			Provider: "auth-provider",
			Name:     "Auth Cap",
		}
		portForwardCap := &Capability{
			Type:     CapabilityTypePortForward,
			Provider: "pf-provider",
			Name:     "Port Forward Cap",
		}
		authCap2 := &Capability{
			Type:     CapabilityTypeAuth,
			Provider: "auth-provider-2",
			Name:     "Auth Cap 2",
		}

		reg.Register(authCap)
		reg.Register(portForwardCap)
		reg.Register(authCap2)

		// List by type
		authCaps := reg.ListByType(CapabilityTypeAuth)
		if len(authCaps) != 2 {
			t.Errorf("Expected 2 auth capabilities, got %d", len(authCaps))
		}

		pfCaps := reg.ListByType(CapabilityTypePortForward)
		if len(pfCaps) != 1 {
			t.Errorf("Expected 1 port forward capability, got %d", len(pfCaps))
		}
	})

	t.Run("ListByProvider", func(t *testing.T) {
		reg := NewRegistry()

		// Register capabilities from different providers
		cap1 := &Capability{
			Type:     CapabilityTypeAuth,
			Provider: "provider-a",
			Name:     "Cap 1",
		}
		cap2 := &Capability{
			Type:     CapabilityTypeDiscovery,
			Provider: "provider-a",
			Name:     "Cap 2",
		}
		cap3 := &Capability{
			Type:     CapabilityTypeAuth,
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

		cap := &Capability{
			ID:       "test-cap-3",
			Type:     CapabilityTypeAuth,
			Provider: "test-provider",
			Name:     "Test Capability",
		}

		reg.Register(cap)

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
		authCaps := reg.ListByType(CapabilityTypeAuth)
		if len(authCaps) != 0 {
			t.Errorf("Expected 0 auth capabilities after unregister, got %d", len(authCaps))
		}
	})

	t.Run("Update", func(t *testing.T) {
		reg := NewRegistry()

		cap := &Capability{
			ID:       "test-cap-4",
			Type:     CapabilityTypeAuth,
			Provider: "test-provider",
			Name:     "Test Capability",
		}

		reg.Register(cap)

		// Update status
		newStatus := CapabilityStatus{
			State:  CapabilityStateUnhealthy,
			Error:  "Connection failed",
			Health: HealthStatusUnhealthy,
		}

		err := reg.Update("test-cap-4", newStatus)
		if err != nil {
			t.Fatalf("Failed to update capability: %v", err)
		}

		// Verify update
		updated, _ := reg.Get("test-cap-4")
		if updated.Status.State != CapabilityStateUnhealthy {
			t.Errorf("Expected state %s, got %s", CapabilityStateUnhealthy, updated.Status.State)
		}
		if updated.Status.Error != "Connection failed" {
			t.Errorf("Expected error 'Connection failed', got %s", updated.Status.Error)
		}
	})

	t.Run("FindMatching", func(t *testing.T) {
		reg := NewRegistry()

		// Register capabilities with different features
		cap1 := &Capability{
			Type:     CapabilityTypeAuth,
			Provider: "provider-1",
			Name:     "Full Auth",
			Features: []string{"login", "refresh", "validate"},
		}
		cap2 := &Capability{
			Type:     CapabilityTypeAuth,
			Provider: "provider-2",
			Name:     "Basic Auth",
			Features: []string{"login"},
		}
		cap3 := &Capability{
			Type:     CapabilityTypeAuth,
			Provider: "provider-3",
			Name:     "Inactive Auth",
			Features: []string{"login", "refresh"},
		}

		reg.Register(cap1)
		reg.Register(cap2)
		reg.Register(cap3)

		// Make cap3 inactive
		reg.Update(cap3.ID, CapabilityStatus{
			State:  CapabilityStateInactive,
			Health: HealthStatusUnhealthy,
		})

		// Find capabilities with login feature
		req := CapabilityRequest{
			Type:     CapabilityTypeAuth,
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

		var registeredCap *Capability
		var unregisteredID string
		var updatedCap *Capability

		// Add callbacks
		reg.OnRegister(func(cap *Capability) {
			registeredCap = cap
		})
		reg.OnUnregister(func(id string) {
			unregisteredID = id
		})
		reg.OnUpdate(func(cap *Capability) {
			updatedCap = cap
		})

		// Test registration callback
		cap := &Capability{
			ID:       "test-cap-5",
			Type:     CapabilityTypeAuth,
			Provider: "test-provider",
			Name:     "Test Capability",
		}
		reg.Register(cap)

		if registeredCap == nil || registeredCap.ID != "test-cap-5" {
			t.Error("OnRegister callback not called correctly")
		}

		// Test update callback
		reg.Update("test-cap-5", CapabilityStatus{
			State: CapabilityStateUnhealthy,
		})

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

		cap := &Capability{
			Type:     CapabilityTypeAuth,
			Provider: "test-provider",
			Name:     "Test Capability",
		}

		err := reg.Register(cap)
		if err != nil {
			t.Fatalf("Failed to register capability: %v", err)
		}

		if cap.ID == "" {
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
			cap := &Capability{
				Type:     CapabilityTypeAuth,
				Provider: "test-provider",
				Name:     "Test Capability",
			}
			reg.Register(cap)
			done <- true
		}(i)
	}

	// Start multiple goroutines listing capabilities
	for i := 0; i < 10; i++ {
		go func() {
			reg.ListAll()
			reg.ListByType(CapabilityTypeAuth)
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