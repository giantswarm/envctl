# envctl Test Framework

## Overview

The envctl test framework provides a comprehensive testing solution for validating all envctl functionality through automated test scenarios. As an envctl developer, you can use this framework to:

- **Verify Core Functionality**: Test all envctl concepts (ServiceClasses, Workflows, MCP Servers, Capabilities, Services)
- **Catch Regressions**: Automatically detect when changes break existing functionality  
- **Validate New Features**: Ensure new implementations work correctly across different scenarios
- **Debug Issues**: Systematically reproduce and diagnose problems
- **Ensure Quality**: Maintain high confidence in envctl reliability and correctness

The framework executes test scenarios written in YAML that define step-by-step operations and expected outcomes, making it easy to create comprehensive tests without writing Go code.

## Quick Start

### 1. Start envctl Aggregator

The test framework communicates with envctl through its MCP aggregator, so make sure it's running:

```bash
# Start the envctl aggregator service
envctl serve

# Verify it's running (in another terminal)
curl http://localhost:8080/health
```

### 2. Run Your First Test

```bash
# Run a simple test to verify everything works
envctl test --scenario=serviceclass-basic-operations --verbose

# If successful, run all behavioral tests
envctl test --category=behavioral
```

### 3. Common Usage Patterns

```bash
# Test specific functionality you're working on
envctl test --concept=serviceclass          # Test all ServiceClass functionality
envctl test --concept=workflow              # Test all Workflow functionality
envctl test --concept=mcpserver             # Test all MCP Server functionality

# Run tests in parallel for faster execution
envctl test --parallel=4

# Get detailed output for debugging
envctl test --verbose --debug

# Stop on first failure for quick feedback
envctl test --fail-fast
```

## How to Execute Test Scenarios

### Filtering Tests

The framework organizes tests by **category** and **concept** to help you run exactly what you need:

```bash
# By Category - Type of testing
envctl test --category=behavioral      # User-facing functionality tests
envctl test --category=integration     # Component interaction tests

# By Concept - What you're testing  
envctl test --concept=serviceclass     # All ServiceClass tests
envctl test --concept=workflow         # All Workflow tests
envctl test --concept=mcpserver        # All MCP Server tests
envctl test --concept=capability       # All Capability tests
envctl test --concept=service          # All Service tests

# Specific scenario
envctl test --scenario=serviceclass-basic-crud-operations
```

### Execution Control

```bash
# Parallel execution (faster, but harder to debug)
envctl test --parallel=4              # Run up to 4 tests simultaneously
envctl test --parallel=1              # Single-threaded (default)

# Timeout control
envctl test --timeout=10m             # Set global timeout to 10 minutes
envctl test --timeout=1h              # Longer timeout for complex tests

# Failure handling
envctl test --fail-fast               # Stop immediately on first failure
envctl test                           # Continue running all tests even if some fail
```

### Output Control

```bash
# Verbosity levels
envctl test --verbose                 # Show detailed progress and results
envctl test --debug                   # Show MCP protocol traces and internal details
envctl test                           # Normal output (default)

# Output formats
envctl test --output-format=text      # Human-readable (default)
envctl test --output-format=json      # Machine-readable JSON
envctl test --output-format=junit     # JUnit XML for CI/CD

# Save results to file
envctl test --report-file=results.json --output-format=json
```

## Understanding Test Categories and Concepts

### What Are Test Categories?

Test categories organize tests by **testing approach**:

- **`behavioral`** - Tests that verify envctl works as users expect it to work
  - Example: "When I create a ServiceClass, I can instantiate a Service from it"
  - Focus: API contracts, user workflows, expected behavior
  
- **`integration`** - Tests that verify components work together correctly
  - Example: "Workflow can orchestrate multiple Services with dependencies"
  - Focus: Component interactions, data flow, end-to-end scenarios

### What Are Test Concepts?

Test concepts organize tests by **what functionality** is being tested:

- **`serviceclass`** - Tests ServiceClass creation, validation, and Service instantiation
- **`workflow`** - Tests Workflow execution, parameter templating, and step dependencies  
- **`mcpserver`** - Tests MCP server registration, tool aggregation, and connection management
- **`capability`** - Tests Capability definitions, operation mapping, and API abstractions
- **`service`** - Tests Service lifecycle, dependency management, and state transitions

### Practical Examples

```bash
# Test if ServiceClass feature works for users
envctl test --concept=serviceclass --category=behavioral

# Test if ServiceClasses integrate properly with other components
envctl test --concept=serviceclass --category=integration

# Test all user-facing functionality across all concepts
envctl test --category=behavioral

# Test specific workflow functionality
envctl test --concept=workflow
```

## How to Debug Failing Test Scenarios

When a test fails, follow this systematic approach to diagnose and fix the issue:

### Step 1: Get Detailed Information

```bash
# Run the specific failing test with maximum verbosity
envctl test --scenario=failing-scenario-name --verbose --debug

# Or run with fail-fast to focus on the first failure
envctl test --concept=serviceclass --fail-fast --verbose
```

This will show you:
- Which step failed
- The exact MCP tool call that was made
- The response received vs. what was expected
- Complete error messages and stack traces

### Step 2: Check envctl Aggregator Status

```bash
# Check if the aggregator is running and healthy
curl http://localhost:8080/health

# Check aggregator logs for errors
journalctl --user -u envctl.service --no-pager | tail -50

# If not running, restart it
envctl serve
```

### Step 3: Verify MCP Tools Are Available

```bash
# Check what tools are available (when mcp-debug is available)
# This helps identify if required tools are missing or misconfigured
```

### Step 4: Isolate the Problem

```bash
# Run just the problematic step manually to understand what's happening
# Look at the test scenario YAML to see what MCP tool and parameters are being used

# Example: If "core_serviceclass_create" is failing, check:
# - Is the YAML in the scenario valid?
# - Are there any resource conflicts (names already exist)?
# - Are all required parameters provided?
```

### Step 5: Common Issues and Solutions

#### Test Fails with "Connection Refused"
**Problem**: envctl aggregator is not running or not accessible

```bash
# Solution: Start the aggregator
envctl serve

# Check if it's listening on the expected port
ss -tlnp | grep 8080
```

#### Test Fails with "Tool Not Found"
**Problem**: Required MCP tool is not registered with the aggregator

```bash
# Solution: Check which tools are available
# Verify that the required MCP servers are running
# Check aggregator logs for registration errors
```

#### Test Fails with "Resource Already Exists"
**Problem**: Previous test run didn't clean up resources

```bash
# Solution: Clean up manually or use unique names
# Check if cleanup steps in the scenario are working properly
```

#### Test Fails with "Timeout"
**Problem**: Operation takes longer than expected

```bash
# Solution: Increase timeout for the scenario or step
envctl test --scenario=slow-scenario --timeout=60m

# Or check if the system is under load
```

#### Test Fails with Validation Errors
**Problem**: Response doesn't match expected values

```bash
# Solution: Check the scenario's "expected" section
# Compare with actual response (shown in debug output)
# Update expectations if the behavior changed intentionally
```

### Step 6: Debug Individual Test Steps

You can examine what each test step is doing by looking at the scenario YAML file:

```yaml
# Example failing step
- name: "create-test-serviceclass"
  tool: "core_serviceclass_create"     # This is the MCP tool being called
  parameters:                          # These are the parameters sent
    yaml: |
      name: test-serviceclass
      # ... rest of YAML
  expected:                           # This is what the test expects
    success: true
    contains: ["created successfully"]
```

To debug this step:
1. Check if `core_serviceclass_create` tool exists
2. Verify the YAML parameters are valid
3. Run similar operations manually to see expected behavior
4. Check if the response actually contains "created successfully"

### Step 7: Update or Fix the Test

After identifying the issue:

- **If envctl behavior changed**: Update the test scenario expectations
- **If envctl has a bug**: Fix the bug in envctl code
- **If test scenario is wrong**: Fix the scenario YAML
- **If test environment issue**: Fix the environment setup

## Parallel Execution

### Worker Pool Configuration

The test framework supports configurable parallel execution:

```bash
# Run with 4 parallel workers (recommended for development)
envctl test --parallel=4

# Run with 8 parallel workers (recommended for CI/CD)
envctl test --parallel=8

# Single-threaded execution (useful for debugging)
envctl test --parallel=1
```

### Best Practices

- **Development**: Use 2-4 workers to balance speed and resource usage
- **CI/CD**: Use 4-8 workers for faster execution
- **Debugging**: Use 1 worker to avoid concurrent execution issues
- **Resource Limits**: Monitor memory usage with large test suites

### Performance Considerations

- Tests are executed in isolation to prevent interference
- Connection pooling reduces MCP overhead
- Cleanup operations are parallelized where safe
- Resource usage scales linearly with worker count

## Reporting

### Output Formats

#### Text Format (Default)
```bash
envctl test --output-format=text
```

Provides human-readable output with:
- Progress indicators during execution
- Detailed step-by-step results
- Summary statistics
- Error details and stack traces

#### JSON Format
```bash
envctl test --output-format=json --report-file=results.json
```

Structured output suitable for:
- CI/CD pipeline integration
- Automated result processing
- External monitoring systems
- Result archiving and analysis

#### JUnit XML Format
```bash
envctl test --output-format=junit --report-file=results.xml
```

Industry-standard format for:
- Jenkins integration
- GitLab CI/CD pipelines
- GitHub Actions
- Test result visualization tools

### Report Structure

#### Test Results Summary
```json
{
  "summary": {
    "total_scenarios": 25,
    "passed": 23,
    "failed": 1,
    "errors": 1,
    "skipped": 0,
    "execution_time": "2m34s",
    "success_rate": 92.0
  },
  "scenarios": [...]
}
```

#### Detailed Scenario Results
```json
{
  "name": "serviceclass-basic-operations",
  "category": "behavioral",
  "concept": "serviceclass",
  "status": "passed",
  "execution_time": "45s",
  "steps": [
    {
      "name": "create-test-serviceclass",
      "status": "passed",
      "execution_time": "12s",
      "tool": "core_serviceclass_create",
      "response": {...}
    }
  ]
}
```

## Troubleshooting

### Common Issues

#### 1. Connection Refused
**Symptom**: `connection refused` errors when running tests
**Solution**: 
```bash
# Ensure envctl aggregator is running
envctl serve

# Check if the service is healthy
systemctl --user status envctl.service
```

#### 2. Tool Not Found
**Symptom**: `tool not found` errors during test execution
**Solution**:
```bash
# List available tools to verify they're registered
# (This would use mcp-debug when available)
```

#### 3. Timeout Errors
**Symptom**: Tests failing with timeout errors
**Solution**:
```bash
# Increase timeout
envctl test --timeout=60m

# Check system performance and resource usage
```

#### 4. Permission Denied
**Symptom**: Permission errors when creating/deleting resources
**Solution**:
```bash
# Check service account permissions
kubectl auth can-i '*' '*'

# Verify kubeconfig
kubectl config current-context
```

#### 5. Scenario Parse Errors
**Symptom**: YAML parsing errors when loading scenarios
**Solution**:
```bash
# Validate scenario syntax
envctl test --scenario=problematic-scenario --debug

# Check YAML syntax manually
```

### Debug Mode

Enable comprehensive debugging:

```bash
envctl test --debug --verbose
```

Debug mode provides:
- Detailed MCP protocol traces
- Step-by-step execution logs
- Parameter and response dumps
- Timing information for each operation
- Resource usage statistics

### Log Analysis

Test execution logs include:
- Timestamp for each operation
- Tool invocation details
- Response validation results
- Error stack traces
- Performance metrics

Example log entry:
```
2024-01-15T10:30:45Z INFO  [serviceclass-basic] Step 'create-test-serviceclass' started
2024-01-15T10:30:45Z DEBUG [serviceclass-basic] Calling tool: core_serviceclass_create
2024-01-15T10:30:45Z DEBUG [serviceclass-basic] Parameters: {"yaml": "name: test-serviceclass..."}
2024-01-15T10:30:47Z DEBUG [serviceclass-basic] Response: {"success": true, "message": "created successfully"}
2024-01-15T10:30:47Z INFO  [serviceclass-basic] Step 'create-test-serviceclass' passed (2.1s)
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: envctl Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup envctl
        run: |
          # Install and configure envctl
          
      - name: Start envctl
        run: envctl serve &
        
      - name: Run Behavioral Tests
        run: |
          envctl test --category=behavioral \
            --output-format=junit \
            --report-file=behavioral-results.xml \
            --parallel=4
            
      - name: Run Integration Tests
        run: |
          envctl test --category=integration \
            --output-format=junit \
            --report-file=integration-results.xml \
            --parallel=2
            
      - name: Upload Test Results
        uses: actions/upload-artifact@v3
        if: always()
        with:
          name: test-results
          path: "*-results.xml"
```

### Jenkins Pipeline Example

```groovy
pipeline {
    agent any
    
    stages {
        stage('Setup') {
            steps {
                sh 'envctl serve &'
                sleep 10  // Wait for service to start
            }
        }
        
        stage('Behavioral Tests') {
            steps {
                sh '''
                    envctl test --category=behavioral \
                      --output-format=junit \
                      --report-file=behavioral-results.xml \
                      --parallel=4
                '''
            }
            post {
                always {
                    junit 'behavioral-results.xml'
                }
            }
        }
        
        stage('Integration Tests') {
            steps {
                sh '''
                    envctl test --category=integration \
                      --output-format=junit \
                      --report-file=integration-results.xml \
                      --parallel=2
                '''
            }
            post {
                always {
                    junit 'integration-results.xml'
                }
            }
        }
    }
    
    post {
        cleanup {
            sh 'pkill envctl || true'
        }
    }
}
```

### Exit Codes

The test framework provides standard exit codes for automation:

- `0`: All tests passed successfully
- `1`: Some tests failed (expected failures)
- `2`: Test framework errors (unexpected failures)
- `3`: Configuration or setup errors

## Advanced Usage

### Custom Scenario Directories

```bash
# Load scenarios from custom directory
envctl test --config-path=/path/to/custom/scenarios

# Load scenarios from multiple directories
envctl test --config-path=/path/one,/path/two
```

### Filtering and Selection

```bash
# Run tests with specific tags
envctl test --tags=smoke,critical

# Exclude tests with specific tags
envctl test --exclude-tags=slow,external

# Run tests matching name pattern
envctl test --name-pattern="serviceclass-*"
```

### Environment-Specific Configuration

```bash
# Development environment
export ENVCTL_TEST_PARALLEL=2
export ENVCTL_TEST_TIMEOUT=10m
envctl test

# CI environment
export ENVCTL_TEST_PARALLEL=8
export ENVCTL_TEST_TIMEOUT=30m
export ENVCTL_TEST_FAIL_FAST=true
envctl test
```

## Configuration

You can customize test execution through environment variables:

```bash
# MCP connection settings
export ENVCTL_MCP_ENDPOINT="http://localhost:8080/sse"    # Default MCP endpoint

# Test execution settings  
export ENVCTL_TEST_TIMEOUT="30m"                         # Default timeout
export ENVCTL_TEST_PARALLEL="4"                          # Default parallel workers
export ENVCTL_TEST_CONFIG="./scenarios"                  # Default scenario directory

# Then run tests
envctl test
```

Or use command-line flags to override settings per run:

```bash
envctl test --timeout=10m --parallel=2 --config-path=/custom/scenarios
```

## How to Create New Test Scenarios

Writing test scenarios helps you verify that envctl functionality works correctly and catch regressions. Here's how to create effective test scenarios:

### Step 1: Plan Your Test

Before writing YAML, think through:

1. **What are you testing?** 
   - Which envctl concept (ServiceClass, Workflow, MCP Server, etc.)
   - What specific functionality or behavior
   
2. **What's the user workflow?**
   - What steps would a user take?
   - What would they expect to happen?
   
3. **What could go wrong?**
   - Error conditions to test
   - Edge cases to validate

### Step 2: Choose Category and Concept

```yaml
name: "my-new-test-scenario"
category: "behavioral"           # or "integration"
concept: "serviceclass"          # serviceclass, workflow, mcpserver, capability, service
description: "Clear description of what this test verifies"
```

**Category Guidelines:**
- Use `behavioral` for testing user-facing functionality
- Use `integration` for testing component interactions

**Concept Guidelines:**
- Use the primary concept being tested
- If testing multiple concepts, choose the main focus

### Step 3: Define Test Steps

Each step should test one specific operation:

```yaml
steps:
  - name: "descriptive-step-name"
    description: "What this step accomplishes"
    tool: "core_serviceclass_create"     # MCP tool to call
    parameters:                          # Parameters for the tool
      yaml: |
        name: test-resource
        # ... configuration
    expected:                           # What you expect to happen
      success: true
      contains: ["created successfully"]
    timeout: "30s"                     # Optional step timeout
```

### Step 4: Add Comprehensive Validation

Don't just check for success - validate the actual behavior:

```yaml
expected:
  success: true
  contains: ["created successfully", "test-resource"]  # Response must contain these
  not_contains: ["error", "failed"]                    # Response must not contain these
  json_path:                                           # Validate structured response
    name: "test-resource"
    status: "created"
    available: true
```

### Step 5: Include Cleanup

Always clean up resources your test creates:

```yaml
cleanup:
  - name: "cleanup-test-resource"
    description: "Remove test resource"
    tool: "core_serviceclass_delete"
    parameters:
      name: "test-resource"
    expected:
      success: true
    continue_on_failure: true          # Continue cleanup even if this fails
    
  - name: "verify-cleanup"
    description: "Verify resource was removed"
    tool: "core_serviceclass_get"
    parameters:
      name: "test-resource"
    expected:
      success: false                   # Should fail because resource is gone
      error_contains: ["not found"]
    continue_on_failure: true
```

### Step 6: Test Your Scenario

Before committing, validate your scenario works:

```bash
# Validate YAML syntax (when validation is implemented)
envctl test --validate-scenario=path/to/your-scenario.yaml

# Run your scenario
envctl test --scenario=my-new-test-scenario --verbose

# Debug any issues
envctl test --scenario=my-new-test-scenario --debug
```

### Example: Complete ServiceClass Test Scenario

```yaml
name: "serviceclass-parameter-validation"
category: "behavioral"
concept: "serviceclass"
description: "Verify ServiceClass parameter validation works correctly"
tags: ["serviceclass", "validation", "parameters"]
timeout: "5m"

steps:
  - name: "create-serviceclass-with-valid-parameters"
    description: "Create ServiceClass with all valid parameters"
    tool: "core_serviceclass_create"
    parameters:
      yaml: |
        name: test-validation-serviceclass
        description: "Test ServiceClass for parameter validation"
        parameters:
          app_name:
            type: string
            required: true
            pattern: "^[a-z][a-z0-9-]*$"
          replicas:
            type: integer
            default: 1
            minimum: 1
            maximum: 10
        tools:
          - name: "core_service_create"
    expected:
      success: true
      contains: ["created successfully", "test-validation-serviceclass"]
    timeout: "1m"

  - name: "verify-serviceclass-available"
    description: "Verify ServiceClass is available for use"
    tool: "core_serviceclass_available"
    parameters:
      name: "test-validation-serviceclass"
    expected:
      success: true
      json_path:
        available: true
        name: "test-validation-serviceclass"

  - name: "test-valid-service-creation"
    description: "Create service with valid parameters"
    tool: "core_service_create"
    parameters:
      serviceClassName: "test-validation-serviceclass"
      label: "test-valid-service"
      parameters:
        app_name: "my-app"
        replicas: 3
    expected:
      success: true
      contains: ["created successfully", "test-valid-service"]

  - name: "test-invalid-app-name"
    description: "Verify invalid app_name is rejected"
    tool: "core_service_create"
    parameters:
      serviceClassName: "test-validation-serviceclass"
      label: "test-invalid-name"
      parameters:
        app_name: "My-App"  # Invalid: contains uppercase
        replicas: 2
    expected:
      success: false
      error_contains: ["invalid parameter", "app_name", "pattern"]

  - name: "test-invalid-replicas"
    description: "Verify replicas outside valid range are rejected"
    tool: "core_service_create"
    parameters:
      serviceClassName: "test-validation-serviceclass"
      label: "test-invalid-replicas"
      parameters:
        app_name: "test-app"
        replicas: 15  # Invalid: exceeds maximum of 10
    expected:
      success: false
      error_contains: ["invalid parameter", "replicas", "maximum"]

cleanup:
  - name: "delete-test-service"
    description: "Clean up valid test service"
    tool: "core_service_delete"
    parameters:
      label: "test-valid-service"
    expected:
      success: true
    continue_on_failure: true

  - name: "delete-test-serviceclass"
    description: "Clean up test ServiceClass"
    tool: "core_serviceclass_delete"
    parameters:
      name: "test-validation-serviceclass"
    expected:
      success: true
    continue_on_failure: true

  - name: "verify-serviceclass-deleted"
    description: "Verify ServiceClass was completely removed"
    tool: "core_serviceclass_get"
    parameters:
      name: "test-validation-serviceclass"
    expected:
      success: false
      error_contains: ["not found"]
    continue_on_failure: true
```

### Best Practices for Writing Tests

#### Use Descriptive Names
```yaml
# ✅ Good - describes what the test does
name: "serviceclass-parameter-validation-with-constraints"

# ❌ Bad - generic and unclear  
name: "test-serviceclass-1"
```

#### Test Both Success and Failure Cases
```yaml
# Test successful operation
- name: "create-valid-serviceclass"
  # ... test valid creation

# Test error conditions
- name: "reject-duplicate-serviceclass-creation"
  # ... test duplicate name rejection
```

#### Use Unique Resource Names
```yaml
# ✅ Good - unique names prevent conflicts
parameters:
  yaml: |
    name: "test-scenario-unique-serviceclass"

# ❌ Bad - generic names cause conflicts
parameters:
  yaml: |
    name: "test-serviceclass"
```

#### Always Include Cleanup
```yaml
# Include cleanup section even for failed tests
cleanup:
  - name: "cleanup-resources"
    # ... cleanup steps
    continue_on_failure: true  # Don't fail the test if cleanup fails
```

#### Test Realistic Scenarios
```yaml
# ✅ Good - tests realistic user workflow
description: "User can create ServiceClass, instantiate Service, and scale it"

# ❌ Bad - tests internal implementation details
description: "Verify ServiceClass internal validation logic"
```

### Where to Put Your Test Scenarios

Organize scenarios by category and concept:

```
internal/testing/scenarios/
├── behavioral/
│   ├── serviceclass/
│   │   ├── basic-crud.yaml
│   │   ├── parameter-validation.yaml          # ← Your new test here
│   │   └── tool-integration.yaml
│   ├── workflow/
│   │   ├── execution-flow.yaml
│   │   └── parameter-templating.yaml
│   └── ...
└── integration/
    ├── end-to-end/
    └── component/
```

### Running Your New Test

```bash
# Run your specific test
envctl test --scenario=serviceclass-parameter-validation

# Run all tests in your concept area
envctl test --concept=serviceclass

# Run with your changes included
envctl test --category=behavioral --concept=serviceclass
```

## Where to Find More Information

- **Scenario Authoring Details**: See [scenarios.md](scenarios.md) for complete YAML reference
- **Example Scenarios**: Check [examples/](examples/) directory for comprehensive examples  
- **Package Documentation**: See `internal/testing/doc.go` for implementation details

---

**Related Issues**: [#71](https://github.com/giantswarm/envctl/issues/71), [#69](https://github.com/giantswarm/envctl/issues/69) 