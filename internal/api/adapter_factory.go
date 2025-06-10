package api

// AdapterFactory is a function type for creating adapters
type AdapterFactory func(configDir string, toolExecutor interface{}) interface {
	Register()
}

// WorkflowAdapterFactory is the factory for creating workflow adapters
var workflowAdapterFactory AdapterFactory

// CapabilityAdapterFactory is the factory for creating capability adapters
var capabilityAdapterFactory AdapterFactory

// SetWorkflowAdapterFactory sets the factory function for creating workflow adapters
func SetWorkflowAdapterFactory(factory AdapterFactory) {
	workflowAdapterFactory = factory
}

// SetCapabilityAdapterFactory sets the factory function for creating capability adapters
func SetCapabilityAdapterFactory(factory AdapterFactory) {
	capabilityAdapterFactory = factory
}

// CreateWorkflowAdapter creates a workflow adapter using the registered factory
func CreateWorkflowAdapter(configDir string, toolExecutor interface{}) interface{ Register() } {
	if workflowAdapterFactory == nil {
		return nil
	}
	return workflowAdapterFactory(configDir, toolExecutor)
}

// CreateCapabilityAdapter creates a capability adapter using the registered factory
func CreateCapabilityAdapter(configDir string, toolExecutor interface{}) interface{ Register() } {
	if capabilityAdapterFactory == nil {
		return nil
	}
	return capabilityAdapterFactory(configDir, toolExecutor)
}
