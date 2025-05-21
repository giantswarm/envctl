package managers

// ServiceType indicates the kind of service being managed.
type ServiceType string

const (
	ServiceTypePortForward ServiceType = "PortForward"
	ServiceTypeMCPServer   ServiceType = "MCPServer"
)

// ManagedServiceConfig wraps a configuration for any manageable service.
type ManagedServiceConfig struct {
	Type   ServiceType
	Label  string      // A unique label for this service instance
	Config interface{} // Actual config (e.g., portforwarding.PortForwardingConfig or mcpserver.MCPServerConfig)
}

// ManagedServiceUpdate carries status updates from a managed service.
type ManagedServiceUpdate struct {
	Type      ServiceType
	Label     string // Matches the label in ManagedServiceConfig
	Status    string // e.g., "Running", "Stopped", "Error", "Starting"
	OutputLog string // Log line from stdout/stderr
	IsError   bool   // True if this update represents a critical error
	IsReady   bool   // True if the service is fully up and ready
	Error     error  // The actual error if IsError is true
}

// ServiceUpdateFunc is the callback for receiving updates from any managed service.
type ServiceUpdateFunc func(update ManagedServiceUpdate)
