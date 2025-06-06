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

// WorkflowStorage manages persistent storage of workflows
type WorkflowStorage struct {
	mu           sync.RWMutex
	userFile     string // User-defined workflows (read-only for agents)
	agentFile    string // Agent-created workflows
	workflows    map[string]*config.WorkflowDefinition
	changeNotify chan struct{} // Notify aggregator of changes
}

// NewWorkflowStorage creates a new workflow storage manager
func NewWorkflowStorage(configDir string) (*WorkflowStorage, error) {
	userFile := filepath.Join(configDir, config.UserWorkflowsFile)
	agentFile := filepath.Join(configDir, config.AgentWorkflowsFile)

	ws := &WorkflowStorage{
		userFile:     userFile,
		agentFile:    agentFile,
		workflows:    make(map[string]*config.WorkflowDefinition),
		changeNotify: make(chan struct{}, 1),
	}

	// Load existing workflows
	if err := ws.LoadWorkflows(); err != nil {
		return nil, err
	}

	return ws, nil
}

// LoadWorkflows loads workflows from both user and agent files
func (ws *WorkflowStorage) LoadWorkflows() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Clear existing workflows
	ws.workflows = make(map[string]*config.WorkflowDefinition)

	// Load user workflows (these are not agent-modifiable by default)
	if err := ws.loadFromFile(ws.userFile, false); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load user workflows: %w", err)
	}

	// Load agent workflows (these are agent-modifiable)
	if err := ws.loadFromFile(ws.agentFile, true); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load agent workflows: %w", err)
	}

	return nil
}

// loadFromFile loads workflows from a specific file
func (ws *WorkflowStorage) loadFromFile(filename string, setModifiable bool) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var wfConfig config.WorkflowConfig
	if err := yaml.Unmarshal(data, &wfConfig); err != nil {
		return err
	}

	for _, wf := range wfConfig.Workflows {
		workflow := wf // Create a copy
		if setModifiable {
			workflow.AgentModifiable = true
		}
		ws.workflows[workflow.Name] = &workflow
	}

	return nil
}

// GetWorkflow retrieves a workflow by name
func (ws *WorkflowStorage) GetWorkflow(name string) (*config.WorkflowDefinition, error) {
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
func (ws *WorkflowStorage) ListWorkflows() []config.WorkflowDefinition {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	workflows := make([]config.WorkflowDefinition, 0, len(ws.workflows))
	for _, wf := range ws.workflows {
		workflows = append(workflows, *wf)
	}
	return workflows
}

// CreateWorkflow creates a new agent workflow
func (ws *WorkflowStorage) CreateWorkflow(workflow config.WorkflowDefinition) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Check if workflow already exists
	if _, exists := ws.workflows[workflow.Name]; exists {
		return fmt.Errorf("workflow %s already exists", workflow.Name)
	}

	// Set metadata
	workflow.CreatedBy = config.WorkflowCreatorAgent
	workflow.CreatedAt = time.Now()
	workflow.LastModified = time.Now()
	workflow.Version = 1
	workflow.AgentModifiable = true

	// Add to memory
	ws.workflows[workflow.Name] = &workflow

	// Persist to agent file
	if err := ws.saveAgentWorkflows(); err != nil {
		delete(ws.workflows, workflow.Name) // Rollback
		return err
	}

	// Notify aggregator of changes
	ws.notifyChange()

	return nil
}

// UpdateWorkflow updates an existing workflow
func (ws *WorkflowStorage) UpdateWorkflow(name string, updates config.WorkflowDefinition) error {
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

	// Persist changes
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

	// Persist changes
	if err := ws.saveAgentWorkflows(); err != nil {
		ws.workflows[name] = existing // Rollback
		return err
	}

	// Notify aggregator of changes
	ws.notifyChange()

	return nil
}

// saveAgentWorkflows persists agent-created workflows to disk
func (ws *WorkflowStorage) saveAgentWorkflows() error {
	var agentWorkflows []config.WorkflowDefinition

	for _, wf := range ws.workflows {
		if wf.CreatedBy == config.WorkflowCreatorAgent || (wf.AgentModifiable && wf.CreatedBy == "") {
			agentWorkflows = append(agentWorkflows, *wf)
		}
	}

	wfConfig := config.WorkflowConfig{Workflows: agentWorkflows}
	data, err := yaml.Marshal(wfConfig)
	if err != nil {
		return err
	}

	// Write atomically
	tempFile := ws.agentFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return err
	}

	return os.Rename(tempFile, ws.agentFile)
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
