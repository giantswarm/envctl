package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// GetCommand handles retrieving MCP resources
type GetCommand struct {
	*BaseCommand
}

// NewGetCommand creates a new get command
func NewGetCommand(client ClientInterface, logger LoggerInterface, transport TransportInterface) *GetCommand {
	return &GetCommand{
		BaseCommand: NewBaseCommand(client, logger, transport),
	}
}

// Execute runs the get command
func (g *GetCommand) Execute(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: %s", g.Usage())
	}

	uri := args[0]

	// Find and validate resource exists
	resources := g.client.GetResourceCache()
	resource := g.getFormatters().FindResource(resources, uri)
	if resource == nil {
		return fmt.Errorf("resource not found: %s", uri)
	}

	// Retrieve the resource
	fmt.Printf("Retrieving resource: %s...\n", uri)
	result, err := g.client.GetResource(ctx, uri)
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
					b, _ := json.MarshalIndent(jsonData, "", "  ")
					fmt.Println(string(b))
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

// Usage returns the usage string
func (g *GetCommand) Usage() string {
	return "get <resource-uri>"
}

// Description returns the command description
func (g *GetCommand) Description() string {
	return "Retrieve a resource"
}

// Completions returns possible completions
func (g *GetCommand) Completions(input string) []string {
	return g.getResourceCompletions()
}

// Aliases returns command aliases
func (g *GetCommand) Aliases() []string {
	return []string{}
}
