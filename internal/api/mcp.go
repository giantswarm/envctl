package api

import (
	"bytes"
	"context"
	"encoding/json"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// mcpServerAPI implements the MCPServerAPI interface
type mcpServerAPI struct {
	eventBus   reporting.EventBus
	stateStore reporting.StateStore
	toolCache  *cache.Cache
	httpClient *http.Client
	mu         sync.RWMutex
}

// NewMCPServerAPI creates a new MCP server API implementation
func NewMCPServerAPI(eventBus reporting.EventBus, stateStore reporting.StateStore) MCPServerAPI {
	return &mcpServerAPI{
		eventBus:   eventBus,
		stateStore: stateStore,
		toolCache:  cache.New(5*time.Minute, 10*time.Minute), // 5 min default expiration, 10 min cleanup
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetTools returns the list of tools exposed by an MCP server
func (m *mcpServerAPI) GetTools(ctx context.Context, serverName string) ([]MCPTool, error) {
	// Check cache first
	if cached, found := m.toolCache.Get(serverName); found {
		logging.Debug("MCPServerAPI", "Returning cached tools for %s", serverName)
		return cached.([]MCPTool), nil
	}

	// Get server status to find proxy port
	status, err := m.GetServerStatus(serverName)
	if err != nil {
		return nil, fmt.Errorf("failed to get server status: %w", err)
	}

	if status.State != reporting.StateRunning {
		return nil, fmt.Errorf("MCP server %s is not running (state: %s)", serverName, status.State)
	}

	if status.ProxyPort == 0 {
		return nil, fmt.Errorf("MCP server %s has no proxy port configured", serverName)
	}

	// Fetch tools from MCP server
	tools, err := m.fetchToolsFromServer(ctx, serverName, status.ProxyPort)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tools from %s: %w", serverName, err)
	}

	// Cache the result
	m.toolCache.Set(serverName, tools, cache.DefaultExpiration)
	logging.Info("MCPServerAPI", "Fetched and cached %d tools for %s", len(tools), serverName)

	return tools, nil
}

// fetchToolsFromServer makes the actual HTTP request to get tools
func (m *mcpServerAPI) fetchToolsFromServer(ctx context.Context, serverName string, port int) ([]MCPTool, error) {
	url := fmt.Sprintf("http://localhost:%d/message", port)

	// Create JSON-RPC request
	requestBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("MCP server returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Tools []struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				InputSchema json.RawMessage `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for JSON-RPC error
	if result.Error != nil {
		return nil, fmt.Errorf("MCP server error %d: %s", result.Error.Code, result.Error.Message)
	}

	// Convert to our MCPTool type
	tools := make([]MCPTool, len(result.Result.Tools))
	for i, tool := range result.Result.Tools {
		tools[i] = MCPTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		}
	}

	return tools, nil
}

// GetToolDetails returns detailed information about a specific tool
func (m *mcpServerAPI) GetToolDetails(ctx context.Context, serverName string, toolName string) (*MCPToolDetails, error) {
	// For now, just get the basic tool info
	tools, err := m.GetTools(ctx, serverName)
	if err != nil {
		return nil, err
	}

	for _, tool := range tools {
		if tool.Name == toolName {
			return &MCPToolDetails{
				MCPTool:     tool,
				Examples:    []MCPToolExample{}, // TODO: Fetch examples if available
				LastUpdated: time.Now(),
			}, nil
		}
	}

	return nil, fmt.Errorf("tool %s not found in server %s", toolName, serverName)
}

// ExecuteTool executes a tool and returns the result
func (m *mcpServerAPI) ExecuteTool(ctx context.Context, serverName string, toolName string, params map[string]interface{}) (interface{}, error) {
	// TODO: Implement tool execution
	return nil, fmt.Errorf("tool execution not yet implemented")
}

// GetServerStatus returns the current status of an MCP server
func (m *mcpServerAPI) GetServerStatus(serverName string) (*MCPServerStatus, error) {
	// Get state from state store
	snapshot, exists := m.stateStore.GetServiceState(serverName)
	if !exists {
		return nil, fmt.Errorf("server %s not found in state store", serverName)
	}

	// Get tool count from cache if available
	toolCount := 0
	if cached, found := m.toolCache.Get(serverName); found {
		tools := cached.([]MCPTool)
		toolCount = len(tools)
	}

	return &MCPServerStatus{
		Name:      serverName,
		State:     snapshot.State,
		ProxyPort: snapshot.ProxyPort,
		ToolCount: toolCount,
		LastCheck: snapshot.LastUpdated,
		Error:     snapshot.ErrorDetail,
	}, nil
}

// SubscribeToToolUpdates subscribes to tool list changes
func (m *mcpServerAPI) SubscribeToToolUpdates(serverName string) <-chan MCPToolUpdateEvent {
	ch := make(chan MCPToolUpdateEvent, 10)

	// Subscribe to service state changes
	filter := reporting.CombineFilters(
		reporting.FilterByType(reporting.EventTypeServiceRunning, reporting.EventTypeServiceFailed),
		reporting.FilterBySource(serverName),
	)

	subscription := m.eventBus.Subscribe(filter, func(event reporting.Event) {
		if serviceEvent, ok := event.(*reporting.ServiceStateEvent); ok {
			// When service becomes running, refresh tools
			if serviceEvent.NewState == reporting.StateRunning && serviceEvent.OldState != reporting.StateRunning {
				go m.refreshToolsAsync(serverName, ch)
			}
			// When service fails, clear cache and notify
			if serviceEvent.NewState == reporting.StateFailed {
				m.toolCache.Delete(serverName)
				ch <- MCPToolUpdateEvent{
					ServerName: serverName,
					EventType:  "cleared",
					Tools:      []MCPTool{},
					Timestamp:  time.Now(),
				}
			}
		}
	})

	// Clean up subscription when channel is closed
	go func() {
		// Wait for channel to be closed by consumer
		for range ch {
			// Drain channel
		}
		m.eventBus.Unsubscribe(subscription)
	}()

	// Do initial fetch if server is already running
	status, err := m.GetServerStatus(serverName)
	if err == nil && status.State == reporting.StateRunning {
		go m.refreshToolsAsync(serverName, ch)
	}

	return ch
}

// refreshToolsAsync fetches tools and sends update event
func (m *mcpServerAPI) refreshToolsAsync(serverName string, ch chan<- MCPToolUpdateEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tools, err := m.GetTools(ctx, serverName)
	if err != nil {
		logging.Error("MCPServerAPI", err, "Failed to refresh tools for %s", serverName)
		return
	}

	select {
	case ch <- MCPToolUpdateEvent{
		ServerName: serverName,
		EventType:  "refreshed",
		Tools:      tools,
		Timestamp:  time.Now(),
	}:
		logging.Debug("MCPServerAPI", "Sent tool update event for %s with %d tools", serverName, len(tools))
	default:
		logging.Warn("MCPServerAPI", "Tool update channel full for %s, dropping event", serverName)
	}
}
