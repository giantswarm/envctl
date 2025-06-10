// Package k8s provides Kubernetes connection services for envctl.
//
// This package implements services that manage connections to Kubernetes clusters
// through various authentication methods. It provides both traditional and
// capability-aware implementations to support the transition to envctl's new
// capability-based architecture.
//
// # Service Types
//
// K8sConnectionService: The traditional K8s connection service that uses the
// kube.Manager interface for authentication and cluster management. This service
// directly calls `tsh kube login` for Teleport-based authentication.
//
// CapabilityK8sConnectionService: A future-ready K8s connection service that
// declares its need for authentication capabilities. While it currently falls
// back to traditional authentication, it's designed to seamlessly switch to
// capability-based authentication when auth providers (like Teleport MCP)
// implement the required tools.
//
// # Capability Support
//
// The CapabilityK8sConnectionService declares an optional dependency on the
// "auth_provider" capability. When auth providers register this capability
// and implement the required tools (x_auth_login, x_auth_validate, etc.),
// the service will automatically use them instead of hardcoded authentication.
//
// This design allows for:
// - Backward compatibility with existing Teleport authentication
// - Forward compatibility with capability-based auth providers
// - Support for alternative authentication methods (AWS, GCP, kubeconfig)
// - No changes required when auth providers become available
//
// # Usage
//
// For traditional authentication:
//
//	service := NewK8sConnectionService("cluster-name", "context-name", true, kubeMgr)
//	err := service.Start(ctx)
//
// For capability-aware authentication (with automatic fallback):
//
//	service := NewCapabilityK8sConnectionService("cluster-name", "context-name", true, kubeMgr)
//	err := service.Start(ctx)
//
// # Health Monitoring
//
// Both services implement health monitoring that periodically checks:
// - Authentication validity
// - Cluster API availability
// - Node health status
//
// The health status is reported as:
// - Healthy: All nodes ready and API responsive
// - Unhealthy: Authentication failed, API unreachable, or nodes not ready
// - Unknown: Health check not yet performed
//
// # Future Enhancements
//
// When auth providers implement the capability interface:
// 1. Remove direct kube.Manager dependency from CapabilityK8sConnectionService
// 2. Implement capability-based health checks
// 3. Add support for token refresh through capabilities
// 4. Enable dynamic auth provider switching
package k8s
