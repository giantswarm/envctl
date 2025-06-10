package k8s

import (
	"context"
	"envctl/internal/capability"
	"envctl/internal/kube"
	"envctl/internal/services"
	"errors"
	"strings"
	"testing"
)

// mockKubeManagerForCapability implements kube.Manager for testing
type mockKubeManagerForCapability struct {
	loginErr      error
	nodeHealth    kube.NodeHealth
	nodeHealthErr error
	loginCalled   bool // Track if Login was called
}

func (m *mockKubeManagerForCapability) Login(clusterName string) (string, string, error) {
	m.loginCalled = true
	if m.loginErr != nil {
		return "", "login error", m.loginErr
	}
	return "Logged in successfully", "", nil
}

func (m *mockKubeManagerForCapability) ListClusters() (*kube.ClusterInfo, error) {
	return nil, nil
}

func (m *mockKubeManagerForCapability) GetCurrentContext() (string, error) {
	return "test-context", nil
}

func (m *mockKubeManagerForCapability) SwitchContext(targetContextName string) error {
	return nil
}

func (m *mockKubeManagerForCapability) GetAvailableContexts() ([]string, error) {
	return []string{"test-context"}, nil
}

func (m *mockKubeManagerForCapability) BuildMcContextName(mcName string) string {
	return "teleport.giantswarm.io-" + mcName
}

func (m *mockKubeManagerForCapability) BuildWcContextName(mcName, wcName string) string {
	return "teleport.giantswarm.io-" + mcName + "-" + wcName
}

func (m *mockKubeManagerForCapability) StripTeleportPrefix(contextName string) string {
	prefix := "teleport.giantswarm.io-"
	if strings.HasPrefix(contextName, prefix) {
		return strings.TrimPrefix(contextName, prefix)
	}
	return contextName
}

func (m *mockKubeManagerForCapability) HasTeleportPrefix(contextName string) bool {
	return strings.HasPrefix(contextName, "teleport.giantswarm.io-")
}

func (m *mockKubeManagerForCapability) GetClusterNodeHealth(ctx context.Context, kubeContextName string) (kube.NodeHealth, error) {
	return m.nodeHealth, m.nodeHealthErr
}

func (m *mockKubeManagerForCapability) DetermineClusterProvider(ctx context.Context, kubeContextName string) (string, error) {
	return "aws", nil
}

func (m *mockKubeManagerForCapability) CheckAPIHealth(ctx context.Context, kubeContextName string) (string, error) {
	return "v1.28.0", nil
}

func (m *mockKubeManagerForCapability) SetAuthProvider(provider kube.AuthProvider) {
	// Mock implementation
}

func (m *mockKubeManagerForCapability) GetAuthProvider() kube.AuthProvider {
	return nil
}

func TestCapabilityK8sConnectionService_WithoutCapability(t *testing.T) {
	// Test that the service works with traditional auth when no capability is available
	kubeMgr := &mockKubeManagerForCapability{
		nodeHealth: kube.NodeHealth{
			ReadyNodes: 3,
			TotalNodes: 3,
		},
	}

	service := NewCapabilityK8sConnectionService("test-cluster", "teleport.giantswarm.io-test", true, kubeMgr)

	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Errorf("Unexpected error starting service: %v", err)
	}

	if service.GetState() != services.StateRunning {
		t.Errorf("Expected state %s, got %s", services.StateRunning, service.GetState())
	}

	// Verify it's using traditional auth
	data := service.GetServiceData()
	if hasCapAuth, ok := data["has_capability_auth"].(bool); !ok || hasCapAuth {
		t.Error("Expected has_capability_auth to be false")
	}

	// Stop the service
	err = service.Stop(ctx)
	if err != nil {
		t.Errorf("Error stopping service: %v", err)
	}
}

func TestCapabilityK8sConnectionService_CapabilityProvided(t *testing.T) {
	// Test that the service recognizes when a capability is provided
	kubeMgr := &mockKubeManagerForCapability{
		nodeHealth: kube.NodeHealth{
			ReadyNodes: 3,
			TotalNodes: 3,
		},
	}

	service := NewCapabilityK8sConnectionService("test-cluster", "teleport.giantswarm.io-test", true, kubeMgr)

	// Simulate capability being provided
	handle := capability.CapabilityHandle{
		ID:       "test-handle",
		Provider: "teleport-mcp",
		Type:     capability.CapabilityTypeAuth,
	}

	err := service.OnCapabilityProvided(handle)
	if err != nil {
		t.Errorf("Error on capability provided: %v", err)
	}

	// Start the service
	ctx := context.Background()
	err = service.Start(ctx)
	if err != nil {
		t.Errorf("Unexpected error starting service: %v", err)
	}

	// Verify it recognizes the capability (but still uses traditional auth)
	data := service.GetServiceData()
	if authProvider, ok := data["auth_provider"].(string); !ok || authProvider != "teleport-mcp" {
		t.Errorf("Expected auth_provider to be teleport-mcp, got %v", authProvider)
	}

	// Stop the service
	service.Stop(ctx)
}

func TestCapabilityK8sConnectionService_HealthCheck(t *testing.T) {
	tests := []struct {
		name           string
		nodeHealth     kube.NodeHealth
		nodeHealthErr  error
		expectedHealth services.HealthStatus
		expectError    bool
	}{
		{
			name: "healthy cluster",
			nodeHealth: kube.NodeHealth{
				ReadyNodes: 3,
				TotalNodes: 3,
			},
			expectedHealth: services.HealthHealthy,
			expectError:    false,
		},
		{
			name: "degraded cluster",
			nodeHealth: kube.NodeHealth{
				ReadyNodes: 2,
				TotalNodes: 3,
			},
			expectedHealth: services.HealthUnhealthy,
			expectError:    true,
		},
		{
			name: "no nodes",
			nodeHealth: kube.NodeHealth{
				ReadyNodes: 0,
				TotalNodes: 0,
			},
			expectedHealth: services.HealthUnhealthy,
			expectError:    true,
		},
		{
			name:           "api error",
			nodeHealthErr:  errors.New("api unavailable"),
			expectedHealth: services.HealthUnhealthy,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeMgr := &mockKubeManagerForCapability{
				nodeHealth:    tt.nodeHealth,
				nodeHealthErr: tt.nodeHealthErr,
			}

			service := NewCapabilityK8sConnectionService("test-cluster", "test-context", true, kubeMgr)

			ctx := context.Background()
			health, err := service.CheckHealth(ctx)

			if (err != nil) != tt.expectError {
				t.Errorf("CheckHealth() error = %v, expectError %v", err, tt.expectError)
			}

			if health != tt.expectedHealth {
				t.Errorf("CheckHealth() health = %v, expected %v", health, tt.expectedHealth)
			}
		})
	}
}

func TestCapabilityK8sConnectionService_CapabilityLost(t *testing.T) {
	kubeMgr := &mockKubeManagerForCapability{
		nodeHealth: kube.NodeHealth{
			ReadyNodes: 3,
			TotalNodes: 3,
		},
	}

	service := NewCapabilityK8sConnectionService("test-cluster", "test-context", true, kubeMgr)

	// Simulate capability being provided
	handle := capability.CapabilityHandle{
		ID:       "test-handle",
		Provider: "teleport-mcp",
		Type:     capability.CapabilityTypeAuth,
	}

	err := service.OnCapabilityProvided(handle)
	if err != nil {
		t.Errorf("Error on capability provided: %v", err)
	}

	// Simulate capability being lost
	err = service.OnCapabilityLost("test-handle")
	if err != nil {
		t.Errorf("Error on capability lost: %v", err)
	}

	// Verify capability is cleared
	data := service.GetServiceData()
	if hasCapAuth, ok := data["has_capability_auth"].(bool); !ok || hasCapAuth {
		t.Error("Expected has_capability_auth to be false after capability lost")
	}
}

func TestCapabilityK8sConnectionService_Restart(t *testing.T) {
	kubeMgr := &mockKubeManagerForCapability{
		nodeHealth: kube.NodeHealth{
			ReadyNodes: 3,
			TotalNodes: 3,
		},
	}

	service := NewCapabilityK8sConnectionService("test-cluster", "test-context", true, kubeMgr)

	ctx := context.Background()

	// Start the service
	err := service.Start(ctx)
	if err != nil {
		t.Errorf("Error starting service: %v", err)
	}

	// Restart the service
	err = service.Restart(ctx)
	if err != nil {
		t.Errorf("Error restarting service: %v", err)
	}

	// Verify it's running
	if service.GetState() != services.StateRunning {
		t.Errorf("Expected state %s after restart, got %s", services.StateRunning, service.GetState())
	}

	// Stop the service
	service.Stop(ctx)
}

func TestCapabilityK8sConnectionService_IsForCluster(t *testing.T) {
	kubeMgr := &mockKubeManagerForCapability{}

	service := NewCapabilityK8sConnectionService("my-cluster", "teleport.giantswarm.io-my-cluster", true, kubeMgr)

	tests := []struct {
		clusterName string
		expected    bool
	}{
		{"my-cluster", true},
		{"other-cluster", false},
		{"teleport.giantswarm.io-my-cluster", true},
		{"", false},
	}

	for _, tt := range tests {
		result := service.IsForCluster(tt.clusterName)
		if result != tt.expected {
			t.Errorf("IsForCluster(%s) = %v, expected %v", tt.clusterName, result, tt.expected)
		}
	}
}

func TestCapabilityK8sConnectionService_NoLoginForNonTeleport(t *testing.T) {
	kubeMgr := &mockKubeManagerForCapability{
		nodeHealth: kube.NodeHealth{
			ReadyNodes: 3,
			TotalNodes: 3,
		},
	}

	// Use a non-teleport context
	service := NewCapabilityK8sConnectionService("local-cluster", "local-context", false, kubeMgr)

	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Errorf("Unexpected error starting service: %v", err)
	}

	// Verify login was not called for non-teleport context
	if kubeMgr.loginCalled {
		t.Error("Login should not be called for non-teleport context")
	}

	service.Stop(ctx)
}
