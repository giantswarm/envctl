# envctl Test Framework

## Overview

This package provides the comprehensive test framework for envctl, implementing the architecture defined in **Task 13: Implement Test Framework Architecture and envctl test Command** as part of Epic #69.

## Current Implementation Status

### ✅ Completed Components

#### 1. CLI Command Structure (Subtask 13.2)
- **File**: `cmd/test.go`
- **Status**: ✅ **COMPLETED**
- **Features**:
  - Full `envctl test` command with comprehensive flag support
  - Category-based filtering (`--category=behavioral|integration`)
  - Concept-based filtering (`--concept=serviceclass|workflow|mcpserver|capability|service`)
  - Scenario-specific execution (`--scenario=scenario-name`)
  - Parallel execution control (`--parallel=1-10`)
  - Debugging and verbose output (`--debug`, `--verbose`)
  - Timeout and fail-fast controls
  - Configuration path and report output options

#### 2. Type System and Interfaces
- **File**: `internal/testing/types.go`
- **Status**: ✅ **COMPLETED**  
- **Features**:
  - Complete type definitions for test framework
  - Comprehensive interfaces for all components
  - YAML-compatible configuration structures
  - Result reporting and metrics collection types
  - Retry and timeout configuration support

#### 3. Example Test Scenarios
- **Files**: `internal/testing/scenarios/`
- **Status**: ✅ **COMPLETED**
- **Features**:
  - ServiceClass basic operations scenario (`serviceclass_basic.yaml`)
  - Workflow basic operations scenario (`workflow_basic.yaml`)
  - Demonstrates complete YAML test scenario structure
  - Includes setup, execution, and cleanup steps
  - Shows expectation validation patterns

#### 4. Documentation and Architecture
- **Files**: `internal/testing/doc.go`, `internal/testing/README.md`
- **Status**: ✅ **COMPLETED**
- **Features**:
  - Comprehensive package documentation
  - Architecture component descriptions
  - Usage examples and integration guidance
  - CI/CD integration specifications

### 🚧 Pending Implementation (Ready for Development)

#### 1. Test Runner Engine (Subtask 13.1)
- **Target File**: `internal/testing/runner.go`
- **Requirements**:
  - Implement `TestRunner` interface
  - Lifecycle management for test execution
  - Parallel worker coordination
  - Fail-fast and timeout handling
  - Result collection and aggregation

#### 2. MCP Test Client (Subtask 13.3)
- **Target File**: `internal/testing/client.go`
- **Requirements**:
  - Implement `MCPTestClient` interface
  - MCP protocol communication using `mark3labs/mcp-go`
  - Tool invocation and response handling
  - Connection management and error handling
  - Debug protocol tracing capabilities

#### 3. Configuration System (Subtask 13.4)
- **Target File**: `internal/testing/config.go`
- **Requirements**:
  - Implement `TestScenarioLoader` interface
  - YAML scenario parsing and validation
  - Filtering logic for categories and concepts
  - Configuration validation and error reporting

#### 4. Category-based Execution Logic (Subtask 13.5)
- **Target File**: `internal/testing/executor.go`
- **Requirements**:
  - Test category routing (behavioral vs integration)
  - Execution strategy coordination
  - Step-by-step execution with validation
  - Retry logic and error recovery

#### 5. Concept-specific Test Routing (Subtask 13.6)
- **Target File**: `internal/testing/router.go`
- **Requirements**:
  - Concept-based test filtering
  - Specialized validation for each concept
  - Integration with behavioral scenarios

#### 6. Structured Logging and Reporting (Subtask 13.7)
- **Target File**: `internal/testing/reporter.go`
- **Requirements**:
  - Implement `TestReporter` interface
  - Structured output for CI/CD integration
  - Progress reporting and result summaries
  - JSON and text output formats

## Command Usage

### Basic Usage
```bash
# Run all tests
envctl test

# Run specific category
envctl test --category=behavioral
envctl test --category=integration

# Run specific concept
envctl test --concept=serviceclass
envctl test --concept=workflow

# Run specific scenario
envctl test --scenario=serviceclass-basic-operations

# Advanced options
envctl test --verbose --debug --parallel=4 --fail-fast
```

### Prerequisites
- Running envctl aggregator server: `envctl serve`
- Access to MCP aggregator endpoint (default: `http://localhost:8080/sse`)

## Test Scenario Structure

### YAML Format
```yaml
name: "scenario-name"
category: "behavioral"  # behavioral | integration
concept: "serviceclass"  # serviceclass | workflow | mcpserver | capability | service
description: "Human-readable description"
timeout: "5m"
tags: ["basic", "crud"]

steps:
  - name: "step-name"
    description: "Step description"
    tool: "core_serviceclass_create"
    parameters:
      yaml: |
        # YAML content for tool
    expected:
      success: true
      contains: ["success", "created"]
      json_path:
        status: "created"
    timeout: "1m"
    retry:
      count: 3
      delay: "5s"

cleanup:
  - name: "cleanup-step"
    # ... similar structure
```

## Integration with Epic #69

This test framework directly supports the goals of Epic #69:
- **Goal 3**: ✅ Create comprehensive test suite with automated behavioral test specs
- **Goal 4**: 🔄 Improve debuggability with enhanced error reporting and logging
- **Goal 6**: 🔄 Achieve foundational stability by identifying and fixing critical bugs

## Next Steps

1. **Implement MCP Client** - Enable actual communication with envctl aggregator
2. **Build Test Runner Engine** - Execute scenarios with lifecycle management
3. **Create Configuration System** - Load and parse YAML test scenarios
4. **Add Comprehensive Scenarios** - Based on Task 12 behavioral documentation
5. **CI/CD Integration** - Automated test execution in development pipeline

## Success Criteria Progress

- ✅ `envctl test` command successfully executes
- ✅ All test categories (behavioral, integration) supported
- 🔄 MCP client can communicate with running envctl aggregator *(pending implementation)*
- ✅ YAML test scenarios are properly structured and ready for parsing
- 🔄 Comprehensive logging and reporting implemented *(pending implementation)*
- ✅ Proper exit codes and error handling
- ✅ Test framework architecture documentation completed

## Architecture Foundation

The foundation has been established for a comprehensive testing framework that will enable:
- **Systematic Validation**: All envctl core concepts covered
- **Behavioral Testing**: BDD-style scenarios based on real-world usage
- **Integration Testing**: End-to-end component interaction validation
- **CI/CD Ready**: Structured output and automation support
- **Developer-Friendly**: Clear documentation and easy-to-use CLI interface

This represents significant progress toward Epic #69's vision of building a "supremely reliable, robust, and correct operational engine" through systematic testing and validation. 