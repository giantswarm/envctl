package capability

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapabilityRegistry(t *testing.T) {
	registry := NewRegistry()

	// Test registering a capability
	cap := &Capability{
		Type:        CapabilityTypeAuth,
		Provider:    "test-provider",
		Name:        "Test Auth Provider",
		Description: "A test authentication provider",
		Features:    []string{"login", "sso"},
		Config:      map[string]interface{}{"url": "https://test.com"},
		Metadata:    map[string]string{"version": "1.0"},
	}

	err := registry.Register(cap)
	require.NoError(t, err)
	assert.NotEmpty(t, cap.ID)

	// Test getting the capability
	retrieved, exists := registry.Get(cap.ID)
	assert.True(t, exists)
	assert.Equal(t, cap.Name, retrieved.Name)

	// Test listing by type
	caps := registry.ListByType(CapabilityTypeAuth)
	assert.Len(t, caps, 1)
	assert.Equal(t, cap.ID, caps[0].ID)

	// Test updating status
	status := CapabilityStatus{
		State:     CapabilityStateActive,
		Health:    HealthStatusHealthy,
		LastCheck: time.Now(),
	}
	err = registry.Update(cap.ID, status)
	require.NoError(t, err)

	// Verify update
	retrieved, _ = registry.Get(cap.ID)
	assert.Equal(t, CapabilityStateActive, retrieved.Status.State)

	// Test unregistering
	err = registry.Unregister(cap.ID)
	require.NoError(t, err)

	_, exists = registry.Get(cap.ID)
	assert.False(t, exists)
}

func TestCapabilityResolver(t *testing.T) {
	registry := NewRegistry()
	resolver := NewResolver(registry)

	// Register two auth providers
	cap1 := &Capability{
		Type:     CapabilityTypeAuth,
		Provider: "provider1",
		Name:     "Provider 1",
		Features: []string{"login", "sso"},
		Status: CapabilityStatus{
			State:  CapabilityStateActive,
			Health: HealthStatusHealthy,
		},
	}
	cap2 := &Capability{
		Type:     CapabilityTypeAuth,
		Provider: "provider2",
		Name:     "Provider 2",
		Features: []string{"login", "mfa"},
		Status: CapabilityStatus{
			State:  CapabilityStateActive,
			Health: HealthStatusHealthy,
		},
	}

	registry.Register(cap1)
	registry.Register(cap2)

	// Test finding matching capabilities
	request := CapabilityRequest{
		Type:     CapabilityTypeAuth,
		Features: []string{"login"},
	}

	matches := registry.FindMatching(request)
	assert.Len(t, matches, 2)

	// Test requesting a capability with SSO feature
	request.Features = []string{"login", "sso"}
	handle, err := resolver.RequestCapability("test-service", request)
	require.NoError(t, err)
	assert.Equal(t, "provider1", handle.Provider)
	assert.Equal(t, CapabilityTypeAuth, handle.Type)

	// Test releasing capability
	err = resolver.ReleaseCapability("test-service", handle.ID)
	assert.NoError(t, err)
}

func TestCapabilityMatching(t *testing.T) {
	registry := NewRegistry()

	// Register capabilities with different features
	caps := []*Capability{
		{
			Type:     CapabilityTypePortForward,
			Provider: "kube-pf",
			Name:     "Kubernetes Port Forward",
			Features: []string{"tcp", "http", "grpc"},
			Status:   CapabilityStatus{State: CapabilityStateActive, Health: HealthStatusHealthy},
		},
		{
			Type:     CapabilityTypePortForward,
			Provider: "ssh-pf",
			Name:     "SSH Port Forward",
			Features: []string{"tcp"},
			Status:   CapabilityStatus{State: CapabilityStateActive, Health: HealthStatusHealthy},
		},
		{
			Type:     CapabilityTypePortForward,
			Provider: "teleport-pf",
			Name:     "Teleport Port Forward",
			Features: []string{"tcp", "http"},
			Status:   CapabilityStatus{State: CapabilityStateUnhealthy, Health: HealthStatusUnhealthy},
		},
	}

	for _, cap := range caps {
		registry.Register(cap)
	}

	// Set the third capability to unhealthy state after registration
	teleportCap := caps[2]
	registry.Update(teleportCap.ID, CapabilityStatus{
		State:  CapabilityStateUnhealthy,
		Health: HealthStatusUnhealthy,
	})

	// Test matching with required features
	request := CapabilityRequest{
		Type:     CapabilityTypePortForward,
		Features: []string{"tcp", "http"},
	}

	matches := registry.FindMatching(request)
	// Should only return healthy capabilities with all required features
	assert.Len(t, matches, 1)
	assert.Equal(t, "kube-pf", matches[0].Provider)

	// Test matching with single feature
	request.Features = []string{"tcp"}
	matches = registry.FindMatching(request)
	assert.Len(t, matches, 2) // kube-pf and ssh-pf
}
