package capability

import "sync"

var (
	// Registry instance
	registry     *Registry
	registryOnce sync.Once
)

// GetRegistry returns the capability registry instance
func GetRegistry() *Registry {
	registryOnce.Do(func() {
		registry = NewRegistry()
	})
	return registry
}
