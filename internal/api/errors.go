package api

import "errors"

// Common errors for API operations
var (
	// Handler not registered errors
	ErrOrchestratorNotRegistered  = errors.New("orchestrator handler not registered")
	ErrMCPServiceNotRegistered    = errors.New("MCP service handler not registered")
	ErrPortForwardNotRegistered   = errors.New("port forward handler not registered")
	ErrK8sServiceNotRegistered    = errors.New("K8s service handler not registered")
	ErrConfigServiceNotRegistered = errors.New("config service handler not registered")
	ErrCapabilityNotRegistered    = errors.New("capability handler not registered")
	ErrWorkflowNotRegistered      = errors.New("workflow handler not registered")
	ErrAggregatorNotRegistered    = errors.New("aggregator handler not registered")

	// Workflow errors
	ErrWorkflowNotFound = errors.New("workflow not found")
)
