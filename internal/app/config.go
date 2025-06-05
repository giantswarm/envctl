package app

import (
	"envctl/internal/config"
)

// Config holds the application configuration
type Config struct {
	// Cluster configuration
	ManagementCluster string
	WorkloadCluster   string

	// UI mode
	NoTUI bool

	// Debug settings
	Debug bool

	// Safety settings
	Yolo bool

	// Environment configuration
	EnvctlConfig *config.EnvctlConfig
}

// NewConfig creates a new application configuration
func NewConfig(managementCluster, workloadCluster string, noTUI, debug, yolo bool) *Config {
	return &Config{
		ManagementCluster: managementCluster,
		WorkloadCluster:   workloadCluster,
		NoTUI:             noTUI,
		Debug:             debug,
		Yolo:              yolo,
	}
}
