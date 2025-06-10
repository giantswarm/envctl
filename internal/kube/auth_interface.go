package kube

import (
	"context"
)

// AuthProvider defines the interface for cluster authentication providers
type AuthProvider interface {
	// Login authenticates to a Kubernetes cluster
	Login(ctx context.Context, clusterName string) (stdout string, stderr string, err error)

	// ListClusters returns information about available clusters
	ListClusters(ctx context.Context) (*ClusterInfo, error)

	// Validate checks if the current authentication is still valid
	Validate(ctx context.Context) error

	// GetProviderName returns the name of the auth provider (e.g., "teleport", "aws", "gcp")
	GetProviderName() string
}

// LegacyTeleportAuthProvider wraps the existing Teleport functions for backward compatibility
type LegacyTeleportAuthProvider struct{}

// NewLegacyTeleportAuthProvider creates a new legacy Teleport auth provider
func NewLegacyTeleportAuthProvider() AuthProvider {
	return &LegacyTeleportAuthProvider{}
}

// Login uses the existing LoginToKubeCluster function
func (p *LegacyTeleportAuthProvider) Login(ctx context.Context, clusterName string) (string, string, error) {
	// Use the existing function variable for backward compatibility
	return LoginToKubeCluster(clusterName)
}

// ListClusters uses the existing GetClusterInfo function
func (p *LegacyTeleportAuthProvider) ListClusters(ctx context.Context) (*ClusterInfo, error) {
	// Use the existing function variable for backward compatibility
	return GetClusterInfo()
}

// Validate checks if tsh is available
func (p *LegacyTeleportAuthProvider) Validate(ctx context.Context) error {
	// Simple validation - just check if we can list clusters
	_, err := GetClusterInfo()
	return err
}

// GetProviderName returns "teleport-legacy"
func (p *LegacyTeleportAuthProvider) GetProviderName() string {
	return "teleport-legacy"
}

// CapabilityAuthProvider uses capability-based authentication
type CapabilityAuthProvider struct {
	providerName string
	executor     ToolExecutor
}

// ToolExecutor interface for executing capability tools
type ToolExecutor interface {
	ExecuteTool(ctx context.Context, provider string, toolName string, params map[string]interface{}) (map[string]interface{}, error)
}

// NewCapabilityAuthProvider creates a new capability-based auth provider
func NewCapabilityAuthProvider(providerName string, executor ToolExecutor) AuthProvider {
	return &CapabilityAuthProvider{
		providerName: providerName,
		executor:     executor,
	}
}

// Login uses capability tools for authentication
func (p *CapabilityAuthProvider) Login(ctx context.Context, clusterName string) (string, string, error) {
	result, err := p.executor.ExecuteTool(ctx, p.providerName, "x_auth_login", map[string]interface{}{
		"cluster": clusterName,
	})
	if err != nil {
		return "", "", err
	}

	stdout, _ := result["stdout"].(string)
	stderr, _ := result["stderr"].(string)
	return stdout, stderr, nil
}

// ListClusters uses capability tools to list clusters
func (p *CapabilityAuthProvider) ListClusters(ctx context.Context) (*ClusterInfo, error) {
	result, err := p.executor.ExecuteTool(ctx, p.providerName, "x_auth_list_clusters", nil)
	if err != nil {
		return nil, err
	}

	// Parse the result and construct ClusterInfo
	info := &ClusterInfo{
		ManagementClusters: []string{},
		WorkloadClusters:   make(map[string][]string),
	}

	if clusters, ok := result["clusters"].([]interface{}); ok {
		for _, cluster := range clusters {
			if clusterMap, ok := cluster.(map[string]interface{}); ok {
				name, _ := clusterMap["name"].(string)
				clusterType, _ := clusterMap["type"].(string)

				if clusterType == "management" {
					info.ManagementClusters = append(info.ManagementClusters, name)
				} else if clusterType == "workload" {
					parent, _ := clusterMap["parent"].(string)
					info.WorkloadClusters[parent] = append(info.WorkloadClusters[parent], name)
				}
			}
		}
	}

	return info, nil
}

// Validate uses capability tools to validate authentication
func (p *CapabilityAuthProvider) Validate(ctx context.Context) error {
	_, err := p.executor.ExecuteTool(ctx, p.providerName, "x_auth_validate", nil)
	return err
}

// GetProviderName returns the provider name
func (p *CapabilityAuthProvider) GetProviderName() string {
	return p.providerName
}
