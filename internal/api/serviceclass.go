package api

// ServiceClass represents a single service class definition and runtime state
// This consolidates ServiceClassDefinition, ServiceClassInfo, and ServiceClassConfig into one type
type ServiceClass struct {
	// Configuration fields (from YAML) - Template/Blueprint definition
	Name        string `yaml:"name" json:"name"`
	Version     string `yaml:"version" json:"version"`
	Description string `yaml:"description" json:"description"`

	// Service lifecycle configuration
	ServiceConfig ServiceConfig                  `yaml:"serviceConfig" json:"serviceConfig"`
	Operations    map[string]OperationDefinition `yaml:"operations" json:"operations"`

	// Runtime state fields (for API responses)
	ServiceType string `json:"serviceType,omitempty" yaml:"-"`
	Available   bool   `json:"available,omitempty" yaml:"-"`
	State       string `json:"state,omitempty" yaml:"-"`
	Health      string `json:"health,omitempty" yaml:"-"`
	Error       string `json:"error,omitempty" yaml:"-"`

	// Tool availability
	CreateToolAvailable      bool     `json:"createToolAvailable,omitempty" yaml:"-"`
	DeleteToolAvailable      bool     `json:"deleteToolAvailable,omitempty" yaml:"-"`
	HealthCheckToolAvailable bool     `json:"healthCheckToolAvailable,omitempty" yaml:"-"`
	StatusToolAvailable      bool     `json:"statusToolAvailable,omitempty" yaml:"-"`
	RequiredTools            []string `json:"requiredTools,omitempty" yaml:"-"`
	MissingTools             []string `json:"missingTools,omitempty" yaml:"-"`
}

// ServiceConfig defines the service lifecycle management configuration
type ServiceConfig struct {
	// Service metadata
	ServiceType  string   `yaml:"serviceType" json:"serviceType"`
	DefaultLabel string   `yaml:"defaultLabel" json:"defaultLabel"`
	Dependencies []string `yaml:"dependencies" json:"dependencies"`

	// Lifecycle tool mappings - these tools will be called by the orchestrator
	LifecycleTools LifecycleTools `yaml:"lifecycleTools" json:"lifecycleTools"`

	// Service behavior configuration
	HealthCheck HealthCheckConfig `yaml:"healthCheck" json:"healthCheck"`
	Timeout     TimeoutConfig     `yaml:"timeout" json:"timeout"`

	// Parameter mapping for service creation
	CreateParameters map[string]ParameterMapping `yaml:"createParameters" json:"createParameters"`
}

// LifecycleTools maps service lifecycle events to aggregator tools
type LifecycleTools struct {
	// Tool to call when starting the service (maps to Service.Start)
	Start ToolCall `yaml:"start" json:"start"`

	// Tool to call when stopping the service (maps to Service.Stop)
	Stop ToolCall `yaml:"stop" json:"stop"`

	// Tool to call for restarting the service (optional, maps to Service.Restart)
	Restart *ToolCall `yaml:"restart,omitempty" json:"restart,omitempty"`

	// Tool to call for health checks (optional)
	HealthCheck *ToolCall `yaml:"healthCheck,omitempty" json:"healthCheck,omitempty"`

	// Tool to call to get service status/info (optional)
	Status *ToolCall `yaml:"status,omitempty" json:"status,omitempty"`
}
