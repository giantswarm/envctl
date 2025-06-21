package api

import (
	"time"
)

// Workflow represents a single workflow definition and runtime state
// This consolidates WorkflowDefinition, WorkflowInfo, and WorkflowConfig into one type
type Workflow struct {
	// Configuration fields (from YAML)
	Name            string                 `yaml:"name" json:"name"`
	Description     string                 `yaml:"description" json:"description"`
	Icon            string                 `yaml:"icon,omitempty" json:"icon,omitempty"`
	Version         int                    `yaml:"version,omitempty" json:"version"`
	AgentModifiable bool                   `yaml:"agentModifiable" json:"agentModifiable"`
	InputSchema     WorkflowInputSchema    `yaml:"inputSchema" json:"inputSchema"`
	Steps           []WorkflowStep         `yaml:"steps" json:"steps"`
	OutputSchema    map[string]interface{} `yaml:"outputSchema,omitempty" json:"outputSchema,omitempty"`

	// Runtime state fields (for API responses only)
	Available bool   `json:"available,omitempty" yaml:"-"`
	State     string `json:"state,omitempty" yaml:"-"`
	Error     string `json:"error,omitempty" yaml:"-"`

	// Metadata fields
	CreatedBy    string    `yaml:"createdBy,omitempty" json:"createdBy,omitempty"`
	CreatedAt    time.Time `yaml:"createdAt,omitempty" json:"createdAt,omitempty"`
	LastModified time.Time `yaml:"lastModified,omitempty" json:"lastModified,omitempty"`
}

// WorkflowConfig for separate workflow files (used for loading multiple workflows)
type WorkflowConfig struct {
	Workflows []Workflow `yaml:"workflows" json:"workflows"`
}

// Workflow-related constants
const (
	// Workflow file names
	UserWorkflowsFile  = "workflows.yaml"
	AgentWorkflowsFile = "agent_workflows.yaml"

	// Workflow creator types
	WorkflowCreatorUser  = "user"
	WorkflowCreatorAgent = "agent"
)
