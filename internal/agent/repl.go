package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/chzyer/readline"
	"github.com/mark3labs/mcp-go/mcp"
)

// REPL represents the Read-Eval-Print Loop for MCP interaction
type REPL struct {
	client           *Client
	logger           *Logger
	rl               *readline.Instance
	notificationChan chan mcp.JSONRPCNotification
	stopChan         chan struct{}
	wg               sync.WaitGroup
}

// NewREPL creates a new REPL instance
func NewREPL(client *Client, logger *Logger) *REPL {
	return &REPL{
		client:           client,
		logger:           logger,
		notificationChan: make(chan mcp.JSONRPCNotification, 10),
		stopChan:         make(chan struct{}),
	}
}

// Run starts the REPL
func (r *REPL) Run(ctx context.Context) error {
	r.logger.Info("Connecting to MCP aggregator at %s using %s transport...", r.client.endpoint, r.client.transport)

	// Create and connect MCP client
	mcpClient, notificationChan, err := r.client.createAndConnectClient(ctx)
	if err != nil {
		return err
	}
	defer mcpClient.Close()

	r.client.client = mcpClient

	// Set up REPL-specific notification channel routing for SSE
	if r.client.transport == TransportSSE && notificationChan != nil {
		go func() {
			for notification := range notificationChan {
				select {
				case r.notificationChan <- notification:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Initialize session and load initial data
	if err := r.client.initializeAndLoadData(ctx); err != nil {
		return err
	}

	// Set up readline with tab completion
	completer := r.createCompleter()
	historyFile := filepath.Join(os.TempDir(), ".envctl_agent_history")

	config := &readline.Config{
		Prompt:          "MCP> ",
		HistoryFile:     historyFile,
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",

		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	}

	rl, err := readline.NewEx(config)
	if err != nil {
		return fmt.Errorf("failed to create readline instance: %w", err)
	}
	defer rl.Close()
	r.rl = rl

	// Start notification listener in background (only for SSE transport)
	if r.client.transport == TransportSSE {
		r.wg.Add(1)
		go r.notificationListener(ctx)
		r.logger.Info("MCP REPL started with notification support. Type 'help' for available commands. Use TAB for completion.")
	} else {
		r.logger.Info("MCP REPL started. Type 'help' for available commands. Use TAB for completion.")
		r.logger.Info("Note: Real-time notifications are not supported with %s transport.", r.client.transport)
	}
	fmt.Println()

	// Main REPL loop
	for {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			if r.client.transport == TransportSSE {
				close(r.stopChan)
				r.wg.Wait()
			}
			r.logger.Info("REPL shutting down...")
			return nil
		default:
		}

		// Read input
		line, err := r.rl.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				continue
			}
		} else if err == io.EOF {
			if r.client.transport == TransportSSE {
				close(r.stopChan)
				r.wg.Wait()
			}
			r.logger.Info("Goodbye!")
			return nil
		} else if err != nil {
			return fmt.Errorf("readline error: %w", err)
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		// Parse and execute command
		if err := r.executeCommand(ctx, input); err != nil {
			if err.Error() == "exit" {
				if r.client.transport == TransportSSE {
					close(r.stopChan)
					r.wg.Wait()
				}
				r.logger.Info("Goodbye!")
				return nil
			}
			r.logger.Error("Error: %v", err)
		}

		fmt.Println()
	}
}

// createCompleter creates the tab completion configuration
func (r *REPL) createCompleter() *readline.PrefixCompleter {
	// Get lists for completion
	r.client.mu.RLock()
	tools := make([]string, len(r.client.toolCache))
	for i, tool := range r.client.toolCache {
		tools[i] = tool.Name
	}

	resources := make([]string, len(r.client.resourceCache))
	for i, resource := range r.client.resourceCache {
		resources[i] = resource.URI
	}

	prompts := make([]string, len(r.client.promptCache))
	for i, prompt := range r.client.promptCache {
		prompts[i] = prompt.Name
	}
	r.client.mu.RUnlock()

	// Create dynamic completers for items
	toolCompleter := make([]readline.PrefixCompleterInterface, len(tools))
	for i, tool := range tools {
		toolCompleter[i] = readline.PcItem(tool)
	}

	resourceCompleter := make([]readline.PrefixCompleterInterface, len(resources))
	for i, resource := range resources {
		resourceCompleter[i] = readline.PcItem(resource)
	}

	promptCompleter := make([]readline.PrefixCompleterInterface, len(prompts))
	for i, prompt := range prompts {
		promptCompleter[i] = readline.PcItem(prompt)
	}

	return readline.NewPrefixCompleter(
		readline.PcItem("help"),
		readline.PcItem("?"),
		readline.PcItem("exit"),
		readline.PcItem("quit"),
		readline.PcItem("list",
			readline.PcItem("tools"),
			readline.PcItem("resources"),
			readline.PcItem("prompts"),
			readline.PcItem("core-tools"),
		),
		readline.PcItem("describe",
			readline.PcItem("tool", toolCompleter...),
			readline.PcItem("resource", resourceCompleter...),
			readline.PcItem("prompt", promptCompleter...),
		),
		readline.PcItem("call", toolCompleter...),
		readline.PcItem("get", resourceCompleter...),
		readline.PcItem("prompt", promptCompleter...),
		readline.PcItem("filter",
			readline.PcItem("tools"),
		),
		readline.PcItem("notifications",
			readline.PcItem("on"),
			readline.PcItem("off"),
		),
	)
}

// filterInput filters input characters for readline
func filterInput(r rune) (rune, bool) {
	switch r {
	// block CtrlZ feature
	case readline.CharCtrlZ:
		return r, false
	}
	return r, true
}

// notificationListener handles notifications in the background
func (r *REPL) notificationListener(ctx context.Context) {
	defer r.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopChan:
			return
		case notification := <-r.notificationChan:
			// Temporarily pause readline
			if r.rl != nil {
				r.rl.Stdout().Write([]byte("\r\033[K"))
			}

			// Handle the notification (this will log it)
			if err := r.client.handleNotification(ctx, notification); err != nil {
				r.logger.Error("Failed to handle notification: %v", err)
			}

			// Update completer if items changed
			switch notification.Method {
			case "notifications/tools/list_changed",
				"notifications/resources/list_changed",
				"notifications/prompts/list_changed":
				if r.rl != nil {
					r.rl.Config.AutoComplete = r.createCompleter()
				}
			}

			// Refresh readline prompt
			if r.rl != nil {
				r.rl.Refresh()
			}
		}
	}
}

// executeCommand parses and executes a command
func (r *REPL) executeCommand(ctx context.Context, input string) error {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	command := strings.ToLower(parts[0])

	switch command {
	case "help", "?":
		return r.showHelp()

	case "list":
		if len(parts) < 2 {
			return fmt.Errorf("usage: list <tools|resources|prompts|core-tools>")
		}
		return r.handleList(ctx, parts[1])

	case "filter":
		if len(parts) < 2 {
			return fmt.Errorf("usage: filter <tools> [pattern] [description_filter]")
		}
		if parts[1] == "tools" {
			return r.handleFilterTools(ctx, parts[2:]...)
		}
		return fmt.Errorf("filter only supports 'tools' currently")

	case "describe":
		if len(parts) < 3 {
			return fmt.Errorf("usage: describe <tool|resource|prompt> <name>")
		}
		return r.handleDescribe(ctx, parts[1], strings.Join(parts[2:], " "))

	case "exit", "quit":
		return fmt.Errorf("exit")

	case "notifications":
		if len(parts) < 2 {
			return fmt.Errorf("usage: notifications <on|off>")
		}
		return r.handleNotifications(parts[1])

	case "call":
		if len(parts) < 2 {
			return fmt.Errorf("usage: call <tool-name> [args...]")
		}
		return r.handleCallTool(ctx, parts[1], strings.Join(parts[2:], " "))

	case "get":
		if len(parts) < 2 {
			return fmt.Errorf("usage: get <resource-uri>")
		}
		return r.handleGetResource(ctx, parts[1])

	case "prompt":
		if len(parts) < 2 {
			return fmt.Errorf("usage: prompt <prompt-name> [args...]")
		}
		return r.handleGetPrompt(ctx, parts[1], strings.Join(parts[2:], " "))

	default:
		return fmt.Errorf("unknown command: %s. Type 'help' for available commands", command)
	}
}

// showHelp displays available commands
func (r *REPL) showHelp() error {
	fmt.Println("Available commands:")
	fmt.Println("  help, ?                      - Show this help message")
	fmt.Println("  list tools                   - List all available tools")
	fmt.Println("  list resources               - List all available resources")
	fmt.Println("  list prompts                 - List all available prompts")
	fmt.Println("  list core-tools              - List core envctl tools (built-in functionality)")
	fmt.Println("  filter tools [pattern] [desc] - Filter tools by name pattern or description")
	fmt.Println("  describe tool <name>         - Show detailed information about a tool")
	fmt.Println("  describe resource <uri>      - Show detailed information about a resource")
	fmt.Println("  describe prompt <name>       - Show detailed information about a prompt")
	fmt.Println("  call <tool> {json}           - Execute a tool with JSON arguments")
	fmt.Println("  get <resource-uri>           - Retrieve a resource")
	fmt.Println("  prompt <name> {json}         - Get a prompt with JSON arguments")
	fmt.Println("  notifications <on|off>       - Enable/disable notification display")
	fmt.Println("  exit, quit                   - Exit the REPL")
	fmt.Println()
	fmt.Println("Keyboard shortcuts:")
	fmt.Println("  TAB                          - Auto-complete commands and arguments")
	fmt.Println("  ↑/↓ (arrow keys)             - Navigate command history")
	fmt.Println("  Ctrl+R                       - Search command history")
	fmt.Println("  Ctrl+C                       - Cancel current line")
	fmt.Println("  Ctrl+D                       - Exit REPL")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  call calculate {\"operation\": \"add\", \"x\": 5, \"y\": 3}")
	fmt.Println("  get docs://readme")
	fmt.Println("  prompt greeting {\"name\": \"Alice\"}")
	fmt.Println("  filter tools *workflow*      - Find tools with 'workflow' in name")
	fmt.Println("  filter tools \"\" \"kubernetes\" - Find tools with 'kubernetes' in description")
	return nil
}

// handleList handles list commands
func (r *REPL) handleList(ctx context.Context, target string) error {
	switch strings.ToLower(target) {
	case "tools", "tool":
		return r.listTools(ctx)
	case "resources", "resource":
		return r.listResources(ctx)
	case "prompts", "prompt":
		return r.listPrompts(ctx)
	case "core-tools", "coretools":
		return r.listCoreTools(ctx)
	default:
		return fmt.Errorf("unknown list target: %s. Use 'tools', 'resources', 'prompts', or 'core-tools'", target)
	}
}

// listTools displays available tools
func (r *REPL) listTools(ctx context.Context) error {
	r.client.mu.RLock()
	tools := r.client.toolCache
	r.client.mu.RUnlock()

	fmt.Println(r.client.formatters.FormatToolsList(tools))
	return nil
}

// listResources displays available resources
func (r *REPL) listResources(ctx context.Context) error {
	r.client.mu.RLock()
	resources := r.client.resourceCache
	r.client.mu.RUnlock()

	fmt.Println(r.client.formatters.FormatResourcesList(resources))
	return nil
}

// listPrompts displays available prompts
func (r *REPL) listPrompts(ctx context.Context) error {
	r.client.mu.RLock()
	prompts := r.client.promptCache
	r.client.mu.RUnlock()

	fmt.Println(r.client.formatters.FormatPromptsList(prompts))
	return nil
}

// handleDescribe handles describe commands
func (r *REPL) handleDescribe(ctx context.Context, targetType, name string) error {
	switch strings.ToLower(targetType) {
	case "tool":
		return r.describeTool(ctx, name)
	case "resource":
		return r.describeResource(ctx, name)
	case "prompt":
		return r.describePrompt(ctx, name)
	default:
		return fmt.Errorf("unknown describe target: %s. Use 'tool', 'resource', or 'prompt'", targetType)
	}
}

// describeTool shows detailed information about a tool
func (r *REPL) describeTool(ctx context.Context, name string) error {
	r.client.mu.RLock()
	tools := r.client.toolCache
	r.client.mu.RUnlock()

	tool := r.client.formatters.FindTool(tools, name)
	if tool == nil {
		return fmt.Errorf("tool not found: %s", name)
	}

	fmt.Println(r.client.formatters.FormatToolDetail(*tool))
	return nil
}

// describeResource shows detailed information about a resource
func (r *REPL) describeResource(ctx context.Context, uri string) error {
	r.client.mu.RLock()
	resources := r.client.resourceCache
	r.client.mu.RUnlock()

	resource := r.client.formatters.FindResource(resources, uri)
	if resource == nil {
		return fmt.Errorf("resource not found: %s", uri)
	}

	fmt.Println(r.client.formatters.FormatResourceDetail(*resource))
	return nil
}

// describePrompt shows detailed information about a prompt
func (r *REPL) describePrompt(ctx context.Context, name string) error {
	r.client.mu.RLock()
	prompts := r.client.promptCache
	r.client.mu.RUnlock()

	prompt := r.client.formatters.FindPrompt(prompts, name)
	if prompt == nil {
		return fmt.Errorf("prompt not found: %s", name)
	}

	fmt.Println(r.client.formatters.FormatPromptDetail(*prompt))
	return nil
}

// handleNotifications toggles notification display
func (r *REPL) handleNotifications(setting string) error {
	// First validate the setting is valid
	switch strings.ToLower(setting) {
	case "on", "off":
		// Valid setting, continue
	default:
		return fmt.Errorf("invalid setting: %s. Use 'on' or 'off'", setting)
	}

	// Check if transport supports notifications
	if r.client.transport != TransportSSE {
		fmt.Printf("Notifications are not supported with %s transport. Use --transport=sse for notification support.\n", r.client.transport)
		return nil
	}

	// Apply the setting
	switch strings.ToLower(setting) {
	case "on":
		r.logger.SetVerbose(true)
		fmt.Println("Notifications enabled")
	case "off":
		r.logger.SetVerbose(false)
		fmt.Println("Notifications disabled")
	}
	return nil
}

// listCoreTools displays core envctl tools
func (r *REPL) listCoreTools(ctx context.Context) error {
	// Define core envctl tools that are built-in functionality
	// These are tools that envctl provides natively, separate from external MCP servers
	// Names are shown with the "x_" prefix as they appear in the aggregator
	coreTools := []map[string]interface{}{
		{
			"name":        "x_capability_create",
			"description": "Create a new capability definition",
			"category":    "capability",
		},
		{
			"name":        "x_capability_list",
			"description": "List all available capabilities",
			"category":    "capability",
		},
		{
			"name":        "x_capability_get",
			"description": "Get detailed information about a specific capability",
			"category":    "capability",
		},
		{
			"name":        "x_capability_update",
			"description": "Update an existing capability definition",
			"category":    "capability",
		},
		{
			"name":        "x_capability_delete",
			"description": "Delete a capability definition",
			"category":    "capability",
		},
		{
			"name":        "x_serviceclass_create",
			"description": "Create a new service class definition",
			"category":    "serviceclass",
		},
		{
			"name":        "x_serviceclass_list",
			"description": "List all available service classes",
			"category":    "serviceclass",
		},
		{
			"name":        "x_serviceclass_get",
			"description": "Get detailed information about a specific service class",
			"category":    "serviceclass",
		},
		{
			"name":        "x_serviceclass_update",
			"description": "Update an existing service class definition",
			"category":    "serviceclass",
		},
		{
			"name":        "x_serviceclass_delete",
			"description": "Delete a service class definition",
			"category":    "serviceclass",
		},
		{
			"name":        "x_workflow_create",
			"description": "Create a new workflow definition",
			"category":    "workflow",
		},
		{
			"name":        "x_workflow_list",
			"description": "List all available workflows",
			"category":    "workflow",
		},
		{
			"name":        "x_workflow_get",
			"description": "Get detailed information about a specific workflow",
			"category":    "workflow",
		},
		{
			"name":        "x_workflow_update",
			"description": "Update an existing workflow definition",
			"category":    "workflow",
		},
		{
			"name":        "x_workflow_delete",
			"description": "Delete a workflow definition",
			"category":    "workflow",
		},
		{
			"name":        "x_workflow_run",
			"description": "Execute a workflow with given inputs",
			"category":    "workflow",
		},
		{
			"name":        "x_mcpserver_create",
			"description": "Create a new MCP server definition",
			"category":    "mcpserver",
		},
		{
			"name":        "x_mcpserver_list",
			"description": "List all available MCP servers",
			"category":    "mcpserver",
		},
		{
			"name":        "x_mcpserver_get",
			"description": "Get detailed information about a specific MCP server",
			"category":    "mcpserver",
		},
		{
			"name":        "x_mcpserver_update",
			"description": "Update an existing MCP server definition",
			"category":    "mcpserver",
		},
		{
			"name":        "x_mcpserver_delete",
			"description": "Delete an MCP server definition",
			"category":    "mcpserver",
		},
		{
			"name":        "x_service_create",
			"description": "Create a new service instance",
			"category":    "service",
		},
		{
			"name":        "x_service_list",
			"description": "List all service instances",
			"category":    "service",
		},
		{
			"name":        "x_service_get",
			"description": "Get detailed information about a service instance",
			"category":    "service",
		},
		{
			"name":        "x_service_start",
			"description": "Start a service instance",
			"category":    "service",
		},
		{
			"name":        "x_service_stop",
			"description": "Stop a service instance",
			"category":    "service",
		},
		{
			"name":        "x_service_restart",
			"description": "Restart a service instance",
			"category":    "service",
		},
		{
			"name":        "x_service_delete",
			"description": "Delete a service instance",
			"category":    "service",
		},
	}

	fmt.Printf("Core envctl tools (%d):\n", len(coreTools))
	
	// Group by category
	categories := make(map[string][]map[string]interface{})
	for _, tool := range coreTools {
		category := tool["category"].(string)
		categories[category] = append(categories[category], tool)
	}

	// Display by category
	for _, category := range []string{"capability", "serviceclass", "workflow", "mcpserver", "service"} {
		if tools, exists := categories[category]; exists {
			// Capitalize the first letter manually
			displayName := strings.ToUpper(category[:1]) + category[1:]
			fmt.Printf("\n%s tools:\n", displayName)
			for i, tool := range tools {
				fmt.Printf("  %d. %-27s - %s\n", i+1, tool["name"], tool["description"])
			}
		}
	}

	return nil
}

// handleFilterTools handles tool filtering in REPL
func (r *REPL) handleFilterTools(ctx context.Context, args ...string) error {
	// Parse command line arguments
	var pattern, descriptionFilter string
	var caseSensitive bool
	
	if len(args) > 0 && args[0] != "" {
		pattern = args[0]
	}
	if len(args) > 1 && args[1] != "" {
		descriptionFilter = args[1]
	}
	if len(args) > 2 && strings.ToLower(args[2]) == "case-sensitive" {
		caseSensitive = true
	}

	// Get tools from cache
	r.client.mu.RLock()
	tools := r.client.toolCache
	r.client.mu.RUnlock()

	if len(tools) == 0 {
		fmt.Println("No tools available to filter")
		return nil
	}

	// Filter tools based on criteria
	var filteredTools []mcp.Tool

	for _, tool := range tools {
		// Check if tool matches the filters
		matches := true

		// Check pattern filter (supports basic wildcard matching)
		if pattern != "" {
			toolName := tool.Name
			searchPattern := pattern
			
			if !caseSensitive {
				toolName = strings.ToLower(toolName)
				searchPattern = strings.ToLower(searchPattern)
			}

			// Simple wildcard matching
			if strings.Contains(searchPattern, "*") {
				// Convert wildcard pattern to prefix/suffix matching
				if strings.HasPrefix(searchPattern, "*") && strings.HasSuffix(searchPattern, "*") {
					// *pattern* - contains
					middle := strings.Trim(searchPattern, "*")
					matches = matches && strings.Contains(toolName, middle)
				} else if strings.HasPrefix(searchPattern, "*") {
					// *pattern - ends with
					suffix := strings.TrimPrefix(searchPattern, "*")
					matches = matches && strings.HasSuffix(toolName, suffix)
				} else if strings.HasSuffix(searchPattern, "*") {
					// pattern* - starts with
					prefix := strings.TrimSuffix(searchPattern, "*")
					matches = matches && strings.HasPrefix(toolName, prefix)
				} else {
					// pattern*pattern - more complex, use simple contains for each part
					parts := strings.Split(searchPattern, "*")
					for _, part := range parts {
						if part != "" && !strings.Contains(toolName, part) {
							matches = false
							break
						}
					}
				}
			} else {
				// Exact match or contains
				matches = matches && strings.Contains(toolName, searchPattern)
			}
		}

		// Check description filter
		if descriptionFilter != "" && matches {
			toolDesc := tool.Description
			searchDesc := descriptionFilter
			
			if !caseSensitive {
				toolDesc = strings.ToLower(toolDesc)
				searchDesc = strings.ToLower(searchDesc)
			}

			matches = matches && strings.Contains(toolDesc, searchDesc)
		}

		// Add to filtered results if it matches
		if matches {
			filteredTools = append(filteredTools, tool)
		}
	}

	// Display results
	fmt.Printf("Filtering tools with:\n")
	if pattern != "" {
		fmt.Printf("  Pattern: %s\n", pattern)
	}
	if descriptionFilter != "" {
		fmt.Printf("  Description filter: %s\n", descriptionFilter)
	}
	fmt.Printf("  Case sensitive: %t\n", caseSensitive)
	fmt.Printf("\nResults: %d of %d tools match\n", len(filteredTools), len(tools))

	if len(filteredTools) == 0 {
		fmt.Println("No tools match the specified filters.")
		return nil
	}

	fmt.Println("\nMatching tools:")
	for i, tool := range filteredTools {
		fmt.Printf("  %d. %-30s - %s\n", i+1, tool.Name, tool.Description)
	}

	return nil
}

// parseJSONArgs parses JSON arguments from string, providing helpful error messages
func (r *REPL) parseJSONArgs(argsStr, itemType, itemName string) (map[string]interface{}, error) {
	if argsStr == "" {
		return nil, nil
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
		fmt.Printf("Error: Arguments must be valid JSON\n")
		fmt.Printf("Example: %s %s {\"param1\": \"value1\", \"param2\": 123}\n", itemType, itemName)
		return nil, fmt.Errorf("invalid JSON arguments: %w", err)
	}

	return args, nil
}

// parseJSONStringArgs parses JSON arguments and converts all values to strings
func (r *REPL) parseJSONStringArgs(argsStr, itemType, itemName string, requiredArgs []mcp.PromptArgument) (map[string]string, error) {
	if argsStr == "" {
		return make(map[string]string), nil
	}

	var jsonArgs map[string]interface{}
	if err := json.Unmarshal([]byte(argsStr), &jsonArgs); err != nil {
		fmt.Printf("Error: Arguments must be valid JSON\n")
		fmt.Printf("Example: %s %s {\"arg1\": \"value1\", \"arg2\": \"value2\"}\n", itemType, itemName)

		// Show required arguments
		if len(requiredArgs) > 0 {
			fmt.Println("Required arguments:")
			for _, arg := range requiredArgs {
				if arg.Required {
					fmt.Printf("  - %s: %s\n", arg.Name, arg.Description)
				}
			}
		}
		return nil, fmt.Errorf("invalid JSON arguments: %w", err)
	}

	// Convert to string map
	args := make(map[string]string)
	for k, v := range jsonArgs {
		args[k] = fmt.Sprintf("%v", v)
	}

	return args, nil
}

// validateRequiredArgs checks that all required arguments are provided
func (r *REPL) validateRequiredArgs(args map[string]string, requiredArgs []mcp.PromptArgument) error {
	for _, arg := range requiredArgs {
		if arg.Required && args[arg.Name] == "" {
			return fmt.Errorf("missing required argument: %s", arg.Name)
		}
	}
	return nil
}

// handleCallTool executes a tool with the given arguments
func (r *REPL) handleCallTool(ctx context.Context, toolName string, argsStr string) error {
	// Find and validate tool exists
	r.client.mu.RLock()
	tools := r.client.toolCache
	r.client.mu.RUnlock()

	tool := r.client.formatters.FindTool(tools, toolName)
	if tool == nil {
		return fmt.Errorf("tool not found: %s", toolName)
	}

	// Parse arguments
	args, err := r.parseJSONArgs(argsStr, "call", toolName)
	if err != nil {
		return err
	}

	// Execute the tool
	fmt.Printf("Executing tool: %s...\n", toolName)
	result, err := r.client.CallTool(ctx, toolName, args)
	if err != nil {
		return fmt.Errorf("tool execution failed: %w", err)
	}

	// Display results
	if result.IsError {
		fmt.Println("Tool returned an error:")
		for _, content := range result.Content {
			if textContent, ok := mcp.AsTextContent(content); ok {
				fmt.Printf("  %s\n", textContent.Text)
			}
		}
	} else {
		fmt.Println("Result:")
		for _, content := range result.Content {
			if textContent, ok := mcp.AsTextContent(content); ok {
				// Try to pretty-print if it's JSON
				var jsonData interface{}
				if err := json.Unmarshal([]byte(textContent.Text), &jsonData); err == nil {
					fmt.Println(PrettyJSON(jsonData))
				} else {
					fmt.Println(textContent.Text)
				}
			} else if imageContent, ok := mcp.AsImageContent(content); ok {
				fmt.Printf("[Image: MIME type %s, %d bytes]\n", imageContent.MIMEType, len(imageContent.Data))
			} else if audioContent, ok := mcp.AsAudioContent(content); ok {
				fmt.Printf("[Audio: MIME type %s, %d bytes]\n", audioContent.MIMEType, len(audioContent.Data))
			}
		}
	}

	return nil
}

// handleGetResource retrieves and displays a resource
func (r *REPL) handleGetResource(ctx context.Context, uri string) error {
	// Find and validate resource exists
	r.client.mu.RLock()
	resources := r.client.resourceCache
	r.client.mu.RUnlock()

	resource := r.client.formatters.FindResource(resources, uri)
	if resource == nil {
		return fmt.Errorf("resource not found: %s", uri)
	}

	// Retrieve the resource
	fmt.Printf("Retrieving resource: %s...\n", uri)
	result, err := r.client.GetResource(ctx, uri)
	if err != nil {
		return fmt.Errorf("resource retrieval failed: %w", err)
	}

	// Display contents
	fmt.Println("Contents:")
	for _, content := range result.Contents {
		if textContent, ok := mcp.AsTextResourceContents(content); ok {
			// Check MIME type for appropriate display
			if resource.MIMEType == "application/json" {
				var jsonData interface{}
				if err := json.Unmarshal([]byte(textContent.Text), &jsonData); err == nil {
					fmt.Println(PrettyJSON(jsonData))
				} else {
					fmt.Println(textContent.Text)
				}
			} else {
				fmt.Println(textContent.Text)
			}
		} else if blobContent, ok := mcp.AsBlobResourceContents(content); ok {
			fmt.Printf("[Binary data: %d bytes]\n", len(blobContent.Blob))
		}
	}

	return nil
}

// handleGetPrompt retrieves and displays a prompt with arguments
func (r *REPL) handleGetPrompt(ctx context.Context, promptName string, argsStr string) error {
	// Find and validate prompt exists
	r.client.mu.RLock()
	prompts := r.client.promptCache
	r.client.mu.RUnlock()

	prompt := r.client.formatters.FindPrompt(prompts, promptName)
	if prompt == nil {
		return fmt.Errorf("prompt not found: %s", promptName)
	}

	// Parse arguments
	args, err := r.parseJSONStringArgs(argsStr, "prompt", promptName, prompt.Arguments)
	if err != nil {
		return err
	}

	// Validate required arguments
	if err := r.validateRequiredArgs(args, prompt.Arguments); err != nil {
		return err
	}

	// Get the prompt
	fmt.Printf("Getting prompt: %s...\n", promptName)
	result, err := r.client.GetPrompt(ctx, promptName, args)
	if err != nil {
		return fmt.Errorf("prompt retrieval failed: %w", err)
	}

	// Display messages
	fmt.Println("Messages:")
	for i, msg := range result.Messages {
		fmt.Printf("\n[%d] Role: %s\n", i+1, msg.Role)
		if textContent, ok := mcp.AsTextContent(msg.Content); ok {
			fmt.Printf("Content: %s\n", textContent.Text)
		} else if imageContent, ok := mcp.AsImageContent(msg.Content); ok {
			fmt.Printf("Content: [Image: MIME type %s, %d bytes]\n", imageContent.MIMEType, len(imageContent.Data))
		} else if audioContent, ok := mcp.AsAudioContent(msg.Content); ok {
			fmt.Printf("Content: [Audio: MIME type %s, %d bytes]\n", audioContent.MIMEType, len(audioContent.Data))
		} else if resource, ok := mcp.AsEmbeddedResource(msg.Content); ok {
			fmt.Printf("Content: [Embedded Resource: %v]\n", resource.Resource)
		} else {
			// Fallback for unknown content types
			fmt.Printf("Content: %+v\n", msg.Content)
		}
	}

	return nil
}
