package orchestrator

import (
	"context"
	"envctl/internal/config"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_registerServices(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		check   func(*testing.T, *Orchestrator)
	}{
		{
			name: "registers all service types",
			cfg: Config{
				MCName: "test-mc",
				WCName: "test-wc",
				PortForwards: []config.PortForwardDefinition{
					{Name: "pf1", Enabled: true},
					{Name: "pf2", Enabled: false}, // Should be skipped
				},
				MCPServers: []config.MCPServerDefinition{
					{Name: "mcp1", Enabled: true},
					{Name: "mcp2", Enabled: false}, // Should be skipped
				},
			},
			check: func(t *testing.T, o *Orchestrator) {
				// Check K8s services
				mcService, exists := o.registry.Get("k8s-mc-test-mc")
				assert.True(t, exists)
				assert.NotNil(t, mcService)

				wcService, exists := o.registry.Get("k8s-wc-test-wc")
				assert.True(t, exists)
				assert.NotNil(t, wcService)

				// Check port forward services
				pf1Service, exists := o.registry.Get("pf1")
				assert.True(t, exists)
				assert.NotNil(t, pf1Service)

				// pf2 should not be registered (disabled)
				_, exists = o.registry.Get("pf2")
				assert.False(t, exists)

				// Check MCP services
				mcp1Service, exists := o.registry.Get("mcp1")
				assert.True(t, exists)
				assert.NotNil(t, mcp1Service)

				// mcp2 should not be registered (disabled)
				_, exists = o.registry.Get("mcp2")
				assert.False(t, exists)
			},
		},
		{
			name: "registers only enabled services",
			cfg: Config{
				MCName: "test-mc",
				PortForwards: []config.PortForwardDefinition{
					{Name: "pf1", Enabled: false},
					{Name: "pf2", Enabled: false},
				},
				MCPServers: []config.MCPServerDefinition{
					{Name: "mcp1", Enabled: false},
				},
			},
			check: func(t *testing.T, o *Orchestrator) {
				// Only MC service should be registered
				allServices := o.registry.GetAll()
				assert.Len(t, allServices, 1)
				assert.Equal(t, "k8s-mc-test-mc", allServices[0].GetLabel())
			},
		},
		{
			name: "empty config registers no services",
			cfg:  Config{},
			check: func(t *testing.T, o *Orchestrator) {
				allServices := o.registry.GetAll()
				assert.Empty(t, allServices)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(tt.cfg)
			err := o.registerServices()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.check != nil {
					tt.check(t, o)
				}
			}
		})
	}
}

func TestOrchestrator_registerK8sServices(t *testing.T) {
	tests := []struct {
		name  string
		cfg   Config
		check func(*testing.T, *Orchestrator)
	}{
		{
			name: "registers MC only",
			cfg: Config{
				MCName: "test-mc",
			},
			check: func(t *testing.T, o *Orchestrator) {
				mcService, exists := o.registry.Get("k8s-mc-test-mc")
				assert.True(t, exists)
				assert.NotNil(t, mcService)

				// No WC service should be registered
				_, exists = o.registry.Get("k8s-wc-test-wc")
				assert.False(t, exists)
			},
		},
		{
			name: "registers MC and WC",
			cfg: Config{
				MCName: "test-mc",
				WCName: "test-wc",
			},
			check: func(t *testing.T, o *Orchestrator) {
				mcService, exists := o.registry.Get("k8s-mc-test-mc")
				assert.True(t, exists)
				assert.NotNil(t, mcService)

				wcService, exists := o.registry.Get("k8s-wc-test-wc")
				assert.True(t, exists)
				assert.NotNil(t, wcService)
			},
		},
		{
			name: "skips WC if no MC",
			cfg: Config{
				WCName: "test-wc", // No MC name
			},
			check: func(t *testing.T, o *Orchestrator) {
				// No services should be registered
				allServices := o.registry.GetAll()
				assert.Empty(t, allServices)
			},
		},
		{
			name: "empty config registers nothing",
			cfg:  Config{},
			check: func(t *testing.T, o *Orchestrator) {
				allServices := o.registry.GetAll()
				assert.Empty(t, allServices)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(tt.cfg)
			err := o.registerK8sServices()
			require.NoError(t, err)

			if tt.check != nil {
				tt.check(t, o)
			}
		})
	}
}

func TestOrchestrator_registerPortForwardServices(t *testing.T) {
	tests := []struct {
		name  string
		cfg   Config
		check func(*testing.T, *Orchestrator)
	}{
		{
			name: "registers enabled port forwards",
			cfg: Config{
				PortForwards: []config.PortForwardDefinition{
					{Name: "pf1", Enabled: true},
					{Name: "pf2", Enabled: true},
					{Name: "pf3", Enabled: false}, // Should be skipped
				},
			},
			check: func(t *testing.T, o *Orchestrator) {
				pf1, exists := o.registry.Get("pf1")
				assert.True(t, exists)
				assert.NotNil(t, pf1)

				pf2, exists := o.registry.Get("pf2")
				assert.True(t, exists)
				assert.NotNil(t, pf2)

				// pf3 should not be registered
				_, exists = o.registry.Get("pf3")
				assert.False(t, exists)
			},
		},
		{
			name: "skips all disabled port forwards",
			cfg: Config{
				PortForwards: []config.PortForwardDefinition{
					{Name: "pf1", Enabled: false},
					{Name: "pf2", Enabled: false},
				},
			},
			check: func(t *testing.T, o *Orchestrator) {
				allServices := o.registry.GetAll()
				assert.Empty(t, allServices)
			},
		},
		{
			name: "empty port forwards list",
			cfg:  Config{},
			check: func(t *testing.T, o *Orchestrator) {
				allServices := o.registry.GetAll()
				assert.Empty(t, allServices)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(tt.cfg)
			err := o.registerPortForwardServices()
			require.NoError(t, err)

			if tt.check != nil {
				tt.check(t, o)
			}
		})
	}
}

func TestOrchestrator_registerMCPServices(t *testing.T) {
	tests := []struct {
		name  string
		cfg   Config
		check func(*testing.T, *Orchestrator)
	}{
		{
			name: "registers enabled MCP servers",
			cfg: Config{
				MCPServers: []config.MCPServerDefinition{
					{Name: "mcp1", Enabled: true},
					{Name: "mcp2", Enabled: true},
					{Name: "mcp3", Enabled: false}, // Should be skipped
				},
			},
			check: func(t *testing.T, o *Orchestrator) {
				mcp1, exists := o.registry.Get("mcp1")
				assert.True(t, exists)
				assert.NotNil(t, mcp1)

				mcp2, exists := o.registry.Get("mcp2")
				assert.True(t, exists)
				assert.NotNil(t, mcp2)

				// mcp3 should not be registered
				_, exists = o.registry.Get("mcp3")
				assert.False(t, exists)
			},
		},
		{
			name: "skips all disabled MCP servers",
			cfg: Config{
				MCPServers: []config.MCPServerDefinition{
					{Name: "mcp1", Enabled: false},
					{Name: "mcp2", Enabled: false},
				},
			},
			check: func(t *testing.T, o *Orchestrator) {
				allServices := o.registry.GetAll()
				assert.Empty(t, allServices)
			},
		},
		{
			name: "empty MCP servers list",
			cfg:  Config{},
			check: func(t *testing.T, o *Orchestrator) {
				allServices := o.registry.GetAll()
				assert.Empty(t, allServices)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(tt.cfg)
			err := o.registerMCPServices()
			require.NoError(t, err)

			if tt.check != nil {
				tt.check(t, o)
			}
		})
	}
}

func TestOrchestrator_StartWithContext(t *testing.T) {
	o := New(Config{MCName: "test-mc"})

	// Test context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := o.Start(ctx)
	assert.NoError(t, err) // Start should succeed even with cancelled context

	// The orchestrator should create its own context
	assert.NotNil(t, o.ctx)
	assert.NotNil(t, o.cancelFunc)

	// Clean up
	o.Stop()
}
