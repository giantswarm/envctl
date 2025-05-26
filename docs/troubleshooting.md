# envctl Troubleshooting Guide

## Table of Contents
1. [Common Issues](#common-issues)
2. [Debugging Techniques](#debugging-techniques)
3. [Service-Specific Issues](#service-specific-issues)
4. [Dependency Issues](#dependency-issues)
5. [Performance Issues](#performance-issues)
6. [Recovery Procedures](#recovery-procedures)

## Common Issues

### 1. Port Already in Use

**Symptom:**
```
Error: listen tcp 127.0.0.1:8080: bind: address already in use
```

**Causes:**
- Another process is using the port
- Previous envctl instance didn't shut down cleanly
- Service was restarted too quickly

**Solutions:**
1. Find the process using the port:
   ```bash
   lsof -i :8080  # macOS/Linux
   netstat -ano | findstr :8080  # Windows
   ```

2. Kill the process or choose a different port in configuration

3. Wait a moment before restarting (envctl adds a 1-second delay automatically)

### 2. Service Won't Start

**Symptom:**
- Service stays in "Starting" state
- Service immediately goes to "Failed" state

**Common Causes:**
- Missing dependencies
- Configuration errors
- Insufficient permissions
- Missing executables (for MCP servers)

**Debugging Steps:**
1. Check the logs panel in TUI (press 'L')
2. Look for error messages in the service output
3. Verify all dependencies are running
4. Check configuration file syntax

### 3. Cascade Stops

**Symptom:**
- Multiple services stop when one service fails
- Services marked as "Stopped (dependency)"

**Understanding:**
This is expected behavior. envctl maintains dependency relationships:
- When a service fails, all dependent services stop
- This prevents services from running without required dependencies

**Recovery:**
- Fix the root cause service
- Restart the failed service - dependents will restart automatically

### 4. Services Keep Restarting

**Symptom:**
- Service enters restart loop
- Rapid Starting → Running → Failed cycle

**Causes:**
- Health check failures
- Configuration pointing to non-existent resources
- Network connectivity issues

**Solutions:**
1. Check health check logs
2. Verify Kubernetes resources exist
3. Test network connectivity manually
4. Temporarily disable health checks for debugging

## Debugging Techniques

### 1. Enable Debug Logging

Set the log level before starting envctl:
```bash
export LOG_LEVEL=DEBUG
envctl connect <mc> <wc>
```

### 2. View Service Logs

In the TUI:
- Press 'L' to toggle log view
- Use arrow keys to select different services
- Press 'c' to clear logs

### 3. Check Service Dependencies

In the TUI:
- Press 'd' to view dependency graph
- Look for broken dependency chains
- Verify all required services are configured

### 4. Manual Service Testing

Test services outside envctl:

**Port Forward:**
```bash
kubectl port-forward -n <namespace> service/<service-name> <local>:<remote>
```

**MCP Server:**
```bash
# Test if MCP server executable exists
which mcp-server-prometheus

# Test MCP server manually
mcp-server-prometheus --prometheus-url http://localhost:8080
```

### 5. State Inspection

Check service states in the TUI:
- Green: Running
- Yellow: Starting/Stopping
- Red: Failed/Stopped
- Gray: Stopped (dependency)

## Service-Specific Issues

### K8s Connection Issues

**Cannot connect to cluster:**
1. Verify kubeconfig:
   ```bash
   kubectl config current-context
   kubectl get nodes
   ```

2. Check Teleport authentication:
   ```bash
   tsh status
   tsh kube ls
   ```

3. Ensure context names match expected format:
   - MC: `teleport.giantswarm.io-<cluster-name>`
   - WC: `teleport.giantswarm.io-<mc-name>-<wc-name>`

### Port Forward Issues

**Port forward fails immediately:**
1. Check if the service exists:
   ```bash
   kubectl get svc -n <namespace> <service-name>
   ```

2. Verify pods are running:
   ```bash
   kubectl get pods -n <namespace> -l <selector>
   ```

3. Check service endpoints:
   ```bash
   kubectl get endpoints -n <namespace> <service-name>
   ```

**Port forward drops connection:**
- Network instability
- Pod restarts
- Resource limits exceeded

### MCP Server Issues

**MCP server not found:**
1. Verify executable is in PATH:
   ```bash
   echo $PATH
   which <mcp-server-name>
   ```

2. Install missing MCP servers:
   ```bash
   # Example for Node.js based servers
   npm install -g @modelcontextprotocol/server-prometheus
   ```

**MCP server health check fails:**
1. Check if server is actually running:
   ```bash
   ps aux | grep mcp
   ```

2. Test health endpoint manually:
   ```bash
   curl -X POST http://localhost:<proxy-port> \
     -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","method":"tools/list","params":{},"id":1}'
   ```

## Dependency Issues

### Understanding Dependency Failures

When a service fails, envctl tracks why dependent services stopped:
- **StopReasonManual**: User explicitly stopped the service
- **StopReasonDependency**: Service stopped due to dependency failure

### Dependency Resolution

1. **Identify the root cause:**
   - Look for the first service that failed
   - Check its logs for error details

2. **Fix in order:**
   - Resolve issues with Level 0 services first (K8s connections)
   - Then Level 1 (Port forwards)
   - Finally Level 2+ (MCP servers)

3. **Automatic recovery:**
   - When a dependency is fixed, dependent services restart automatically
   - No need to manually restart the entire chain

### Common Dependency Patterns

```
K8s Connection Failed
├── All port forwards using that connection stop
└── All MCP servers depending on those port forwards stop

Port Forward Failed
└── All MCP servers depending on it stop

MCP Server Failed
└── No cascade (MCP servers don't have dependents)
```

## Performance Issues

### High CPU Usage

**Causes:**
- Too frequent health checks
- Large log buffers
- Inefficient MCP server implementation

**Solutions:**
1. Increase health check intervals in config
2. Clear logs regularly (press 'c' in log view)
3. Profile MCP server performance

### Memory Leaks

**Symptoms:**
- Increasing memory usage over time
- System becomes sluggish

**Debugging:**
1. Monitor process memory:
   ```bash
   top -p $(pgrep envctl)
   ```

2. Check for goroutine leaks:
   - Enable pprof in development builds
   - Look for increasing goroutine count

### Slow Startup

**Causes:**
- Many services to start
- Slow Kubernetes API responses
- Network latency

**Optimizations:**
1. Disable unused services in configuration
2. Use local Kubernetes contexts when possible
3. Reduce health check frequency during startup

## Recovery Procedures

### 1. Complete Reset

If envctl is in an inconsistent state:

1. Stop envctl (Ctrl+C or 'q' in TUI)
2. Kill any remaining processes:
   ```bash
   pkill -f "mcp-server"
   pkill -f "kubectl port-forward"
   ```
3. Clear temporary files:
   ```bash
   rm -rf /tmp/envctl-*
   ```
4. Restart envctl

### 2. Partial Recovery

To recover specific services:

1. In TUI, select the failed service
2. Press 'r' to restart
3. Dependencies will be checked and started if needed

### 3. Configuration Recovery

If configuration is corrupted:

1. Back up current config:
   ```bash
   cp ~/.config/envctl/config.yaml ~/.config/envctl/config.yaml.bak
   ```

2. Reset to defaults:
   ```bash
   rm ~/.config/envctl/config.yaml
   ```

3. envctl will use built-in defaults on next start

### 4. Emergency Stop

If services are misbehaving:

1. Press 'q' in TUI for graceful shutdown
2. If that fails, use Ctrl+C
3. If still running, force kill:
   ```bash
   kill -9 $(pgrep envctl)
   ```

## Getting Help

### Collect Diagnostic Information

When reporting issues, include:

1. **Version information:**
   ```bash
   envctl version
   ```

2. **Configuration:**
   ```bash
   cat ~/.config/envctl/config.yaml
   ```

3. **Debug logs:**
   ```bash
   LOG_LEVEL=DEBUG envctl connect <mc> <wc> 2>&1 | tee envctl.log
   ```

4. **System information:**
   ```bash
   uname -a
   kubectl version
   docker version  # if using containerized MCPs
   ```

### Log Analysis

Key log patterns to look for:

- `[ERROR]` - Critical failures
- `[WARN]` - Potential issues
- `Failed to` - Operation failures
- `correlationID` - Track related operations
- `cascade` - Dependency-related stops

### Common Error Messages

| Error | Meaning | Solution |
|-------|---------|----------|
| `context not found` | Kubernetes context doesn't exist | Check kubeconfig |
| `no such host` | DNS resolution failed | Check network/DNS |
| `connection refused` | Service not listening | Check if service is running |
| `timeout` | Operation took too long | Check network latency |
| `permission denied` | Insufficient privileges | Check RBAC/file permissions |
| `executable file not found` | MCP server not installed | Install the MCP server |

## Best Practices

1. **Regular Health Monitoring:**
   - Watch for yellow/red services in TUI
   - Check logs periodically
   - Monitor system resources

2. **Graceful Shutdown:**
   - Always use 'q' to quit
   - Allow services time to stop
   - Verify all processes terminated

3. **Configuration Management:**
   - Keep configuration in version control
   - Document custom settings
   - Test changes in development first

4. **Dependency Awareness:**
   - Understand service dependencies
   - Start with core services first
   - Fix root causes, not symptoms 