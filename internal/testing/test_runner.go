package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// testRunner implements the TestRunner interface
type testRunner struct {
	client          MCPTestClient
	loader          TestScenarioLoader
	reporter        TestReporter
	instanceManager EnvCtlInstanceManager
	debug           bool
	logger          TestLogger
}

// NewTestRunner creates a new test runner
func NewTestRunner(client MCPTestClient, loader TestScenarioLoader, reporter TestReporter, instanceManager EnvCtlInstanceManager, debug bool) TestRunner {
	return &testRunner{
		client:          client,
		loader:          loader,
		reporter:        reporter,
		instanceManager: instanceManager,
		debug:           debug,
		logger:          NewStdoutLogger(false, debug), // Default to stdout logger
	}
}

// NewTestRunnerWithLogger creates a new test runner with custom logger
func NewTestRunnerWithLogger(client MCPTestClient, loader TestScenarioLoader, reporter TestReporter, instanceManager EnvCtlInstanceManager, debug bool, logger TestLogger) TestRunner {
	return &testRunner{
		client:          client,
		loader:          loader,
		reporter:        reporter,
		instanceManager: instanceManager,
		debug:           debug,
		logger:          logger,
	}
}

// Run executes test scenarios according to the configuration
func (r *testRunner) Run(ctx context.Context, config TestConfiguration, scenarios []TestScenario) (*TestSuiteResult, error) {
	// Create the test suite result
	result := &TestSuiteResult{
		StartTime:       time.Now(),
		TotalScenarios:  len(scenarios),
		ScenarioResults: make([]TestScenarioResult, 0, len(scenarios)),
		Configuration:   config,
	}

	// Report test start
	r.reporter.ReportStart(config)

	// Filter scenarios based on configuration
	filteredScenarios := r.loader.FilterScenarios(scenarios, config)
	result.TotalScenarios = len(filteredScenarios)

	if len(filteredScenarios) == 0 {
		r.reporter.ReportSuiteResult(*result)
		return result, nil
	}

	// Execute scenarios based on parallel configuration
	// Each scenario now manages its own envctl instance
	if config.Parallel <= 1 {
		// Sequential execution
		for _, scenario := range filteredScenarios {
			scenarioResult := r.runScenario(ctx, scenario, config)
			result.ScenarioResults = append(result.ScenarioResults, scenarioResult)

			// Update counters
			r.updateCounters(result, scenarioResult)

			// Report individual scenario result
			r.reporter.ReportScenarioResult(scenarioResult)

			// Check fail-fast
			if config.FailFast && scenarioResult.Result == ResultFailed {
				break
			}
		}
	} else {
		// Parallel execution
		results := r.runScenariosParallel(ctx, filteredScenarios, config)
		result.ScenarioResults = results

		// Update counters for all results
		for _, scenarioResult := range results {
			r.updateCounters(result, scenarioResult)
			r.reporter.ReportScenarioResult(scenarioResult)
		}
	}

	// Finalize result
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Report final suite result
	r.reporter.ReportSuiteResult(*result)

	return result, nil
}

// runScenariosParallel executes scenarios in parallel with a worker pool
// Each scenario gets its own envctl instance
func (r *testRunner) runScenariosParallel(ctx context.Context, scenarios []TestScenario, config TestConfiguration) []TestScenarioResult {
	// Create channels
	scenarioChan := make(chan TestScenario, len(scenarios))
	resultChan := make(chan TestScenarioResult, len(scenarios))

	// Send scenarios to channel
	for _, scenario := range scenarios {
		scenarioChan <- scenario
	}
	close(scenarioChan)

	// Create worker pool
	var wg sync.WaitGroup
	numWorkers := config.Parallel
	if numWorkers > len(scenarios) {
		numWorkers = len(scenarios)
	}

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for scenario := range scenarioChan {
				if r.debug {
					r.logger.Debug("🔄 Worker %d executing scenario: %s\n", workerID, scenario.Name)
				}

				// Each worker runs scenario with its own envctl instance
				scenarioResult := r.runScenario(ctx, scenario, config)
				resultChan <- scenarioResult

				// Check if we should stop due to fail-fast
				if config.FailFast && scenarioResult.Result == ResultFailed {
					// Drain remaining scenarios
					for range scenarioChan {
						// Skip remaining scenarios
					}
					return
				}
			}
		}(i)
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var results []TestScenarioResult
	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

// collectInstanceLogs collects logs from an envctl instance and stores them in the result
func (r *testRunner) collectInstanceLogs(instance *EnvCtlInstance, result *TestScenarioResult) {
	if instance == nil {
		return
	}

	// Get the managed process to collect logs
	if manager, ok := r.instanceManager.(*envCtlInstanceManager); ok {
		manager.mu.RLock()
		if managedProc, exists := manager.processes[instance.ID]; exists && managedProc != nil && managedProc.logCapture != nil {
			// Get logs without closing the capture yet (defer will handle that)
			instance.Logs = managedProc.logCapture.getLogs()
			result.InstanceLogs = instance.Logs
			if r.debug {
				r.logger.Debug("📋 Collected instance logs for result: stdout=%d chars, stderr=%d chars\n",
					len(instance.Logs.Stdout), len(instance.Logs.Stderr))
			}
		}
		manager.mu.RUnlock()
	}
}

// runScenario executes a single test scenario
func (r *testRunner) runScenario(ctx context.Context, scenario TestScenario, config TestConfiguration) TestScenarioResult {
	result := TestScenarioResult{
		Scenario:    scenario,
		StartTime:   time.Now(),
		StepResults: make([]TestStepResult, 0, len(scenario.Steps)),
		Result:      ResultPassed,
	}

	// Report scenario start
	r.reporter.ReportScenarioStart(scenario)

	// Apply scenario timeout if specified
	scenarioCtx := ctx
	if scenario.Timeout > 0 {
		var cancel context.CancelFunc
		scenarioCtx, cancel = context.WithTimeout(ctx, scenario.Timeout)
		defer cancel()
	}

	// Create and start envctl instance for this scenario
	var instance *EnvCtlInstance
	var err error

	if r.debug {
		r.logger.Debug("🏗️  Creating envctl instance for scenario: %s\n", scenario.Name)
	}

	instance, err = r.instanceManager.CreateInstance(scenarioCtx, scenario.Name, scenario.PreConfiguration)
	if err != nil {
		result.Result = ResultError
		result.Error = fmt.Sprintf("failed to create envctl instance: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)

		r.collectInstanceLogs(instance, &result)

		return result
	}

	if r.debug {
		r.logger.Debug("✅ Created envctl instance %s (port: %d)\n", instance.ID, instance.Port)
	}

	// Ensure cleanup of instance
	defer func() {
		if instance != nil {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := r.instanceManager.DestroyInstance(cleanupCtx, instance); err != nil {
				if r.debug {
					r.logger.Debug("⚠️  Failed to destroy envctl instance %s: %v\n", instance.ID, err)
				}
			} else {
				// Final log storage - may have been updated during destruction
				if instance.Logs != nil && result.InstanceLogs == nil {
					result.InstanceLogs = instance.Logs
				}
				if r.debug {
					r.logger.Debug("✅ Cleanup complete for envctl instance %s\n", instance.ID)
				}
			}
		}
	}()

	// Wait for instance to be ready
	if err := r.instanceManager.WaitForReady(scenarioCtx, instance); err != nil {
		result.Result = ResultError
		result.Error = fmt.Sprintf("envctl instance not ready: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)

		r.collectInstanceLogs(instance, &result)

		return result
	}

	// Create isolated MCP client for this scenario
	// This ensures each parallel scenario has its own client and context
	scenarioClient := NewMCPTestClientWithLogger(r.debug, r.logger)

	// Connect the isolated MCP client to this specific instance
	if err := scenarioClient.Connect(scenarioCtx, instance.Endpoint); err != nil {
		result.Result = ResultError
		result.Error = fmt.Sprintf("failed to connect to envctl instance: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)

		r.collectInstanceLogs(instance, &result)

		return result
	}

	// Ensure isolated MCP client is closed properly
	defer func() {
		if r.debug {
			r.logger.Debug("🔌 Closing isolated MCP client connection to %s\n", instance.Endpoint)
		}

		// Close with timeout to avoid hanging
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer closeCancel()

		done := make(chan struct{})
		go func() {
			scenarioClient.Close()
			close(done)
		}()

		select {
		case <-done:
			if r.debug {
				r.logger.Debug("✅ Isolated MCP client closed successfully\n")
			}
		case <-closeCtx.Done():
			if r.debug {
				r.logger.Debug("⏰ Isolated MCP client close timeout - connection may have been reset\n")
			}
		}

		// Give a small delay to ensure close request is processed
		time.Sleep(100 * time.Millisecond)
	}()

	if r.debug {
		r.logger.Debug("✅ Connected isolated MCP client to envctl instance %s at %s\n", instance.ID, instance.Endpoint)
	}

	// Execute steps using the isolated client
	for _, step := range scenario.Steps {
		stepResult := r.runStepWithClient(scenarioCtx, step, config, scenarioClient)
		result.StepResults = append(result.StepResults, stepResult)

		// Report step result
		r.reporter.ReportStepResult(stepResult)

		// Check if step failed
		if stepResult.Result == ResultFailed || stepResult.Result == ResultError {
			result.Result = stepResult.Result
			result.Error = stepResult.Error
			break
		}
	}

	// Execute cleanup steps regardless of main scenario outcome using the isolated client
	if len(scenario.Cleanup) > 0 {
		for _, cleanupStep := range scenario.Cleanup {
			stepResult := r.runStepWithClient(scenarioCtx, cleanupStep, config, scenarioClient)
			result.StepResults = append(result.StepResults, stepResult)
			r.reporter.ReportStepResult(stepResult)

			// Cleanup step failures should also fail the scenario
			if stepResult.Result == ResultFailed || stepResult.Result == ResultError {
				// Only update if the scenario hasn't already failed
				if result.Result == ResultPassed {
					result.Result = stepResult.Result
					result.Error = stepResult.Error
				}
			}
		}
	}

	// Finalize result - collect instance logs before ending
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Collect instance logs by triggering the destroy process early
	// The defer cleanup will handle the actual cleanup, but we need logs now
	r.collectInstanceLogs(instance, &result)

	return result
}

// runStep executes a single test step using the shared client (for backward compatibility)
func (r *testRunner) runStep(ctx context.Context, step TestStep, config TestConfiguration) TestStepResult {
	return r.runStepWithClient(ctx, step, config, r.client)
}

// runStepWithClient executes a single test step using the specified MCP client
func (r *testRunner) runStepWithClient(ctx context.Context, step TestStep, config TestConfiguration, client MCPTestClient) TestStepResult {
	result := TestStepResult{
		Step:      step,
		StartTime: time.Now(),
		Result:    ResultPassed,
	}

	// Apply step timeout if specified
	stepCtx := ctx
	if step.Timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		defer cancel()
	}

	// Execute step with retries
	maxAttempts := 1
	if step.Retry != nil && step.Retry.Count > 0 {
		maxAttempts = step.Retry.Count + 1
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			result.RetryCount = attempt

			// Apply retry delay
			if step.Retry != nil && step.Retry.Delay > 0 {
				delay := step.Retry.Delay
				if step.Retry.BackoffMultiplier > 0 {
					for i := 1; i < attempt; i++ {
						delay = time.Duration(float64(delay) * step.Retry.BackoffMultiplier)
					}
				}

				if r.debug {
					r.logger.Debug("⏳ Retrying step '%s' in %v (attempt %d/%d)\n", step.ID, delay, attempt+1, maxAttempts)
				}

				select {
				case <-time.After(delay):
				case <-stepCtx.Done():
					result.Result = ResultError
					result.Error = "step cancelled during retry delay"
					result.EndTime = time.Now()
					result.Duration = result.EndTime.Sub(result.StartTime)
					return result
				}
			}
		}

		// Execute the tool call
		response, err := client.CallTool(stepCtx, step.Tool, step.Args)

		// Store response (even if there's an error)
		result.Response = response

		// Validate expectations (always check, even with errors - they might be expected)
		if !r.validateExpectations(step.Expected, response, err) {
			lastErr = err
			if lastErr == nil {
				lastErr = fmt.Errorf("expectations not met")
			}
			if attempt < maxAttempts-1 {
				continue // Retry
			}

			if err != nil {
				result.Result = ResultError
				result.Error = fmt.Sprintf("tool call failed: %v", err)
			} else {
				result.Result = ResultFailed
				result.Error = "step expectations not met"
			}
			break
		}

		// Success - expectations met, even if there was an error
		result.Result = ResultPassed
		lastErr = nil // Clear any error since expectations were met
		break
	}

	// Set final error if we exhausted retries
	if lastErr != nil && result.Error == "" {
		result.Result = ResultError
		result.Error = lastErr.Error()
	}

	// Finalize result
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result
}

// validateExpectations checks if the step response meets the expected criteria
func (r *testRunner) validateExpectations(expected TestExpectation, response interface{}, err error) bool {
	// Check if response indicates an error (for MCP responses)
	isResponseError := false
	if response != nil {
		// Check if this is a CallToolResult with IsError field
		if mcpResult, ok := response.(*mcp.CallToolResult); ok {
			isResponseError = mcpResult.IsError
		} else if mcpResponse, ok := response.(map[string]interface{}); ok {
			// The response might be a map with an isError field
			if isErr, exists := mcpResponse["isError"]; exists {
				if isErrBool, ok := isErr.(bool); ok {
					isResponseError = isErrBool
				}
			}
		} else {
			// Try to access IsError field via reflection or struct field
			// This handles the case where response is a struct type
			responseStr := fmt.Sprintf("%+v", response)
			if containsText(responseStr, "IsError:true") || containsText(responseStr, "IsError: true") {
				isResponseError = true
			}
		}
	}

	// Check success expectation
	if expected.Success && (err != nil || isResponseError) {
		if r.debug {
			if err != nil {
				r.logger.Debug("❌ Expected success but got error: %v\n", err)
			} else {
				r.logger.Debug("❌ Expected success but response indicates error\n")
			}
		}
		return false
	}

	if !expected.Success && err == nil && !isResponseError {
		if r.debug {
			r.logger.Debug("❌ Expected failure but got success (no error and no response error flag)\n")
		}
		return false
	}

	// Check error content expectations
	if len(expected.ErrorContains) > 0 {
		var errorText string

		// First, try to get error text from Go error
		if err != nil {
			errorText = err.Error()
		} else if isResponseError && response != nil {
			// If no Go error but response indicates error, extract error text from response
			if mcpResult, ok := response.(*mcp.CallToolResult); ok {
				// Extract text from MCP result content
				for _, content := range mcpResult.Content {
					if textContent, ok := mcp.AsTextContent(content); ok {
						errorText += textContent.Text + " "
					}
				}
				errorText = strings.TrimSpace(errorText)
			} else {
				responseStr := fmt.Sprintf("%v", response)
				errorText = responseStr
			}
		}

		if errorText == "" {
			if r.debug {
				r.logger.Debug("❌ Expected error containing text but got no error text (err=%v, isResponseError=%v)", err, isResponseError)
			}
			return false
		}

		for _, expectedText := range expected.ErrorContains {
			if !containsText(errorText, expectedText) {
				if r.debug {
					r.logger.Debug("❌ Error text '%s' does not contain expected text '%s'", errorText, expectedText)
				}
				return false
			}
		}

		if r.debug {
			r.logger.Debug("✅ Error expectations met: found all expected text in '%s'", errorText)
		}
	}

	// Check response content expectations
	if response != nil {
		var responseStr string
		if mcpResult, ok := response.(*mcp.CallToolResult); ok {
			// Extract text content from MCP result for text matching
			var textParts []string
			for _, content := range mcpResult.Content {
				if textContent, ok := mcp.AsTextContent(content); ok {
					textParts = append(textParts, textContent.Text)
				}
			}
			responseStr = strings.Join(textParts, " ")
		} else {
			responseStr = fmt.Sprintf("%v", response)
		}

		// Check contains expectations
		for _, expectedText := range expected.Contains {
			if !containsText(responseStr, expectedText) {
				if r.debug {
					r.logger.Debug("❌ Response does not contain expected text '%s'\n", expectedText)
				}
				return false
			}
		}

		// Check not contains expectations
		for _, unexpectedText := range expected.NotContains {
			if containsText(responseStr, unexpectedText) {
				if r.debug {
					r.logger.Debug("❌ Response contains unexpected text '%s'\n", unexpectedText)
				}
				return false
			}
		}

		// Check JSON path expectations
		if len(expected.JSONPath) > 0 {
			// Parse response as JSON-like map
			var responseMap map[string]interface{}

			// Handle different response types
			if respMap, ok := response.(map[string]interface{}); ok {
				responseMap = respMap
			} else {
				// Try to extract JSON from MCP CallToolResult structure
				responseMap = r.extractJSONFromMCPResponse(response)
				if responseMap == nil {
					if r.debug {
						r.logger.Debug("❌ JSON path validation failed: could not extract JSON from response type %T\n", response)
					}
					return false
				}
			}

			// Check each JSON path expectation
			for jsonPath, expectedValue := range expected.JSONPath {
				actualValue, exists := responseMap[jsonPath]
				if !exists {
					if r.debug {
						r.logger.Debug("❌ JSON path '%s' not found in response\n", jsonPath)
					}
					return false
				}

				// Compare values
				if !compareValues(actualValue, expectedValue) {
					if r.debug {
						r.logger.Debug("❌ JSON path '%s': expected %v, got %v\n", jsonPath, expectedValue, actualValue)
					}
					return false
				}

				if r.debug {
					r.logger.Debug("✅ JSON path '%s': expected %v, got %v ✓\n", jsonPath, expectedValue, actualValue)
				}
			}
		}
	}

	if r.debug {
		r.logger.Debug("✅ All expectations met for step\n")
	}

	return true
}

// containsText checks if text contains the expected substring (case-insensitive)
func containsText(text, expected string) bool {
	// Simple case-insensitive contains check
	// In production, this could be more sophisticated
	return len(text) >= len(expected) &&
		containsSubstring(text, expected)
}

// containsSubstring performs case-insensitive substring search
func containsSubstring(text, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(text) < len(substr) {
		return false
	}

	// Convert to lowercase for case-insensitive comparison
	textLower := toLower(text)
	substrLower := toLower(substr)

	for i := 0; i <= len(textLower)-len(substrLower); i++ {
		if textLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

// toLower converts string to lowercase
func toLower(s string) string {
	result := make([]byte, len(s))
	for i, b := range []byte(s) {
		if b >= 'A' && b <= 'Z' {
			result[i] = b + 32
		} else {
			result[i] = b
		}
	}
	return string(result)
}

// extractJSONFromMCPResponse attempts to extract JSON from MCP CallToolResult
func (r *testRunner) extractJSONFromMCPResponse(response interface{}) map[string]interface{} {
	// Try to handle MCP CallToolResult structure
	responseStr := fmt.Sprintf("%+v", response)

	// Look for patterns that indicate this is an MCP response with JSON content
	if !containsText(responseStr, "Content:[") {
		return nil
	}

	// Try to extract the JSON text from the response structure
	// The format is usually: Content:[{...Text:{"json":"here"}...}]
	// We'll use string parsing to extract the JSON content

	// Find the Text: field in the string representation
	textStart := -1
	textEnd := -1

	// Look for "Text:" pattern
	textPattern := "Text:"
	for i := 0; i <= len(responseStr)-len(textPattern); i++ {
		if responseStr[i:i+len(textPattern)] == textPattern {
			textStart = i + len(textPattern)
			break
		}
	}

	if textStart == -1 {
		if r.debug {
			r.logger.Debug("🔍 Could not find 'Text:' in response: %s\n", responseStr[:min(200, len(responseStr))])
		}
		return nil
	}

	// Find the end of the JSON text (look for the closing brace followed by '}')
	braceCount := 0
	inJson := false
	for i := textStart; i < len(responseStr); i++ {
		char := responseStr[i]
		if char == '{' {
			if !inJson {
				inJson = true
				textStart = i // Start from the actual JSON opening brace
			}
			braceCount++
		} else if char == '}' {
			braceCount--
			if braceCount == 0 && inJson {
				textEnd = i + 1
				break
			}
		}
	}

	if textEnd == -1 || !inJson {
		if r.debug {
			r.logger.Debug("🔍 Could not find complete JSON in response text\n")
		}
		return nil
	}

	// Extract the JSON string
	jsonStr := responseStr[textStart:textEnd]
	if r.debug {
		r.logger.Debug("🔍 Extracted JSON string: %s\n", jsonStr)
	}

	// Parse the JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		if r.debug {
			r.logger.Debug("🔍 Failed to parse JSON: %v\n", err)
		}
		return nil
	}

	return result
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// compareValues compares two values for equality, handling type conversions
func compareValues(actual, expected interface{}) bool {
	// Direct equality check first
	if actual == expected {
		return true
	}

	// Handle nil cases
	if actual == nil || expected == nil {
		return actual == expected
	}

	// Handle boolean comparisons
	if expectedBool, ok := expected.(bool); ok {
		if actualBool, ok := actual.(bool); ok {
			return actualBool == expectedBool
		}
		// Convert string to bool if needed
		if actualStr, ok := actual.(string); ok {
			if actualStr == "true" {
				return expectedBool == true
			}
			if actualStr == "false" {
				return expectedBool == false
			}
		}
	}

	// Handle string comparisons
	if expectedStr, ok := expected.(string); ok {
		if actualStr, ok := actual.(string); ok {
			return actualStr == expectedStr
		}
		// Convert other types to string for comparison
		actualStr := fmt.Sprintf("%v", actual)
		return actualStr == expectedStr
	}

	// Handle numeric comparisons (int, float64, etc.)
	if expectedFloat, ok := expected.(float64); ok {
		if actualFloat, ok := actual.(float64); ok {
			return actualFloat == expectedFloat
		}
		if actualInt, ok := actual.(int); ok {
			return float64(actualInt) == expectedFloat
		}
	}

	if expectedInt, ok := expected.(int); ok {
		if actualInt, ok := actual.(int); ok {
			return actualInt == expectedInt
		}
		if actualFloat, ok := actual.(float64); ok {
			return actualFloat == float64(expectedInt)
		}
	}

	// For other types, convert both to strings and compare
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	return actualStr == expectedStr
}

// updateCounters updates the result counters based on a scenario result
func (r *testRunner) updateCounters(suiteResult *TestSuiteResult, scenarioResult TestScenarioResult) {
	switch scenarioResult.Result {
	case ResultPassed:
		suiteResult.PassedScenarios++
	case ResultFailed:
		suiteResult.FailedScenarios++
	case ResultSkipped:
		suiteResult.SkippedScenarios++
	case ResultError:
		suiteResult.ErrorScenarios++
	}
}
