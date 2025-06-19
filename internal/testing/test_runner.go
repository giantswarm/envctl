package testing

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// testRunner implements the TestRunner interface
type testRunner struct {
	client          MCPTestClient
	loader          TestScenarioLoader
	reporter        TestReporter
	instanceManager EnvCtlInstanceManager
	debug           bool
}

// NewTestRunner creates a new test runner
func NewTestRunner(client MCPTestClient, loader TestScenarioLoader, reporter TestReporter, instanceManager EnvCtlInstanceManager, debug bool) TestRunner {
	return &testRunner{
		client:          client,
		loader:          loader,
		reporter:        reporter,
		instanceManager: instanceManager,
		debug:           debug,
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
					fmt.Printf("🔄 Worker %d executing scenario: %s\n", workerID, scenario.Name)
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
		fmt.Printf("🏗️  Creating envctl instance for scenario: %s\n", scenario.Name)
	}

	instance, err = r.instanceManager.CreateInstance(scenarioCtx, scenario.Name, scenario.PreConfiguration)
	if err != nil {
		result.Result = ResultError
		result.Error = fmt.Sprintf("failed to create envctl instance: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result
	}

	// Ensure cleanup of instance
	defer func() {
		if instance != nil {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := r.instanceManager.DestroyInstance(cleanupCtx, instance); err != nil {
				if r.debug {
					fmt.Printf("⚠️  Failed to destroy envctl instance %s: %v\n", instance.ID, err)
				}
			} else {
				// Final log storage - may have been updated during destruction
				if instance.Logs != nil && result.InstanceLogs == nil {
					result.InstanceLogs = instance.Logs
				}
				if r.debug {
					fmt.Printf("✅ Cleanup complete for envctl instance %s\n", instance.ID)
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
		return result
	}

	// Connect MCP client to the instance
	if err := r.client.Connect(scenarioCtx, instance.Endpoint); err != nil {
		result.Result = ResultError
		result.Error = fmt.Sprintf("failed to connect to envctl instance: %v", err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result
	}

	// Ensure MCP client is closed properly
	defer func() {
		if r.debug {
			fmt.Printf("🔌 Closing MCP client connection to %s\n", instance.Endpoint)
		}

		// Close with timeout to avoid hanging
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer closeCancel()

		done := make(chan struct{})
		go func() {
			r.client.Close()
			close(done)
		}()

		select {
		case <-done:
			if r.debug {
				fmt.Printf("✅ MCP client closed successfully\n")
			}
		case <-closeCtx.Done():
			if r.debug {
				fmt.Printf("⏰ MCP client close timeout - connection may have been reset\n")
			}
		}

		// Give a small delay to ensure close request is processed
		time.Sleep(100 * time.Millisecond)
	}()

	if r.debug {
		fmt.Printf("✅ Connected to envctl instance %s at %s\n", instance.ID, instance.Endpoint)
	}

	// Execute steps
	for _, step := range scenario.Steps {
		stepResult := r.runStep(scenarioCtx, step, config)
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

	// Execute cleanup steps regardless of main scenario outcome
	if len(scenario.Cleanup) > 0 {
		for _, cleanupStep := range scenario.Cleanup {
			stepResult := r.runStep(scenarioCtx, cleanupStep, config)
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
	if instance != nil {
		// Get the managed process to collect logs before destruction
		if manager, ok := r.instanceManager.(*envCtlInstanceManager); ok {
			manager.mu.RLock()
			if managedProc, exists := manager.processes[instance.ID]; exists && managedProc != nil && managedProc.logCapture != nil {
				// Get logs without closing the capture yet (defer will handle that)
				instance.Logs = managedProc.logCapture.getLogs()
				result.InstanceLogs = instance.Logs
				if r.debug {
					fmt.Printf("📋 Collected instance logs for result: stdout=%d chars, stderr=%d chars\n",
						len(instance.Logs.Stdout), len(instance.Logs.Stderr))
				}
			}
			manager.mu.RUnlock()
		}
	}

	return result
}

// runStep executes a single test step
func (r *testRunner) runStep(ctx context.Context, step TestStep, config TestConfiguration) TestStepResult {
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
					fmt.Printf("⏳ Retrying step '%s' in %v (attempt %d/%d)\n", step.ID, delay, attempt+1, maxAttempts)
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
		response, err := r.client.CallTool(stepCtx, step.Tool, step.Args)
		if err != nil {
			lastErr = err
			if attempt < maxAttempts-1 {
				continue // Retry
			}
			result.Result = ResultError
			result.Error = fmt.Sprintf("tool call failed: %v", err)
			break
		}

		// Store response
		result.Response = response

		// Validate expectations
		if !r.validateExpectations(step.Expected, response, err) {
			lastErr = fmt.Errorf("expectations not met")
			if attempt < maxAttempts-1 {
				continue // Retry
			}
			result.Result = ResultFailed
			result.Error = "step expectations not met"
			break
		}

		// Success - break out of retry loop
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
		// The response might be a struct with an isError field
		if mcpResponse, ok := response.(map[string]interface{}); ok {
			if isErr, exists := mcpResponse["isError"]; exists {
				if isErrBool, ok := isErr.(bool); ok {
					isResponseError = isErrBool
				}
			}
		} else {
			// Try to access isError field via reflection or struct field
			// This handles the case where response is a struct type
			responseStr := fmt.Sprintf("%+v", response)
			if containsText(responseStr, "isError:true") || containsText(responseStr, "isError: true") {
				isResponseError = true
			}
		}
	}

	if r.debug {
		fmt.Printf("🔍 Validation Debug:\n")
		fmt.Printf("   Expected Success: %v\n", expected.Success)
		fmt.Printf("   Call Error: %v\n", err)
		fmt.Printf("   Response Error Flag: %v\n", isResponseError)
		fmt.Printf("   Response Type: %T\n", response)
		fmt.Printf("   Response Value: %+v\n", response)
	}

	// Check success expectation
	if expected.Success && (err != nil || isResponseError) {
		if r.debug {
			if err != nil {
				fmt.Printf("❌ Expected success but got error: %v\n", err)
			} else {
				fmt.Printf("❌ Expected success but response indicates error\n")
			}
		}
		return false
	}

	if !expected.Success && err == nil && !isResponseError {
		if r.debug {
			fmt.Printf("❌ Expected failure but got success (no error and no response error flag)\n")
		}
		return false
	}

	// Check error content expectations
	if len(expected.ErrorContains) > 0 {
		if err == nil {
			if r.debug {
				fmt.Printf("❌ Expected error containing text but got no error\n")
			}
			return false
		}

		errStr := err.Error()
		for _, expectedText := range expected.ErrorContains {
			if !containsText(errStr, expectedText) {
				if r.debug {
					fmt.Printf("❌ Error '%s' does not contain expected text '%s'\n", errStr, expectedText)
				}
				return false
			}
		}
	}

	// Check response content expectations
	if response != nil {
		responseStr := fmt.Sprintf("%v", response)

		// Check contains expectations
		for _, expectedText := range expected.Contains {
			if !containsText(responseStr, expectedText) {
				if r.debug {
					fmt.Printf("❌ Response does not contain expected text '%s'\n", expectedText)
				}
				return false
			}
		}

		// Check not contains expectations
		for _, unexpectedText := range expected.NotContains {
			if containsText(responseStr, unexpectedText) {
				if r.debug {
					fmt.Printf("❌ Response contains unexpected text '%s'\n", unexpectedText)
				}
				return false
			}
		}

		// TODO: Implement JSON path validation for expected.JSONPath
		// This would require parsing the response as JSON and checking specific fields
	}

	if r.debug {
		fmt.Printf("✅ All expectations met for step\n")
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
