package managers

import (
	"bytes"
	"context"
	"encoding/json"
	"envctl/internal/config"
	"envctl/internal/kube"
	"envctl/pkg/logging"
	"fmt"
	"io"
	"net"
	"net/http"
)

// k8sConnectionHealthChecker implements health checking for K8s connections
type k8sConnectionHealthChecker struct {
	config K8sConnectionConfig
}

// CheckHealth checks if the K8s connection is healthy by querying node status
func (k *k8sConnectionHealthChecker) CheckHealth(ctx context.Context) error {
	subsystem := fmt.Sprintf("K8sHealthCheck-%s", k.config.Name)

	// Use kube package to create clientset for the specific context
	clientset, err := kube.GetClientsetForContext(ctx, k.config.ContextName)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	// Perform the health check using kube package
	readyNodes, totalNodes, err := kube.GetNodeStatus(clientset)
	if err != nil {
		return fmt.Errorf("failed to get node status: %w", err)
	}

	// Check if cluster is healthy
	if totalNodes == 0 {
		return fmt.Errorf("no nodes found in cluster")
	}

	if readyNodes < totalNodes {
		return fmt.Errorf("cluster degraded: only %d/%d nodes ready", readyNodes, totalNodes)
	}

	logging.Debug(subsystem, "K8s connection healthy: %d/%d nodes ready", readyNodes, totalNodes)
	return nil
}

// portForwardHealthChecker implements health checking for port forwards
type portForwardHealthChecker struct {
	config config.PortForwardDefinition
}

// CheckHealth checks if the port forward is healthy by attempting to connect to the local port
func (p *portForwardHealthChecker) CheckHealth(ctx context.Context) error {
	subsystem := fmt.Sprintf("PFHealthCheck-%s", p.config.Name)

	// Create a dialer with the context
	dialer := &net.Dialer{}

	// LocalPort is a string in the config
	address := fmt.Sprintf("localhost:%s", p.config.LocalPort)

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("failed to connect to port forward on %s: %w", address, err)
	}
	defer conn.Close()

	logging.Debug(subsystem, "Port forward healthy, connection successful to %s", address)
	return nil
}

// mcpServerHealthChecker implements health checking for MCP servers
type mcpServerHealthChecker struct {
	config config.MCPServerDefinition
	port   int
}

// CheckHealth checks if the MCP server is healthy by attempting to list its tools
func (m *mcpServerHealthChecker) CheckHealth(ctx context.Context) error {
	subsystem := fmt.Sprintf("MCPHealthCheck-%s", m.config.Name)

	if m.port == 0 {
		return fmt.Errorf("MCP server port not available")
	}

	// Build the URL for the MCP server
	url := fmt.Sprintf("http://localhost:%d/message", m.port)

	// Create the JSON-RPC request to list tools
	requestBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Use a client without timeout (let context handle it)
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to MCP server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("MCP server returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response to ensure it's valid
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Check if we got a valid tools list response
	if _, ok := result["result"]; !ok {
		if errorData, hasError := result["error"]; hasError {
			return fmt.Errorf("MCP server error: %v", errorData)
		}
		return fmt.Errorf("invalid response from MCP server")
	}

	logging.Debug(subsystem, "MCP server healthy, tools list retrieved successfully")
	return nil
}
