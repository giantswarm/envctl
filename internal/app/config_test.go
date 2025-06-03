package app

import (
	"testing"
)

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name              string
		managementCluster string
		workloadCluster   string
		noTUI             bool
		debug             bool
	}{
		{
			name:              "full configuration",
			managementCluster: "mc-test",
			workloadCluster:   "wc-test",
			noTUI:             true,
			debug:             true,
		},
		{
			name:              "minimal configuration",
			managementCluster: "mc-only",
			workloadCluster:   "",
			noTUI:             false,
			debug:             false,
		},
		{
			name:              "empty configuration",
			managementCluster: "",
			workloadCluster:   "",
			noTUI:             false,
			debug:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig(tt.managementCluster, tt.workloadCluster, tt.noTUI, tt.debug)

			if cfg.ManagementCluster != tt.managementCluster {
				t.Errorf("ManagementCluster = %v, want %v", cfg.ManagementCluster, tt.managementCluster)
			}
			if cfg.WorkloadCluster != tt.workloadCluster {
				t.Errorf("WorkloadCluster = %v, want %v", cfg.WorkloadCluster, tt.workloadCluster)
			}
			if cfg.NoTUI != tt.noTUI {
				t.Errorf("NoTUI = %v, want %v", cfg.NoTUI, tt.noTUI)
			}
			if cfg.Debug != tt.debug {
				t.Errorf("Debug = %v, want %v", cfg.Debug, tt.debug)
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
		ManagementCluster: "test-mc",
		WorkloadCluster:   "test-wc",
		NoTUI:             true,
		Debug:             true,
	}

	if cfg.ManagementCluster != "test-mc" {
		t.Errorf("ManagementCluster = %v, want %v", cfg.ManagementCluster, "test-mc")
	}
	if cfg.WorkloadCluster != "test-wc" {
		t.Errorf("WorkloadCluster = %v, want %v", cfg.WorkloadCluster, "test-wc")
	}
	if !cfg.NoTUI {
		t.Error("NoTUI should be true")
	}
	if !cfg.Debug {
		t.Error("Debug should be true")
	}
}
