// Package capability provides a framework for registering and managing capabilities
// in envctl. Capabilities represent pluggable functionality that can be provided
// by MCP servers, such as authentication, discovery, and port forwarding.
//
// # Overview
//
// The capability system enables envctl to be extended with new functionality
// without modifying the core code. MCP servers can register capabilities they
// provide, and services can declare capability requirements.
//
// # Core Concepts
//
// Capability: A specific functionality that can be provided by an MCP server.
// Examples include auth_provider, discovery_provider, and portforward_provider.
//
// Capability Provider: An MCP server that registers and provides one or more
// capabilities.
//
// Capability Consumer: A service that requires certain capabilities to function.
//
// Capability Registry: Central registry that tracks all available capabilities
// and matches requirements to providers.
//
// # Usage
//
// MCP servers register capabilities:
//
//	capability := &Capability{
//	    Type: CapabilityTypeAuth,
//	    Provider: "teleport-mcp",
//	    Name: "Teleport Authentication",
//	    Features: []string{"login", "refresh", "validate"},
//	}
//	registry.Register(capability)
//
// Services declare requirements:
//
//	requirements := []CapabilityRequirement{
//	    {
//	        Type: CapabilityTypeAuth,
//	        Config: map[string]interface{}{
//	            "cluster_role": "workload",
//	        },
//	    },
//	}
//
// The orchestrator resolves requirements to providers and manages dependencies.
package capability