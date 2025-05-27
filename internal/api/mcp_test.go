package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"envctl/internal/reporting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock implementations for testing
type mockEventBus struct {
	subscriptions []*reporting.EventSubscription
}

func (m *mockEventBus) Publish(event reporting.Event) {}

func (m *mockEventBus) Subscribe(filter reporting.EventFilter, handler reporting.EventHandler) *reporting.EventSubscription {
	sub := &reporting.EventSubscription{
		ID:      "test-sub",
		Filter:  filter,
		Handler: handler,
	}
	m.subscriptions = append(m.subscriptions, sub)
	return sub
}

func (m *mockEventBus) SubscribeChannel(filter reporting.EventFilter, bufferSize int) *reporting.EventSubscription {
	return &reporting.EventSubscription{ID: "test-sub"}
}

func (m *mockEventBus) Unsubscribe(subscription *reporting.EventSubscription) {}

func (m *mockEventBus) GetMetrics() reporting.EventBusMetrics {
	return reporting.EventBusMetrics{}
}

func (m *mockEventBus) Close() {}

type mockStateStore struct {
	states map[string]reporting.ServiceStateSnapshot
}

func newMockStateStore() *mockStateStore {
	return &mockStateStore{
		states: make(map[string]reporting.ServiceStateSnapshot),
	}
}

func (m *mockStateStore) GetServiceState(label string) (reporting.ServiceStateSnapshot, bool) {
	state, exists := m.states[label]
	return state, exists
}

func (m *mockStateStore) SetServiceState(update reporting.ManagedServiceUpdate) (bool, error) {
	snapshot := reporting.ServiceStateSnapshot{
		Label:       update.SourceLabel,
		SourceType:  update.SourceType,
		State:       update.State,
		ProxyPort:   update.ProxyPort,
		LastUpdated: time.Now(),
	}
	m.states[update.SourceLabel] = snapshot
	return true, nil
}

func (m *mockStateStore) GetAllServiceStates() map[string]reporting.ServiceStateSnapshot {
	return m.states
}

func (m *mockStateStore) GetServicesByType(serviceType reporting.ServiceType) map[string]reporting.ServiceStateSnapshot {
	result := make(map[string]reporting.ServiceStateSnapshot)
	for k, v := range m.states {
		if v.SourceType == serviceType {
			result[k] = v
		}
	}
	return result
}

func (m *mockStateStore) GetServicesByState(state reporting.ServiceState) map[string]reporting.ServiceStateSnapshot {
	result := make(map[string]reporting.ServiceStateSnapshot)
	for k, v := range m.states {
		if v.State == state {
			result[k] = v
		}
	}
	return result
}

func (m *mockStateStore) Subscribe(label string) *reporting.StateSubscription {
	return &reporting.StateSubscription{ID: "test-sub"}
}

func (m *mockStateStore) Unsubscribe(subscription *reporting.StateSubscription) {}

func (m *mockStateStore) Clear(label string) bool {
	delete(m.states, label)
	return true
}

func (m *mockStateStore) ClearAll() {
	m.states = make(map[string]reporting.ServiceStateSnapshot)
}

func (m *mockStateStore) GetMetrics() reporting.StateStoreMetrics {
	return reporting.StateStoreMetrics{}
}

func (m *mockStateStore) RecordStateTransition(transition reporting.StateTransition) error {
	return nil
}

func (m *mockStateStore) GetStateTransitions(label string) []reporting.StateTransition {
	return []reporting.StateTransition{}
}

func (m *mockStateStore) GetAllStateTransitions() []reporting.StateTransition {
	return []reporting.StateTransition{}
}

func (m *mockStateStore) RecordCascadeOperation(cascade reporting.CascadeInfo) error {
	return nil
}

func (m *mockStateStore) GetCascadeOperations() []reporting.CascadeInfo {
	return []reporting.CascadeInfo{}
}

func (m *mockStateStore) GetCascadesByCorrelationID(correlationID string) []reporting.CascadeInfo {
	return []reporting.CascadeInfo{}
}

// Test helpers
func createTestMCPServer(t *testing.T, tools []MCPTool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		if req["method"] == "tools/list" {
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"tools": tools,
				},
			}
			json.NewEncoder(w).Encode(response)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// Tests
func TestMCPServerAPI_GetServerStatus(t *testing.T) {
	eventBus := &mockEventBus{}
	stateStore := newMockStateStore()

	// Add a test server to state store
	stateStore.states["test-server"] = reporting.ServiceStateSnapshot{
		Label:       "test-server",
		SourceType:  reporting.ServiceTypeMCPServer,
		State:       reporting.StateRunning,
		ProxyPort:   8001,
		LastUpdated: time.Now(),
	}

	api := NewMCPServerAPI(eventBus, stateStore)

	t.Run("existing server", func(t *testing.T) {
		status, err := api.GetServerStatus("test-server")
		require.NoError(t, err)
		assert.Equal(t, "test-server", status.Name)
		assert.Equal(t, reporting.StateRunning, status.State)
		assert.Equal(t, 8001, status.ProxyPort)
	})

	t.Run("non-existent server", func(t *testing.T) {
		status, err := api.GetServerStatus("non-existent")
		assert.Error(t, err)
		assert.Nil(t, status)
		assert.Contains(t, err.Error(), "not found in state store")
	})
}

func TestMCPServerAPI_GetTools(t *testing.T) {
	eventBus := &mockEventBus{}
	stateStore := newMockStateStore()

	// Create test tools
	testTools := []MCPTool{
		{
			Name:        "test-tool-1",
			Description: "Test tool 1",
			InputSchema: json.RawMessage(`{"type": "object"}`),
		},
		{
			Name:        "test-tool-2",
			Description: "Test tool 2",
			InputSchema: json.RawMessage(`{"type": "string"}`),
		},
	}

	// Create test MCP server
	server := createTestMCPServer(t, testTools)
	defer server.Close()

	// Extract port from test server URL
	var port int
	_, err := fmt.Sscanf(server.URL, "http://127.0.0.1:%d", &port)
	require.NoError(t, err)

	// Add server to state store
	stateStore.states["test-server"] = reporting.ServiceStateSnapshot{
		Label:       "test-server",
		SourceType:  reporting.ServiceTypeMCPServer,
		State:       reporting.StateRunning,
		ProxyPort:   port,
		LastUpdated: time.Now(),
	}

	api := NewMCPServerAPI(eventBus, stateStore)

	t.Run("fetch tools from running server", func(t *testing.T) {
		ctx := context.Background()
		tools, err := api.GetTools(ctx, "test-server")
		require.NoError(t, err)
		assert.Len(t, tools, 2)
		assert.Equal(t, "test-tool-1", tools[0].Name)
		assert.Equal(t, "test-tool-2", tools[1].Name)
	})

	t.Run("cached tools", func(t *testing.T) {
		// Second call should use cache
		ctx := context.Background()
		tools, err := api.GetTools(ctx, "test-server")
		require.NoError(t, err)
		assert.Len(t, tools, 2)
	})

	t.Run("server not running", func(t *testing.T) {
		// Update server state to stopped
		stateStore.states["test-server"] = reporting.ServiceStateSnapshot{
			Label:       "test-server",
			SourceType:  reporting.ServiceTypeMCPServer,
			State:       reporting.StateStopped,
			ProxyPort:   port,
			LastUpdated: time.Now(),
		}

		// Clear cache by creating new API instance
		api2 := NewMCPServerAPI(eventBus, stateStore)
		ctx := context.Background()
		tools, err := api2.GetTools(ctx, "test-server")
		assert.Error(t, err)
		assert.Nil(t, tools)
		assert.Contains(t, err.Error(), "not running")
	})
}

func TestMCPServerAPI_GetToolDetails(t *testing.T) {
	eventBus := &mockEventBus{}
	stateStore := newMockStateStore()

	// Create test tools
	testTools := []MCPTool{
		{
			Name:        "test-tool",
			Description: "Test tool",
			InputSchema: json.RawMessage(`{"type": "object"}`),
		},
	}

	// Create test MCP server
	server := createTestMCPServer(t, testTools)
	defer server.Close()

	// Extract port from test server URL
	var port int
	_, err := fmt.Sscanf(server.URL, "http://127.0.0.1:%d", &port)
	require.NoError(t, err)

	// Add server to state store
	stateStore.states["test-server"] = reporting.ServiceStateSnapshot{
		Label:       "test-server",
		SourceType:  reporting.ServiceTypeMCPServer,
		State:       reporting.StateRunning,
		ProxyPort:   port,
		LastUpdated: time.Now(),
	}

	api := NewMCPServerAPI(eventBus, stateStore)

	t.Run("get existing tool details", func(t *testing.T) {
		ctx := context.Background()
		details, err := api.GetToolDetails(ctx, "test-server", "test-tool")
		require.NoError(t, err)
		assert.Equal(t, "test-tool", details.Name)
		assert.Equal(t, "Test tool", details.Description)
		assert.NotNil(t, details.LastUpdated)
	})

	t.Run("tool not found", func(t *testing.T) {
		ctx := context.Background()
		details, err := api.GetToolDetails(ctx, "test-server", "non-existent")
		assert.Error(t, err)
		assert.Nil(t, details)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestMCPServerAPI_SubscribeToToolUpdates(t *testing.T) {
	eventBus := &mockEventBus{}
	stateStore := newMockStateStore()

	// Add server to state store
	stateStore.states["test-server"] = reporting.ServiceStateSnapshot{
		Label:       "test-server",
		SourceType:  reporting.ServiceTypeMCPServer,
		State:       reporting.StateRunning,
		ProxyPort:   8001,
		LastUpdated: time.Now(),
	}

	api := NewMCPServerAPI(eventBus, stateStore)

	t.Run("subscribe to updates", func(t *testing.T) {
		ch := api.SubscribeToToolUpdates("test-server")
		assert.NotNil(t, ch)

		// Should have created a subscription
		assert.Len(t, eventBus.subscriptions, 1)

		// Drain channel to prevent goroutine leak
		go func() {
			for range ch {
				// Drain
			}
		}()
	})
}
