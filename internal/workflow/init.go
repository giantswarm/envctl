package workflow

import (
	"envctl/internal/api"
)

func init() {
	// Register the workflow adapter factory
	api.SetWorkflowAdapterFactory(func(configDir string, toolExecutor interface{}) interface{ Register() } {
		// Type assert to the expected interface
		toolCaller, ok := toolExecutor.(api.ToolCaller)
		if !ok {
			return nil
		}

		adapter, err := NewAdapter(configDir, toolCaller)
		if err != nil {
			// Log error but return nil
			return nil
		}
		return adapter
	})
}
