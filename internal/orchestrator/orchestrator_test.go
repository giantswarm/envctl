package orchestrator

import (
	"context"
	"envctl/internal/config"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want func(*testing.T, *Orchestrator)
	}{
		{
			name: "creates orchestrator with empty config",
			cfg:  Config{},
			want: func(t *testing.T, o *Orchestrator) {
				assert.NotNil(t, o)
				assert.NotNil(t, o.registry)
				assert.NotNil(t, o.kubeMgr)
				assert.NotNil(t, o.stopReasons)
				assert.NotNil(t, o.pendingRestarts)
				assert.NotNil(t, o.healthCheckers)
				assert.Empty(t, o.mcName)
				assert.Empty(t, o.wcName)
				assert.Empty(t, o.portForwards)
				assert.Empty(t, o.mcpServers)
			},
		},
		{
			name: "creates orchestrator with full config",
			cfg: Config{
				MCName: "test-mc",
				WCName: "test-wc",
				PortForwards: []config.PortForwardDefinition{
					{Name: "pf1", Enabled: true},
				},
				MCPServers: []config.MCPServerDefinition{
					{Name: "mcp1", Enabled: true},
				},
			},
			want: func(t *testing.T, o *Orchestrator) {
				assert.NotNil(t, o)
				assert.Equal(t, "test-mc", o.mcName)
				assert.Equal(t, "test-wc", o.wcName)
				assert.Len(t, o.portForwards, 1)
				assert.Len(t, o.mcpServers, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.cfg)
			tt.want(t, got)
		})
	}
}

func TestOrchestrator_Start(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "starts with empty config",
			cfg:  Config{},
		},
		{
			name: "starts with MC only",
			cfg: Config{
				MCName: "test-mc",
			},
		},
		{
			name: "starts with MC and WC",
			cfg: Config{
				MCName: "test-mc",
				WCName: "test-wc",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(tt.cfg)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			err := o.Start(ctx)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, o.ctx)
				assert.NotNil(t, o.cancelFunc)
				assert.NotNil(t, o.depGraph)
			}

			// Clean up
			o.Stop()
		})
	}
}

func TestOrchestrator_Stop(t *testing.T) {
	o := New(Config{MCName: "test-mc"})
	ctx := context.Background()

	// Start the orchestrator
	err := o.Start(ctx)
	require.NoError(t, err)

	// Stop should work without error
	err = o.Stop()
	assert.NoError(t, err)

	// Multiple stops should be safe
	err = o.Stop()
	assert.NoError(t, err)
}

func TestOrchestrator_StartService(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(*mockService)
		serviceLabel string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "service not found",
			serviceLabel: "non-existent",
			wantErr:      true,
			errContains:  "not found",
		},
		{
			name: "service starts successfully",
			setupMock: func(m *mockService) {
				m.label = "test-service"
				m.startFunc = func(ctx context.Context) error {
					return nil
				}
			},
			serviceLabel: "test-service",
			wantErr:      false,
		},
		{
			name: "service start fails",
			setupMock: func(m *mockService) {
				m.label = "test-service"
				m.startFunc = func(ctx context.Context) error {
					return assert.AnError
				}
			},
			serviceLabel: "test-service",
			wantErr:      true,
			errContains:  "failed to start service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(Config{})
			ctx := context.Background()
			err := o.Start(ctx)
			require.NoError(t, err)
			defer o.Stop()

			if tt.setupMock != nil {
				mock := &mockService{}
				tt.setupMock(mock)
				o.registry.Register(mock)
			}

			err = o.StartService(tt.serviceLabel)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOrchestrator_StopService(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(*mockService)
		serviceLabel string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "service not found",
			serviceLabel: "non-existent",
			wantErr:      true,
			errContains:  "not found",
		},
		{
			name: "service stops successfully",
			setupMock: func(m *mockService) {
				m.label = "test-service"
				m.stopFunc = func(ctx context.Context) error {
					return nil
				}
			},
			serviceLabel: "test-service",
			wantErr:      false,
		},
		{
			name: "service stop fails",
			setupMock: func(m *mockService) {
				m.label = "test-service"
				m.stopFunc = func(ctx context.Context) error {
					return assert.AnError
				}
			},
			serviceLabel: "test-service",
			wantErr:      true,
			errContains:  "failed to stop service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(Config{})
			ctx := context.Background()
			err := o.Start(ctx)
			require.NoError(t, err)
			defer o.Stop()

			if tt.setupMock != nil {
				mock := &mockService{}
				tt.setupMock(mock)
				o.registry.Register(mock)
			}

			err = o.StopService(tt.serviceLabel)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				// Verify service was marked as manually stopped
				o.mu.RLock()
				reason, exists := o.stopReasons[tt.serviceLabel]
				o.mu.RUnlock()
				assert.True(t, exists)
				assert.Equal(t, StopReasonManual, reason)
			}
		})
	}
}

func TestOrchestrator_RestartService(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(*mockService)
		serviceLabel string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "service not found",
			serviceLabel: "non-existent",
			wantErr:      true,
			errContains:  "not found",
		},
		{
			name: "service restarts successfully",
			setupMock: func(m *mockService) {
				m.label = "test-service"
				m.restartFunc = func(ctx context.Context) error {
					return nil
				}
			},
			serviceLabel: "test-service",
			wantErr:      false,
		},
		{
			name: "service restart fails",
			setupMock: func(m *mockService) {
				m.label = "test-service"
				m.restartFunc = func(ctx context.Context) error {
					return assert.AnError
				}
			},
			serviceLabel: "test-service",
			wantErr:      true,
			errContains:  "failed to restart service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(Config{})
			ctx := context.Background()
			err := o.Start(ctx)
			require.NoError(t, err)
			defer o.Stop()

			if tt.setupMock != nil {
				mock := &mockService{}
				tt.setupMock(mock)
				o.registry.Register(mock)
			}

			err = o.RestartService(tt.serviceLabel)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOrchestrator_GetServiceRegistry(t *testing.T) {
	o := New(Config{})
	registry := o.GetServiceRegistry()
	assert.NotNil(t, registry)
	assert.Equal(t, o.registry, registry)
}
