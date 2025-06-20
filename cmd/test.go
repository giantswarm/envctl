package cmd

import (
	"context"
	"envctl/internal/agent"
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
	testMCPServer  bool
	testBasePort   int
	// New flags for mock MCP server
	testMockMCPServer bool
	testConfigName    string
	testMockConfig    string
)

// completeCategoryFlag provides shell completion for the category flag
func completeCategoryFlag(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"behavioral", "integration"}, cobra.ShellCompDirectiveDefault
}

// completeConceptFlag provides shell completion for the concept flag
func completeConceptFlag(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return []string{"serviceclass", "workflow", "mcpserver", "capability", "service"}, cobra.ShellCompDirectiveDefault
}

// completeScenarioFlag provides shell completion for the scenario flag by loading available scenarios
func completeScenarioFlag(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Get the config path, use default if not specified
	configPath := testConfigPath
	if configPath == "" {
		configPath = testing.GetDefaultScenarioPath()
	}

	// Create a loader to get available scenarios
	loader := testing.NewTestScenarioLoader(false) // Don't enable debug for completion
	scenarios, err := loader.LoadScenarios(configPath)
	if err != nil {
		// Return empty completion on error
		return []string{}, cobra.ShellCompDirectiveDefault
	}

	// Extract scenario names
	var scenarioNames []string
	for _, scenario := range scenarios {
		scenarioNames = append(scenarioNames, scenario.Name)
	}

	return scenarioNames, cobra.ShellCompDirectiveDefault
}

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Execute comprehensive behavioral and integration tests for envctl",
	Long: `The test command executes comprehensive behavioral and integration tests
for envctl by creating clean, isolated instances of envctl serve for each test scenario.

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
5. MCP Server mode (--mcp-server): Runs an MCP server that exposes test functionality via stdio

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
  envctl test --base-port=19000           # Use port 19000+ for test instances
  envctl test --mcp-server                # Run as MCP server (stdio transport)

In MCP Server mode:
- The test command acts as an MCP server using stdio transport
- It exposes all test functionality as MCP tools
- It's designed for integration with AI assistants like Claude or Cursor
- Configure it in your AI assistant's MCP settings

The test framework uses YAML-based test scenario definitions and automatically
creates clean, isolated envctl serve instances for each test scenario.
Each scenario can specify pre-configuration including MCP servers, workflows,
capabilities, service classes, and service instances.

Test results are reported with structured output suitable for CI/CD integration.`,
	RunE: runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)

	// Test execution configuration
	testCmd.Flags().DurationVar(&testTimeout, "timeout", 10*time.Minute, "Overall test execution timeout")
	testCmd.Flags().IntVar(&testBasePort, "base-port", 18000, "Starting port number for test envctl instances")

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

	// MCP Server mode
	testCmd.Flags().BoolVar(&testMCPServer, "mcp-server", false, "Run as MCP server (stdio transport)")

	// New flags for mock MCP server
	testCmd.Flags().BoolVar(&testMockMCPServer, "mock-mcp-server", false, "Run as mock MCP server")
	testCmd.Flags().StringVar(&testConfigName, "config-name", "", "Name of the mock MCP server configuration")
	testCmd.Flags().StringVar(&testMockConfig, "mock-config", "", "Path to mock MCP server configuration file")

	// Shell completion for test flags
	_ = testCmd.RegisterFlagCompletionFunc("category", completeCategoryFlag)
	_ = testCmd.RegisterFlagCompletionFunc("concept", completeConceptFlag)
	_ = testCmd.RegisterFlagCompletionFunc("scenario", completeScenarioFlag)

	// Mark flags as mutually exclusive with MCP server mode
	testCmd.MarkFlagsMutuallyExclusive("mcp-server", "category")
	testCmd.MarkFlagsMutuallyExclusive("mcp-server", "concept")
	testCmd.MarkFlagsMutuallyExclusive("mcp-server", "scenario")
	testCmd.MarkFlagsMutuallyExclusive("mcp-server", "fail-fast")
	testCmd.MarkFlagsMutuallyExclusive("mcp-server", "parallel")

	// Mark flags as mutually exclusive with mock MCP server mode
	testCmd.MarkFlagsMutuallyExclusive("mock-mcp-server", "category")
	testCmd.MarkFlagsMutuallyExclusive("mock-mcp-server", "concept")
	testCmd.MarkFlagsMutuallyExclusive("mock-mcp-server", "scenario")
	testCmd.MarkFlagsMutuallyExclusive("mock-mcp-server", "fail-fast")
	testCmd.MarkFlagsMutuallyExclusive("mock-mcp-server", "mcp-server")

	// Validate parallel flag
	testCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if !testMCPServer && !testMockMCPServer && (testParallel < 1 || testParallel > 10) {
			return fmt.Errorf("parallel workers must be between 1 and 10, got %d", testParallel)
		}
		if testMockMCPServer && testMockConfig == "" {
			return fmt.Errorf("--mock-config is required when using --mock-mcp-server")
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
		if !testMCPServer && !testMockMCPServer {
			fmt.Println("\nReceived interrupt signal, stopping tests gracefully...")
		}
		cancel()
	}()

	// Run in MCP Server mode if requested
	if testMCPServer {
		// For MCP server mode, we still need an endpoint for existing functionality
		endpoint := "http://localhost:8090/mcp"
		detectedEndpoint, err := cli.DetectAggregatorEndpoint()
		if err == nil {
			endpoint = detectedEndpoint
		}

		// Create logger for MCP server
		logger := agent.NewLogger(testVerbose, true, testDebug)

		// Create test MCP server
		server, err := agent.NewTestMCPServer(endpoint, logger, testConfigPath, testDebug)
		if err != nil {
			return fmt.Errorf("failed to create test MCP server: %w", err)
		}

		logger.Info("Starting envctl test MCP server (stdio transport)...")
		logger.Info("Connecting to aggregator at: %s", endpoint)

		if err := server.Start(ctx); err != nil {
			return fmt.Errorf("test MCP server error: %w", err)
		}
		return nil
	}

	// Run in Mock MCP Server mode if requested
	if testMockMCPServer {
		// Create mock MCP server using the provided config file
		mockServer, err := testing.NewMockMCPServerFromFile(testMockConfig, testDebug)
		if err != nil {
			return fmt.Errorf("failed to create mock MCP server: %w", err)
		}

		if testDebug {
			fmt.Printf("🔧 Starting mock MCP server with config '%s' (stdio transport)...\n", testMockConfig)
		}

		if err := mockServer.Start(ctx); err != nil {
			return fmt.Errorf("mock MCP server error: %w", err)
		}
		return nil
	}

	// Create timeout context for normal test execution
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, testTimeout)
	defer timeoutCancel()

	// Create test configuration
	testConfig := testing.TestConfiguration{
		Timeout:    testTimeout,
		Parallel:   testParallel,
		FailFast:   testFailFast,
		Verbose:    testVerbose,
		Debug:      testDebug,
		ConfigPath: testConfigPath,
		ReportPath: testReportPath,
		BasePort:   testBasePort,
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

	// Create test framework with proper verbose and debug flags
	framework, err := testing.NewTestFrameworkWithVerbose(testVerbose, testDebug, testBasePort, testReportPath)
	if err != nil {
		return fmt.Errorf("failed to create test framework: %w", err)
	}
	defer framework.Cleanup()

	// Load test scenarios
	scenarios, err := framework.Loader.LoadScenarios(scenarioPath)
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
	result, err := framework.Runner.Run(timeoutCtx, testConfig, scenarios)
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
