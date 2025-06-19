package agent

import (
	"context"
	"fmt"

	"envctl/internal/testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TestMCPServer wraps the test framework functionality and exposes it via MCP
type TestMCPServer struct {
	client       client.MCPClient
	endpoint     string
	logger       *Logger
	mcpServer    *server.MCPServer
	configPath   string
	debug        bool
	testRunner   testing.TestRunner
	testClient   testing.MCPTestClient
	testLoader   testing.TestScenarioLoader
	testReporter testing.TestReporter
	lastResult   *testing.TestSuiteResult
}

// NewTestMCPServer creates a new test MCP server that exposes test functionality
func NewTestMCPServer(endpoint string, logger *Logger, configPath string, debug bool) (*TestMCPServer, error) {
	// Create MCP server
	mcpServer := server.NewMCPServer(
		"envctl-test",
		"1.0.0",
		server.WithToolCapabilities(false), // No tool notifications needed for test server
		server.WithResourceCapabilities(false, false),
		server.WithPromptCapabilities(false),
	)

	// Create test framework using factory
	framework, err := testing.NewTestFramework(debug, 18000)
	if err != nil {
		return nil, fmt.Errorf("failed to create test framework: %w", err)
	}

	ts := &TestMCPServer{
		endpoint:     endpoint,
		logger:       logger,
		mcpServer:    mcpServer,
		configPath:   configPath,
		debug:        debug,
		testRunner:   framework.Runner,
		testClient:   framework.Client,
		testLoader:   framework.Loader,
		testReporter: framework.Reporter,
	}

	// Register all test tools
	ts.registerTools()

	return ts, nil
}

// Start starts the test MCP server using stdio transport
func (t *TestMCPServer) Start(ctx context.Context) error {
	// Start the stdio server
	return server.ServeStdio(t.mcpServer)
}

// registerTools registers all test MCP tools
func (t *TestMCPServer) registerTools() {
	// test_run_scenarios tool
	runScenariosTool := mcp.NewTool("test_run_scenarios",
		mcp.WithDescription("Execute test scenarios with configuration"),
		mcp.WithString("category",
			mcp.Description("Filter by category (behavioral, integration)"),
		),
		mcp.WithString("concept",
			mcp.Description("Filter by concept (serviceclass, workflow, mcpserver, capability, service)"),
		),
		mcp.WithString("scenario",
			mcp.Description("Run specific scenario by name"),
		),
		mcp.WithString("config_path",
			mcp.Description("Path to scenario files"),
		),
		mcp.WithNumber("parallel",
			mcp.Description("Number of parallel workers"),
		),
		mcp.WithBoolean("fail_fast",
			mcp.Description("Stop on first failure"),
		),
		mcp.WithBoolean("verbose",
			mcp.Description("Enable verbose output"),
		),
	)
	t.mcpServer.AddTool(runScenariosTool, t.handleRunScenarios)

	// test_list_scenarios tool
	listScenariosTool := mcp.NewTool("test_list_scenarios",
		mcp.WithDescription("List available test scenarios with filtering"),
		mcp.WithString("config_path",
			mcp.Description("Path to scenario files"),
		),
		mcp.WithString("category",
			mcp.Description("Filter by category"),
		),
		mcp.WithString("concept",
			mcp.Description("Filter by concept"),
		),
	)
	t.mcpServer.AddTool(listScenariosTool, t.handleListScenarios)

	// test_validate_scenario tool
	validateScenarioTool := mcp.NewTool("test_validate_scenario",
		mcp.WithDescription("Validate YAML scenario files for syntax and completeness"),
		mcp.WithString("scenario_path",
			mcp.Required(),
			mcp.Description("Path to scenario file or directory"),
		),
	)
	t.mcpServer.AddTool(validateScenarioTool, t.handleValidateScenario)

	// test_get_results tool
	getResultsTool := mcp.NewTool("test_get_results",
		mcp.WithDescription("Retrieve results from the last test execution"),
	)
	t.mcpServer.AddTool(getResultsTool, t.handleGetResults)
}
