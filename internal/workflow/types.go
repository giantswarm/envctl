package workflow

import (
	"time"
)

// WorkflowDefinition defines a sequence of MCP tool calls
type WorkflowDefinition struct {
	Name            string              `yaml:"name"`
	Description     string              `yaml:"description"`
	Icon            string              `yaml:"icon,omitempty"`
	AgentModifiable bool                `yaml:"agentModifiable"`
	CreatedBy       string              `yaml:"createdBy,omitempty"`
	CreatedAt       time.Time           `yaml:"createdAt,omitempty"`
	LastModified    time.Time           `yaml:"lastModified,omitempty"`
	Version         int                 `yaml:"version,omitempty"`
	InputSchema     WorkflowInputSchema `yaml:"inputSchema"`
	Steps           []WorkflowStep      `yaml:"steps"`
}

// WorkflowInputSchema defines the input parameters for a workflow
type WorkflowInputSchema struct {
	Type       string                    `yaml:"type"`
	Properties map[string]SchemaProperty `yaml:"properties"`
	Required   []string                  `yaml:"required,omitempty"`
}

// SchemaProperty defines a single property in the schema
type SchemaProperty struct {
	Type        string      `yaml:"type"`
	Description string      `yaml:"description"`
	Default     interface{} `yaml:"default,omitempty"`
}

// WorkflowStep defines a single step in a workflow
type WorkflowStep struct {
	ID    string                 `yaml:"id"`
	Tool  string                 `yaml:"tool"`
	Args  map[string]interface{} `yaml:"args"`
	Store string                 `yaml:"store,omitempty"`
}

// WorkflowConfig for separate workflow files
type WorkflowConfig struct {
	Workflows []WorkflowDefinition `yaml:"workflows"`
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
