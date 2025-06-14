package workflow

import (
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/pkg/logging"
)

func init() {
	// Register the workflow adapter factory
	api.SetWorkflowAdapterFactory(func(configDir string, toolExecutor interface{}) interface{ Register() } {
		// Type assert to the expected interfaces
		toolCaller, ok := toolExecutor.(api.ToolCaller)
		if !ok {
			logging.Error("Workflow", nil, "toolExecutor does not implement api.ToolCaller")
			return nil
		}

		toolChecker, ok := toolExecutor.(config.ToolAvailabilityChecker)
		if !ok {
			logging.Error("Workflow", nil, "toolExecutor does not implement config.ToolAvailabilityChecker")
			return nil
		}

		adapter, err := NewAdapter(configDir, toolCaller, toolChecker)
		if err != nil {
			logging.Error("Workflow", err, "Failed to create workflow adapter")
			return nil
		}
		return adapter
	})
}
