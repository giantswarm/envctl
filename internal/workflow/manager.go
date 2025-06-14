package workflow

import (
	"context"
	"fmt"
	"sync"

	"envctl/internal/config"
	"envctl/pkg/logging"

	"github.com/mark3labs/mcp-go/mcp"
)

// WorkflowManager manages workflows and their execution
type WorkflowManager struct {
	storage     *WorkflowStorage
	executor    *WorkflowExecutor
	toolChecker config.ToolAvailabilityChecker
	mu          sync.RWMutex
	stopped     bool
}

// NewWorkflowManager creates a new workflow manager
func NewWorkflowManager(configDir string, toolCaller ToolCaller, toolChecker config.ToolAvailabilityChecker) (*WorkflowManager, error) {
	storage, err := NewWorkflowStorage(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow storage: %w", err)
	}

	executor := NewWorkflowExecutor(toolCaller)

	wm := &WorkflowManager{
		storage:     storage,
		executor:    executor,
		toolChecker: toolChecker,
	}

	// Start watching for changes
	go wm.watchChanges()

	return wm, nil
}

// LoadDefinitions loads workflow definitions (implements common manager interface)
func (wm *WorkflowManager) LoadDefinitions() error {
	return wm.storage.LoadWorkflows()
}

// GetDefinition returns a workflow definition by name (implements common manager interface)
func (wm *WorkflowManager) GetDefinition(name string) (WorkflowDefinition, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workflow, err := wm.storage.GetWorkflow(name)
	if err != nil {
		return WorkflowDefinition{}, false
	}
	return *workflow, true
}

// ListDefinitions returns all workflow definitions (implements common manager interface)
func (wm *WorkflowManager) ListDefinitions() []WorkflowDefinition {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	return wm.storage.ListWorkflows()
}

// ListAvailableDefinitions returns only workflow definitions that have all required tools available
func (wm *WorkflowManager) ListAvailableDefinitions() []WorkflowDefinition {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workflows := wm.storage.ListWorkflows()
	var available []WorkflowDefinition

	for _, workflow := range workflows {
		if wm.isWorkflowAvailable(&workflow) {
			available = append(available, workflow)
		}
	}

	return available
}

// IsAvailable checks if a workflow is available (has all required tools) (implements common manager interface)
func (wm *WorkflowManager) IsAvailable(name string) bool {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workflow, err := wm.storage.GetWorkflow(name)
	if err != nil {
		return false
	}

	return wm.isWorkflowAvailable(workflow)
}

// isWorkflowAvailable checks if a workflow has all required tools available
func (wm *WorkflowManager) isWorkflowAvailable(workflow *WorkflowDefinition) bool {
	if wm.toolChecker == nil {
		return true // Assume available if no tool checker
	}

	// Check each step's tool availability
	for _, step := range workflow.Steps {
		if !wm.toolChecker.IsToolAvailable(step.Tool) {
			return false
		}
	}

	return true
}

// RefreshAvailability refreshes the availability status of all workflows (implements common manager interface)
func (wm *WorkflowManager) RefreshAvailability() {
	// Workflow availability is checked dynamically, so no caching needed
	logging.Debug("WorkflowManager", "Refreshed workflow availability (dynamic checking)")
}

// GetDefinitionsPath returns the paths where workflow definitions are loaded from (implements common manager interface)
func (wm *WorkflowManager) GetDefinitionsPath() string {
	userDir, projectDir, err := config.GetConfigurationPaths()
	if err != nil {
		logging.Error("WorkflowManager", err, "Failed to get configuration paths")
		return "error determining paths"
	}
	
	userPath := fmt.Sprintf("%s/workflows", userDir)
	projectPath := fmt.Sprintf("%s/workflows", projectDir)
	
	return fmt.Sprintf("User: %s, Project: %s", userPath, projectPath)
}

// GetWorkflows returns all available workflows as MCP tools
func (wm *WorkflowManager) GetWorkflows() []mcp.Tool {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workflows := wm.storage.ListWorkflows()
	tools := make([]mcp.Tool, 0, len(workflows))

	for _, wf := range workflows {
		// Only include workflows that have all required tools available
		if wm.isWorkflowAvailable(&wf) {
			tool := wm.workflowToTool(wf)
			tools = append(tools, tool)
		}
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

	// Check if workflow is available before execution
	if !wm.isWorkflowAvailable(workflow) {
		return nil, fmt.Errorf("workflow %s is not available (missing required tools)", name)
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
