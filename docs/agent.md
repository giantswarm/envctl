# Agent Command

The `envctl agent` command acts as an MCP (Model Context Protocol) client to debug and test the aggregator's SSE server. It connects to the aggregator endpoint, logs all JSON-RPC communication, and demonstrates dynamic tool updates.

## Usage

```bash
envctl agent [--endpoint <sse-endpoint>] [--timeout <duration>] [--verbose] [--no-color]
             [--json-rpc] [--repl] [--mcp-server]
```

## Modes

The agent command can run in three different modes:

### 1. Normal Mode (default)
Connects to the aggregator, lists tools, and waits for notifications about tool changes.

```bash
# Basic usage
envctl agent

# With custom endpoint
envctl agent --endpoint http://localhost:8090/sse

# With verbose logging
envctl agent --verbose --json-rpc
```

### 2. REPL Mode
Provides an interactive interface to explore and execute tools.

```bash
# Start REPL
envctl agent --repl

# REPL with custom endpoint
envctl agent --repl --endpoint http://localhost:8090/sse
```

### 3. MCP Server Mode
Runs as an MCP server exposing all REPL functionality via stdio transport for AI assistant integration.

```bash
# Start MCP server
envctl agent --mcp-server

# MCP server with custom endpoint
envctl agent --mcp-server --endpoint http://localhost:8090/sse
```

## Options

- `--endpoint`: SSE endpoint URL (default: from config or http://localhost:8080/sse)
- `--timeout`: Timeout for waiting for notifications (default: 5m)
- `--verbose`: Enable verbose logging (show keepalive messages)
- `--no-color`: Disable colored output
- `--json-rpc`: Enable full JSON-RPC message logging
- `--repl`: Start interactive REPL mode
- `--mcp-server`: Run as MCP server (stdio transport)

Note: The `--repl` and `--mcp-server` flags are mutually exclusive.

## Examples

### Debugging Tool Changes

```bash
# Watch for tool changes with verbose logging
envctl agent --verbose --json-rpc --timeout 10m
```

### Interactive Tool Exploration

```bash
# Start REPL to explore tools
envctl agent --repl

# In REPL:
MCP> list tools
MCP> describe tool x:kubernetes:get_pods
MCP> call x:kubernetes:get_pods {"namespace": "default"}
```

### AI Assistant Integration

Configure in your AI assistant's MCP settings:

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

## Configuration

By default, the agent connects to the aggregator endpoint configured in your envctl configuration file. The endpoint is constructed as:

```
http://<aggregator.host>:<aggregator.port>/sse
```

You can override this with the `--endpoint` flag. 