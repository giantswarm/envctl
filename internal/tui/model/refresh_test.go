package model

import (
	"context"
	"envctl/internal/api"
	"errors"
	"testing"
)

// Mock K8sServiceAPI for testing
type mockK8sServiceAPI struct {
	listConnectionsFunc func(ctx context.Context) ([]*api.K8sConnectionInfo, error)
}

func (m *mockK8sServiceAPI) ListConnections(ctx context.Context) ([]*api.K8sConnectionInfo, error) {
	if m.listConnectionsFunc != nil {
		return m.listConnectionsFunc(ctx)
	}
	return []*api.K8sConnectionInfo{}, nil
}

func (m *mockK8sServiceAPI) GetConnectionInfo(ctx context.Context, label string) (*api.K8sConnectionInfo, error) {
	return nil, nil
}

func (m *mockK8sServiceAPI) GetConnectionByContext(ctx context.Context, contextName string) (*api.K8sConnectionInfo, error) {
	return nil, nil
}

// Mock PortForwardServiceAPI for testing
type mockPortForwardServiceAPI struct {
	listForwardsFunc func(ctx context.Context) ([]*api.PortForwardServiceInfo, error)
}

func (m *mockPortForwardServiceAPI) ListForwards(ctx context.Context) ([]*api.PortForwardServiceInfo, error) {
	if m.listForwardsFunc != nil {
		return m.listForwardsFunc(ctx)
	}
	return []*api.PortForwardServiceInfo{}, nil
}

func (m *mockPortForwardServiceAPI) GetForwardInfo(ctx context.Context, label string) (*api.PortForwardServiceInfo, error) {
	return nil, nil
}

// Mock MCPServiceAPI for testing
type mockMCPServiceAPI struct {
	listServersFunc func(ctx context.Context) ([]*api.MCPServerInfo, error)
}

func (m *mockMCPServiceAPI) ListServers(ctx context.Context) ([]*api.MCPServerInfo, error) {
	if m.listServersFunc != nil {
		return m.listServersFunc(ctx)
	}
	return []*api.MCPServerInfo{}, nil
}

func (m *mockMCPServiceAPI) GetServerInfo(ctx context.Context, label string) (*api.MCPServerInfo, error) {
	return nil, nil
}

func TestRefreshServiceData(t *testing.T) {
	tests := []struct {
		name           string
		k8sConnections []*api.K8sConnectionInfo
		portForwards   []*api.PortForwardServiceInfo
		mcpServers     []*api.MCPServerInfo
		k8sError       error
		pfError        error
		mcpError       error
		wantError      bool
	}{
		{
			name: "successful refresh",
			k8sConnections: []*api.K8sConnectionInfo{
				{Label: "k8s1", Health: "healthy"},
				{Label: "k8s2", Health: "unhealthy"},
			},
			portForwards: []*api.PortForwardServiceInfo{
				{Label: "pf1", State: "running"},
				{Label: "pf2", State: "stopped"},
			},
			mcpServers: []*api.MCPServerInfo{
				{Name: "mcp1", Port: 8080},
				{Name: "mcp2", Port: 9090},
			},
			wantError: false,
		},
		{
			name:      "k8s connections error",
			k8sError:  errors.New("k8s error"),
			wantError: true,
		},
		{
			name: "port forwards error",
			k8sConnections: []*api.K8sConnectionInfo{
				{Label: "k8s1", Health: "healthy"},
			},
			pfError:   errors.New("port forward error"),
			wantError: true,
		},
		{
			name: "mcp servers error",
			k8sConnections: []*api.K8sConnectionInfo{
				{Label: "k8s1", Health: "healthy"},
			},
			portForwards: []*api.PortForwardServiceInfo{
				{Label: "pf1", State: "running"},
			},
			mcpError:  errors.New("mcp error"),
			wantError: true,
		},
		{
			name:           "empty results",
			k8sConnections: []*api.K8sConnectionInfo{},
			portForwards:   []*api.PortForwardServiceInfo{},
			mcpServers:     []*api.MCPServerInfo{},
			wantError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Model{
				K8sServiceAPI: &mockK8sServiceAPI{
					listConnectionsFunc: func(ctx context.Context) ([]*api.K8sConnectionInfo, error) {
						return tt.k8sConnections, tt.k8sError
					},
				},
				PortForwardAPI: &mockPortForwardServiceAPI{
					listForwardsFunc: func(ctx context.Context) ([]*api.PortForwardServiceInfo, error) {
						return tt.portForwards, tt.pfError
					},
				},
				MCPServiceAPI: &mockMCPServiceAPI{
					listServersFunc: func(ctx context.Context) ([]*api.MCPServerInfo, error) {
						return tt.mcpServers, tt.mcpError
					},
				},
				K8sConnections:     make(map[string]*api.K8sConnectionInfo),
				PortForwards:       make(map[string]*api.PortForwardServiceInfo),
				MCPServers:         make(map[string]*api.MCPServerInfo),
				K8sConnectionOrder: []string{},
				PortForwardOrder:   []string{},
				MCPServerOrder:     []string{},
			}

			err := m.RefreshServiceData()

			if (err != nil) != tt.wantError {
				t.Errorf("RefreshServiceData() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError {
				// Verify K8s connections
				if len(m.K8sConnections) != len(tt.k8sConnections) {
					t.Errorf("K8sConnections count = %v, want %v", len(m.K8sConnections), len(tt.k8sConnections))
				}
				for _, conn := range tt.k8sConnections {
					if _, exists := m.K8sConnections[conn.Label]; !exists {
						t.Errorf("K8sConnection %s not found", conn.Label)
					}
				}

				// Verify port forwards
				if len(m.PortForwards) != len(tt.portForwards) {
					t.Errorf("PortForwards count = %v, want %v", len(m.PortForwards), len(tt.portForwards))
				}
				for _, pf := range tt.portForwards {
					if _, exists := m.PortForwards[pf.Label]; !exists {
						t.Errorf("PortForward %s not found", pf.Label)
					}
				}

				// Verify MCP servers
				if len(m.MCPServers) != len(tt.mcpServers) {
					t.Errorf("MCPServers count = %v, want %v", len(m.MCPServers), len(tt.mcpServers))
				}
				for _, mcp := range tt.mcpServers {
					if _, exists := m.MCPServers[mcp.Name]; !exists {
						t.Errorf("MCPServer %s not found", mcp.Name)
					}
				}

				// Verify ordering is preserved
				if len(m.K8sConnectionOrder) != len(tt.k8sConnections) {
					t.Errorf("K8sConnectionOrder length = %v, want %v", len(m.K8sConnectionOrder), len(tt.k8sConnections))
				}
				if len(m.PortForwardOrder) != len(tt.portForwards) {
					t.Errorf("PortForwardOrder length = %v, want %v", len(m.PortForwardOrder), len(tt.portForwards))
				}
				if len(m.MCPServerOrder) != len(tt.mcpServers) {
					t.Errorf("MCPServerOrder length = %v, want %v", len(m.MCPServerOrder), len(tt.mcpServers))
				}
			}
		})
	}
}

func TestRefreshServiceData_PreservesOrder(t *testing.T) {
	// Test that subsequent refreshes preserve the original order
	m := &Model{
		K8sServiceAPI: &mockK8sServiceAPI{
			listConnectionsFunc: func(ctx context.Context) ([]*api.K8sConnectionInfo, error) {
				return []*api.K8sConnectionInfo{
					{Label: "k8s1"},
					{Label: "k8s2"},
				}, nil
			},
		},
		PortForwardAPI: &mockPortForwardServiceAPI{
			listForwardsFunc: func(ctx context.Context) ([]*api.PortForwardServiceInfo, error) {
				return []*api.PortForwardServiceInfo{
					{Label: "pf1"},
					{Label: "pf2"},
				}, nil
			},
		},
		MCPServiceAPI: &mockMCPServiceAPI{
			listServersFunc: func(ctx context.Context) ([]*api.MCPServerInfo, error) {
				return []*api.MCPServerInfo{
					{Name: "mcp1"},
					{Name: "mcp2"},
				}, nil
			},
		},
		K8sConnections:     make(map[string]*api.K8sConnectionInfo),
		PortForwards:       make(map[string]*api.PortForwardServiceInfo),
		MCPServers:         make(map[string]*api.MCPServerInfo),
		K8sConnectionOrder: []string{},
		PortForwardOrder:   []string{},
		MCPServerOrder:     []string{},
	}

	// First refresh
	err := m.RefreshServiceData()
	if err != nil {
		t.Fatalf("First RefreshServiceData() error = %v", err)
	}

	// Store original order
	origK8sOrder := make([]string, len(m.K8sConnectionOrder))
	copy(origK8sOrder, m.K8sConnectionOrder)

	origPFOrder := make([]string, len(m.PortForwardOrder))
	copy(origPFOrder, m.PortForwardOrder)

	origMCPOrder := make([]string, len(m.MCPServerOrder))
	copy(origMCPOrder, m.MCPServerOrder)

	// Update APIs to return different order
	m.K8sServiceAPI = &mockK8sServiceAPI{
		listConnectionsFunc: func(ctx context.Context) ([]*api.K8sConnectionInfo, error) {
			return []*api.K8sConnectionInfo{
				{Label: "k8s2"}, // Swapped order
				{Label: "k8s1"},
			}, nil
		},
	}

	// Second refresh
	err = m.RefreshServiceData()
	if err != nil {
		t.Fatalf("Second RefreshServiceData() error = %v", err)
	}

	// Verify order is preserved
	for i, label := range origK8sOrder {
		if m.K8sConnectionOrder[i] != label {
			t.Errorf("K8sConnectionOrder[%d] = %v, want %v", i, m.K8sConnectionOrder[i], label)
		}
	}
}
