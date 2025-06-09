package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/pkg/logging"

	"gopkg.in/yaml.v3"
)

// ConfigAdapter adapts the app Config to implement api.ConfigHandler
type ConfigAdapter struct {
	config     *config.EnvctlConfig
	configPath string
	mu         sync.RWMutex
}

// NewConfigAdapter creates a new configuration API adapter
func NewConfigAdapter(cfg *config.EnvctlConfig, configPath string) *ConfigAdapter {
	return &ConfigAdapter{
		config:     cfg,
		configPath: configPath,
	}
}

// GetConfig returns the current configuration
func (a *ConfigAdapter) GetConfig() (*config.EnvctlConfig, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Return a copy to avoid concurrent modification
	return a.config, nil
}

// UpdateClusters updates the clusters configuration
func (a *ConfigAdapter) UpdateClusters(clusters []config.ClusterDefinition) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.config.Clusters = clusters
	return nil
}

// UpdateActiveClusters updates the active clusters mapping
func (a *ConfigAdapter) UpdateActiveClusters(activeClusters map[config.ClusterRole]string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.config.ActiveClusters = activeClusters
	return nil
}

// UpdateMCPServer updates or adds an MCP server definition
func (a *ConfigAdapter) UpdateMCPServer(server config.MCPServerDefinition) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Find and update or add
	found := false
	for i, s := range a.config.MCPServers {
		if s.Name == server.Name {
			a.config.MCPServers[i] = server
			found = true
			break
		}
	}
	if !found {
		a.config.MCPServers = append(a.config.MCPServers, server)
	}

	logging.Info("ConfigAdapter", "Updated MCP server: %s", server.Name)
	return nil
}

// UpdatePortForward updates or adds a port forward definition
func (a *ConfigAdapter) UpdatePortForward(portForward config.PortForwardDefinition) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Find and update or add
	found := false
	for i, pf := range a.config.PortForwards {
		if pf.Name == portForward.Name {
			a.config.PortForwards[i] = portForward
			found = true
			break
		}
	}
	if !found {
		a.config.PortForwards = append(a.config.PortForwards, portForward)
	}

	logging.Info("ConfigAdapter", "Updated port forward: %s", portForward.Name)
	return nil
}

// UpdateWorkflow updates or adds a workflow definition
func (a *ConfigAdapter) UpdateWorkflow(workflow config.WorkflowDefinition) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Find and update or add
	found := false
	for i, w := range a.config.Workflows {
		if w.Name == workflow.Name {
			a.config.Workflows[i] = workflow
			found = true
			break
		}
	}
	if !found {
		a.config.Workflows = append(a.config.Workflows, workflow)
	}

	logging.Info("ConfigAdapter", "Updated workflow: %s", workflow.Name)
	return nil
}

// UpdateAggregatorConfig updates the aggregator configuration
func (a *ConfigAdapter) UpdateAggregatorConfig(aggregator config.AggregatorConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.config.Aggregator = aggregator
	logging.Info("ConfigAdapter", "Updated aggregator configuration")
	return nil
}

// UpdateGlobalSettings updates the global settings
func (a *ConfigAdapter) UpdateGlobalSettings(settings config.GlobalSettings) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.config.GlobalSettings = settings
	logging.Info("ConfigAdapter", "Updated global settings")
	return nil
}

// DeleteMCPServer removes an MCP server by name
func (a *ConfigAdapter) DeleteMCPServer(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	servers := []config.MCPServerDefinition{}
	found := false
	for _, s := range a.config.MCPServers {
		if s.Name != name {
			servers = append(servers, s)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("MCP server %s not found", name)
	}

	a.config.MCPServers = servers
	logging.Info("ConfigAdapter", "Deleted MCP server: %s", name)
	return nil
}

// DeletePortForward removes a port forward by name
func (a *ConfigAdapter) DeletePortForward(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	forwards := []config.PortForwardDefinition{}
	found := false
	for _, pf := range a.config.PortForwards {
		if pf.Name != name {
			forwards = append(forwards, pf)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("port forward %s not found", name)
	}

	a.config.PortForwards = forwards
	logging.Info("ConfigAdapter", "Deleted port forward: %s", name)
	return nil
}

// DeleteWorkflow removes a workflow by name
func (a *ConfigAdapter) DeleteWorkflow(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	workflows := []config.WorkflowDefinition{}
	found := false
	for _, w := range a.config.Workflows {
		if w.Name != name {
			workflows = append(workflows, w)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("workflow %s not found", name)
	}

	a.config.Workflows = workflows
	logging.Info("ConfigAdapter", "Deleted workflow: %s", name)
	return nil
}

// DeleteCluster removes a cluster by name
func (a *ConfigAdapter) DeleteCluster(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	clusters := []config.ClusterDefinition{}
	found := false
	for _, c := range a.config.Clusters {
		if c.Name != name {
			clusters = append(clusters, c)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("cluster %s not found", name)
	}

	a.config.Clusters = clusters

	// Also remove from active clusters if it was active
	for role, activeName := range a.config.ActiveClusters {
		if activeName == name {
			delete(a.config.ActiveClusters, role)
		}
	}

	logging.Info("ConfigAdapter", "Deleted cluster: %s", name)
	return nil
}

// SaveConfig persists the configuration to file
func (a *ConfigAdapter) SaveConfig() error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Determine config path if not set
	configPath := a.configPath
	if configPath == "" {
		// Try project config first
		projectPath, err := getProjectConfigPath()
		if err == nil {
			// Check if project config directory exists
			projectDir := filepath.Dir(projectPath)
			if _, err := os.Stat(projectDir); err == nil {
				configPath = projectPath
			}
		}

		// Fall back to user config
		if configPath == "" {
			userPath, err := getUserConfigPath()
			if err != nil {
				return fmt.Errorf("could not determine config path: %w", err)
			}
			configPath = userPath
		}
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal configuration to YAML
	data, err := yaml.Marshal(a.config)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	logging.Info("ConfigAdapter", "Saved configuration to: %s", configPath)
	return nil
}

// Register registers this adapter with the API package
func (a *ConfigAdapter) Register() {
	api.RegisterConfigHandler(a)
}

// Helper functions to get config paths
func getUserConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "envctl", "config.yaml"), nil
}

func getProjectConfigPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, ".envctl", "config.yaml"), nil
}
