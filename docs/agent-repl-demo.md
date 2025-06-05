# Agent REPL Demo

This guide demonstrates the new tab completion and command history features in the envctl agent REPL.

## Starting the REPL

```bash
./envctl agent --repl
```

## Tab Completion Examples

### 1. Command Completion

Type `li` and press TAB:
```
MCP> li[TAB]
# Completes to:
MCP> list
```

### 2. Subcommand Completion

Type `list ` and press TAB:
```
MCP> list [TAB]
# Shows options:
tools      resources      prompts
```

### 3. Tool Name Completion

Type `describe tool mcp_` and press TAB:
```
MCP> describe tool mcp_[TAB]
# Shows all tools starting with mcp_:
mcp_envctl-mcp_execute_query       mcp_envctl-mcp_execute_range_query
```

### 4. Resource URI Completion

Type `get ` and press TAB to see all available resource URIs:
```
MCP> get [TAB]
# Shows available resources
```

## Command History

### 1. Navigate History

Use the up and down arrow keys to navigate through previously executed commands.

### 2. Search History

Press Ctrl+R and type to search through command history:
```
(reverse-i-search)`call': call mcp_envctl-mcp_execute_query {"query": "up"}
```

### 3. Persistent History

Command history is saved in `~/.envctl_agent_history` and persists between REPL sessions.

## Advanced Editing

- **Ctrl+A**: Move to beginning of line
- **Ctrl+E**: Move to end of line
- **Ctrl+W**: Delete word before cursor
- **Ctrl+K**: Delete from cursor to end of line
- **Ctrl+U**: Delete entire line
- **Ctrl+L**: Clear screen

## Example Workflow

1. Start REPL:
   ```bash
   ./envctl agent --repl
   ```

2. List available tools (with tab completion):
   ```
   MCP> li[TAB]st to[TAB]ols
   ```

3. Describe a tool (with tab completion):
   ```
   MCP> desc[TAB]ribe tool mcp_envctl[TAB]-mcp_execute_query
   ```

4. Execute the tool:
   ```
   MCP> call mcp_envctl-mcp_execute_query {"query": "up"}
   ```

5. Use history to re-run the command:
   ```
   MCP> [↑] # Shows previous command
   ```

6. Search history for a specific command:
   ```
   MCP> [Ctrl+R]query # Finds commands containing "query"
   ```

## Tips

- Tab completion updates dynamically when tools/resources/prompts change
- Use double-TAB to see all available completions
- History file location: `/tmp/.envctl_agent_history`
- Completions are context-aware (e.g., only shows tool names after "describe tool") 