# ServiceClass Debugging Workflow

This document provides a systematic approach to debugging ServiceClass implementations using `mcp-debug`, with real examples from debugging the `portforward` ServiceClass.

## 🔍 **Overview**

ServiceClasses in envctl provide a standardized way to manage infrastructure services (like port forwarding, monitoring, etc.). When issues arise, this workflow helps identify and resolve problems systematically.

## 📋 **Phase 1: System Health Verification**

### **1.1 Check Overall System Health**
```bash
# Via mcp-debug
core_service_list
```

**Expected Output:**
- All MCP servers should show `"health": "Healthy"` and `"state": "Running"`
- Any failed services will show `"health": "Unhealthy"` and `"state": "Failed"`

**Example Issue Found:**
```json
{
  "label": "kubernetes",
  "service_type": "MCPServer", 
  "state": "Failed",
  "health": "Unhealthy",
  "error": "failed to initialize MCP client for kubernetes: failed to initialize MCP protocol: transport error: context deadline exceeded"
}
```

### **1.2 Verify MCP Server Registration**
```bash
# Check if all expected MCP servers are registered
core_mcp_server_list
```

### **1.3 Check ServiceClass Availability**
```bash
# List all ServiceClasses
core_serviceclass_list
```

**Key Fields to Check:**
- `available`: Should be `true` for working ServiceClasses
- `requiredTools`: Lists the tools this ServiceClass depends on
- `missingTools`: Shows which required tools are unavailable

**Example Issue:**
```json
{
  "name": "portforward",
  "available": false,
  "requiredTools": ["x_kubernetes_port_forward", "x_kubernetes_stop_port_forward"],
  "missingTools": ["x_kubernetes_port_forward", "x_kubernetes_stop_port_forward"]
}
```

## 🔧 **Phase 2: ServiceClass-Specific Testing**

### **2.1 Test ServiceClass Availability**
```bash
# Check if a specific ServiceClass is available
core_serviceclass_available --name=portforward
```

### **2.2 Get ServiceClass Details**
```bash
# Get detailed information about the ServiceClass
core_serviceclass_get --name=portforward
```

### **2.3 Test ServiceClass Instantiation**
```bash
# Try to create an instance (will fail gracefully if dependencies aren't met)
core_serviceclass_instance_create \
  --serviceClassName=portforward \
  --label=test-debug \
  --parameters='{"namespace":"default","resource_type":"service","resource_name":"test","local_port":8080,"remote_port":80}'
```

**Expected Behaviors:**
- ✅ **Success**: Instance created and tools called correctly
- ⚠️ **Graceful Failure**: Clear error showing why it failed (e.g., "pod not found")
- ❌ **System Error**: Tool not found, parameter conversion issues, etc.

## 🛠️ **Phase 3: Dependency Verification**

### **3.1 Check Required Tools Directly**
```bash
# Test if the underlying tools work
x_kubernetes_port_forward \
  --resourceType=service \
  --resourceName=coredns \
  --namespace=kube-system \
  --localPort=9053 \
  --targetPort=53
```

### **3.2 Test Kubernetes Connectivity**
```bash
# Basic connectivity test
x_kubernetes_kubectl_get --resourceType=namespaces
```

### **3.3 Find Suitable Test Resources**
```bash
# Find resources to test with
x_kubernetes_kubectl_get --resourceType=services --namespace=kube-system
x_kubernetes_kubectl_describe --resourceType=service --name=coredns --namespace=kube-system
```

## 🔄 **Phase 4: Service Recovery**

### **4.1 Restart Failed Services**
```bash
# Restart a specific MCP server
core_service_restart --label=kubernetes
```

### **4.2 Check Service Status**
```bash
# Get detailed status of a specific service
core_service_status --label=kubernetes
```

### **4.3 Verify Recovery**
```bash
# Re-run the ServiceClass availability check
core_serviceclass_available --name=portforward
```

## 📊 **Phase 5: Instance Management**

### **5.1 List Active Instances**
```bash
# See what ServiceClass instances are currently running
core_serviceclass_instance_list
```

### **5.2 Clean Up Test Instances**
```bash
# Remove test instances
core_serviceclass_instance_delete --instanceId=test-debug
```

## 🚨 **Common Issues & Solutions**

### **Issue 1: ServiceClass Shows as "Not Available"**
**Symptoms:**
- `available: false` in ServiceClass list
- `missingTools` array shows required tools

**Root Cause:** Dependent MCP server is not running or healthy

**Solution:**
1. Check `core_service_list` for failed services
2. Restart the failed service: `core_service_restart --label=<service>`
3. If restart fails, check logs and investigate the underlying MCP server

### **Issue 2: Parameter Type Conversion Errors**
**Symptoms:**
- Numeric parameters passed as strings with decimals (e.g., "8053.000000")
- Tools reject malformed parameters

**Root Cause:** ServiceClass parameter template conversion issues

**Solution:**
1. Check ServiceClass definition with `core_serviceclass_get`
2. Ensure parameter types match expected tool inputs
3. Test with string parameters when numeric conversion fails

### **Issue 3: "Unsupported Content Type" Errors**
**Symptoms:**
- Tool executes but returns content type errors
- Port forwarding appears to fail despite successful setup

**Root Cause:** Often this is a false positive - the operation may have succeeded

**Solution:**
1. Check if the service is actually running: `core_service_list`
2. Test the forwarded port externally
3. Check `core_serviceclass_instance_list` for active instances

### **Issue 4: Resource Not Found**
**Symptoms:**
- "pod not found", "service not found" errors
- ServiceClass instantiation fails

**Root Cause:** Testing with non-existent Kubernetes resources

**Solution:**
1. Use `x_kubernetes_kubectl_get` to find available resources
2. Use `x_kubernetes_kubectl_describe` to get resource details
3. Test with known system resources (e.g., `coredns` in `kube-system`)

## 🎯 **Best Practices**

### **Testing Strategy**
1. **Start with System Resources**: Use well-known system services like `coredns` for initial testing
2. **Use Correct Ports**: Check service definitions for available ports and their names
3. **Test Incrementally**: Start with basic connectivity, then move to complex operations
4. **Clean Up**: Always remove test instances after debugging

### **Parameter Guidelines**
- Use string values for ports when numeric conversion fails
- Check service/pod definitions for correct port names vs numbers
- Prefer targetPort over port for services when available

### **Error Interpretation**
- **"Tool not found"** = MCP server issue
- **"Resource not found"** = Kubernetes connectivity/resources issue  
- **"Parameter error"** = ServiceClass definition issue
- **"Content type error"** = Often a false positive, check if operation succeeded

## 📝 **Debugging Checklist**

- [ ] All MCP servers healthy (`core_service_list`)
- [ ] ServiceClass shows as available (`core_serviceclass_available`)
- [ ] Required tools present (no `missingTools`)
- [ ] Test resources exist in cluster
- [ ] Parameter types match tool expectations
- [ ] Network connectivity to cluster working
- [ ] Test instances cleaned up after debugging

## 🔧 **Example: Complete portforward Debug Session**

```bash
# 1. Check system health
core_service_list
# Found: kubernetes service failed

# 2. Check ServiceClass status  
core_serviceclass_list
# Found: portforward not available, missing tools

# 3. Restart failed service
core_service_restart --label=kubernetes
# Result: Still failing with timeout

# 4. Check what resources are available for testing
x_kubernetes_kubectl_get --resourceType=services --namespace=kube-system
# Found: coredns service available

# 5. Test ServiceClass with real resource
core_serviceclass_instance_create \
  --serviceClassName=portforward \
  --label=test-coredns \
  --parameters='{"namespace":"kube-system","resource_type":"service","resource_name":"coredns","local_port":"9053","remote_port":"53"}'
# Result: Would work if kubernetes service was healthy
```

## ✅ **Resolved Issues**

### **Issue 5: Kubernetes MCP Server JSON Parsing (RESOLVED)**
**Previous Symptoms:**
- `tool call failed: failed to unmarshal response: unexpected end of JSON input`
- ServiceClass registration works but tool execution fails

**Status:** ✅ **FIXED** - All Kubernetes MCP tools now work correctly
- ✅ Basic operations: `x_kubernetes_context_get_current` 
- ✅ Complex operations: `x_kubernetes_list`
- ✅ Port forwarding: `x_kubernetes_port_forward` (with proper error handling)
- ✅ ServiceClass infrastructure: Full end-to-end functionality

**Remaining:** Minor template processing issues in some ServiceClass definitions

This workflow provides a systematic approach to identifying and resolving ServiceClass issues, ensuring that both the infrastructure and the ServiceClass implementations work correctly together. 