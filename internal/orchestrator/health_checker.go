package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"envctl/internal/config"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
)

// ServiceHealthChecker defines the interface for checking service health
type ServiceHealthChecker interface {
	CheckHealth(ctx context.Context) error
}

// MCPHealthChecker checks the health of an MCP server by listing its tools
type MCPHealthChecker struct {
	config config.MCPServerDefinition
	port   int
}

// NewMCPHealthChecker creates a new MCP health checker
func NewMCPHealthChecker(cfg config.MCPServerDefinition, port int) *MCPHealthChecker {
	return &MCPHealthChecker{
		config: cfg,
		port:   port,
	}
}

// CheckHealth checks if the MCP server is healthy by attempting to list its tools
func (m *MCPHealthChecker) CheckHealth(ctx context.Context) error {
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

	// Use a client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

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

	logging.Debug("MCPHealthChecker", "MCP server %s is healthy, tools list retrieved", m.config.Name)
	return nil
}

// PortForwardHealthChecker checks the health of a port forward by attempting to connect
type PortForwardHealthChecker struct {
	config config.PortForwardDefinition
}

// NewPortForwardHealthChecker creates a new port forward health checker
func NewPortForwardHealthChecker(cfg config.PortForwardDefinition) *PortForwardHealthChecker {
	return &PortForwardHealthChecker{
		config: cfg,
	}
}

// CheckHealth checks if the port forward is healthy by attempting to connect to the local port
func (p *PortForwardHealthChecker) CheckHealth(ctx context.Context) error {
	// Create a dialer with the context
	dialer := &net.Dialer{
		Timeout: 3 * time.Second,
	}

	// LocalPort is a string in the config, so we use it directly
	address := fmt.Sprintf("localhost:%s", p.config.LocalPort)
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("failed to connect to port forward on %s: %w", address, err)
	}
	defer conn.Close()

	logging.Debug("PortForwardHealthChecker", "Port forward %s is healthy, connection successful", p.config.Name)
	return nil
}

// CheckServiceHealth performs a health check on a service and reports the result
func (o *Orchestrator) CheckServiceHealth(ctx context.Context, label string, serviceType reporting.ServiceType) {
	o.mu.RLock()
	cfg, exists := o.serviceConfigs[label]
	o.mu.RUnlock()

	if !exists {
		return
	}

	var checker ServiceHealthChecker
	var err error

	switch serviceType {
	case reporting.ServiceTypeMCPServer:
		if mcpCfg, ok := cfg.Config.(config.MCPServerDefinition); ok {
			// Get the current port from the state store
			snapshot, exists := o.reporter.GetStateStore().GetServiceState(label)
			if !exists || snapshot.ProxyPort == 0 {
				return // No port available yet
			}
			checker = NewMCPHealthChecker(mcpCfg, snapshot.ProxyPort)
		}
	case reporting.ServiceTypePortForward:
		if pfCfg, ok := cfg.Config.(config.PortForwardDefinition); ok {
			checker = NewPortForwardHealthChecker(pfCfg)
		}
	default:
		return // No health check for this service type
	}

	if checker == nil {
		return
	}

	// Perform the health check
	err = checker.CheckHealth(ctx)

	// Report the health status
	if o.reporter != nil {
		healthUpdate := reporting.ManagedServiceUpdate{
			Timestamp:   time.Now(),
			SourceType:  serviceType,
			SourceLabel: label,
			State:       reporting.StateRunning, // Keep current state
			IsReady:     err == nil,             // Health check result
			ErrorDetail: err,
			CausedBy:    "health_check",
		}

		// Get current state to preserve other fields
		if snapshot, exists := o.reporter.GetStateStore().GetServiceState(label); exists {
			healthUpdate.State = snapshot.State
			healthUpdate.ProxyPort = snapshot.ProxyPort
			healthUpdate.PID = snapshot.PID
		}

		o.reporter.Report(healthUpdate)
	}
}

// StartServiceHealthMonitoring starts periodic health checks for all running services
func (o *Orchestrator) StartServiceHealthMonitoring(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.performServiceHealthChecks(ctx)
		}
	}
}

// performServiceHealthChecks runs health checks on all running services
func (o *Orchestrator) performServiceHealthChecks(ctx context.Context) {
	states := o.reporter.GetStateStore().GetAllServiceStates()

	for label, snapshot := range states {
		// Only check services that are supposed to be running
		if snapshot.State != reporting.StateRunning {
			continue
		}

		// Run health check in a goroutine to avoid blocking
		go o.CheckServiceHealth(ctx, label, snapshot.SourceType)
	}
}
