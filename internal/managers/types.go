package managers

import "envctl/internal/reporting"

// ServiceType indicates the kind of service being managed.
// This will be replaced by reporting.ServiceType
// type ServiceType string // REMOVED

// const (
// 	ServiceTypePortForward ServiceType = "PortForward" // REMOVED
// 	ServiceTypeMCPServer   ServiceType = "MCPServer"   // REMOVED
// )

// ManagedServiceConfig wraps a configuration for any manageable service.
// This will use reporting.ServiceType or be adapted
type ManagedServiceConfig struct {
	Type   reporting.ServiceType // Changed to reporting.ServiceType
	Label  string                // A unique label for this service instance
	Config interface{}           // Actual config (e.g., portforwarding.PortForwardingConfig or mcpserver.MCPServerConfig)
}

// ManagedServiceUpdate type will be removed from here and reporting.ManagedServiceUpdate will be used.
// type ManagedServiceUpdate struct { ... } // REMOVED

// ServiceUpdateFunc type will be removed from here.
// type ServiceUpdateFunc func(update ManagedServiceUpdate) // REMOVED
