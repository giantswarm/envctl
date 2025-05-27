package kube

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	// Test creating a new manager
	mgr := NewManager(nil)

	if mgr == nil {
		t.Error("Expected NewManager to return non-nil manager")
	}
}

func TestManagerContextMethods(t *testing.T) {
	mgr := NewManager(nil)

	// Test BuildMcContextName
	mcName := "test-mc"
	mcContext := mgr.BuildMcContextName(mcName)
	expected := "teleport.giantswarm.io-" + mcName
	if mcContext != expected {
		t.Errorf("Expected MC context '%s', got '%s'", expected, mcContext)
	}

	// Test BuildWcContextName
	wcName := "test-wc"
	wcContext := mgr.BuildWcContextName(mcName, wcName)
	expected = "teleport.giantswarm.io-" + mcName + "-" + wcName
	if wcContext != expected {
		t.Errorf("Expected WC context '%s', got '%s'", expected, wcContext)
	}
}

func TestStripTeleportPrefix(t *testing.T) {
	mgr := NewManager(nil)

	tests := []struct {
		input    string
		expected string
	}{
		{"teleport.giantswarm.io-test-cluster", "test-cluster"},
		{"teleport.giantswarm.io-mc-wc", "mc-wc"},
		{"no-prefix", "no-prefix"},
		{"teleport.giantswarm.io-", ""},
		{"", ""},
	}

	for _, test := range tests {
		result := mgr.StripTeleportPrefix(test.input)
		if result != test.expected {
			t.Errorf("StripTeleportPrefix(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

func TestHasTeleportPrefix(t *testing.T) {
	mgr := NewManager(nil)

	tests := []struct {
		input    string
		expected bool
	}{
		{"teleport.giantswarm.io-test-cluster", true},
		{"teleport.giantswarm.io-mc-wc", true},
		{"teleport.giantswarm.io-", true},
		{"no-prefix", false},
		{"", false},
		{"teleport-old-format", false},
	}

	for _, test := range tests {
		result := mgr.HasTeleportPrefix(test.input)
		if result != test.expected {
			t.Errorf("HasTeleportPrefix(%s) = %t, expected %t", test.input, result, test.expected)
		}
	}
}

func TestBuildMcContextName(t *testing.T) {
	mgr := NewManager(nil)

	tests := []struct {
		name     string
		mcName   string
		expected string
	}{
		{
			name:     "simple MC name",
			mcName:   "mymc",
			expected: "teleport.giantswarm.io-mymc",
		},
		{
			name:     "MC name with dashes",
			mcName:   "my-management-cluster",
			expected: "teleport.giantswarm.io-my-management-cluster",
		},
		{
			name:     "MC name with numbers",
			mcName:   "mc123",
			expected: "teleport.giantswarm.io-mc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.BuildMcContextName(tt.mcName)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestBuildWcContextName(t *testing.T) {
	mgr := NewManager(nil)

	tests := []struct {
		name     string
		mcName   string
		wcName   string
		expected string
	}{
		{
			name:     "simple MC and WC names",
			mcName:   "mymc",
			wcName:   "mywc",
			expected: "teleport.giantswarm.io-mymc-mywc",
		},
		{
			name:     "MC and WC names with dashes",
			mcName:   "my-mc",
			wcName:   "my-wc",
			expected: "teleport.giantswarm.io-my-mc-my-wc",
		},
		{
			name:     "MC and WC names with numbers",
			mcName:   "mc1",
			wcName:   "wc2",
			expected: "teleport.giantswarm.io-mc1-wc2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.BuildWcContextName(tt.mcName, tt.wcName)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestManagerInterface(t *testing.T) {
	// Test that NewManager returns a Manager interface
	var mgr Manager
	mgr = NewManager(nil)

	if mgr == nil {
		t.Error("Expected NewManager to return non-nil Manager")
	}

	// Test that the manager has the expected methods
	// This is a compile-time check
	_ = mgr.BuildMcContextName("test")
	_ = mgr.BuildWcContextName("mc", "wc")
}

// Note: We don't test methods that require external dependencies like:
// - Login (requires tsh command)
// - ListClusters (requires tsh command)
// - GetCurrentContext (requires kubectl)
// - SwitchContext (requires kubectl)
// - GetAvailableContexts (requires kubectl)
// - GetClusterNodeHealth (requires kubectl and cluster access)
// - DetermineClusterProvider (requires kubectl and cluster access)
//
// These would require integration tests or mocking of external commands.
