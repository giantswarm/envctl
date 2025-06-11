package app

import (
	"envctl/internal/config"
)

// Config holds the application configuration
type Config struct {
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
func NewConfig(noTUI, debug, yolo bool) *Config {
	return &Config{
		NoTUI: noTUI,
		Debug: debug,
		Yolo:  yolo,
	}
}
