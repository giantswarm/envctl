package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// FilterCommand handles filtering tools
type FilterCommand struct {
	*BaseCommand
}

// NewFilterCommand creates a new filter command
func NewFilterCommand(client ClientInterface, logger LoggerInterface, transport TransportInterface) *FilterCommand {
	return &FilterCommand{
		BaseCommand: NewBaseCommand(client, logger, transport),
	}
}

// Execute runs the filter command
func (f *FilterCommand) Execute(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: %s", f.Usage())
	}

	if strings.ToLower(args[0]) != "tools" {
		return fmt.Errorf("filter only supports 'tools' currently")
	}

	// Parse command line arguments
	var pattern, descriptionFilter string
	var caseSensitive bool

	if len(args) > 1 && args[1] != "" {
		pattern = args[1]
	}
	if len(args) > 2 && args[2] != "" {
		descriptionFilter = args[2]
	}
	if len(args) > 3 && strings.ToLower(args[3]) == "case-sensitive" {
		caseSensitive = true
	}

	// Get tools from cache
	tools := f.client.GetToolCache()

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

// Usage returns the usage string
func (f *FilterCommand) Usage() string {
	return "filter <tools> [pattern] [description_filter]"
}

// Description returns the command description
func (f *FilterCommand) Description() string {
	return "Filter tools by name pattern or description"
}

// Completions returns possible completions
func (f *FilterCommand) Completions(input string) []string {
	return []string{"tools"}
}

// Aliases returns command aliases
func (f *FilterCommand) Aliases() []string {
	return []string{}
}
