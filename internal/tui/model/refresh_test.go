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

func TestRefreshServiceData(t *testing.T) {
	tests := []struct {
		name           string
		k8sConnections []*api.K8sConnectionInfo
		portForwards   []*api.PortForwardServiceInfo
		k8sError       error
		pfError        error
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
			name:           "empty results",
			k8sConnections: []*api.K8sConnectionInfo{},
			portForwards:   []*api.PortForwardServiceInfo{},
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
				OrchestratorAPI:    &mockOrchestratorAPI{},
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

				// Verify ordering is preserved
				if len(m.K8sConnectionOrder) != len(tt.k8sConnections) {
					t.Errorf("K8sConnectionOrder length = %v, want %v", len(m.K8sConnectionOrder), len(tt.k8sConnections))
				}
				if len(m.PortForwardOrder) != len(tt.portForwards) {
					t.Errorf("PortForwardOrder length = %v, want %v", len(m.PortForwardOrder), len(tt.portForwards))
				}
			}
		})
	}
}
