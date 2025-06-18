// Package testing provides the comprehensive test framework for envctl.
//
// This package implements the test framework architecture for Task 13, providing
// behavioral and integration testing capabilities for all envctl core concepts.
//
// ## Architecture Components
//
// The testing framework is organized into several key components:
//
// ### Test Runner Engine (runner.go)
// - Manages test execution lifecycle
// - Handles parallel test execution
// - Provides fail-fast and timeout capabilities
// - Coordinates test result collection and reporting
//
// ### MCP Client (client.go)
// - Implements MCP protocol communication for testing
// - Connects to the envctl aggregator MCP server
// - Handles tool invocation and response validation
// - Provides debugging and protocol tracing capabilities
//
// ### Configuration System (config.go)
// - Parses YAML-based test scenario definitions
// - Validates test configuration structure
// - Provides category and concept-based filtering
// - Manages test scenario metadata and execution parameters
//
// ### Test Categories
// - **Behavioral Tests**: BDD-style scenarios based on Task 12 specifications
// - **Integration Tests**: Component interaction and end-to-end validation
//
// ### Core Concepts Coverage
// - **ServiceClass**: Management and dynamic instantiation testing
// - **Workflow**: Execution and parameter templating validation
// - **MCPServer**: Registration and tool aggregation testing
// - **Capability**: API abstraction and operation testing
// - **Service**: Lifecycle and dependency management testing
//
// ## Test Scenario Structure
//
// Test scenarios are defined in YAML format with the following structure:
//
//	```yaml
//	name: "scenario-name"
//	category: "behavioral"
//	concept: "serviceclass"
//	description: "Human-readable scenario description"
//	steps:
//	  - name: "step-name"
//	    tool: "core_serviceclass_create"
//	    parameters:
//	      yaml: |
//	        name: test-serviceclass
//	        # ... YAML content
//	    expected:
//	      success: true
//	      contains: ["created successfully"]
//	```
//
// ## Usage
//
// The testing framework is invoked through the `envctl test` command:
//
//	```bash
//	envctl test                          # Run all tests
//	envctl test --category=behavioral    # Category-specific
//	envctl test --concept=serviceclass  # Concept-specific
//	envctl test --scenario=basic-create # Scenario-specific
//	```
//
// ## Dependencies
//
// The testing framework requires:
// - A running envctl aggregator server (envctl serve)
// - MCP protocol communication capabilities
// - Access to all core_* MCP tools exposed by the aggregator
//
// ## Integration with CI/CD
//
// The framework provides structured output and proper exit codes for
// integration with continuous integration and deployment pipelines.
package testing
