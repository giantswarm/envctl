package testing

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// testReporter implements the TestReporter interface
type testReporter struct {
	verbose    bool
	debug      bool
	reportPath string
}

// NewTestReporter creates a new test reporter
func NewTestReporter(verbose, debug bool, reportPath string) TestReporter {
	return &testReporter{
		verbose:    verbose,
		debug:      debug,
		reportPath: reportPath,
	}
}

// ReportStart is called when test execution begins
func (r *testReporter) ReportStart(config TestConfiguration) {
	fmt.Printf("🧪 Starting envctl Test Framework\n")
	fmt.Printf("📡 Endpoint: %s\n", config.Endpoint)

	if r.verbose {
		fmt.Printf("⚙️  Configuration:\n")
		fmt.Printf("   • Category: %s\n", r.stringOrDefault(string(config.Category), "all"))
		fmt.Printf("   • Concept: %s\n", r.stringOrDefault(string(config.Concept), "all"))
		fmt.Printf("   • Scenario: %s\n", r.stringOrDefault(config.Scenario, "all"))
		fmt.Printf("   • Parallel workers: %d\n", config.Parallel)
		fmt.Printf("   • Fail fast: %t\n", config.FailFast)
		fmt.Printf("   • Debug mode: %t\n", config.Debug)
		fmt.Printf("   • Timeout: %v\n", config.Timeout)
		if config.ConfigPath != "" {
			fmt.Printf("   • Config path: %s\n", config.ConfigPath)
		}
		if config.ReportPath != "" {
			fmt.Printf("   • Report path: %s\n", config.ReportPath)
		}
		fmt.Printf("\n")
	}
}

// ReportScenarioStart is called when a scenario begins
func (r *testReporter) ReportScenarioStart(scenario TestScenario) {
	if r.verbose {
		fmt.Printf("🎯 Starting scenario: %s (%s/%s)\n",
			scenario.Name, scenario.Category, scenario.Concept)
		if scenario.Description != "" {
			fmt.Printf("   📝 %s\n", scenario.Description)
		}
		if len(scenario.Tags) > 0 {
			fmt.Printf("   🏷️  Tags: %s\n", strings.Join(scenario.Tags, ", "))
		}
		fmt.Printf("   📋 Steps: %d\n", len(scenario.Steps))
		if len(scenario.Cleanup) > 0 {
			fmt.Printf("   🧹 Cleanup steps: %d\n", len(scenario.Cleanup))
		}
		if scenario.Timeout > 0 {
			fmt.Printf("   ⏱️  Timeout: %v\n", scenario.Timeout)
		}
		fmt.Printf("\n")
	} else {
		fmt.Printf("🎯 %s... ", scenario.Name)
	}
}

// ReportStepResult is called when a step completes
func (r *testReporter) ReportStepResult(stepResult TestStepResult) {
	if r.verbose {
		symbol := r.getResultSymbol(stepResult.Result)
		fmt.Printf("   %s Step: %s (%v)\n",
			symbol, stepResult.Step.Name, stepResult.Duration)

		if stepResult.RetryCount > 0 {
			fmt.Printf("     🔄 Retries: %d\n", stepResult.RetryCount)
		}

		if stepResult.Error != "" {
			fmt.Printf("     ❌ Error: %s\n", stepResult.Error)
		}

		if r.debug && stepResult.Response != nil {
			responseStr := r.formatResponse(stepResult.Response)
			if responseStr != "" {
				fmt.Printf("     📤 Response: %s\n", responseStr)
			}
		}
	}
}

// ReportScenarioResult is called when a scenario completes
func (r *testReporter) ReportScenarioResult(scenarioResult TestScenarioResult) {
	symbol := r.getResultSymbol(scenarioResult.Result)

	if r.verbose {
		fmt.Printf("%s Scenario completed: %s (%v)\n",
			symbol, scenarioResult.Scenario.Name, scenarioResult.Duration)

		if scenarioResult.Error != "" {
			fmt.Printf("   ❌ Error: %s\n", scenarioResult.Error)
		}

		// Show step summary
		passed := 0
		failed := 0
		errors := 0

		for _, stepResult := range scenarioResult.StepResults {
			switch stepResult.Result {
			case ResultPassed:
				passed++
			case ResultFailed:
				failed++
			case ResultError:
				errors++
			}
		}

		fmt.Printf("   📊 Steps: %d passed", passed)
		if failed > 0 {
			fmt.Printf(", %d failed", failed)
		}
		if errors > 0 {
			fmt.Printf(", %d errors", errors)
		}
		fmt.Printf("\n\n")
	} else {
		// Compact output
		fmt.Printf("%s (%v)\n", symbol, scenarioResult.Duration)
	}
}

// ReportSuiteResult is called when all tests complete
func (r *testReporter) ReportSuiteResult(suiteResult TestSuiteResult) {
	fmt.Printf("\n🏁 Test Suite Complete\n")
	fmt.Printf("⏱️  Duration: %v\n", suiteResult.Duration)
	fmt.Printf("📊 Results:\n")
	fmt.Printf("   ✅ Passed: %d\n", suiteResult.PassedScenarios)

	if suiteResult.FailedScenarios > 0 {
		fmt.Printf("   ❌ Failed: %d\n", suiteResult.FailedScenarios)
	}

	if suiteResult.ErrorScenarios > 0 {
		fmt.Printf("   💥 Errors: %d\n", suiteResult.ErrorScenarios)
	}

	if suiteResult.SkippedScenarios > 0 {
		fmt.Printf("   ⏭️  Skipped: %d\n", suiteResult.SkippedScenarios)
	}

	fmt.Printf("   📈 Total: %d\n", suiteResult.TotalScenarios)

	// Calculate success rate
	successRate := 0.0
	if suiteResult.TotalScenarios > 0 {
		successRate = float64(suiteResult.PassedScenarios) / float64(suiteResult.TotalScenarios) * 100
	}
	fmt.Printf("   📏 Success Rate: %.1f%%\n", successRate)

	// Overall result
	if suiteResult.FailedScenarios == 0 && suiteResult.ErrorScenarios == 0 {
		fmt.Printf("\n🎉 All tests passed!\n")
	} else {
		fmt.Printf("\n💔 Some tests failed\n")
	}

	// Save detailed report if requested
	if r.reportPath != "" {
		if err := r.saveDetailedReport(suiteResult); err != nil {
			fmt.Printf("⚠️  Failed to save detailed report: %v\n", err)
		} else {
			fmt.Printf("📄 Detailed report saved to: %s\n", r.reportPath)
		}
	}
}

// saveDetailedReport saves a detailed JSON report to file
func (r *testReporter) saveDetailedReport(suiteResult TestSuiteResult) error {
	// Create report directory if it doesn't exist
	if err := os.MkdirAll(r.reportPath, 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("envctl-test-report-%s.json", timestamp)
	fullPath := fmt.Sprintf("%s/%s", r.reportPath, filename)

	// Convert to JSON
	jsonData, err := json.MarshalIndent(suiteResult, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report to JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(fullPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	return nil
}

// getResultSymbol returns an appropriate symbol for the test result
func (r *testReporter) getResultSymbol(result TestResult) string {
	switch result {
	case ResultPassed:
		return "✅"
	case ResultFailed:
		return "❌"
	case ResultSkipped:
		return "⏭️"
	case ResultError:
		return "💥"
	default:
		return "❓"
	}
}

// formatResponse formats response data for display
func (r *testReporter) formatResponse(response interface{}) string {
	if response == nil {
		return ""
	}

	// Try to format as JSON if it's a map or slice
	switch v := response.(type) {
	case map[string]interface{}, []interface{}:
		if jsonBytes, err := json.MarshalIndent(v, "", "  "); err == nil {
			return string(jsonBytes)
		}
	}

	// Fallback to string representation
	responseStr := fmt.Sprintf("%v", response)

	// Truncate very long responses
	const maxLength = 200
	if len(responseStr) > maxLength {
		return responseStr[:maxLength] + "..."
	}

	return responseStr
}

// stringOrDefault returns the string if not empty, otherwise returns the default
func (r *testReporter) stringOrDefault(s, defaultValue string) string {
	if s == "" {
		return defaultValue
	}
	return s
}

// NewQuietReporter creates a reporter that only outputs essential information
func NewQuietReporter() TestReporter {
	return &quietReporter{}
}

// quietReporter implements minimal output for CI/CD integration
type quietReporter struct{}

func (r *quietReporter) ReportStart(config TestConfiguration) {
	// Silent start
}

func (r *quietReporter) ReportScenarioStart(scenario TestScenario) {
	// Silent scenario start
}

func (r *quietReporter) ReportStepResult(stepResult TestStepResult) {
	// Silent step reporting
}

func (r *quietReporter) ReportScenarioResult(scenarioResult TestScenarioResult) {
	// Only report failures
	if scenarioResult.Result == ResultFailed || scenarioResult.Result == ResultError {
		symbol := "❌"
		if scenarioResult.Result == ResultError {
			symbol = "💥"
		}
		fmt.Printf("%s %s: %s\n", symbol, scenarioResult.Scenario.Name, scenarioResult.Error)
	}
}

func (r *quietReporter) ReportSuiteResult(suiteResult TestSuiteResult) {
	// Only output final summary
	if suiteResult.FailedScenarios == 0 && suiteResult.ErrorScenarios == 0 {
		fmt.Printf("✅ All %d tests passed\n", suiteResult.PassedScenarios)
	} else {
		fmt.Printf("❌ %d/%d tests failed\n",
			suiteResult.FailedScenarios+suiteResult.ErrorScenarios,
			suiteResult.TotalScenarios)
	}
}

// NewJSONReporter creates a reporter that outputs JSON for CI/CD integration
func NewJSONReporter() TestReporter {
	return &jsonReporter{}
}

// jsonReporter implements JSON output for machine consumption
type jsonReporter struct {
	results []TestScenarioResult
	config  TestConfiguration
}

func (r *jsonReporter) ReportStart(config TestConfiguration) {
	r.config = config
	r.results = make([]TestScenarioResult, 0)
}

func (r *jsonReporter) ReportScenarioStart(scenario TestScenario) {
	// Silent
}

func (r *jsonReporter) ReportStepResult(stepResult TestStepResult) {
	// Silent
}

func (r *jsonReporter) ReportScenarioResult(scenarioResult TestScenarioResult) {
	r.results = append(r.results, scenarioResult)
}

func (r *jsonReporter) ReportSuiteResult(suiteResult TestSuiteResult) {
	// Output complete result as JSON
	jsonData, err := json.MarshalIndent(suiteResult, "", "  ")
	if err != nil {
		fmt.Printf(`{"error": "Failed to marshal results: %v"}`, err)
		return
	}

	fmt.Println(string(jsonData))
}
