package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// REPLAdapter adapts the REPL functionality for use in the TUI
type REPLAdapter struct {
	client *Client
	logger *Logger
	mu     sync.RWMutex

	// State
	connected bool
	tools     []mcp.Tool
	resources []mcp.Resource
	prompts   []mcp.Prompt
}

// NewREPLAdapter creates a new REPL adapter
func NewREPLAdapter(endpoint string, logger *Logger) (*REPLAdapter, error) {
	client := NewClient(endpoint, logger)
	return &REPLAdapter{
		client: client,
		logger: logger,
	}, nil
}

// Connect initializes the connection to the MCP aggregator
func (r *REPLAdapter) Connect(ctx context.Context) error {
	r.logger.Info("Connecting to MCP aggregator at %s...", r.client.endpoint)

	// Create SSE client
	sseClient, err := client.NewSSEMCPClient(r.client.endpoint)
	if err != nil {
		return fmt.Errorf("failed to create SSE client: %w", err)
	}
	r.client.client = sseClient

	// Start the SSE transport
	if err := sseClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start SSE client: %w", err)
	}

	// Initialize the session
	if err := r.client.initialize(ctx); err != nil {
		sseClient.Close()
		return fmt.Errorf("initialization failed: %w", err)
	}

	// List tools, resources, and prompts initially
	if err := r.client.listTools(ctx, false); err != nil {
		r.logger.Info("Failed to list tools: %v", err)
	}

	if err := r.client.listResources(ctx, false); err != nil {
		r.logger.Info("Failed to list resources: %v", err)
	}

	if err := r.client.listPrompts(ctx, false); err != nil {
		r.logger.Info("Failed to list prompts: %v", err)
	}

	// Cache the lists
	r.mu.Lock()
	r.connected = true
	r.tools = r.client.toolCache
	r.resources = r.client.resourceCache
	r.prompts = r.client.promptCache
	r.mu.Unlock()

	return nil
}

// Execute implements CommandExecutor
func (r *REPLAdapter) Execute(ctx context.Context, command string) (string, error) {
	if !r.IsConnected() {
		return "", fmt.Errorf("not connected to MCP aggregator")
	}

	// Parse command
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", nil
	}

	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "help", "?":
		return r.getHelpText(), nil

	case "list":
		if len(parts) < 2 {
			return "", fmt.Errorf("usage: list <tools|resources|prompts>")
		}
		return r.handleList(ctx, parts[1])

	case "describe":
		if len(parts) < 3 {
			return "", fmt.Errorf("usage: describe <tool|resource|prompt> <name>")
		}
		return r.handleDescribe(ctx, parts[1], strings.Join(parts[2:], " "))

	case "call":
		if len(parts) < 2 {
			return "", fmt.Errorf("usage: call <tool-name> [args...]")
		}
		return r.handleCallTool(ctx, parts[1], strings.Join(parts[2:], " "))

	case "get":
		if len(parts) < 2 {
			return "", fmt.Errorf("usage: get <resource-uri>")
		}
		return r.handleGetResource(ctx, parts[1])

	case "prompt":
		if len(parts) < 2 {
			return "", fmt.Errorf("usage: prompt <prompt-name> [args...]")
		}
		return r.handleGetPrompt(ctx, parts[1], strings.Join(parts[2:], " "))

	case "exit", "quit":
		return "Use Esc to close the REPL overlay", nil

	default:
		return "", fmt.Errorf("unknown command: %s. Type 'help' for available commands", cmd)
	}
}

// GetCompletions implements CommandExecutor
func (r *REPLAdapter) GetCompletions(partial string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var completions []string

	// Basic commands
	commands := []string{"help", "?", "list", "describe", "call", "get", "prompt", "exit", "quit"}

	// Add subcommands
	if strings.HasPrefix(partial, "list ") {
		for _, sub := range []string{"list tools", "list resources", "list prompts"} {
			if strings.HasPrefix(sub, partial) {
				completions = append(completions, sub)
			}
		}
	} else if strings.HasPrefix(partial, "describe ") {
		for _, sub := range []string{"describe tool", "describe resource", "describe prompt"} {
			if strings.HasPrefix(sub, partial) {
				completions = append(completions, sub)
			}
		}

		// Add specific items
		if strings.HasPrefix(partial, "describe tool ") {
			prefix := strings.TrimPrefix(partial, "describe tool ")
			for _, tool := range r.tools {
				if strings.HasPrefix(tool.Name, prefix) {
					completions = append(completions, "describe tool "+tool.Name)
				}
			}
		}
	} else if strings.HasPrefix(partial, "call ") {
		prefix := strings.TrimPrefix(partial, "call ")
		for _, tool := range r.tools {
			if strings.HasPrefix(tool.Name, prefix) {
				completions = append(completions, "call "+tool.Name)
			}
		}
	} else {
		// Top-level command completion
		for _, cmd := range commands {
			if strings.HasPrefix(cmd, partial) {
				completions = append(completions, cmd)
			}
		}
	}

	return completions
}

// IsConnected implements CommandExecutor
func (r *REPLAdapter) IsConnected() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.connected
}

// Close closes the connection
func (r *REPLAdapter) Close() error {
	r.mu.Lock()
	r.connected = false
	r.mu.Unlock()

	// The client should have a Close method
	// We don't need to type assert to SSEMCPClient
	if r.client != nil && r.client.client != nil {
		// Just call Close on the aggregator SSE client
		// The actual closing will be handled by the client's transport
		r.client.client = nil
	}
	return nil
}

// Helper methods

func (r *REPLAdapter) getHelpText() string {
	return `Available commands:
  help, ?                      - Show this help message
  list tools                   - List all available tools
  list resources               - List all available resources
  list prompts                 - List all available prompts
  describe tool <name>         - Show detailed information about a tool
  describe resource <uri>      - Show detailed information about a resource
  describe prompt <name>       - Show detailed information about a prompt
  call <tool> {json}           - Execute a tool with JSON arguments
  get <resource-uri>           - Retrieve a resource
  prompt <name> {json}         - Get a prompt with JSON arguments

Keyboard shortcuts:
  Tab                          - Auto-complete commands and arguments
  ↑/↓ (arrow keys)             - Navigate command history
  Esc                          - Close REPL overlay`
}

func (r *REPLAdapter) handleList(ctx context.Context, target string) (string, error) {
	switch strings.ToLower(target) {
	case "tools", "tool":
		return r.listTools(), nil
	case "resources", "resource":
		return r.listResources(), nil
	case "prompts", "prompt":
		return r.listPrompts(), nil
	default:
		return "", fmt.Errorf("unknown list target: %s. Use 'tools', 'resources', or 'prompts'", target)
	}
}

func (r *REPLAdapter) listTools() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.tools) == 0 {
		return "No tools available."
	}

	var output []string
	output = append(output, fmt.Sprintf("Available tools (%d):", len(r.tools)))
	for i, tool := range r.tools {
		output = append(output, fmt.Sprintf("  %d. %-30s - %s", i+1, tool.Name, tool.Description))
	}
	return strings.Join(output, "\n")
}

func (r *REPLAdapter) listResources() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.resources) == 0 {
		return "No resources available."
	}

	var output []string
	output = append(output, fmt.Sprintf("Available resources (%d):", len(r.resources)))
	for i, resource := range r.resources {
		desc := resource.Description
		if desc == "" {
			desc = resource.Name
		}
		output = append(output, fmt.Sprintf("  %d. %-40s - %s", i+1, resource.URI, desc))
	}
	return strings.Join(output, "\n")
}

func (r *REPLAdapter) listPrompts() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.prompts) == 0 {
		return "No prompts available."
	}

	var output []string
	output = append(output, fmt.Sprintf("Available prompts (%d):", len(r.prompts)))
	for i, prompt := range r.prompts {
		output = append(output, fmt.Sprintf("  %d. %-30s - %s", i+1, prompt.Name, prompt.Description))
	}
	return strings.Join(output, "\n")
}

func (r *REPLAdapter) handleDescribe(ctx context.Context, targetType, name string) (string, error) {
	switch strings.ToLower(targetType) {
	case "tool":
		return r.describeTool(name), nil
	case "resource":
		return r.describeResource(name), nil
	case "prompt":
		return r.describePrompt(name), nil
	default:
		return "", fmt.Errorf("unknown describe target: %s. Use 'tool', 'resource', or 'prompt'", targetType)
	}
}

func (r *REPLAdapter) describeTool(name string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, tool := range r.tools {
		if tool.Name == name {
			var output []string
			output = append(output, fmt.Sprintf("Tool: %s", tool.Name))
			output = append(output, fmt.Sprintf("Description: %s", tool.Description))
			output = append(output, "Input Schema:")
			output = append(output, PrettyJSON(tool.InputSchema))
			return strings.Join(output, "\n")
		}
	}

	return fmt.Sprintf("Tool not found: %s", name)
}

func (r *REPLAdapter) describeResource(uri string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, resource := range r.resources {
		if resource.URI == uri {
			var output []string
			output = append(output, fmt.Sprintf("Resource: %s", resource.URI))
			output = append(output, fmt.Sprintf("Name: %s", resource.Name))
			if resource.Description != "" {
				output = append(output, fmt.Sprintf("Description: %s", resource.Description))
			}
			if resource.MIMEType != "" {
				output = append(output, fmt.Sprintf("MIME Type: %s", resource.MIMEType))
			}
			return strings.Join(output, "\n")
		}
	}

	return fmt.Sprintf("Resource not found: %s", uri)
}

func (r *REPLAdapter) describePrompt(name string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, prompt := range r.prompts {
		if prompt.Name == name {
			var output []string
			output = append(output, fmt.Sprintf("Prompt: %s", prompt.Name))
			output = append(output, fmt.Sprintf("Description: %s", prompt.Description))
			if len(prompt.Arguments) > 0 {
				output = append(output, "Arguments:")
				for _, arg := range prompt.Arguments {
					required := ""
					if arg.Required {
						required = " (required)"
					}
					output = append(output, fmt.Sprintf("  - %s%s: %s", arg.Name, required, arg.Description))
				}
			}
			return strings.Join(output, "\n")
		}
	}

	return fmt.Sprintf("Prompt not found: %s", name)
}

func (r *REPLAdapter) handleCallTool(ctx context.Context, toolName, args string) (string, error) {
	// Parse arguments
	var argsMap map[string]interface{}
	if args != "" {
		if err := json.Unmarshal([]byte(args), &argsMap); err != nil {
			return "", fmt.Errorf("invalid JSON arguments: %w. Example: {\"param\": \"value\"}", err)
		}
	}

	result, err := r.client.CallTool(ctx, toolName, argsMap)
	if err != nil {
		return "", err
	}

	// Format the result
	var output []string
	if result.IsError {
		output = append(output, "Tool returned an error:")
		for _, content := range result.Content {
			if textContent, ok := mcp.AsTextContent(content); ok {
				output = append(output, textContent.Text)
			}
		}
	} else {
		for _, content := range result.Content {
			if textContent, ok := mcp.AsTextContent(content); ok {
				// Try to pretty-print if it's JSON
				var jsonData interface{}
				if err := json.Unmarshal([]byte(textContent.Text), &jsonData); err == nil {
					output = append(output, PrettyJSON(jsonData))
				} else {
					output = append(output, textContent.Text)
				}
			} else if imageContent, ok := mcp.AsImageContent(content); ok {
				output = append(output, fmt.Sprintf("[Image: MIME type %s, %d bytes]", imageContent.MIMEType, len(imageContent.Data)))
			}
		}
	}

	return strings.Join(output, "\n"), nil
}

func (r *REPLAdapter) handleGetResource(ctx context.Context, uri string) (string, error) {
	result, err := r.client.GetResource(ctx, uri)
	if err != nil {
		return "", err
	}

	// Format the result
	var output []string
	for _, content := range result.Contents {
		if textContent, ok := mcp.AsTextResourceContents(content); ok {
			// Try to pretty-print if it's JSON
			var jsonData interface{}
			if err := json.Unmarshal([]byte(textContent.Text), &jsonData); err == nil {
				output = append(output, PrettyJSON(jsonData))
			} else {
				output = append(output, textContent.Text)
			}
		} else if blobContent, ok := mcp.AsBlobResourceContents(content); ok {
			output = append(output, fmt.Sprintf("[Binary data: %d bytes]", len(blobContent.Blob)))
		}
	}

	return strings.Join(output, "\n"), nil
}

func (r *REPLAdapter) handleGetPrompt(ctx context.Context, promptName, args string) (string, error) {
	// Parse arguments
	argsMap := make(map[string]string)
	if args != "" {
		var jsonArgs map[string]interface{}
		if err := json.Unmarshal([]byte(args), &jsonArgs); err != nil {
			return "", fmt.Errorf("invalid JSON arguments: %w. Example: {\"arg\": \"value\"}", err)
		}
		// Convert to string map
		for k, v := range jsonArgs {
			argsMap[k] = fmt.Sprintf("%v", v)
		}
	}

	result, err := r.client.GetPrompt(ctx, promptName, argsMap)
	if err != nil {
		return "", err
	}

	// Format the result
	var output []string
	output = append(output, "Messages:")
	for i, msg := range result.Messages {
		output = append(output, fmt.Sprintf("\n[%d] Role: %s", i+1, msg.Role))
		if textContent, ok := mcp.AsTextContent(msg.Content); ok {
			output = append(output, fmt.Sprintf("Content: %s", textContent.Text))
		} else if imageContent, ok := mcp.AsImageContent(msg.Content); ok {
			output = append(output, fmt.Sprintf("Content: [Image: MIME type %s, %d bytes]", imageContent.MIMEType, len(imageContent.Data)))
		}
	}

	return strings.Join(output, "\n"), nil
}
