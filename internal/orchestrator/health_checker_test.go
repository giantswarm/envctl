package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"envctl/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPHealthChecker_CheckHealth(t *testing.T) {
	tests := []struct {
		name        string
		serverFunc  func(w http.ResponseWriter, r *http.Request)
		expectError bool
		errorMsg    string
	}{
		{
			name: "healthy MCP server with tools",
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				// Verify the request
				var req map[string]interface{}
				json.NewDecoder(r.Body).Decode(&req)
				assert.Equal(t, "2.0", req["jsonrpc"])
				assert.Equal(t, "tools/list", req["method"])

				// Send a valid response
				response := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      1,
					"result": map[string]interface{}{
						"tools": []map[string]interface{}{
							{
								"name":        "test-tool",
								"description": "A test tool",
							},
						},
					},
				}
				json.NewEncoder(w).Encode(response)
			},
			expectError: false,
		},
		{
			name: "MCP server returns error",
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				response := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      1,
					"error": map[string]interface{}{
						"code":    -32601,
						"message": "Method not found",
					},
				}
				json.NewEncoder(w).Encode(response)
			},
			expectError: true,
			errorMsg:    "MCP server error:",
		},
		{
			name: "MCP server returns 500",
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error"))
			},
			expectError: true,
			errorMsg:    "MCP server returned status 500",
		},
		{
			name: "invalid JSON response",
			serverFunc: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("not json"))
			},
			expectError: true,
			errorMsg:    "failed to decode response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(tt.serverFunc))
			defer server.Close()

			// Extract port from server URL
			_, portStr, _ := net.SplitHostPort(server.Listener.Addr().String())
			port := 0
			fmt.Sscanf(portStr, "%d", &port)

			// Create health checker
			cfg := config.MCPServerDefinition{
				Name: "test-mcp",
			}
			checker := NewMCPHealthChecker(cfg, port)

			// Check health
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := checker.CheckHealth(ctx)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPortForwardHealthChecker_CheckHealth(t *testing.T) {
	tests := []struct {
		name        string
		setupServer bool
		expectError bool
	}{
		{
			name:        "healthy port forward",
			setupServer: true,
			expectError: false,
		},
		{
			name:        "port forward not available",
			setupServer: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var listener net.Listener
			var port int

			if tt.setupServer {
				// Create a test TCP server
				var err error
				listener, err = net.Listen("tcp", "localhost:0")
				require.NoError(t, err)
				defer listener.Close()

				// Get the port
				_, portStr, _ := net.SplitHostPort(listener.Addr().String())
				fmt.Sscanf(portStr, "%d", &port)

				// Accept connections in background
				go func() {
					for {
						conn, err := listener.Accept()
						if err != nil {
							return
						}
						conn.Close()
					}
				}()
			} else {
				// Use a port that's likely not in use
				port = 59999
			}

			// Create health checker
			cfg := config.PortForwardDefinition{
				Name:      "test-pf",
				LocalPort: fmt.Sprintf("%d", port),
			}
			checker := NewPortForwardHealthChecker(cfg)

			// Check health
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			err := checker.CheckHealth(ctx)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "failed to connect")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPortForwardHealthChecker_Timeout(t *testing.T) {
	// Use a port that's very unlikely to have a listener
	// Port 1 is privileged and typically not used for user services
	cfg := config.PortForwardDefinition{
		Name:      "test-pf-timeout",
		LocalPort: "1", // This port is unlikely to have a listener
	}

	checker := NewPortForwardHealthChecker(cfg)

	// Check health - this should fail quickly with connection refused
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	checkErr := checker.CheckHealth(ctx)
	duration := time.Since(start)

	// Should fail quickly with connection refused
	assert.Error(t, checkErr)
	assert.Contains(t, checkErr.Error(), "failed to connect")
	assert.Less(t, duration, 500*time.Millisecond) // Should fail quickly, not timeout
}
