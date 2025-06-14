package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"envctl/pkg/logging"

	"gopkg.in/yaml.v3"
)

// For mocking in tests
var osUserHomeDir = os.UserHomeDir
var osGetwd = os.Getwd

const (
	userConfigDir    = ".config/envctl"
	projectConfigDir = ".envctl"
	configFileName   = "config.yaml"
)

// LoadedFile represents a configuration file that was loaded
type LoadedFile struct {
	Path   string // Full path to the file
	Source string // "user" or "project"
	Name   string // Base filename without extension
}

// ConfigurationLoader provides common layered loading for all configuration types.
// This utility ensures NO DIFFERENCE between packages in how they handle configuration loading.
type ConfigurationLoader struct {
	userConfigDir    string
	projectConfigDir string
}

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

// NewConfigurationLoader creates a new configuration loader
func NewConfigurationLoader() (*ConfigurationLoader, error) {
	userDir, err := GetUserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine user config directory: %w", err)
	}

	projectDir, err := getProjectConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine project config directory: %w", err)
	}

	return &ConfigurationLoader{
		userConfigDir:    userDir,
		projectConfigDir: projectDir,
	}, nil
}

// LoadYAMLFiles loads YAML files from both user and project directories with layered override.
// Project files override user files with the same base name.
// Returns slice of file paths in order: user files first, then project files (for override behavior)
func (cl *ConfigurationLoader) LoadYAMLFiles(subDir string) ([]LoadedFile, error) {
	var allFiles []LoadedFile
	nameMap := make(map[string]bool) // Track file names for override detection

	// 1. Load from user directory first
	userPath := filepath.Join(cl.userConfigDir, subDir)
	userFiles, err := cl.loadFilesFromDirectory(userPath, "user")
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load user %s: %w", subDir, err)
	}

	// Add user files to result
	for _, file := range userFiles {
		allFiles = append(allFiles, file)
		nameMap[file.Name] = true
	}

	// 2. Load from project directory (overrides user)
	projectPath := filepath.Join(cl.projectConfigDir, subDir)
	projectFiles, err := cl.loadFilesFromDirectory(projectPath, "project")
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load project %s: %w", subDir, err)
	}

	// Handle project overrides
	for _, projectFile := range projectFiles {
		if nameMap[projectFile.Name] {
			// Remove user file with same name
			for i, userFile := range allFiles {
				if userFile.Name == projectFile.Name && userFile.Source == "user" {
					allFiles = append(allFiles[:i], allFiles[i+1:]...)
					logging.Info("ConfigurationLoader", "Project %s overriding user %s in %s", projectFile.Name, userFile.Name, subDir)
					break
				}
			}
		}
		allFiles = append(allFiles, projectFile)
		nameMap[projectFile.Name] = true
	}

	logging.Info("ConfigurationLoader", "Loaded %d files from %s (%d user, %d project)",
		len(allFiles), subDir, len(userFiles), len(projectFiles))

	return allFiles, nil
}

// loadFilesFromDirectory loads all YAML files from a directory
func (cl *ConfigurationLoader) loadFilesFromDirectory(dirPath, source string) ([]LoadedFile, error) {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return nil, nil // Directory doesn't exist, return empty
	}

	var allFiles []string

	// Load .yaml files
	yamlPattern := filepath.Join(dirPath, "*.yaml")
	yamlFiles, err := filepath.Glob(yamlPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob yaml files: %w", err)
	}
	allFiles = append(allFiles, yamlFiles...)

	// Load .yml files
	ymlPattern := filepath.Join(dirPath, "*.yml")
	ymlFiles, err := filepath.Glob(ymlPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob yml files: %w", err)
	}
	allFiles = append(allFiles, ymlFiles...)

	// Convert to LoadedFile structs
	var result []LoadedFile
	for _, filePath := range allFiles {
		basename := filepath.Base(filePath)
		name := strings.TrimSuffix(basename, filepath.Ext(basename))

		result = append(result, LoadedFile{
			Path:   filePath,
			Source: source,
			Name:   name,
		})
	}

	return result, nil
}

// LoadAndParseYAML is a generic utility for loading and parsing YAML files into any type.
// This ensures consistent loading behavior across all packages.
func LoadAndParseYAML[T any](subDir string, validator func(T) error) ([]T, error) {
	loader, err := NewConfigurationLoader()
	if err != nil {
		return nil, err
	}

	files, err := loader.LoadYAMLFiles(subDir)
	if err != nil {
		return nil, err
	}

	var results []T

	for _, file := range files {
		var item T
		data, err := os.ReadFile(file.Path)
		if err != nil {
			logging.Error("ConfigurationLoader", err, "Failed to read %s file: %s", file.Source, file.Path)
			continue
		}

		if err := yaml.Unmarshal(data, &item); err != nil {
			logging.Error("ConfigurationLoader", err, "Failed to parse %s YAML file: %s", file.Source, file.Path)
			continue
		}

		// Validate if validator provided
		if validator != nil {
			if err := validator(item); err != nil {
				logging.Error("ConfigurationLoader", err, "Validation failed for %s file: %s", file.Source, file.Path)
				continue
			}
		}

		results = append(results, item)
		logging.Info("ConfigurationLoader", "Loaded %s configuration from %s: %s", file.Source, subDir, file.Name)
	}

	return results, nil
}

// Path helper functions

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

// GetUserConfigDir returns the user configuration directory path
func GetUserConfigDir() (string, error) {
	homeDir, err := osUserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, userConfigDir), nil
}

// getProjectConfigDir returns the project configuration directory path
func getProjectConfigDir() (string, error) {
	wd, err := osGetwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, projectConfigDir), nil
}

// GetConfigurationPaths returns both user and project configuration directory paths
func GetConfigurationPaths() (userDir, projectDir string, err error) {
	userDir, err = GetUserConfigDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to determine user config directory: %w", err)
	}

	projectDir, err = getProjectConfigDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to determine project config directory: %w", err)
	}

	return userDir, projectDir, nil
}

// Config file loading functions

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
