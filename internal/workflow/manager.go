package workflow

import (
	"context"
	"fmt"
	"sync"

	"envctl/internal/config"
	"envctl/pkg/logging"

	"github.com/mark3labs/mcp-go/mcp"
	"gopkg.in/yaml.v3"
)

// WorkflowManager manages workflows and their execution
type WorkflowManager struct {
	storage     *config.DynamicStorage         // Use the new DynamicStorage
	workflows   map[string]*WorkflowDefinition // In-memory workflow storage
	executor    *WorkflowExecutor
	toolChecker config.ToolAvailabilityChecker
	mu          sync.RWMutex
	stopped     bool
}

// NewWorkflowManager creates a new workflow manager
func NewWorkflowManager(storage *config.DynamicStorage, toolCaller ToolCaller, toolChecker config.ToolAvailabilityChecker) (*WorkflowManager, error) {
	executor := NewWorkflowExecutor(toolCaller)

	wm := &WorkflowManager{
		storage:     storage,
		workflows:   make(map[string]*WorkflowDefinition),
		executor:    executor,
		toolChecker: toolChecker,
	}

	return wm, nil
}

// LoadDefinitions loads workflow definitions from storage
func (wm *WorkflowManager) LoadDefinitions() error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	names, err := wm.storage.List("workflows")
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	// Clear existing in-memory workflows
	wm.workflows = make(map[string]*WorkflowDefinition)

	for _, name := range names {
		data, err := wm.storage.Load("workflows", name)
		if err != nil {
			logging.Warn("WorkflowManager", "Failed to load workflow %s: %v", name, err)
			continue
		}

		var wf WorkflowDefinition
		if err := yaml.Unmarshal(data, &wf); err != nil {
			logging.Warn("WorkflowManager", "Failed to parse workflow %s: %v", name, err)
			continue
		}
		// The name from the filesystem is the source of truth
		wf.Name = name
		wm.workflows[name] = &wf
	}

	logging.Info("WorkflowManager", "Loaded %d workflows", len(wm.workflows))
	return nil
}

// GetDefinition returns a workflow definition by name (implements common manager interface)
func (wm *WorkflowManager) GetDefinition(name string) (WorkflowDefinition, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workflow, exists := wm.workflows[name]
	if !exists {
		return WorkflowDefinition{}, false
	}
	return *workflow, true
}

// ListDefinitions returns all workflow definitions (implements common manager interface)
func (wm *WorkflowManager) ListDefinitions() []WorkflowDefinition {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workflows := make([]WorkflowDefinition, 0, len(wm.workflows))
	for _, wf := range wm.workflows {
		workflows = append(workflows, *wf)
	}
	return workflows
}

// ListAvailableDefinitions returns only workflow definitions that have all required tools available
func (wm *WorkflowManager) ListAvailableDefinitions() []WorkflowDefinition {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	var available []WorkflowDefinition
	for _, wf := range wm.workflows {
		if wm.isWorkflowAvailable(wf) {
			available = append(available, *wf)
		}
	}

	return available
}

// IsAvailable checks if a workflow is available (has all required tools) (implements common manager interface)
func (wm *WorkflowManager) IsAvailable(name string) bool {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workflow, exists := wm.workflows[name]
	if !exists {
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

	tools := make([]mcp.Tool, 0, len(wm.workflows))

	for _, wf := range wm.workflows {
		// Only include workflows that have all required tools available
		if wm.isWorkflowAvailable(wf) {
			tool := wm.workflowToTool(*wf)
			tools = append(tools, tool)
		}
	}

	return tools
}

// ExecuteWorkflow executes a workflow by name
func (wm *WorkflowManager) ExecuteWorkflow(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workflow, exists := wm.workflows[name]
	if !exists {
		return nil, fmt.Errorf("workflow %s not found", name)
	}

	// Check if workflow is available before execution
	if !wm.isWorkflowAvailable(workflow) {
		return nil, fmt.Errorf("workflow %s is not available (missing required tools)", name)
	}

	return wm.executor.ExecuteWorkflow(ctx, workflow, args)
}

// Stop gracefully stops the workflow manager
func (wm *WorkflowManager) Stop() {
	wm.mu.Lock()
	wm.stopped = true
	wm.mu.Unlock()
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

// CreateWorkflow creates and persists a new workflow
func (wm *WorkflowManager) CreateWorkflow(wf WorkflowDefinition) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if _, exists := wm.workflows[wf.Name]; exists {
		return fmt.Errorf("workflow '%s' already exists", wf.Name)
	}

	data, err := yaml.Marshal(wf)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow %s: %w", wf.Name, err)
	}

	if err := wm.storage.Save("workflows", wf.Name, data); err != nil {
		return fmt.Errorf("failed to save workflow %s: %w", wf.Name, err)
	}

	// Add to in-memory store after successful save
	wm.workflows[wf.Name] = &wf
	logging.Info("WorkflowManager", "Created workflow %s", wf.Name)
	return nil
}

// UpdateWorkflow updates and persists an existing workflow
func (wm *WorkflowManager) UpdateWorkflow(name string, wf WorkflowDefinition) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if _, exists := wm.workflows[name]; !exists {
		return fmt.Errorf("workflow '%s' not found", name)
	}
	// Ensure the name in the object matches the name being updated
	wf.Name = name

	data, err := yaml.Marshal(wf)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow %s: %w", name, err)
	}

	if err := wm.storage.Save("workflows", name, data); err != nil {
		return fmt.Errorf("failed to save workflow %s: %w", name, err)
	}

	// Update in-memory store after successful save
	wm.workflows[name] = &wf
	logging.Info("WorkflowManager", "Updated workflow %s", name)
	return nil
}

// DeleteWorkflow deletes a workflow from memory and storage
func (wm *WorkflowManager) DeleteWorkflow(name string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if _, exists := wm.workflows[name]; !exists {
		return fmt.Errorf("workflow '%s' not found", name)
	}

	if err := wm.storage.Delete("workflows", name); err != nil {
		return fmt.Errorf("failed to delete workflow %s from storage: %w", name, err)
	}

	// Delete from in-memory store after successful deletion from storage
	delete(wm.workflows, name)
	logging.Info("WorkflowManager", "Deleted workflow %s", name)
	return nil
}
