# Agent Command

The `envctl agent` command acts as an MCP (Model Context Protocol) client to debug and test the aggregator's SSE server. It connects to the aggregator endpoint, logs all JSON-RPC communication, and demonstrates the dynamic tool update mechanism.

## Usage

```bash
envctl agent [--endpoint <sse-endpoint>] [--timeout <duration>] [--verbose] [--no-color]
```

## Flags

- `--endpoint`: Override the default SSE endpoint URL (default: from config, typically `http://localhost:8090/sse`)
- `--timeout`: How long to wait for notifications before exiting (default: 5 minutes)
- `--verbose`: Enable verbose logging (shows keepalive messages and other debug info)
- `--no-color`: Disable colored output
- `--json-rpc`: Enable full JSON-RPC message logging (shows complete request/response bodies)

## Default Behavior

1. Connects to the MCP aggregator using the endpoint from your envctl configuration
2. Initializes an MCP session as a client
3. Lists all available tools
4. Waits for `tools/list_changed` notifications
5. When tools change, lists them again and shows the differences

## Example Output

### Default Mode (Simple)

```
[2024-01-10 15:30:01] Connecting to MCP aggregator at http://localhost:8090/sse...
[2024-01-10 15:30:01] Initializing MCP session...
[2024-01-10 15:30:01] Session initialized successfully (protocol: 2025-03-26)
[2024-01-10 15:30:01] Listing available tools...
[2024-01-10 15:30:01] Found 45 tools
[2024-01-10 15:30:01] Waiting for notifications (press Ctrl+C to exit)...

[2024-01-10 15:30:15] Tools list changed! Fetching updated list...
[2024-01-10 15:30:15] Listing available tools...
[2024-01-10 15:30:15] Found 46 tools
[2024-01-10 15:30:15] Tool changes detected:
  ✓ Unchanged: add_activity_to_incident
  ✓ Unchanged: cleanup
  ✓ Unchanged: create_incident
  ... (43 more unchanged)
  + Added: prometheus.query
```

### JSON-RPC Mode (with --json-rpc flag)

```
[2024-01-10 15:30:01] Connecting to MCP aggregator at http://localhost:8090/sse...
[2024-01-10 15:30:01] → REQUEST (initialize):
{
  "jsonrpc": "2.0",
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "clientInfo": {
      "name": "envctl-agent",
      "version": "1.0.0"
    },
    "capabilities": {}
  },
  "id": 1
}

[2024-01-10 15:30:01] ← RESPONSE (initialize):
{
  "jsonrpc": "2.0",
  "result": {
    "protocolVersion": "2025-03-26",
    "serverInfo": {
      "name": "envctl-aggregator",
      "version": "1.0.0"
    },
    "capabilities": {
      "tools": {
        "listChanged": true
      }
    }
  },
  "id": 1
}

[2024-01-10 15:30:01] → REQUEST (tools/list):
{
  "jsonrpc": "2.0",
  "method": "tools/list",
  "params": {},
  "id": 2
}

[2024-01-10 15:30:01] ← RESPONSE (tools/list):
{
  "jsonrpc": "2.0",
  "result": {
    "tools": [
      {
        "name": "add_activity_to_incident",
        "description": "Add a note to an incident...",
        "inputSchema": { ... }
      },
      ... (complete tool definitions)
    ]
  },
  "id": 2
}

[2024-01-10 15:30:01] Waiting for notifications (press Ctrl+C to exit)...
```

## Color Coding

When colors are enabled (default), the output uses the following color scheme:

- **Blue (→)**: Outgoing requests
- **Green (←)**: Incoming responses
- **Yellow (←)**: Incoming notifications
- **Red**: Errors and removed tools
- **Gray**: Debug messages (in verbose mode)

## Use Cases

1. **Testing Dynamic Tool Updates**: Verify that the aggregator properly notifies clients when MCP servers register or deregister
2. **Debugging Aggregator Issues**: See the exact JSON-RPC messages being exchanged
3. **Monitoring Tool Availability**: Track which tools are available at any given time
4. **Integration Testing**: Ensure your MCP server implementations work correctly with the aggregator

## Tips

- Use `--json-rpc` to see full JSON-RPC messages for debugging protocol issues
- Use `--verbose` to see keepalive messages and other protocol-level details
- Use `--no-color` when piping output to files or other tools
- The agent will exit after the timeout period or when you press Ctrl+C
- The default endpoint comes from your envctl configuration file (`~/.envctl.yaml`)
- Combine `--json-rpc` and `--verbose` for maximum debugging information

## Configuration

The agent command respects the aggregator configuration in your `~/.envctl.yaml` file:

```yaml
aggregator:
  port: 8090
  host: localhost
```

You can override this with the `--endpoint` flag if needed. 