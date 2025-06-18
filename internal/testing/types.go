package testing

import (
	"context"
	"time"
)

// TestCategory represents the category of tests to execute
type TestCategory string

const (
	// CategoryBehavioral represents BDD-style behavioral tests
	CategoryBehavioral TestCategory = "behavioral"
	// CategoryIntegration represents integration and end-to-end tests
	CategoryIntegration TestCategory = "integration"
)

// TestConcept represents the core envctl concept being tested
type TestConcept string

const (
	// ConceptServiceClass represents ServiceClass management tests
	ConceptServiceClass TestConcept = "serviceclass"
	// ConceptWorkflow represents Workflow execution tests
	ConceptWorkflow TestConcept = "workflow"
	// ConceptMCPServer represents MCPServer management tests
	ConceptMCPServer TestConcept = "mcpserver"
	// ConceptCapability represents Capability definition tests
	ConceptCapability TestConcept = "capability"
	// ConceptService represents Service lifecycle tests
	ConceptService TestConcept = "service"
)

// TestResult represents the result of test execution
type TestResult string

const (
	// ResultPassed indicates the test passed successfully
	ResultPassed TestResult = "PASSED"
	// ResultFailed indicates the test failed
	ResultFailed TestResult = "FAILED"
	// ResultSkipped indicates the test was skipped
	ResultSkipped TestResult = "SKIPPED"
	// ResultError indicates an error occurred during test execution
	ResultError TestResult = "ERROR"
)

// TestConfiguration defines the overall test execution configuration
type TestConfiguration struct {
	// Endpoint is the MCP aggregator endpoint URL
	Endpoint string `yaml:"endpoint"`
	// Timeout is the overall test execution timeout
	Timeout time.Duration `yaml:"timeout"`
	// Category filter for test execution
	Category TestCategory `yaml:"category,omitempty"`
	// Concept filter for test execution
	Concept TestConcept `yaml:"concept,omitempty"`
	// Scenario filter for specific scenario execution
	Scenario string `yaml:"scenario,omitempty"`
	// Parallel is the number of parallel test workers
	Parallel int `yaml:"parallel"`
	// FailFast stops execution on first failure
	FailFast bool `yaml:"fail_fast"`
	// Verbose enables detailed output
	Verbose bool `yaml:"verbose"`
	// Debug enables debug logging and MCP tracing
	Debug bool `yaml:"debug"`
	// ConfigPath is the path to test scenario definitions
	ConfigPath string `yaml:"config_path,omitempty"`
	// ReportPath is the path to save detailed test reports
	ReportPath string `yaml:"report_path,omitempty"`
}

// TestScenario defines a single test scenario
type TestScenario struct {
	// Name is the unique identifier for the scenario
	Name string `yaml:"name"`
	// Category is the test category (behavioral, integration)
	Category TestCategory `yaml:"category"`
	// Concept is the core concept being tested
	Concept TestConcept `yaml:"concept"`
	// Description provides human-readable scenario description
	Description string `yaml:"description"`
	// Prerequisites define setup requirements
	Prerequisites []string `yaml:"prerequisites,omitempty"`
	// Steps define the test execution steps
	Steps []TestStep `yaml:"steps"`
	// Cleanup defines teardown steps
	Cleanup []TestStep `yaml:"cleanup,omitempty"`
	// Timeout for this specific scenario
	Timeout time.Duration `yaml:"timeout,omitempty"`
	// Tags for additional categorization
	Tags []string `yaml:"tags,omitempty"`
}

// TestStep defines a single step within a test scenario
type TestStep struct {
	// Name is the step identifier
	Name string `yaml:"name"`
	// Description explains what the step does
	Description string `yaml:"description,omitempty"`
	// Tool is the MCP tool to invoke
	Tool string `yaml:"tool"`
	// Parameters are the tool parameters as a map
	Parameters map[string]interface{} `yaml:"parameters"`
	// Expected defines the expected outcome
	Expected TestExpectation `yaml:"expected"`
	// Retry configuration for this step
	Retry *RetryConfig `yaml:"retry,omitempty"`
	// Timeout for this specific step
	Timeout time.Duration `yaml:"timeout,omitempty"`
}

// TestExpectation defines what result is expected from a test step
type TestExpectation struct {
	// Success indicates whether the tool call should succeed
	Success bool `yaml:"success"`
	// ErrorContains checks if error message contains specific text
	ErrorContains []string `yaml:"error_contains,omitempty"`
	// Contains checks if response contains specific text
	Contains []string `yaml:"contains,omitempty"`
	// NotContains checks if response does not contain specific text
	NotContains []string `yaml:"not_contains,omitempty"`
	// JSONPath allows checking specific JSON response fields
	JSONPath map[string]interface{} `yaml:"json_path,omitempty"`
	// StatusCode for HTTP-based expectations
	StatusCode int `yaml:"status_code,omitempty"`
}

// RetryConfig defines retry behavior for test steps
type RetryConfig struct {
	// Count is the number of retry attempts
	Count int `yaml:"count"`
	// Delay between retry attempts
	Delay time.Duration `yaml:"delay"`
	// BackoffMultiplier for exponential backoff
	BackoffMultiplier float64 `yaml:"backoff_multiplier,omitempty"`
}

// TestSuiteResult represents the overall result of test suite execution
type TestSuiteResult struct {
	// StartTime when test execution began
	StartTime time.Time `json:"start_time"`
	// EndTime when test execution completed
	EndTime time.Time `json:"end_time"`
	// Duration of test execution
	Duration time.Duration `json:"duration"`
	// TotalScenarios is the total number of scenarios executed
	TotalScenarios int `json:"total_scenarios"`
	// PassedScenarios is the number of scenarios that passed
	PassedScenarios int `json:"passed_scenarios"`
	// FailedScenarios is the number of scenarios that failed
	FailedScenarios int `json:"failed_scenarios"`
	// SkippedScenarios is the number of scenarios that were skipped
	SkippedScenarios int `json:"skipped_scenarios"`
	// ErrorScenarios is the number of scenarios that had errors
	ErrorScenarios int `json:"error_scenarios"`
	// ScenarioResults contains individual scenario results
	ScenarioResults []TestScenarioResult `json:"scenario_results"`
	// Configuration used for this test run
	Configuration TestConfiguration `json:"configuration"`
}

// TestScenarioResult represents the result of a single test scenario
type TestScenarioResult struct {
	// Scenario is the scenario that was executed
	Scenario TestScenario `json:"scenario"`
	// Result is the overall result of the scenario
	Result TestResult `json:"result"`
	// StartTime when scenario execution began
	StartTime time.Time `json:"start_time"`
	// EndTime when scenario execution completed
	EndTime time.Time `json:"end_time"`
	// Duration of scenario execution
	Duration time.Duration `json:"duration"`
	// StepResults contains individual step results
	StepResults []TestStepResult `json:"step_results"`
	// Error message if the scenario failed or had an error
	Error string `json:"error,omitempty"`
	// Output from scenario execution
	Output string `json:"output,omitempty"`
}

// TestStepResult represents the result of a single test step
type TestStepResult struct {
	// Step is the step that was executed
	Step TestStep `json:"step"`
	// Result is the result of the step
	Result TestResult `json:"result"`
	// StartTime when step execution began
	StartTime time.Time `json:"start_time"`
	// EndTime when step execution completed
	EndTime time.Time `json:"end_time"`
	// Duration of step execution
	Duration time.Duration `json:"duration"`
	// Response from the MCP tool call
	Response interface{} `json:"response,omitempty"`
	// Error message if the step failed
	Error string `json:"error,omitempty"`
	// RetryCount is the number of retries attempted
	RetryCount int `json:"retry_count"`
}

// TestRunner interface defines the test execution engine
type TestRunner interface {
	// Run executes test scenarios according to the configuration
	Run(ctx context.Context, config TestConfiguration, scenarios []TestScenario) (*TestSuiteResult, error)
}

// MCPTestClient interface defines the MCP client for testing
type MCPTestClient interface {
	// Connect establishes connection to the MCP aggregator
	Connect(ctx context.Context, endpoint string) error
	// CallTool invokes an MCP tool with the given parameters
	CallTool(ctx context.Context, toolName string, parameters map[string]interface{}) (interface{}, error)
	// ListTools returns available MCP tools
	ListTools(ctx context.Context) ([]string, error)
	// Close closes the MCP connection
	Close() error
}

// TestScenarioLoader interface defines how test scenarios are loaded
type TestScenarioLoader interface {
	// LoadScenarios loads test scenarios from the given path
	LoadScenarios(configPath string) ([]TestScenario, error)
	// FilterScenarios filters scenarios based on the configuration
	FilterScenarios(scenarios []TestScenario, config TestConfiguration) []TestScenario
}

// TestReporter interface defines how test results are reported
type TestReporter interface {
	// ReportStart is called when test execution begins
	ReportStart(config TestConfiguration)
	// ReportScenarioStart is called when a scenario begins
	ReportScenarioStart(scenario TestScenario)
	// ReportStepResult is called when a step completes
	ReportStepResult(stepResult TestStepResult)
	// ReportScenarioResult is called when a scenario completes
	ReportScenarioResult(scenarioResult TestScenarioResult)
	// ReportSuiteResult is called when all tests complete
	ReportSuiteResult(suiteResult TestSuiteResult)
}
