package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"envctl/internal/config"
	"envctl/pkg/logging"

	"gopkg.in/yaml.v3"
)

// For testing - allows injecting custom configuration paths
var getConfigurationPaths = config.GetConfigurationPaths

// ConfigurationLoader interface for testing
type ConfigurationLoader interface {
	LoadAndParseYAML(subDir string, validator func(WorkflowDefinition) error) ([]WorkflowDefinition, error)
}

// defaultConfigurationLoader wraps the config package loader
type defaultConfigurationLoader struct{}

func (d *defaultConfigurationLoader) LoadAndParseYAML(subDir string, validator func(WorkflowDefinition) error) ([]WorkflowDefinition, error) {
	return config.LoadAndParseYAML[WorkflowDefinition](subDir, validator)
}

// WorkflowStorage manages persistent storage of workflows
type WorkflowStorage struct {
	mu           sync.RWMutex
	loader       ConfigurationLoader
	configDir    string // For agent workflow management
	workflows    map[string]*WorkflowDefinition
	changeNotify chan struct{} // Notify aggregator of changes
}

// NewWorkflowStorage creates a new workflow storage manager
func NewWorkflowStorage(configDir string) (*WorkflowStorage, error) {
	return NewWorkflowStorageWithLoader(configDir, &defaultConfigurationLoader{})
}

// NewWorkflowStorageWithLoader creates a new workflow storage manager with custom loader (for testing)
func NewWorkflowStorageWithLoader(configDir string, loader ConfigurationLoader) (*WorkflowStorage, error) {
	ws := &WorkflowStorage{
		loader:       loader,
		configDir:    configDir,
		workflows:    make(map[string]*WorkflowDefinition),
		changeNotify: make(chan struct{}, 1),
	}

	// Load existing workflows
	if err := ws.LoadWorkflows(); err != nil {
		return nil, err
	}

	return ws, nil
}

// LoadWorkflows loads workflows using layered configuration loading
func (ws *WorkflowStorage) LoadWorkflows() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Clear existing workflows
	ws.workflows = make(map[string]*WorkflowDefinition)

	// Load workflow definitions using the configuration loader
	definitions, err := ws.loader.LoadAndParseYAML("workflows", func(def WorkflowDefinition) error {
		return ws.validateDefinition(&def)
	})
	if err != nil {
		return fmt.Errorf("failed to load workflow definitions: %w", err)
	}

	logging.Info("WorkflowStorage", "Loading %d workflow definitions", len(definitions))

	// Process each definition
	for _, def := range definitions {
		ws.workflows[def.Name] = &def
		logging.Info("WorkflowStorage", "Loaded workflow definition: %s", def.Name)
	}

	// Also load legacy agent workflows for backward compatibility
	if err := ws.loadLegacyAgentWorkflows(); err != nil {
		logging.Warn("WorkflowStorage", "Failed to load legacy agent workflows: %v", err)
	}

	return nil
}

// validateDefinition validates a workflow definition
func (ws *WorkflowStorage) validateDefinition(def *WorkflowDefinition) error {
	if def.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if len(def.Steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}
	for i, step := range def.Steps {
		if step.Tool == "" {
			return fmt.Errorf("step %d: tool is required", i)
		}
		if step.ID == "" {
			return fmt.Errorf("step %d: id is required", i)
		}
	}
	return nil
}

// loadLegacyAgentWorkflows loads agent workflows from legacy agent_workflows.yaml file
func (ws *WorkflowStorage) loadLegacyAgentWorkflows() error {
	agentFile := filepath.Join(ws.configDir, AgentWorkflowsFile)
	
	data, err := os.ReadFile(agentFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No legacy file exists, which is fine
		}
		return err
	}

	var wfConfig WorkflowConfig
	if err := yaml.Unmarshal(data, &wfConfig); err != nil {
		return err
	}

	for _, wf := range wfConfig.Workflows {
		workflow := wf // Create a copy
		workflow.AgentModifiable = true
		
		// Only add if not already loaded from directory (directory takes precedence)
		if _, exists := ws.workflows[workflow.Name]; !exists {
			ws.workflows[workflow.Name] = &workflow
			logging.Info("WorkflowStorage", "Loaded legacy agent workflow: %s", workflow.Name)
		}
	}

	return nil
}

// GetWorkflow retrieves a workflow by name
func (ws *WorkflowStorage) GetWorkflow(name string) (*WorkflowDefinition, error) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	wf, exists := ws.workflows[name]
	if !exists {
		return nil, fmt.Errorf("workflow %s not found", name)
	}

	// Return a copy to prevent external modification
	copy := *wf
	return &copy, nil
}

// ListWorkflows returns all workflows
func (ws *WorkflowStorage) ListWorkflows() []WorkflowDefinition {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	workflows := make([]WorkflowDefinition, 0, len(ws.workflows))
	for _, wf := range ws.workflows {
		workflows = append(workflows, *wf)
	}
	return workflows
}

// CreateWorkflow creates a new agent workflow
func (ws *WorkflowStorage) CreateWorkflow(workflow WorkflowDefinition) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Check if workflow already exists
	if _, exists := ws.workflows[workflow.Name]; exists {
		return fmt.Errorf("workflow %s already exists", workflow.Name)
	}

	// Set metadata
	workflow.CreatedBy = WorkflowCreatorAgent
	workflow.CreatedAt = time.Now()
	workflow.LastModified = time.Now()
	workflow.Version = 1
	workflow.AgentModifiable = true

	// Add to memory
	ws.workflows[workflow.Name] = &workflow

	// Persist to agent file (legacy for now)
	if err := ws.saveAgentWorkflows(); err != nil {
		delete(ws.workflows, workflow.Name) // Rollback
		return err
	}

	// Notify aggregator of changes
	ws.notifyChange()

	return nil
}

// UpdateWorkflow updates an existing workflow
func (ws *WorkflowStorage) UpdateWorkflow(name string, updates WorkflowDefinition) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	existing, exists := ws.workflows[name]
	if !exists {
		return fmt.Errorf("workflow %s not found", name)
	}

	if !existing.AgentModifiable {
		return fmt.Errorf("workflow %s is not modifiable by agents", name)
	}

	// Preserve metadata
	updates.CreatedBy = existing.CreatedBy
	updates.CreatedAt = existing.CreatedAt
	updates.LastModified = time.Now()
	updates.Version = existing.Version + 1
	updates.AgentModifiable = true
	updates.Name = name // Ensure name doesn't change

	// Update in memory
	ws.workflows[name] = &updates

	// Persist changes (legacy for now)
	if err := ws.saveAgentWorkflows(); err != nil {
		ws.workflows[name] = existing // Rollback
		return err
	}

	// Notify aggregator of changes
	ws.notifyChange()

	return nil
}

// DeleteWorkflow deletes a workflow
func (ws *WorkflowStorage) DeleteWorkflow(name string) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	existing, exists := ws.workflows[name]
	if !exists {
		return fmt.Errorf("workflow %s not found", name)
	}

	if !existing.AgentModifiable {
		return fmt.Errorf("workflow %s is not deletable by agents", name)
	}

	// Remove from memory
	delete(ws.workflows, name)

	// Persist changes (legacy for now)
	if err := ws.saveAgentWorkflows(); err != nil {
		ws.workflows[name] = existing // Rollback
		return err
	}

	// Notify aggregator of changes
	ws.notifyChange()

	return nil
}

// saveAgentWorkflows persists agent-created workflows to disk (legacy file approach)
func (ws *WorkflowStorage) saveAgentWorkflows() error {
	var agentWorkflows []WorkflowDefinition

	for _, wf := range ws.workflows {
		if wf.CreatedBy == WorkflowCreatorAgent || (wf.AgentModifiable && wf.CreatedBy == "") {
			agentWorkflows = append(agentWorkflows, *wf)
		}
	}

	wfConfig := WorkflowConfig{Workflows: agentWorkflows}
	data, err := yaml.Marshal(wfConfig)
	if err != nil {
		return err
	}

	// Ensure directory exists
	agentFile := filepath.Join(ws.configDir, AgentWorkflowsFile)
	if err := os.MkdirAll(filepath.Dir(agentFile), 0755); err != nil {
		return err
	}

	// Write atomically
	tempFile := agentFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}

	return os.Rename(tempFile, agentFile)
}

// GetChangeChannel returns a channel that notifies of workflow changes
func (ws *WorkflowStorage) GetChangeChannel() <-chan struct{} {
	return ws.changeNotify
}

// notifyChange sends a notification that workflows have changed
func (ws *WorkflowStorage) notifyChange() {
	select {
	case ws.changeNotify <- struct{}{}:
		logging.Debug("WorkflowStorage", "Workflow change notification sent")
	default:
		// Channel already has a notification pending
		logging.Debug("WorkflowStorage", "Workflow change notification already pending")
	}
}
