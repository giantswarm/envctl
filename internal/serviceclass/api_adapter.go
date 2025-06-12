package serviceclass

import (
	"envctl/internal/api"
	"envctl/pkg/logging"
	"fmt"
)

// Adapter implements the api.ServiceClassManagerHandler interface
// This adapter bridges the ServiceClassManager implementation with the central API layer
type Adapter struct {
	manager *ServiceClassManager
}

// NewAdapter creates a new API adapter for the ServiceClassManager
func NewAdapter(manager *ServiceClassManager) *Adapter {
	return &Adapter{
		manager: manager,
	}
}

// Register registers this adapter with the central API layer
// This method follows the project's API Service Locator Pattern
func (a *Adapter) Register() {
	api.RegisterServiceClassManager(a)
	logging.Info("ServiceClassAdapter", "Registered ServiceClass manager with API layer")
}

// ListServiceClasses returns information about all registered service classes
func (a *Adapter) ListServiceClasses() []api.ServiceClassInfo {
	if a.manager == nil {
		return []api.ServiceClassInfo{}
	}

	// Get service class info from the manager
	managerInfo := a.manager.ListServiceClasses()

	// Convert to API types
	result := make([]api.ServiceClassInfo, len(managerInfo))
	for i, info := range managerInfo {
		result[i] = api.ServiceClassInfo{
			Name:                     info.Name,
			Type:                     info.Type,
			Version:                  info.Version,
			Description:              info.Description,
			ServiceType:              info.ServiceType,
			Available:                info.Available,
			CreateToolAvailable:      info.CreateToolAvailable,
			DeleteToolAvailable:      info.DeleteToolAvailable,
			HealthCheckToolAvailable: info.HealthCheckToolAvailable,
			StatusToolAvailable:      info.StatusToolAvailable,
			RequiredTools:            info.RequiredTools,
			MissingTools:             info.MissingTools,
		}
	}

	return result
}

// GetServiceClass returns a specific service class definition by name
func (a *Adapter) GetServiceClass(name string) (*api.ServiceClassDefinition, error) {
	if a.manager == nil {
		return nil, fmt.Errorf("service class manager not available")
	}

	def, exists := a.manager.GetServiceClassDefinition(name)
	if !exists {
		return nil, fmt.Errorf("service class %s not found", name)
	}

	// Convert to API type (lightweight version)
	apiDef := &api.ServiceClassDefinition{
		Name:        def.Name,
		Type:        def.Type,
		Version:     def.Version,
		Description: def.Description,
		Metadata:    def.Metadata,
	}

	return apiDef, nil
}

// IsServiceClassAvailable checks if a service class is available
func (a *Adapter) IsServiceClassAvailable(name string) bool {
	if a.manager == nil {
		return false
	}

	return a.manager.IsServiceClassAvailable(name)
}

// LoadServiceDefinitions loads all service class definitions from the configured path
func (a *Adapter) LoadServiceDefinitions() error {
	if a.manager == nil {
		return fmt.Errorf("service class manager not available")
	}

	return a.manager.LoadServiceDefinitions()
}

// RefreshAvailability refreshes the availability status of all service classes
func (a *Adapter) RefreshAvailability() {
	if a.manager == nil {
		logging.Warn("ServiceClassAdapter", "Cannot refresh availability: manager not available")
		return
	}

	a.manager.RefreshAvailability()
}

// RegisterDefinition registers a service class definition programmatically
func (a *Adapter) RegisterDefinition(apiDef *api.ServiceClassDefinition) error {
	if a.manager == nil {
		return fmt.Errorf("service class manager not available")
	}

	// Convert from API type to internal type
	// Note: This is a basic conversion - full ServiceClassDefinition would need more fields
	internalDef := &ServiceClassDefinition{
		Name:        apiDef.Name,
		Type:        apiDef.Type,
		Version:     apiDef.Version,
		Description: apiDef.Description,
		Metadata:    apiDef.Metadata,
		// ServiceConfig and Operations would need to be provided separately
		// for a complete programmatic registration
	}

	return a.manager.RegisterDefinition(internalDef)
}

// UnregisterDefinition unregisters a service class definition
func (a *Adapter) UnregisterDefinition(name string) error {
	if a.manager == nil {
		return fmt.Errorf("service class manager not available")
	}

	return a.manager.UnregisterDefinition(name)
}

// GetDefinitionsPath returns the path where service class definitions are loaded from
func (a *Adapter) GetDefinitionsPath() string {
	if a.manager == nil {
		return ""
	}

	return a.manager.GetDefinitionsPath()
}

// GetManager returns the underlying ServiceClassManager (for internal use)
// This should only be used by other internal packages that need direct access
func (a *Adapter) GetManager() *ServiceClassManager {
	return a.manager
}
