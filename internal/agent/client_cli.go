package agent

import (
	"fmt"
	"time"

	"envctl/internal/config"

	"github.com/mark3labs/mcp-go/mcp"
)

// DetectAggregatorEndpoint detects the aggregator endpoint from configuration
func DetectAggregatorEndpoint() (string, error) {
	// Load configuration to get aggregator settings
	cfg, err := config.LoadConfig()
	if err != nil {
		// Use default if config cannot be loaded
		endpoint := "http://localhost:8090/mcp"
		return endpoint, nil
	}

	// Build endpoint from config
	host := cfg.Aggregator.Host
	if host == "" {
		host = "localhost"
	}
	port := cfg.Aggregator.Port
	if port == 0 {
		port = 8090
	}
	endpoint := fmt.Sprintf("http://%s:%d/mcp", host, port)

	return endpoint, nil
}

// NewCLIClient creates a new CLI client with auto-detected endpoint  
func NewCLIClient() (*Client, error) {
	endpoint, err := DetectAggregatorEndpoint()
	if err != nil {
		return nil, fmt.Errorf("failed to detect aggregator endpoint: %w", err)
	}

	return &Client{
		endpoint:      endpoint,
		transport:     TransportStreamableHTTP,
		logger:        nil, // No logging for CLI usage
		toolCache:     []mcp.Tool{},
		resourceCache: []mcp.Resource{},
		promptCache:   []mcp.Prompt{},
		timeout:       30 * time.Second,
		cacheEnabled:  false, // No caching for CLI usage
		formatters:    NewFormatters(),
	}, nil
}

// NewCLIClientWithEndpoint creates a new CLI client with a specific endpoint
func NewCLIClientWithEndpoint(endpoint string) *Client {
	return &Client{
		endpoint:      endpoint,
		transport:     TransportStreamableHTTP,
		logger:        nil, // No logging for CLI usage
		toolCache:     []mcp.Tool{},
		resourceCache: []mcp.Resource{},
		promptCache:   []mcp.Prompt{},
		timeout:       30 * time.Second,
		cacheEnabled:  false, // No caching for CLI usage
		formatters:    NewFormatters(),
	}
} 