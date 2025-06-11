package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3" // Assuming YAML parsing library
)

// For mocking in tests
var osUserHomeDir = os.UserHomeDir
var osGetwd = os.Getwd

const (
	userConfigDir    = ".config/envctl"
	projectConfigDir = ".envctl"
	configFileName   = "config.yaml"
)

// LoadConfig loads the envctl configuration by layering default, user, and project settings.
func LoadConfig() (EnvctlConfig, error) {
	// 1. Start with the default configuration
	config := GetDefaultConfigWithRoles()

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
			config = mergeConfigs(config, userConfig)
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
			config = mergeConfigs(config, projectConfig)
		}
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
func mergeConfigs(base, overlay EnvctlConfig) EnvctlConfig {
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

// GetUserConfigDir returns the user configuration directory path
func GetUserConfigDir() (string, error) {
	homeDir, err := osUserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, userConfigDir), nil
}
