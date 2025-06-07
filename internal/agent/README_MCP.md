# Envctl Agent MCP Server Mode

The Envctl Agent MCP Server mode exposes all REPL functionality through the Model Context Protocol (MCP), allowing AI assistants to interact with envctl programmatically.

## Overview

The MCP server mode is part of `envctl agent` and acts as a bridge between AI assistants and the envctl aggregator, providing access to all tools, resources, and prompts available in the aggregator through a standardized MCP interface.

## Architecture

```
AI Assistant (MCP Client) --[stdio]--> envctl agent --mcp-server --[SSE/HTTP]--> Aggregator ---> Backend MCP Servers
```

Key points:
- The MCP server mode uses **stdio transport** (not network sockets)
- AI assistants communicate with envctl through standard input/output
- The agent connects to the Aggregator as a client via SSE/HTTP
- The `--endpoint` parameter configures where the agent connects to

## Available Tools

The MCP server mode exposes the following tools:

### Discovery Tools
- **list_tools** - List all available tools from connected MCP servers
- **list_resources** - List all available resources from connected MCP servers  
- **list_prompts** - List all available prompts from connected MCP servers

### Information Tools
- **describe_tool** - Get detailed information about a specific tool
  - Parameters: `name` (string, required)
- **describe_resource** - Get detailed information about a specific resource
  - Parameters: `uri` (string, required)
- **describe_prompt** - Get detailed information about a specific prompt
  - Parameters: `name` (string, required)

### Execution Tools
- **call_tool** - Execute a tool with the given arguments
  - Parameters: `name` (string, required), `arguments` (object, optional)
- **get_resource** - Retrieve the contents of a resource
  - Parameters: `uri` (string, required)
- **get_prompt** - Get a prompt with the given arguments
  - Parameters: `name` (string, required), `arguments` (object, optional)

## Usage

### Running the MCP Server

```bash
# Connect to aggregator at default location (from config or localhost:8080)
envctl agent --mcp-server

# Connect to aggregator with custom endpoint
envctl agent --mcp-server --endpoint http://localhost:8090/sse

# Enable verbose logging
envctl agent --mcp-server --verbose

# Disable colored output
envctl agent --mcp-server --no-color

# Enable JSON-RPC logging
envctl agent --mcp-server --json-rpc
```

### Command Line Options

When using `--mcp-server` mode, these options are available:

- `--endpoint` - Full aggregator SSE endpoint URL (default: from config)
- `--verbose` - Enable verbose logging
- `--no-color` - Disable colored output
- `--json-rpc` - Enable JSON-RPC logging

Note: The `--repl` and `--mcp-server` flags are mutually exclusive.

### Configuration for AI Assistants

To use this MCP server with AI assistants like Claude or Cursor, add it to their MCP configuration:

```json
{
  "mcpServers": {
    "envctl": {
      "command": "/path/to/envctl",
      "args": ["agent", "--mcp-server"]
    }
  }
}
```

Or with a custom endpoint:

```json
{
  "mcpServers": {
    "envctl": {
      "command": "/path/to/envctl",
      "args": ["agent", "--mcp-server", "--endpoint", "http://myserver:8090/sse"]
    }
  }
}
```

**Note**: The MCP server mode doesn't listen on any network port - it communicates via stdio. The endpoint parameter is for connecting to the envctl aggregator.

### Example Interactions

Here are some example prompts you can use with AI assistants:

1. **List available tools:**
   ```
   "Use the envctl server to list all available tools"
   ```

2. **Get information about a specific tool:**
   ```
   "Describe the 'x:kubernetes:get_pods' tool using envctl"
   ```

3. **Execute a tool:**
   ```
   "Use envctl to call the 'x:kubernetes:get_pods' tool with namespace 'default'"
   ```

4. **Retrieve a resource:**
   ```
   "Get the 'config://kubeconfig' resource using envctl"
   ```

## Integration with Envctl

The MCP server mode integrates seamlessly with the existing envctl ecosystem:

1. It connects to the aggregator using the same SSE protocol as the REPL
2. It maintains synchronized caches of tools, resources, and prompts
3. It forwards all operations to the aggregator, which then routes them to the appropriate backend MCP servers

## Programmatic Usage

You can also use the MCP server programmatically in your Go applications:

```go
package main

import (
    "context"
    "log"
    "envctl/internal/agent"
)

func main() {
    // Create logger
    logger := agent.NewLogger(true, true, false)
    
    // Create MCP server (connects to aggregator at specified endpoint)
    server, err := agent.NewMCPServer("http://localhost:8080/sse", logger, false)
    if err != nil {
        log.Fatal(err)
    }
    
    // Start the server (stdio transport)
    ctx := context.Background()
    if err := server.Start(ctx); err != nil {
        log.Fatal(err)
    }
}
```

## Error Handling

The MCP server provides detailed error messages for common issues:

- Connection failures to the aggregator
- Tool/resource/prompt not found errors
- Invalid arguments or parameters
- Execution failures from backend servers

All errors are returned as MCP error results with descriptive messages.

## Debugging

Enable verbose logging to see detailed information about:
- Connection establishment to the aggregator
- MCP protocol handshake
- Tool/resource/prompt discovery
- Request/response details
- Error conditions

```bash
envctl agent --mcp-server --verbose --json-rpc
```

## Security Considerations

The MCP server:
- Connects only to the specified aggregator endpoint
- Does not expose any network services (uses stdio transport)
- Inherits all security features from the aggregator (tool denylists, etc.)
- Runs with the permissions of the user executing it 