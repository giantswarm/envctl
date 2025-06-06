package config

import (
	"envctl/internal/kube"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3" // Assuming YAML parsing library
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
	config := GetDefaultConfigWithRoles(mcName, wcName)

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

	// Merge Clusters - overlay completely replaces base clusters
	if len(overlay.Clusters) > 0 {
		mergedConfig.Clusters = overlay.Clusters
	}

	// Merge ActiveClusters - overlay entries override base entries
	if len(overlay.ActiveClusters) > 0 {
		if mergedConfig.ActiveClusters == nil {
			mergedConfig.ActiveClusters = make(map[ClusterRole]string)
		}
		for role, clusterName := range overlay.ActiveClusters {
			mergedConfig.ActiveClusters[role] = clusterName
		}
	}

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

	// Merge Aggregator settings
	if overlay.Aggregator.Port != 0 {
		mergedConfig.Aggregator.Port = overlay.Aggregator.Port
	}
	if overlay.Aggregator.Host != "" {
		mergedConfig.Aggregator.Host = overlay.Aggregator.Host
	}
	// Merge Enabled field - only if explicitly set in overlay
	mergedConfig.Aggregator.Enabled = overlay.Aggregator.Enabled

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
			pf.KubeContextTarget = kube.BuildMcContext(mcName)
		case kubeContextWC:
			if wcName == "" {
				return fmt.Errorf("port-forward '%s' requires WC context, but wcName is not provided (mcName: %s)", pf.Name, mcName)
			}
			if mcName == "" { // Should not happen if wcName is set, but good practice
				return fmt.Errorf("port-forward '%s' requires WC context, but mcName is not provided for building WC context name", pf.Name)
			}
			pf.KubeContextTarget = kube.BuildWcContext(mcName, wcName)
		default:
			// If it's not "mc" or "wc", assume it's an explicit context name or already resolved.
			// No action needed. Or, we could validate if it's a valid looking context.
		}
	}
	return nil
}

// GetUserConfigDir returns the user configuration directory path
func GetUserConfigDir() (string, error) {
	homeDir, err := osUserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, userConfigDir), nil
}
