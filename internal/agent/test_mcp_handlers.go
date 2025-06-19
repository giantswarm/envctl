package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"envctl/internal/testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// handleRunScenarios handles the test_run_scenarios MCP tool
func (t *TestMCPServer) handleRunScenarios(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	// Extract and validate parameters
	var config testing.TestConfiguration
	config.Timeout = 10 * time.Minute // Default timeout
	config.Parallel = 1               // Default parallel workers
	config.Verbose = true             // Always verbose for MCP
	config.BasePort = 18000           // Default base port for test instances

	// Parse optional parameters
	if category, ok := args["category"].(string); ok && category != "" {
		switch category {
		case "behavioral":
			config.Category = testing.CategoryBehavioral
		case "integration":
			config.Category = testing.CategoryIntegration
		default:
			return mcp.NewToolResultError(fmt.Sprintf("Invalid category '%s', must be 'behavioral' or 'integration'", category)), nil
		}
	}

	if concept, ok := args["concept"].(string); ok && concept != "" {
		switch concept {
		case "serviceclass":
			config.Concept = testing.ConceptServiceClass
		case "workflow":
			config.Concept = testing.ConceptWorkflow
		case "mcpserver":
			config.Concept = testing.ConceptMCPServer
		case "capability":
			config.Concept = testing.ConceptCapability
		case "service":
			config.Concept = testing.ConceptService
		default:
			return mcp.NewToolResultError(fmt.Sprintf("Invalid concept '%s', must be one of: serviceclass, workflow, mcpserver, capability, service", concept)), nil
		}
	}

	if scenario, ok := args["scenario"].(string); ok {
		config.Scenario = scenario
	}

	if configPath, ok := args["config_path"].(string); ok {
		config.ConfigPath = configPath
	} else {
		config.ConfigPath = t.configPath
	}

	if parallel, ok := args["parallel"].(float64); ok {
		if parallel < 1 || parallel > 10 {
			return mcp.NewToolResultError("parallel workers must be between 1 and 10"), nil
		}
		config.Parallel = int(parallel)
	}

	if failFast, ok := args["fail_fast"].(bool); ok {
		config.FailFast = failFast
	}

	if verbose, ok := args["verbose"].(bool); ok {
		config.Verbose = verbose
	}

	config.Debug = t.debug

	// Determine scenario path
	scenarioPath := config.ConfigPath
	if scenarioPath == "" {
		scenarioPath = testing.GetDefaultScenarioPath()
	}

	// Load test scenarios
	scenarios, err := t.testLoader.LoadScenarios(scenarioPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load test scenarios: %v", err)), nil
	}

	if len(scenarios) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No test scenarios found in %s", scenarioPath)), nil
	}

	// Execute test suite
	result, err := t.testRunner.Run(ctx, config, scenarios)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Test execution failed: %v", err)), nil
	}

	// Store result for later retrieval
	t.lastResult = result

	// Format result as JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format test results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// handleListScenarios handles the test_list_scenarios MCP tool
func (t *TestMCPServer) handleListScenarios(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	// Extract config path
	configPath := t.configPath
	if path, ok := args["config_path"].(string); ok && path != "" {
		configPath = path
	}

	if configPath == "" {
		configPath = testing.GetDefaultScenarioPath()
	}

	// Load scenarios
	scenarios, err := t.testLoader.LoadScenarios(configPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to load scenarios: %v", err)), nil
	}

	// Apply filters if provided
	var filteredScenarios []testing.TestScenario
	for _, scenario := range scenarios {
		// Apply category filter
		if category, ok := args["category"].(string); ok && category != "" {
			var targetCategory testing.TestCategory
			switch category {
			case "behavioral":
				targetCategory = testing.CategoryBehavioral
			case "integration":
				targetCategory = testing.CategoryIntegration
			default:
				return mcp.NewToolResultError(fmt.Sprintf("Invalid category '%s'", category)), nil
			}
			if scenario.Category != targetCategory {
				continue
			}
		}

		// Apply concept filter
		if concept, ok := args["concept"].(string); ok && concept != "" {
			var targetConcept testing.TestConcept
			switch concept {
			case "serviceclass":
				targetConcept = testing.ConceptServiceClass
			case "workflow":
				targetConcept = testing.ConceptWorkflow
			case "mcpserver":
				targetConcept = testing.ConceptMCPServer
			case "capability":
				targetConcept = testing.ConceptCapability
			case "service":
				targetConcept = testing.ConceptService
			default:
				return mcp.NewToolResultError(fmt.Sprintf("Invalid concept '%s'", concept)), nil
			}
			if scenario.Concept != targetConcept {
				continue
			}
		}

		filteredScenarios = append(filteredScenarios, scenario)
	}

	// Format scenarios for output
	type ScenarioInfo struct {
		Name         string   `json:"name"`
		Category     string   `json:"category"`
		Concept      string   `json:"concept"`
		Description  string   `json:"description"`
		StepCount    int      `json:"step_count"`
		CleanupCount int      `json:"cleanup_count"`
		Tags         []string `json:"tags,omitempty"`
		Timeout      string   `json:"timeout,omitempty"`
	}

	scenarioList := make([]ScenarioInfo, len(filteredScenarios))
	for i, scenario := range filteredScenarios {
		info := ScenarioInfo{
			Name:         scenario.Name,
			Category:     string(scenario.Category),
			Concept:      string(scenario.Concept),
			Description:  scenario.Description,
			StepCount:    len(scenario.Steps),
			CleanupCount: len(scenario.Cleanup),
			Tags:         scenario.Tags,
		}

		if scenario.Timeout > 0 {
			info.Timeout = scenario.Timeout.String()
		}

		scenarioList[i] = info
	}

	jsonData, err := json.MarshalIndent(scenarioList, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format scenarios: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// handleValidateScenario handles the test_validate_scenario MCP tool
func (t *TestMCPServer) handleValidateScenario(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	scenarioPath, err := request.RequireString("scenario_path")
	if err != nil {
		return mcp.NewToolResultError("scenario_path parameter is required"), nil
	}

	// Try to load scenarios from the path
	scenarios, err := t.testLoader.LoadScenarios(scenarioPath)
	if err != nil {
		// Return detailed validation error
		return mcp.NewToolResultError(fmt.Sprintf("Validation failed: %v", err)), nil
	}

	// If loading succeeded, create validation report
	type ScenarioValidation struct {
		Name         string   `json:"name"`
		Valid        bool     `json:"valid"`
		Errors       []string `json:"errors,omitempty"`
		Warnings     []string `json:"warnings,omitempty"`
		StepCount    int      `json:"step_count"`
		CleanupCount int      `json:"cleanup_count"`
	}

	type ValidationResult struct {
		Valid         bool                 `json:"valid"`
		ScenarioCount int                  `json:"scenario_count"`
		Scenarios     []ScenarioValidation `json:"scenarios"`
		Path          string               `json:"path"`
	}

	result := ValidationResult{
		Valid:         true,
		ScenarioCount: len(scenarios),
		Path:          scenarioPath,
		Scenarios:     make([]ScenarioValidation, len(scenarios)),
	}

	for i, scenario := range scenarios {
		validation := ScenarioValidation{
			Name:         scenario.Name,
			Valid:        true,
			StepCount:    len(scenario.Steps),
			CleanupCount: len(scenario.Cleanup),
		}

		// Perform additional validations
		var errors []string
		var warnings []string

		// Check for empty description
		if scenario.Description == "" {
			warnings = append(warnings, "Missing description")
		}

		// Check for steps without descriptions
		for j, step := range scenario.Steps {
			if step.Description == "" {
				warnings = append(warnings, fmt.Sprintf("Step %d (%s) missing description", j+1, step.ID))
			}
		}

		// Check for missing timeouts on long scenarios
		if len(scenario.Steps) > 5 && scenario.Timeout == 0 {
			warnings = append(warnings, "Consider adding timeout for scenario with many steps")
		}

		validation.Errors = errors
		validation.Warnings = warnings

		if len(errors) > 0 {
			validation.Valid = false
			result.Valid = false
		}

		result.Scenarios[i] = validation
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format validation result: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// handleGetResults handles the test_get_results MCP tool
func (t *TestMCPServer) handleGetResults(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if t.lastResult == nil {
		return mcp.NewToolResultText("No test results available. Run test_run_scenarios first."), nil
	}

	// Format the last result as JSON
	jsonData, err := json.MarshalIndent(t.lastResult, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format test results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
