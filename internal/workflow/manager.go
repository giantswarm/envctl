package workflow

import (
	"context"
	"fmt"
	"sync"

	"envctl/pkg/logging"

	"github.com/mark3labs/mcp-go/mcp"
)

// WorkflowManager manages workflows and their execution
type WorkflowManager struct {
	storage  *WorkflowStorage
	executor *WorkflowExecutor
	mu       sync.RWMutex
	stopped  bool
}

// NewWorkflowManager creates a new workflow manager
func NewWorkflowManager(configDir string, toolCaller ToolCaller) (*WorkflowManager, error) {
	storage, err := NewWorkflowStorage(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow storage: %w", err)
	}

	executor := NewWorkflowExecutor(toolCaller)

	wm := &WorkflowManager{
		storage:  storage,
		executor: executor,
	}

	// Start watching for changes
	go wm.watchChanges()

	return wm, nil
}

// GetWorkflows returns all available workflows as MCP tools
func (wm *WorkflowManager) GetWorkflows() []mcp.Tool {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workflows := wm.storage.ListWorkflows()
	tools := make([]mcp.Tool, 0, len(workflows))

	for _, wf := range workflows {
		tool := wm.workflowToTool(wf)
		tools = append(tools, tool)
	}

	return tools
}

// ExecuteWorkflow executes a workflow by name
func (wm *WorkflowManager) ExecuteWorkflow(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workflow, err := wm.storage.GetWorkflow(name)
	if err != nil {
		return nil, fmt.Errorf("workflow not found: %w", err)
	}

	return wm.executor.ExecuteWorkflow(ctx, workflow, args)
}

// watchChanges monitors for workflow file changes
func (wm *WorkflowManager) watchChanges() {
	changeChannel := wm.storage.GetChangeChannel()

	for {
		select {
		case <-changeChannel:
			logging.Debug("WorkflowManager", "Workflows changed, reloading")

			wm.mu.Lock()
			if wm.stopped {
				wm.mu.Unlock()
				return
			}

			// Reload workflows
			if err := wm.storage.LoadWorkflows(); err != nil {
				logging.Error("WorkflowManager", err, "Failed to reload workflows")
			}
			wm.mu.Unlock()
		}
	}
}

// Stop gracefully stops the workflow manager
func (wm *WorkflowManager) Stop() {
	wm.mu.Lock()
	wm.stopped = true
	wm.mu.Unlock()
}

// GetStorage returns the workflow storage for management tools
func (wm *WorkflowManager) GetStorage() *WorkflowStorage {
	return wm.storage
}

// workflowToTool converts a workflow definition to an MCP tool
func (wm *WorkflowManager) workflowToTool(workflow WorkflowDefinition) mcp.Tool {
	// Convert workflow input schema to MCP tool input schema
	properties := make(map[string]interface{})
	required := workflow.InputSchema.Required

	for name, prop := range workflow.InputSchema.Properties {
		propSchema := map[string]interface{}{
			"type":        prop.Type,
			"description": prop.Description,
		}
		if prop.Default != nil {
			propSchema["default"] = prop.Default
		}
		properties[name] = propSchema
	}

	inputSchema := mcp.ToolInputSchema{
		Type:       workflow.InputSchema.Type,
		Properties: properties,
		Required:   required,
	}

	// Prefix workflow tools with "action_" to indicate they are high-level actions
	toolName := fmt.Sprintf("action_%s", workflow.Name)

	return mcp.Tool{
		Name:        toolName,
		Description: workflow.Description,
		InputSchema: inputSchema,
	}
}
