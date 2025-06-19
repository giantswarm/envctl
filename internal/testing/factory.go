package testing

import (
	"fmt"
	"time"
)

// DefaultTestConfiguration returns a default test configuration
func DefaultTestConfiguration() TestConfiguration {
	return TestConfiguration{
		Timeout:    30 * time.Minute,
		Parallel:   1,
		FailFast:   false,
		Verbose:    false,
		Debug:      false,
		ConfigPath: GetDefaultScenarioPath(),
		BasePort:   18000, // Start from port 18000 for test instances
	}
}

// TestFramework holds all components needed for testing
type TestFramework struct {
	Runner          TestRunner
	Client          MCPTestClient
	Loader          TestScenarioLoader
	Reporter        TestReporter
	InstanceManager EnvCtlInstanceManager
}

// NewTestFramework creates a fully configured test framework
func NewTestFramework(debug bool, basePort int) (*TestFramework, error) {
	// Create instance manager
	instanceManager, err := NewEnvCtlInstanceManager(debug, basePort)
	if err != nil {
		return nil, fmt.Errorf("failed to create instance manager: %w", err)
	}

	// Create MCP client
	client := NewMCPTestClient(debug)

	// Create scenario loader
	loader := NewTestScenarioLoader(debug)

	// Create reporter
	reporter := NewTestReporter(debug, debug, "")

	// Create runner
	runner := NewTestRunner(client, loader, reporter, instanceManager, debug)

	return &TestFramework{
		Runner:          runner,
		Client:          client,
		Loader:          loader,
		Reporter:        reporter,
		InstanceManager: instanceManager,
	}, nil
}

// Cleanup cleans up resources used by the test framework
func (tf *TestFramework) Cleanup() error {
	if manager, ok := tf.InstanceManager.(*envCtlInstanceManager); ok {
		return manager.Cleanup()
	}
	return nil
}

// ValidateConfiguration validates a test configuration
func ValidateConfiguration(config TestConfiguration) error {
	if config.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if config.Parallel < 1 {
		return fmt.Errorf("parallel workers must be at least 1")
	}

	if config.BasePort < 1024 || config.BasePort > 65535 {
		return fmt.Errorf("base port must be between 1024 and 65535")
	}

	return nil
}

// NewTestConfigurationFromFile loads test configuration from a file
func NewTestConfigurationFromFile(configPath string) (TestConfiguration, error) {
	// This would load configuration from a YAML file
	// For now, return default configuration
	config := DefaultTestConfiguration()
	config.ConfigPath = configPath
	return config, nil
}
