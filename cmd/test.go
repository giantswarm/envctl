package cmd

import (
	"context"
	"envctl/internal/cli"
	"envctl/internal/testing"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	testEndpoint   string
	testTimeout    time.Duration
	testVerbose    bool
	testDebug      bool
	testCategory   string
	testConcept    string
	testScenario   string
	testConfigPath string
	testReportPath string
	testFailFast   bool
	testParallel   int
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Execute comprehensive behavioral and integration tests for envctl",
	Long: `The test command executes comprehensive behavioral and integration tests
against the running envctl aggregator server using MCP protocol communication.

This command validates all core envctl concepts including:
- ServiceClass management and templating
- Workflow execution and parameter resolution
- MCPServer registration and tool aggregation
- Capability definitions and API abstraction
- Service lifecycle management and dependencies

Test execution modes:
1. Full Test Suite (default): Runs all behavioral and integration tests
2. Category-based: Run specific test categories (--category)
3. Concept-based: Run tests for specific concepts (--concept)
4. Scenario-based: Run individual test scenarios (--scenario)

Test Categories:
- behavioral: BDD-style scenarios validating expected behavior
- integration: Component interaction and end-to-end validation

Core Concepts:
- serviceclass: ServiceClass management and dynamic instantiation
- workflow: Workflow execution and parameter templating
- mcpserver: MCP server registration and tool aggregation
- capability: Capability definitions and API operations
- service: Service lifecycle and dependency management

Example usage:
  envctl test                              # Run all tests
  envctl test --category=behavioral        # Run behavioral tests only
  envctl test --concept=serviceclass      # Run ServiceClass tests
  envctl test --scenario=basic-create     # Run specific scenario
  envctl test --verbose --debug           # Detailed output and debugging
  envctl test --fail-fast                 # Stop on first failure
  envctl test --parallel=4                # Run with 4 parallel workers

The test framework uses YAML-based test scenario definitions and requires
a running envctl aggregator server. Use 'envctl serve' to start the server
before running tests.

Test results are reported with structured output suitable for CI/CD integration.`,
	RunE: runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)

	// Connection and endpoint configuration
	testCmd.Flags().StringVar(&testEndpoint, "endpoint", "", "Aggregator MCP endpoint URL (default: from config)")
	testCmd.Flags().DurationVar(&testTimeout, "timeout", 10*time.Minute, "Overall test execution timeout")

	// Output and debugging
	testCmd.Flags().BoolVar(&testVerbose, "verbose", false, "Enable verbose test output")
	testCmd.Flags().BoolVar(&testDebug, "debug", false, "Enable debug logging and MCP protocol tracing")

	// Test selection and filtering
	testCmd.Flags().StringVar(&testCategory, "category", "", "Run tests for specific category (behavioral, integration)")
	testCmd.Flags().StringVar(&testConcept, "concept", "", "Run tests for specific concept (serviceclass, workflow, mcpserver, capability, service)")
	testCmd.Flags().StringVar(&testScenario, "scenario", "", "Run specific test scenario by name")

	// Test configuration and reporting
	testCmd.Flags().StringVar(&testConfigPath, "config", "", "Path to test configuration directory (default: internal test scenarios)")
	testCmd.Flags().StringVar(&testReportPath, "report", "", "Path to save detailed test report (default: stdout only)")

	// Test execution control
	testCmd.Flags().BoolVar(&testFailFast, "fail-fast", false, "Stop test execution on first failure")
	testCmd.Flags().IntVar(&testParallel, "parallel", 1, "Number of parallel test workers (1-10)")

	// Validate parallel flag
	testCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if testParallel < 1 || testParallel > 10 {
			return fmt.Errorf("parallel workers must be between 1 and 10, got %d", testParallel)
		}
		return nil
	}
}

func runTest(cmd *cobra.Command, args []string) error {
	// Create context with signal handling
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Handle interrupts gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, stopping tests gracefully...")
		cancel()
	}()

	// Create timeout context
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, testTimeout)
	defer timeoutCancel()

	// Determine endpoint using the same logic as CLI commands
	endpoint := testEndpoint
	if endpoint == "" {
		// Use the same endpoint detection logic as CLI commands
		detectedEndpoint, err := cli.DetectAggregatorEndpoint()
		if err != nil {
			// Use fallback default that matches system defaults
			endpoint = "http://localhost:8090/mcp"
			if testVerbose {
				fmt.Printf("Warning: Could not detect endpoint (%v), using default: %s\n", err, endpoint)
			}
		} else {
			endpoint = detectedEndpoint
		}
	}

	// Create test configuration
	testConfig := testing.TestConfiguration{
		Endpoint:   endpoint,
		Timeout:    testTimeout,
		Parallel:   testParallel,
		FailFast:   testFailFast,
		Verbose:    testVerbose,
		Debug:      testDebug,
		ConfigPath: testConfigPath,
		ReportPath: testReportPath,
	}

	// Parse category filter
	if testCategory != "" {
		switch testCategory {
		case "behavioral":
			testConfig.Category = testing.CategoryBehavioral
		case "integration":
			testConfig.Category = testing.CategoryIntegration
		default:
			return fmt.Errorf("invalid category '%s', must be 'behavioral' or 'integration'", testCategory)
		}
	}

	// Parse concept filter
	if testConcept != "" {
		switch testConcept {
		case "serviceclass":
			testConfig.Concept = testing.ConceptServiceClass
		case "workflow":
			testConfig.Concept = testing.ConceptWorkflow
		case "mcpserver":
			testConfig.Concept = testing.ConceptMCPServer
		case "capability":
			testConfig.Concept = testing.ConceptCapability
		case "service":
			testConfig.Concept = testing.ConceptService
		default:
			return fmt.Errorf("invalid concept '%s', must be one of: serviceclass, workflow, mcpserver, capability, service", testConcept)
		}
	}

	// Set scenario filter
	testConfig.Scenario = testScenario

	// Determine config path for scenarios
	scenarioPath := testConfigPath
	if scenarioPath == "" {
		scenarioPath = testing.GetDefaultScenarioPath()
	}

	// Create test framework components
	client := testing.NewMCPTestClient(testDebug)
	loader := testing.NewTestScenarioLoader(testDebug)
	reporter := testing.NewTestReporter(testVerbose, testDebug, testReportPath)
	runner := testing.NewTestRunner(client, loader, reporter, testDebug)

	// Load test scenarios
	scenarios, err := loader.LoadScenarios(scenarioPath)
	if err != nil {
		return fmt.Errorf("failed to load test scenarios: %w", err)
	}

	if len(scenarios) == 0 {
		fmt.Printf("⚠️  No test scenarios found in %s\n", scenarioPath)
		fmt.Printf("💡 Available test scenario files:\n")
		fmt.Printf("   • internal/testing/scenarios/serviceclass_basic.yaml\n")
		fmt.Printf("   • internal/testing/scenarios/workflow_basic.yaml\n")
		fmt.Printf("\n")
		fmt.Printf("📚 For more information, see:\n")
		fmt.Printf("   • docs/behavioral-scenarios/\n")
		return nil
	}

	// Execute test suite
	result, err := runner.Run(timeoutCtx, testConfig, scenarios)
	if err != nil {
		return fmt.Errorf("test execution failed: %w", err)
	}

	// Set exit code based on results
	if result.FailedScenarios > 0 || result.ErrorScenarios > 0 {
		os.Exit(1)
	}

	return nil
}

// getValueOrDefault returns the value if not empty, otherwise returns the default
func getValueOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
