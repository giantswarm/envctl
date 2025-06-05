# Agent REPL Overlay in TUI

The Agent REPL (Read-Eval-Print Loop) overlay provides an interactive interface within the TUI for communicating with MCP (Model Context Protocol) servers without leaving the dashboard.

## Overview

The Agent REPL overlay allows you to:
- Execute commands against connected MCP servers
- List and inspect available tools, resources, and prompts
- Call tools with JSON arguments
- Retrieve resources
- Execute prompts with parameters
- View command history and output

## Accessing the REPL

Press `A` while in the main dashboard to open the Agent REPL overlay.

## Features

### Command Entry
- Type commands at the prompt (`MCP>`)
- Full command history with up/down arrow navigation
- Tab completion for commands and arguments
- Multi-line output display with scrolling

### Available Commands

| Command | Description | Example |
|---------|-------------|---------|
| `help` or `?` | Show available commands | `help` |
| `list tools` | List all available tools | `list tools` |
| `list resources` | List all available resources | `list resources` |
| `list prompts` | List all available prompts | `list prompts` |
| `describe tool <name>` | Show tool details | `describe tool calculate` |
| `describe resource <uri>` | Show resource details | `describe resource docs://readme` |
| `describe prompt <name>` | Show prompt details | `describe prompt greeting` |
| `call <tool> {json}` | Execute a tool | `call calculate {"x": 5, "y": 3}` |
| `get <resource-uri>` | Retrieve a resource | `get docs://readme` |
| `prompt <name> {json}` | Execute a prompt | `prompt greeting {"name": "Alice"}` |

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Enter` | Execute command |
| `Tab` | Auto-complete command or show completions |
| `↑/↓` | Navigate command history |
| `PgUp/PgDn` | Scroll output viewport |
| `Esc` or `A` | Close REPL overlay |

### Tab Completion

The REPL supports intelligent tab completion:
- Complete command names (e.g., `li<Tab>` → `list`)
- Complete subcommands (e.g., `list t<Tab>` → `list tools`)
- Complete tool/resource/prompt names
- Show available options when multiple completions exist

### Connection Management

The REPL automatically connects to the MCP aggregator when:
1. You first open the overlay
2. The aggregator is running (MCP servers are started)

If the aggregator is not running, you'll see an error message prompting you to start MCP servers first.

## Examples

### Listing Available Tools
```
MCP> list tools
Available tools (3):
  1. calculate               - Perform mathematical calculations
  2. weather                 - Get weather information
  3. translate               - Translate text between languages
```

### Describing a Tool
```
MCP> describe tool calculate
Tool: calculate
Description: Perform mathematical calculations
Input Schema:
{
  "type": "object",
  "properties": {
    "operation": {
      "type": "string",
      "enum": ["add", "subtract", "multiply", "divide"]
    },
    "x": {"type": "number"},
    "y": {"type": "number"}
  },
  "required": ["operation", "x", "y"]
}
```

### Calling a Tool
```
MCP> call calculate {"operation": "add", "x": 5, "y": 3}
Result:
{
  "result": 8
}
```

### Getting a Resource
```
MCP> get docs://readme
Contents:
# Project README
This is the content of the README file...
```

## Integration with TUI

The Agent REPL overlay integrates seamlessly with the TUI:
- Shares the same activity log for debugging
- Respects dark/light mode settings
- Maintains connection state across overlay opens/closes
- Properly cleans up connections when the TUI exits

## Troubleshooting

### "Aggregator not running" Error
This means the MCP servers haven't been started yet. Close the REPL overlay and ensure at least one MCP server is running in the dashboard.

### "Invalid JSON arguments" Error
Tool calls and prompts require valid JSON for arguments. Example:
- ✅ Correct: `call tool {"param": "value"}`
- ❌ Wrong: `call tool {param: value}`

### Connection Issues
If you experience connection problems:
1. Check that the aggregator is running (visible in the dashboard)
2. Verify MCP servers are in "Running" state
3. Check the activity log (`L` key) for error messages

## Implementation Details

The Agent REPL overlay uses:
- **Bubbles Text Input**: For command entry with built-in editing features
- **Viewport**: For scrollable output display
- **REPLAdapter**: Bridges the agent REPL functionality with the TUI
- **CommandExecutor Interface**: Provides a clean abstraction for command execution

The REPL maintains its own connection to the MCP aggregator and caches tool/resource/prompt lists for efficient tab completion and validation. 