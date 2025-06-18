package testing

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// testRunner implements the TestRunner interface
type testRunner struct {
	client   MCPTestClient
	loader   TestScenarioLoader
	reporter TestReporter
	debug    bool
}

// NewTestRunner creates a new test runner
func NewTestRunner(client MCPTestClient, loader TestScenarioLoader, reporter TestReporter, debug bool) TestRunner {
	return &testRunner{
		client:   client,
		loader:   loader,
		reporter: reporter,
		debug:    debug,
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

	// Connect to MCP aggregator
	if err := r.client.Connect(ctx, config.Endpoint); err != nil {
		return nil, fmt.Errorf("failed to connect to MCP aggregator: %w", err)
	}
	defer r.client.Close()

	// Execute scenarios based on parallel configuration
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
		}
	}

	// Finalize result
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

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
					fmt.Printf("⏳ Retrying step '%s' in %v (attempt %d/%d)\n", step.Name, delay, attempt+1, maxAttempts)
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
		response, err := r.client.CallTool(stepCtx, step.Tool, step.Parameters)
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
	// Check success expectation
	if expected.Success && err != nil {
		if r.debug {
			fmt.Printf("❌ Expected success but got error: %v\n", err)
		}
		return false
	}

	if !expected.Success && err == nil {
		if r.debug {
			fmt.Printf("❌ Expected failure but got success\n")
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
