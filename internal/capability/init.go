package capability

import (
	"envctl/internal/api"
	"path/filepath"
)

func init() {
	// Register the capability adapter factory
	api.SetCapabilityAdapterFactory(func(configDir string, toolExecutor interface{}) interface{ Register() } {
		// Type assert to the expected interfaces
		toolChecker, ok := toolExecutor.(ToolAvailabilityChecker)
		if !ok {
			return nil
		}

		toolCaller, ok := toolExecutor.(api.ToolCaller)
		if !ok {
			return nil
		}

		// Determine capability definitions path
		definitionsPath := filepath.Join(configDir, "capability", "definitions")

		// Create the adapter
		return NewAdapter(definitionsPath, toolChecker, toolCaller)
	})
}
