package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3" // Assuming YAML parsing library

	"envctl/internal/utils" // For BuildMcContext, BuildWcContext
)

// For mocking in tests
var osUserHomeDir = os.UserHomeDir
var osGetwd = os.Getwd

const (
	userConfigDir          = ".config/envctl"
	projectConfigDir       = ".envctl"
	configFileName         = "config.yaml"
	kubeContextMC    Alias = "mc"
	kubeContextWC    Alias = "wc"
)

// Alias is a type for placeholders like "mc" or "wc"
type Alias string

// LoadConfig loads the envctl configuration by layering default, user, and project settings.
// mcName and wcName are the canonical names provided by the user.
func LoadConfig(mcName, wcName string) (EnvctlConfig, error) {
	// 1. Start with the default configuration
	config := GetDefaultConfig(mcName, wcName)

	// 2. Determine user-specific configuration path
	userConfigPath, err := getUserConfigPath()
	if err != nil {
		// Log this error but don't fail; user config is optional
		fmt.Fprintf(os.Stderr, "Warning: Could not determine user config path: %v\n", err)
	} else {
		if _, err := os.Stat(userConfigPath); !os.IsNotExist(err) {
			userConfig, err := loadConfigFromFile(userConfigPath)
			if err != nil {
				return EnvctlConfig{}, fmt.Errorf("error loading user config from %s: %w", userConfigPath, err)
			}
			config = mergeConfigs(config, userConfig, mcName, wcName)
		}
	}

	// 3. Determine project-specific configuration path
	projectConfigPath, err := getProjectConfigPath()
	if err != nil {
		// Log this error but don't fail; project config is optional
		fmt.Fprintf(os.Stderr, "Warning: Could not determine project config path: %v\n", err)
	} else {
		if _, err := os.Stat(projectConfigPath); !os.IsNotExist(err) {
			projectConfig, err := loadConfigFromFile(projectConfigPath)
			if err != nil {
				return EnvctlConfig{}, fmt.Errorf("error loading project config from %s: %w", projectConfigPath, err)
			}
			config = mergeConfigs(config, projectConfig, mcName, wcName)
		}
	}

	// 4. Resolve KubeContextTarget placeholders in the final configuration
	err = resolveKubeContextPlaceholders(&config, mcName, wcName)
	if err != nil {
		return EnvctlConfig{}, fmt.Errorf("error resolving KubeContextTarget placeholders: %w", err)
	}

	return config, nil
}

var getUserConfigPath = func() (string, error) {
	homeDir, err := osUserHomeDir() // Use mockable variable
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, userConfigDir, configFileName), nil
}

var getProjectConfigPath = func() (string, error) {
	wd, err := osGetwd() // Use mockable variable
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, projectConfigDir, configFileName), nil
}

// loadConfigFromFile loads an EnvctlConfig from a YAML file.
func loadConfigFromFile(filePath string) (EnvctlConfig, error) {
	var config EnvctlConfig
	data, err := os.ReadFile(filePath)
	if err != nil {
		return EnvctlConfig{}, err
	}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return EnvctlConfig{}, err
	}
	return config, nil
}

// mergeConfigs merges 'overlay' config into 'base' config.
// mcName and wcName are passed for potential dynamic resolutions during merge if needed,
// though primary resolution happens in resolveKubeContextPlaceholders.
func mergeConfigs(base, overlay EnvctlConfig, mcName, wcName string) EnvctlConfig {
	mergedConfig := base

	// Merge GlobalSettings (overlay overrides base)
	if overlay.GlobalSettings.DefaultContainerRuntime != "" {
		mergedConfig.GlobalSettings.DefaultContainerRuntime = overlay.GlobalSettings.DefaultContainerRuntime
	}
	// Add merging for other GlobalSettings fields here if any

	// Merge MCPServers
	mcpServersMap := make(map[string]MCPServerDefinition)
	for _, srv := range mergedConfig.MCPServers {
		mcpServersMap[srv.Name] = srv
	}
	for _, srv := range overlay.MCPServers {
		mcpServersMap[srv.Name] = srv // Replace if name exists, otherwise adds
	}
	mergedConfig.MCPServers = nil
	for _, srv := range mcpServersMap {
		mergedConfig.MCPServers = append(mergedConfig.MCPServers, srv)
	}

	// Merge PortForwards
	portForwardsMap := make(map[string]PortForwardDefinition)
	for _, pf := range mergedConfig.PortForwards {
		portForwardsMap[pf.Name] = pf
	}
	for _, pf := range overlay.PortForwards {
		portForwardsMap[pf.Name] = pf // Replace if name exists, otherwise adds
	}
	mergedConfig.PortForwards = nil
	for _, pf := range portForwardsMap {
		mergedConfig.PortForwards = append(mergedConfig.PortForwards, pf)
	}

	return mergedConfig
}

// resolveKubeContextPlaceholders iterates through PortForwardDefinitions and
// resolves "mc" or "wc" placeholders in KubeContextTarget.
func resolveKubeContextPlaceholders(config *EnvctlConfig, mcName, wcName string) error {
	for i := range config.PortForwards {
		pf := &config.PortForwards[i] // Get a pointer to modify in place
		switch Alias(pf.KubeContextTarget) {
		case kubeContextMC:
			if mcName == "" {
				return fmt.Errorf("port-forward '%s' requires MC context, but mcName is not provided", pf.Name)
			}
			pf.KubeContextTarget = utils.BuildMcContext(mcName)
		case kubeContextWC:
			if wcName == "" {
				// If wcName is not provided, it could fall back to MC context or error.
				// For now, let's assume it's an error if "wc" is specified but no wcName.
				// Alternative: Fallback to MC context if wcName is empty.
				// pf.KubeContextTarget = utils.BuildMcContext(mcName)
				return fmt.Errorf("port-forward '%s' requires WC context, but wcName is not provided (mcName: %s)", pf.Name, mcName)

			}
			if mcName == "" { // Should not happen if wcName is set, but good practice
				return fmt.Errorf("port-forward '%s' requires WC context, but mcName is not provided for building WC context name", pf.Name)
			}
			pf.KubeContextTarget = utils.BuildWcContext(mcName, wcName)
		default:
			// If it's not "mc" or "wc", assume it's an explicit context name or already resolved.
			// No action needed. Or, we could validate if it's a valid looking context.
		}
	}
	return nil
}

// ExampleContainerizedConfig shows how to configure containerized MCP servers
// This is not used in the code but serves as documentation
func ExampleContainerizedConfig() EnvctlConfig {
	return EnvctlConfig{
		MCPServers: []MCPServerDefinition{
			{
				Name:           "kubernetes",
				Type:           MCPServerTypeContainer,
				Enabled:        true,
				Icon:           "☸️",
				Category:       "Core",
				Image:          "giantswarm/mcp-server-kubernetes:latest",
				ProxyPort:      8001,
				ContainerPorts: []string{"8001:3000"}, // host:container
				ContainerVolumes: []string{
					"~/.kube/config:/home/mcpuser/.kube/config:ro",
				},
				ContainerEnv: map[string]string{
					"KUBECONFIG": "/home/mcpuser/.kube/config",
				},
			},
			{
				Name:           "prometheus",
				Type:           MCPServerTypeContainer,
				Enabled:        true,
				Icon:           "🔥",
				Category:       "Monitoring",
				Image:          "giantswarm/mcp-server-prometheus:latest",
				ProxyPort:      8002,
				ContainerPorts: []string{"8002:3000"},
				ContainerEnv: map[string]string{
					"PROMETHEUS_URL": "http://host.docker.internal:8080/prometheus",
					"ORG_ID":         "giantswarm",
				},
				RequiresPortForwards: []string{"mc-prometheus"},
			},
			{
				Name:           "grafana",
				Type:           MCPServerTypeContainer,
				Enabled:        true,
				Icon:           "📊",
				Category:       "Monitoring",
				Image:          "giantswarm/mcp-server-grafana:latest",
				ProxyPort:      8003,
				ContainerPorts: []string{"8003:3000"},
				ContainerEnv: map[string]string{
					"GRAFANA_URL": "http://host.docker.internal:3000",
				},
				RequiresPortForwards: []string{"mc-grafana"},
			},
		},
		GlobalSettings: GlobalSettings{
			DefaultContainerRuntime: "docker",
		},
	}
}
