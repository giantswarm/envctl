package capability

import (
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/pkg/logging"
)

func init() {
	// Register the capability adapter factory
	api.SetCapabilityAdapterFactory(func(configDir string, toolExecutor interface{}) interface{ Register() } {
		// Type assert to the expected interfaces
		toolChecker, ok := toolExecutor.(config.ToolAvailabilityChecker)
		if !ok {
			logging.Error("Capability", nil, "toolExecutor does not implement config.ToolAvailabilityChecker")
			return nil
		}

		toolCaller, ok := toolExecutor.(api.ToolCaller)
		if !ok {
			logging.Error("Capability", nil, "toolExecutor does not implement api.ToolCaller")
			return nil
		}

		// Create the adapter (now uses layered configuration loading)
		adapter, err := NewAdapter(toolChecker, toolCaller)
		if err != nil {
			logging.Error("Capability", err, "Failed to create capability adapter")
			return nil
		}

		return adapter
	})
}
