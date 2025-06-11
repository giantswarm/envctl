package app

import (
	"testing"
)

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name  string
		noTUI bool
		debug bool
		yolo  bool
	}{
		{
			name:  "full configuration",
			noTUI: true,
			debug: true,
			yolo:  true,
		},
		{
			name:  "minimal configuration",
			noTUI: false,
			debug: false,
			yolo:  false,
		},
		{
			name:  "debug only",
			noTUI: false,
			debug: true,
			yolo:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig(tt.noTUI, tt.debug, tt.yolo)

			if cfg.NoTUI != tt.noTUI {
				t.Errorf("NoTUI = %v, want %v", cfg.NoTUI, tt.noTUI)
			}
			if cfg.Debug != tt.debug {
				t.Errorf("Debug = %v, want %v", cfg.Debug, tt.debug)
			}
			if cfg.Yolo != tt.yolo {
				t.Errorf("Yolo = %v, want %v", cfg.Yolo, tt.yolo)
			}
			if cfg.EnvctlConfig != nil {
				t.Error("EnvctlConfig should be nil before loading")
			}
		})
	}
}

func TestConfigFields(t *testing.T) {
	// Test that all fields can be set and retrieved
	cfg := &Config{
		NoTUI: true,
		Debug: true,
		Yolo:  true,
	}

	if !cfg.NoTUI {
		t.Error("NoTUI should be true")
	}
	if !cfg.Debug {
		t.Error("Debug should be true")
	}
	if !cfg.Yolo {
		t.Error("Yolo should be true")
	}
}
