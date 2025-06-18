# Testing envctl via MCP

## Overview

This guide provides comprehensive documentation for testing envctl using its Model Context Protocol (MCP) server integration. The testing framework exposes MCP tools that enable LLM agents and IDE integrations to execute, manage, and validate envctl functionality through standardized protocols.

**Key Benefits:**
- **AI-Powered Testing**: LLM agents can autonomously execute test scenarios and validate functionality
- **IDE Integration**: Direct testing from development environments with MCP-enabled tools  
- **Standardized Interface**: Consistent tool-based approach across different testing contexts
- **Automated Validation**: Comprehensive scenario execution with built-in result verification

## MCP Tools Overview

The envctl testing framework exposes four primary MCP tools through the aggregator:

### 1. `mcp_envctl-test_test_run_scenarios`
**Purpose**: Execute test scenarios with comprehensive configuration options

**Parameters**:
- `category` (string, optional): Filter by category ("behavioral", "integration")
- `concept` (string, optional): Filter by concept ("serviceclass", "workflow", "mcpserver", "capability", "service")
- `scenario` (string, optional): Run specific scenario by name
- `config_path` (string, optional): Path to scenario files
- `parallel` (number, optional): Number of parallel workers (default: 1)
- `fail_fast` (boolean, optional): Stop on first failure (default: false)
- `verbose` (boolean, optional): Enable verbose output (default: false)

**Response Format**:
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
  "scenarios": [
    {
      "name": "serviceclass-basic-operations",
      "status": "passed",
      "execution_time": "45s",
      "steps": [...]
    }
  ]
}
```

### 2. `mcp_envctl-test_test_list_scenarios`
**Purpose**: Discover available test scenarios with filtering capabilities

**Parameters**:
- `category` (string, optional): Filter by category
- `concept` (string, optional): Filter by concept  
- `config_path` (string, optional): Path to scenario files

**Response Format**:
```json
{
  "scenarios": [
    {
      "name": "serviceclass-basic-operations",
      "category": "behavioral", 
      "concept": "serviceclass",
      "description": "Basic ServiceClass management operations",
      "tags": ["basic", "crud", "serviceclass"],
      "steps": 6,
      "estimated_duration": "5m"
    }
  ],
  "total_count": 15,
  "categories": ["behavioral", "integration"],
  "concepts": ["serviceclass", "workflow", "mcpserver"]
}
```

### 3. `mcp_envctl-test_test_validate_scenario`
**Purpose**: Validate YAML scenario files for syntax and completeness

**Parameters**:
- `scenario_path` (string, required): Path to scenario file or directory

**Response Format**:
```json
{
  "valid": true,
  "errors": [],
  "warnings": [
    "Step 'create-test-service' has no timeout specified"
  ],
  "scenarios_validated": 3
}
```

### 4. `mcp_envctl-test_test_get_results`
**Purpose**: Retrieve detailed results from the last test execution

**Parameters**:
- `random_string` (string, required): Dummy parameter (use any value)

**Response Format**:
```json
{
  "last_execution": {
    "start_time": "2024-01-15T10:30:00Z",
    "end_time": "2024-01-15T10:35:30Z", 
    "duration": "5m30s",
    "configuration": {...},
    "detailed_results": {...}
  },
  "available": true
}
```

## Usage Patterns

### Basic Test Execution

#### Run All Tests
```json
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "verbose": true
  }
}
```

#### Run Category-Specific Tests
```json
{
  "tool": "mcp_envctl-test_test_run_scenarios", 
  "parameters": {
    "category": "behavioral",
    "verbose": true
  }
}
```

#### Run Concept-Specific Tests
```json
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "concept": "serviceclass",
    "parallel": 2
  }
}
```

#### Run Single Scenario
```json
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "scenario": "serviceclass-basic-operations",
    "verbose": true
  }
}
```

### Advanced Filtering and Configuration

#### Parallel Execution with Fail-Fast
```json
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "category": "integration",
    "parallel": 4,
    "fail_fast": true,
    "verbose": true
  }
}
```

#### Custom Scenario Path
```json
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "config_path": "/custom/scenarios",
    "concept": "workflow"
  }
}
```

### Scenario Discovery and Validation

#### List Available Scenarios
```json
{
  "tool": "mcp_envctl-test_test_list_scenarios",
  "parameters": {
    "concept": "serviceclass"
  }
}
```

#### Validate Scenario Files
```json
{
  "tool": "mcp_envctl-test_test_validate_scenario",
  "parameters": {
    "scenario_path": "internal/testing/scenarios/"
  }
}
```

#### Get Latest Results
```json
{
  "tool": "mcp_envctl-test_test_get_results",
  "parameters": {
    "random_string": "get_results"
  }
}
```

## Workflow Examples

### Development Workflow

#### Pre-Commit Testing Validation
```bash
# 1. Start envctl aggregator
./scripts/dev-restart.sh

# 2. Quick validation of core functionality
```
```json
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "category": "behavioral",
    "parallel": 2,
    "fail_fast": true
  }
}
```

#### Local Development Testing Pattern
```bash
# 1. Restart envctl with changes
./scripts/dev-restart.sh

# 2. Test specific concept you're working on
```
```json
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "concept": "workflow",
    "verbose": true
  }
}
```

#### Quick Feedback Loop
```json
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "scenario": "serviceclass-basic-operations",
    "verbose": true
  }
}
```

### Scenario Authoring Workflow

#### 1. Create New Test Scenario
```yaml
# Create: internal/testing/scenarios/my-new-test.yaml
name: "my-new-feature-test"
category: "behavioral"
concept: "serviceclass"
description: "Test my new ServiceClass feature"

steps:
  - name: "test-new-feature"
    tool: "core_serviceclass_create"
    parameters:
      yaml: |
        name: test-new-feature
        # ... rest of YAML
    expected:
      success: true
      contains: ["created successfully"]
```

#### 2. Validate Scenario Syntax
```json
{
  "tool": "mcp_envctl-test_test_validate_scenario",
  "parameters": {
    "scenario_path": "internal/testing/scenarios/my-new-test.yaml"
  }
}
```

#### 3. Test New Scenario
```json
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "scenario": "my-new-feature-test",
    "verbose": true
  }
}
```

#### 4. Iterate Based on Results
```json
{
  "tool": "mcp_envctl-test_test_get_results",
  "parameters": {
    "random_string": "check_results"
  }
}
```

### Debugging Workflow

#### 1. Identify Failing Tests
```json
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "concept": "serviceclass",
    "fail_fast": true,
    "verbose": true
  }
}
```

#### 2. Analyze Detailed Results
```json
{
  "tool": "mcp_envctl-test_test_get_results",
  "parameters": {
    "random_string": "debug_analysis"
  }
}
```

#### 3. Test Single Failing Scenario
```json
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "scenario": "specific-failing-scenario",
    "verbose": true
  }
}
```

#### 4. Validate Environment
```bash
# Check envctl status
journalctl --user -u envctl.service --no-pager | tail -50

# Restart if needed
./scripts/dev-restart.sh
```

## Best Practices

### Test Execution Strategies

#### 1. **Layered Testing Approach**
```json
// Start with behavioral tests
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "category": "behavioral",
    "fail_fast": true
  }
}

// Then run integration tests
{
  "tool": "mcp_envctl-test_test_run_scenarios", 
  "parameters": {
    "category": "integration",
    "parallel": 2
  }
}
```

#### 2. **Concept-Driven Development**
```json
// Test the concept you're actively developing
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "concept": "workflow",
    "verbose": true
  }
}
```

#### 3. **Fast Feedback with Fail-Fast**
```json
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "fail_fast": true,
    "parallel": 4,
    "verbose": true
  }
}
```

### Scenario Organization Patterns

#### 1. **Naming Conventions**
- Use descriptive names: `serviceclass-crud-operations`
- Include complexity level: `workflow-basic-parameter-templating`
- Group by functionality: `mcpserver-connection-management`

#### 2. **Category Usage**
- **`behavioral`**: User-facing functionality, API contracts, expected behaviors
- **`integration`**: Component interactions, end-to-end workflows, system integration

#### 3. **Concept Organization**
- Group related functionality together
- Use tags for cross-cutting concerns
- Maintain clear dependency hierarchies

#### 4. **Test Isolation**
- Ensure each scenario can run independently
- Include proper cleanup steps
- Use unique resource names

### Error Handling and Recovery

#### 1. **Graceful Failure Handling**
```json
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "fail_fast": false,  // Continue execution
    "verbose": true      // Get detailed error info
  }
}
```

#### 2. **Result Analysis Pattern**
```json
// Always check results after execution
{
  "tool": "mcp_envctl-test_test_get_results",
  "parameters": {
    "random_string": "post_execution_check"
  }
}
```

#### 3. **Validation Before Execution**
```json
// Validate scenarios before running
{
  "tool": "mcp_envctl-test_test_validate_scenario",
  "parameters": {
    "scenario_path": "internal/testing/scenarios/"
  }
}
```

## Troubleshooting

### Common Issues and Solutions

#### 1. **Connection Refused**
**Symptoms**: 
```json
{
  "error": "connection refused",
  "tool": "mcp_envctl-test_test_run_scenarios"
}
```

**Solutions**:
```bash
# Check if envctl aggregator is running
curl http://localhost:8080/health

# Restart envctl service
./scripts/dev-restart.sh

# Check service status
systemctl --user status envctl.service --no-pager

# Check service logs
journalctl --user -u envctl.service --no-pager | tail -50
```

#### 2. **Tool Not Found**
**Symptoms**:
```json
{
  "error": "tool not found: core_serviceclass_create",
  "scenario": "serviceclass-basic-operations"
}
```

**Investigation Steps**:
```bash
# Check available tools via aggregator
curl http://localhost:8080/tools

# Check envctl logs for registration errors
journalctl --user -u envctl.service --no-pager | grep -i "register\|tool"

# Restart to re-register tools
./scripts/dev-restart.sh
```

**Source Code Investigation**:
- Check `internal/serviceclass/` package for implementation issues
- Verify `api_adapter.go` files for tool registration
- Look at `internal/api/handlers.go` for interface definitions

#### 3. **Scenario Validation Errors**
**Symptoms**:
```json
{
  "tool": "mcp_envctl-test_test_validate_scenario",
  "result": {
    "valid": false,
    "errors": ["invalid YAML syntax at line 15"]
  }
}
```

**Solutions**:
```bash
# Validate YAML syntax manually
yamllint internal/testing/scenarios/problem-scenario.yaml

# Check against scenario schema
# Review existing working scenarios for reference patterns
```

#### 4. **Test Timeouts**
**Symptoms**:
```json
{
  "error": "scenario timeout exceeded",
  "scenario": "slow-integration-test"
}
```

**Solutions**:
```json
// Increase timeout for specific scenarios
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "scenario": "slow-integration-test",
    "timeout": "10m"
  }
}
```

```yaml
# Or update scenario YAML
timeout: "10m"
steps:
  - name: "slow-operation"
    timeout: "5m"
    # ...
```

#### 5. **Resource Conflicts**
**Symptoms**:
```json
{
  "error": "resource already exists: test-serviceclass",
  "step": "create-test-serviceclass"
}
```

**Solutions**:
```bash
# Clean up resources manually
kubectl delete serviceclass test-serviceclass

# Or use unique names in tests
```

```yaml
# Update scenario to use unique names
parameters:
  yaml: |
    name: test-serviceclass-{{ .timestamp }}
```

### Debugging Techniques

#### 1. **Understanding Package Structure for Test Failures**

When a test scenario fails, the issue is typically in the source code package related to the concept being tested:

| **Concept** | **Primary Package** | **Key Files** |
|-------------|-------------------|---------------|
| `serviceclass` | `internal/serviceclass/` | `manager.go`, `api_adapter.go` |
| `workflow` | `internal/workflow/` | `executor.go`, `manager.go` |
| `mcpserver` | `internal/mcpserver/` | `manager.go`, `client.go` |
| `capability` | `internal/capability/` | `manager.go`, `registry.go` |
| `service` | `internal/services/` | `registry.go`, `instance.go` |

**Investigation Process**:
1. Identify which concept the failing test belongs to
2. Check the corresponding package's implementation
3. Look for recent changes in API adapters
4. Verify tool registration in `internal/api/` handlers

#### 2. **Using envctl Restart Script**

```bash
# Restart envctl with your changes
./scripts/dev-restart.sh

# This script:
# 1. Stops the current envctl service
# 2. Rebuilds envctl with latest code changes  
# 3. Restarts the service
# 4. Waits for it to be ready
```

**When to Use**:
- After making code changes
- When tools are not registering properly
- When experiencing connection issues
- Before running test scenarios

#### 3. **Analyzing envctl Logs**

```bash
# Get recent logs
journalctl --user -u envctl.service --no-pager | tail -50

# Filter for specific components
journalctl --user -u envctl.service --no-pager | grep -i "serviceclass"

# Follow logs in real-time during testing
journalctl --user -u envctl.service --no-pager -f
```

**Key Log Patterns to Look For**:
- Tool registration: `"registered tool: core_serviceclass_create"`
- Connection issues: `"failed to connect"`, `"connection refused"`
- API errors: `"handler error"`, `"failed to process"`
- Resource conflicts: `"already exists"`, `"conflict"`

#### 4. **Systematic Debugging Steps**

1. **Verify Environment**:
   ```bash
   # Check if envctl is running
   systemctl --user status envctl.service --no-pager
   
   # Test basic connectivity
   curl http://localhost:8080/health
   ```

2. **Isolate the Problem**:
   ```json
   // Run just the failing scenario
   {
     "tool": "mcp_envctl-test_test_run_scenarios",
     "parameters": {
       "scenario": "failing-scenario-name",
       "verbose": true
     }
   }
   ```

3. **Check Tool Availability**:
   ```bash
   # List all available tools (when mcp-debug is available)
   # This helps identify missing or misconfigured tools
   ```

4. **Analyze Results**:
   ```json
   {
     "tool": "mcp_envctl-test_test_get_results", 
     "parameters": {
       "random_string": "detailed_analysis"
     }
   }
   ```

5. **Fix and Verify**:
   ```bash
   # Make necessary code changes
   # Restart envctl
   ./scripts/dev-restart.sh
   
   # Re-run the test
   ```

### Advanced Troubleshooting

#### 1. **MCP Protocol Issues**
```bash
# Check MCP aggregator status
curl http://localhost:8080/mcp/status

# Verify tool registration
curl http://localhost:8080/mcp/tools
```

#### 2. **Parallel Execution Issues**
```json
// Reduce parallelism to isolate race conditions
{
  "tool": "mcp_envctl-test_test_run_scenarios",
  "parameters": {
    "parallel": 1,
    "verbose": true
  }
}
```

#### 3. **Resource Cleanup Issues**
```bash
# Manual cleanup of test resources
kubectl get all -A | grep test-

# Check for orphaned resources
kubectl get serviceclass,workflow,capability -A
```

## Integration Examples

### IDE Integration with MCP

#### VS Code with MCP Extension
```json
// Configure MCP connection to envctl
{
  "mcp.servers": {
    "envctl-test": {
      "command": "envctl",
      "args": ["serve", "--mcp"],
      "env": {}
    }
  }
}
```

#### Cursor with Built-in MCP
```typescript
// Use MCP tools directly in Cursor
const testResult = await mcp.callTool("mcp_envctl-test_test_run_scenarios", {
  concept: "serviceclass",
  verbose: true
});
```

### CI/CD Pipeline Integration

#### GitHub Actions
```yaml
name: envctl Tests via MCP
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Start envctl
        run: |
          envctl serve &
          sleep 10
        
      - name: Run Tests via MCP
        run: |
          # Use MCP client to execute tests
          mcp-client call mcp_envctl-test_test_run_scenarios \
            '{"category": "behavioral", "parallel": 4}'
```

#### Jenkins Pipeline
```groovy
pipeline {
    agent any
    stages {
        stage('Test') {
            steps {
                sh 'envctl serve &'
                sh 'sleep 10'
                sh '''
                    mcp-client call mcp_envctl-test_test_run_scenarios \
                        '{"category": "behavioral", "fail_fast": true}'
                '''
            }
        }
    }
}
```

## Summary

The envctl MCP testing framework provides a powerful, standardized way to execute comprehensive tests through AI-powered workflows. Key takeaways:

- **Four Core Tools**: `test_run_scenarios`, `test_list_scenarios`, `test_validate_scenario`, `test_get_results`
- **Flexible Filtering**: By category, concept, or specific scenario
- **Parallel Execution**: Configurable worker pools for faster testing
- **Comprehensive Results**: Detailed execution reports with step-by-step analysis
- **Integration Ready**: Works with IDEs, CI/CD pipelines, and LLM agents

**Best Practices Summary**:
1. Always restart envctl after code changes: `./scripts/dev-restart.sh`
2. Use fail-fast for quick feedback during development
3. Check logs for debugging: `journalctl --user -u envctl.service --no-pager`
4. Validate scenarios before execution
5. Analyze detailed results for troubleshooting
6. Test concept-specific functionality during development
7. Use parallel execution for comprehensive test suites

This MCP-based testing approach enables seamless integration between envctl development and AI-powered development workflows, providing both automated validation and intelligent debugging capabilities. 