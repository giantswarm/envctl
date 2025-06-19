package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"envctl/internal/template"

	"gopkg.in/yaml.v3"
)

// MockMCPServer implements a configurable mock MCP server for testing
type MockMCPServer struct {
	name           string
	config         MockMCPServerConfig
	tools          map[string]*MockToolHandler
	templateEngine *template.Engine
	debug          bool
}

// NewMockMCPServer creates a new mock MCP server from configuration
func NewMockMCPServer(configName, scenarioPath string, debug bool) (*MockMCPServer, error) {
	config, err := loadMockServerConfig(configName, scenarioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load mock server config: %w", err)
	}

	server := &MockMCPServer{
		name:           configName,
		config:         config,
		tools:          make(map[string]*MockToolHandler),
		templateEngine: template.New(),
		debug:          debug,
	}

	// Initialize tool handlers
	for _, toolConfig := range config.Tools {
		handler := NewMockToolHandler(toolConfig, server.templateEngine, debug)
		server.tools[toolConfig.Name] = handler
	}

	if debug {
		fmt.Printf("🔧 Mock MCP server '%s' initialized with %d tools\n", configName, len(server.tools))
		for toolName := range server.tools {
			fmt.Printf("  • %s\n", toolName)
		}
	}

	return server, nil
}

// Start starts the mock MCP server using stdio transport
func (s *MockMCPServer) Start(ctx context.Context) error {
	if s.debug {
		fmt.Printf("🚀 Starting mock MCP server '%s' on stdio transport\n", s.name)
	}

	// Create MCP server instance and handle communication
	return s.handleMCPCommunication(ctx, os.Stdin, os.Stdout)
}

// handleMCPCommunication handles the MCP protocol communication
func (s *MockMCPServer) handleMCPCommunication(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	decoder := json.NewDecoder(stdin)
	encoder := json.NewEncoder(stdout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Read MCP message
			var message map[string]interface{}
			if err := decoder.Decode(&message); err != nil {
				if err == io.EOF {
					return nil
				}
				return fmt.Errorf("failed to decode message: %w", err)
			}

			// Process message and send response
			response := s.processMessage(message)
			if response != nil {
				if err := encoder.Encode(response); err != nil {
					return fmt.Errorf("failed to encode response: %w", err)
				}
			}
		}
	}
}

// processMessage processes an incoming MCP message and returns a response
func (s *MockMCPServer) processMessage(message map[string]interface{}) map[string]interface{} {
	method, _ := message["method"].(string)
	id := message["id"]
	params, _ := message["params"].(map[string]interface{})

	if s.debug {
		fmt.Printf("📥 Received MCP message: method=%s, id=%v\n", method, id)
	}

	switch method {
	case "initialize":
		return s.handleInitialize(id, params)
	case "tools/list":
		return s.handleToolsList(id)
	case "tools/call":
		return s.handleToolCall(id, params)
	default:
		return s.createErrorResponse(id, -32601, "Method not found", method)
	}
}

// handleInitialize handles the MCP initialize request
func (s *MockMCPServer) handleInitialize(id interface{}, params map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    fmt.Sprintf("mock-%s", s.name),
				"version": "1.0.0",
			},
		},
	}
}

// handleToolsList handles the tools/list request
func (s *MockMCPServer) handleToolsList(id interface{}) map[string]interface{} {
	var tools []map[string]interface{}

	for _, handler := range s.tools {
		tools = append(tools, map[string]interface{}{
			"name":        handler.config.Name,
			"description": handler.config.Description,
			"inputSchema": handler.config.InputSchema,
		})
	}

	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"tools": tools,
		},
	}
}

// handleToolCall handles the tools/call request
func (s *MockMCPServer) handleToolCall(id interface{}, params map[string]interface{}) map[string]interface{} {
	toolName, _ := params["name"].(string)
	arguments, _ := params["arguments"].(map[string]interface{})

	if s.debug {
		fmt.Printf("🔧 Tool call: %s with arguments: %v\n", toolName, arguments)
	}

	handler, exists := s.tools[toolName]
	if !exists {
		return s.createErrorResponse(id, -32602, "Tool not found", toolName)
	}

	// Handle the tool call
	result, err := handler.HandleCall(arguments)
	if err != nil {
		return s.createErrorResponse(id, -32603, err.Error(), nil)
	}

	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
}

// createErrorResponse creates an MCP error response
func (s *MockMCPServer) createErrorResponse(id interface{}, code int, message string, data interface{}) map[string]interface{} {
	errorObj := map[string]interface{}{
		"code":    code,
		"message": message,
	}
	if data != nil {
		errorObj["data"] = data
	}

	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   errorObj,
	}
}

// loadMockServerConfig loads mock server configuration from test scenarios
func loadMockServerConfig(configName, scenarioPath string) (MockMCPServerConfig, error) {
	var config MockMCPServerConfig

	// Search for the mock server configuration in test scenarios
	err := filepath.WalkDir(scenarioPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || (!strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml")) {
			return nil
		}

		// Read and parse the scenario file
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var scenario TestScenario
		if err := yaml.Unmarshal(content, &scenario); err != nil {
			return nil // Skip invalid files
		}

		// Check if this scenario has the mock server we're looking for
		if scenario.PreConfiguration != nil {
			for _, mcpServer := range scenario.PreConfiguration.MCPServers {
				if mcpServer.Name == configName && mcpServer.Type == "mock" && mcpServer.MockConfig != nil {
					config = *mcpServer.MockConfig
					return filepath.SkipAll // Found it, stop searching
				}
			}
		}

		return nil
	})

	if err != nil {
		return config, fmt.Errorf("failed to search for mock server config: %w", err)
	}

	if len(config.Tools) == 0 {
		return config, fmt.Errorf("mock server configuration '%s' not found in scenario path '%s'", configName, scenarioPath)
	}

	return config, nil
}
