package orchestrator

import (
	"context"
	"envctl/internal/config"
	"envctl/internal/dependency"
	"envctl/internal/services"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_buildDependencyGraph(t *testing.T) {
	tests := []struct {
		name  string
		cfg   Config
		check func(*testing.T, *dependency.Graph)
	}{
		{
			name: "builds graph with MC only",
			cfg: Config{
				MCName: "test-mc",
			},
			check: func(t *testing.T, g *dependency.Graph) {
				mcNode := g.Get(dependency.NodeID("k8s-mc-test-mc"))
				assert.NotNil(t, mcNode)
				assert.Equal(t, "K8s MC: test-mc", mcNode.FriendlyName)
				assert.Equal(t, dependency.KindK8sConnection, mcNode.Kind)
				assert.Empty(t, mcNode.DependsOn)
			},
		},
		{
			name: "builds graph with MC and WC",
			cfg: Config{
				MCName: "test-mc",
				WCName: "test-wc",
			},
			check: func(t *testing.T, g *dependency.Graph) {
				// Check MC node
				mcNode := g.Get(dependency.NodeID("k8s-mc-test-mc"))
				assert.NotNil(t, mcNode)
				assert.Empty(t, mcNode.DependsOn)

				// Check WC node
				wcNode := g.Get(dependency.NodeID("k8s-wc-test-wc"))
				assert.NotNil(t, wcNode)
				assert.Empty(t, wcNode.DependsOn)
			},
		},
		{
			name: "builds graph with port forwards",
			cfg: Config{
				MCName: "test-mc",
				WCName: "test-wc",
				PortForwards: []config.PortForwardDefinition{
					{
						Name:              "pf1",
						Enabled:           true,
						KubeContextTarget: "gs-test-mc",
					},
					{
						Name:              "pf2",
						Enabled:           true,
						KubeContextTarget: "gs-test-mc-test-wc",
					},
					{
						Name:    "pf3",
						Enabled: false, // Should be skipped
					},
				},
			},
			check: func(t *testing.T, g *dependency.Graph) {
				// Check pf1 depends on MC
				pf1Node := g.Get(dependency.NodeID("pf:pf1"))
				assert.NotNil(t, pf1Node)
				assert.Equal(t, dependency.KindPortForward, pf1Node.Kind)
				assert.Contains(t, pf1Node.DependsOn, dependency.NodeID("k8s-mc-test-mc"))

				// Check pf2 depends on WC
				pf2Node := g.Get(dependency.NodeID("pf:pf2"))
				assert.NotNil(t, pf2Node)
				assert.Contains(t, pf2Node.DependsOn, dependency.NodeID("k8s-wc-test-wc"))

				// pf3 should not be in graph
				pf3Node := g.Get(dependency.NodeID("pf:pf3"))
				assert.Nil(t, pf3Node)
			},
		},
		{
			name: "builds graph with MCP servers",
			cfg: Config{
				PortForwards: []config.PortForwardDefinition{
					{Name: "pf1", Enabled: true},
					{Name: "pf2", Enabled: true},
				},
				MCPServers: []config.MCPServerDefinition{
					{
						Name:                 "mcp1",
						Enabled:              true,
						RequiresPortForwards: []string{"pf1", "pf2"},
					},
					{
						Name:                 "mcp2",
						Enabled:              true,
						RequiresPortForwards: []string{"pf3"}, // Non-existent PF
					},
				},
			},
			check: func(t *testing.T, g *dependency.Graph) {
				// Check mcp1 depends on pf1 and pf2
				mcp1Node := g.Get(dependency.NodeID("mcp:mcp1"))
				assert.NotNil(t, mcp1Node)
				assert.Equal(t, dependency.KindMCP, mcp1Node.Kind)
				assert.Contains(t, mcp1Node.DependsOn, dependency.NodeID("pf:pf1"))
				assert.Contains(t, mcp1Node.DependsOn, dependency.NodeID("pf:pf2"))

				// Check mcp2 has no dependencies (pf3 doesn't exist)
				mcp2Node := g.Get(dependency.NodeID("mcp:mcp2"))
				assert.NotNil(t, mcp2Node)
				assert.Empty(t, mcp2Node.DependsOn)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(tt.cfg)

			// Instead of starting the orchestrator, just register services and build the graph
			// This avoids the aggregator startup issue
			o.ctx = context.Background()

			// Register services without starting them
			err := o.registerServices()
			require.NoError(t, err)

			// Build the dependency graph
			o.depGraph = o.buildDependencyGraph()

			graph := o.depGraph
			assert.NotNil(t, graph)

			if tt.check != nil {
				tt.check(t, graph)
			}
		})
	}
}

func TestOrchestrator_checkDependencies(t *testing.T) {
	tests := []struct {
		name         string
		setupOrch    func(*Orchestrator)
		serviceLabel string
		wantErr      bool
		errContains  string
	}{
		{
			name: "no dependencies returns nil",
			setupOrch: func(o *Orchestrator) {
				o.depGraph = dependency.New()
				// Service not in graph means no dependencies
			},
			serviceLabel: "test-service",
			wantErr:      false,
		},
		{
			name: "all dependencies running",
			setupOrch: func(o *Orchestrator) {
				// Create dependency graph
				o.depGraph = dependency.New()
				o.depGraph.AddNode(dependency.Node{
					ID:        "test-service",
					DependsOn: []dependency.NodeID{"dep1"},
				})

				// Register mock dependency service
				depService := &mockService{
					label: "dep1",
					state: services.StateRunning,
				}
				o.registry.Register(depService)
			},
			serviceLabel: "test-service",
			wantErr:      false,
		},
		{
			name: "dependency not found",
			setupOrch: func(o *Orchestrator) {
				o.depGraph = dependency.New()
				o.depGraph.AddNode(dependency.Node{
					ID:        "test-service",
					DependsOn: []dependency.NodeID{"missing-dep"},
				})
			},
			serviceLabel: "test-service",
			wantErr:      true,
			errContains:  "dependency missing-dep not found",
		},
		{
			name: "dependency not running",
			setupOrch: func(o *Orchestrator) {
				o.depGraph = dependency.New()
				o.depGraph.AddNode(dependency.Node{
					ID:        "test-service",
					DependsOn: []dependency.NodeID{"dep1"},
				})

				// Register mock dependency service that's stopped
				depService := &mockService{
					label: "dep1",
					state: services.StateStopped,
				}
				o.registry.Register(depService)
			},
			serviceLabel: "test-service",
			wantErr:      true,
			errContains:  "dependency dep1 is not running",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(Config{})
			ctx := context.Background()
			err := o.Start(ctx)
			require.NoError(t, err)
			defer o.Stop()

			if tt.setupOrch != nil {
				tt.setupOrch(o)
			}

			err = o.checkDependencies(tt.serviceLabel)
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

func TestOrchestrator_stopDependentServices(t *testing.T) {
	tests := []struct {
		name         string
		setupOrch    func(*Orchestrator)
		serviceLabel string
		wantStopped  []string
	}{
		{
			name: "stops direct dependents",
			setupOrch: func(o *Orchestrator) {
				// Build dependency graph
				o.depGraph = dependency.New()
				o.depGraph.AddNode(dependency.Node{ID: "base"})
				o.depGraph.AddNode(dependency.Node{
					ID:        "dep1",
					DependsOn: []dependency.NodeID{"base"},
				})
				o.depGraph.AddNode(dependency.Node{
					ID:        "dep2",
					DependsOn: []dependency.NodeID{"base"},
				})

				// Register mock services
				for _, label := range []string{"dep1", "dep2"} {
					svc := &mockService{
						label: label,
						state: services.StateRunning,
						stopFunc: func(ctx context.Context) error {
							return nil
						},
					}
					o.registry.Register(svc)
				}
			},
			serviceLabel: "base",
			wantStopped:  []string{"dep1", "dep2"},
		},
		{
			name: "handles stop errors gracefully",
			setupOrch: func(o *Orchestrator) {
				o.depGraph = dependency.New()
				o.depGraph.AddNode(dependency.Node{ID: "base"})
				o.depGraph.AddNode(dependency.Node{
					ID:        "dep1",
					DependsOn: []dependency.NodeID{"base"},
				})

				// Register service that fails to stop
				svc := &mockService{
					label: "dep1",
					state: services.StateRunning,
					stopFunc: func(ctx context.Context) error {
						return errors.New("stop failed")
					},
				}
				o.registry.Register(svc)
			},
			serviceLabel: "base",
			wantStopped:  []string{}, // Should still mark as stopped due to dependency
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(Config{})
			ctx := context.Background()
			err := o.Start(ctx)
			require.NoError(t, err)
			defer o.Stop()

			if tt.setupOrch != nil {
				tt.setupOrch(o)
			}

			err = o.stopDependentServices(tt.serviceLabel)
			// Error is logged but not returned for individual service failures

			// Check that services were marked as stopped due to dependency
			for _, label := range tt.wantStopped {
				o.mu.RLock()
				reason, exists := o.stopReasons[label]
				o.mu.RUnlock()
				assert.True(t, exists)
				assert.Equal(t, StopReasonDependency, reason)
			}
		})
	}
}

func TestOrchestrator_getNodeIDForService(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		svcType  services.ServiceType
		expected string
	}{
		{
			name:     "port forward service",
			label:    "my-pf",
			svcType:  services.TypePortForward,
			expected: "pf:my-pf",
		},
		{
			name:     "MCP server service",
			label:    "my-mcp",
			svcType:  services.TypeMCPServer,
			expected: "mcp:my-mcp",
		},
		{
			name:     "K8s connection service",
			label:    "k8s-mc-test",
			svcType:  services.TypeKubeConnection,
			expected: "k8s-mc-test",
		},
		{
			name:     "unknown service type",
			label:    "unknown",
			svcType:  services.ServiceType("Unknown"),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(Config{})

			// Register mock service
			svc := &mockService{
				label:       tt.label,
				serviceType: tt.svcType,
			}
			o.registry.Register(svc)

			result := o.getNodeIDForService(tt.label)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOrchestrator_getLabelFromNodeID(t *testing.T) {
	tests := []struct {
		name     string
		nodeID   string
		expected string
	}{
		{
			name:     "port forward node ID",
			nodeID:   "pf:my-pf",
			expected: "my-pf",
		},
		{
			name:     "MCP server node ID",
			nodeID:   "mcp:my-mcp",
			expected: "my-mcp",
		},
		{
			name:     "plain node ID",
			nodeID:   "k8s-mc-test",
			expected: "k8s-mc-test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(Config{})
			result := o.getLabelFromNodeID(tt.nodeID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOrchestrator_stopAllServices(t *testing.T) {
	o := New(Config{})
	ctx := context.Background()
	err := o.Start(ctx)
	require.NoError(t, err)

	// Register services of different types
	serviceList := []struct {
		label   string
		svcType services.ServiceType
	}{
		{"mcp1", services.TypeMCPServer},
		{"mcp2", services.TypeMCPServer},
		{"pf1", services.TypePortForward},
		{"pf2", services.TypePortForward},
		{"k8s1", services.TypeKubeConnection},
		{"k8s2", services.TypeKubeConnection},
	}

	var mu sync.Mutex
	stoppedOrder := []string{}
	for _, s := range serviceList {
		label := s.label // Capture for closure
		svc := &mockService{
			label:       label,
			serviceType: s.svcType,
			state:       services.StateRunning,
			stopFunc: func(ctx context.Context) error {
				// Record stop order
				mu.Lock()
				stoppedOrder = append(stoppedOrder, label)
				mu.Unlock()
				return nil
			},
		}
		o.registry.Register(svc)
	}

	// Stop all services
	err = o.stopAllServices()
	assert.NoError(t, err)

	// Verify stop order: MCP first, then PF, then K8s
	// Note: Within each type, order may vary due to concurrent stops
	mu.Lock()
	assert.Len(t, stoppedOrder, 6)

	// Check that MCPs were stopped before PFs
	lastMCPIndex := -1
	firstPFIndex := len(stoppedOrder)
	for i, label := range stoppedOrder {
		if label == "mcp1" || label == "mcp2" {
			lastMCPIndex = i
		}
		if (label == "pf1" || label == "pf2") && i < firstPFIndex {
			firstPFIndex = i
		}
	}
	if lastMCPIndex >= 0 && firstPFIndex < len(stoppedOrder) {
		assert.Less(t, lastMCPIndex, firstPFIndex)
	}
	mu.Unlock()
}

func TestOrchestrator_getServicesToStart(t *testing.T) {
	tests := []struct {
		name          string
		setupOrch     func(*Orchestrator)
		expectedCount int
		excludes      []string
	}{
		{
			name: "returns all services when none manually stopped",
			setupOrch: func(o *Orchestrator) {
				// Register services without manual stop reasons
				o.registry.Register(&mockService{label: "svc1"})
				o.registry.Register(&mockService{label: "svc2"})
				o.registry.Register(&mockService{label: "svc3"})
			},
			expectedCount: 3,
			excludes:      []string{},
		},
		{
			name: "excludes manually stopped services",
			setupOrch: func(o *Orchestrator) {
				// Register services
				o.registry.Register(&mockService{label: "svc1"})
				o.registry.Register(&mockService{label: "svc2"})
				o.registry.Register(&mockService{label: "svc3"})

				// Mark svc2 as manually stopped
				o.mu.Lock()
				o.stopReasons["svc2"] = StopReasonManual
				o.mu.Unlock()
			},
			expectedCount: 2,
			excludes:      []string{"svc2"},
		},
		{
			name: "includes dependency-stopped services",
			setupOrch: func(o *Orchestrator) {
				o.registry.Register(&mockService{label: "svc1"})
				o.registry.Register(&mockService{label: "svc2"})

				// Mark svc2 as stopped due to dependency
				o.mu.Lock()
				o.stopReasons["svc2"] = StopReasonDependency
				o.mu.Unlock()
			},
			expectedCount: 2,
			excludes:      []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(Config{})
			ctx := context.Background()
			err := o.Start(ctx)
			require.NoError(t, err)
			defer o.Stop()

			if tt.setupOrch != nil {
				tt.setupOrch(o)
			}

			result := o.getServicesToStart()
			assert.Len(t, result, tt.expectedCount)

			// Verify excluded services are not in result
			for _, excluded := range tt.excludes {
				assert.NotContains(t, result, excluded)
			}
		})
	}
}

func TestOrchestrator_startK8sConnectionsInParallel(t *testing.T) {
	o := New(Config{})
	ctx := context.Background()
	err := o.Start(ctx)
	require.NoError(t, err)
	defer o.Stop()

	var mu sync.Mutex
	startOrder := []string{}

	// Register K8s services
	for _, label := range []string{"k8s1", "k8s2"} {
		svc := &mockService{
			label:       label,
			serviceType: services.TypeKubeConnection,
			state:       services.StateStopped,
			startFunc: func(ctx context.Context) error {
				mu.Lock()
				startOrder = append(startOrder, label)
				mu.Unlock()
				return nil
			},
		}
		o.registry.Register(svc)
	}

	// Register non-K8s service (should be ignored)
	pfSvc := &mockService{
		label:       "pf1",
		serviceType: services.TypePortForward,
		state:       services.StateStopped,
		startFunc: func(ctx context.Context) error {
			t.Error("port forward should not be started by K8s parallel start")
			return nil
		},
	}
	o.registry.Register(pfSvc)

	servicesToStart := []string{"k8s1", "k8s2", "pf1"}

	// Start K8s connections in parallel
	err = o.startK8sConnectionsInParallel(servicesToStart)
	assert.NoError(t, err)

	// Verify both K8s services were started
	mu.Lock()
	assert.Len(t, startOrder, 2)
	assert.Contains(t, startOrder, "k8s1")
	assert.Contains(t, startOrder, "k8s2")
	mu.Unlock()
}

func TestOrchestrator_waitForDependencies(t *testing.T) {
	tests := []struct {
		name         string
		setupOrch    func(*Orchestrator)
		serviceLabel string
		timeout      time.Duration
		wantErr      bool
		errContains  string
	}{
		{
			name: "no dependencies returns immediately",
			setupOrch: func(o *Orchestrator) {
				o.depGraph = dependency.New()
				// Service not in graph = no dependencies
			},
			serviceLabel: "no-deps",
			timeout:      1 * time.Second,
			wantErr:      false,
		},
		{
			name: "all dependencies ready",
			setupOrch: func(o *Orchestrator) {
				o.depGraph = dependency.New()
				o.depGraph.AddNode(dependency.Node{
					ID:        "test-service",
					DependsOn: []dependency.NodeID{"dep1", "dep2"},
				})

				// Register running dependencies
				for _, label := range []string{"dep1", "dep2"} {
					o.registry.Register(&mockService{
						label: label,
						state: services.StateRunning,
					})
				}
			},
			serviceLabel: "test-service",
			timeout:      1 * time.Second,
			wantErr:      false,
		},
		{
			name: "dependency not found",
			setupOrch: func(o *Orchestrator) {
				o.depGraph = dependency.New()
				o.depGraph.AddNode(dependency.Node{
					ID:        "test-service",
					DependsOn: []dependency.NodeID{"missing-dep"},
				})
			},
			serviceLabel: "test-service",
			timeout:      1 * time.Second,
			wantErr:      true,
			errContains:  "dependency missing-dep not found",
		},
		{
			name: "timeout waiting for dependencies",
			setupOrch: func(o *Orchestrator) {
				o.depGraph = dependency.New()
				o.depGraph.AddNode(dependency.Node{
					ID:        "test-service",
					DependsOn: []dependency.NodeID{"slow-dep"},
				})

				// Register dependency that's not running
				o.registry.Register(&mockService{
					label: "slow-dep",
					state: services.StateStarting,
				})
			},
			serviceLabel: "test-service",
			timeout:      100 * time.Millisecond,
			wantErr:      true,
			errContains:  "timeout waiting for dependencies",
		},
		{
			name: "fail fast when dependency is failed",
			setupOrch: func(o *Orchestrator) {
				o.depGraph = dependency.New()
				o.depGraph.AddNode(dependency.Node{
					ID:        "test-service",
					DependsOn: []dependency.NodeID{"failed-dep"},
				})

				// Register dependency that's failed
				o.registry.Register(&mockService{
					label: "failed-dep",
					state: services.StateFailed,
				})
			},
			serviceLabel: "test-service",
			timeout:      10 * time.Second, // Long timeout to ensure we fail fast
			wantErr:      true,
			errContains:  "dependency failed-dep is in failed state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := New(Config{})
			ctx := context.Background()
			err := o.Start(ctx)
			require.NoError(t, err)
			defer o.Stop()

			if tt.setupOrch != nil {
				tt.setupOrch(o)
			}

			err = o.waitForDependencies(tt.serviceLabel, tt.timeout)
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

func TestOrchestrator_startPortForwardsInParallel(t *testing.T) {
	o := New(Config{})
	ctx := context.Background()
	err := o.Start(ctx)
	require.NoError(t, err)
	defer o.Stop()

	// Setup dependency graph
	o.depGraph = dependency.New()
	o.depGraph.AddNode(dependency.Node{ID: "k8s1"}) // No dependencies
	o.depGraph.AddNode(dependency.Node{
		ID:        "pf:pf1",
		DependsOn: []dependency.NodeID{"k8s1"},
	})
	o.depGraph.AddNode(dependency.Node{
		ID:        "pf:pf2",
		DependsOn: []dependency.NodeID{"k8s1"},
	})

	// Register K8s dependency (running)
	o.registry.Register(&mockService{
		label: "k8s1",
		state: services.StateRunning,
	})

	var mu sync.Mutex
	startOrder := []string{}

	// Register port forward services
	for _, label := range []string{"pf1", "pf2"} {
		svc := &mockService{
			label:       label,
			serviceType: services.TypePortForward,
			state:       services.StateStopped,
			startFunc: func(ctx context.Context) error {
				mu.Lock()
				startOrder = append(startOrder, label)
				mu.Unlock()
				return nil
			},
		}
		o.registry.Register(svc)
	}

	servicesToStart := []string{"pf1", "pf2"}

	// Start port forwards in parallel
	err = o.startPortForwardsInParallel(servicesToStart)
	assert.NoError(t, err)

	// Verify both port forwards were started
	mu.Lock()
	assert.Len(t, startOrder, 2)
	assert.Contains(t, startOrder, "pf1")
	assert.Contains(t, startOrder, "pf2")
	mu.Unlock()
}

func TestOrchestrator_startPortForwardsInParallel_SkipsFailedDependencies(t *testing.T) {
	o := New(Config{})
	ctx := context.Background()
	err := o.Start(ctx)
	require.NoError(t, err)
	defer o.Stop()

	// Setup dependency graph
	o.depGraph = dependency.New()
	o.depGraph.AddNode(dependency.Node{ID: "k8s-running"})
	o.depGraph.AddNode(dependency.Node{ID: "k8s-failed"})
	o.depGraph.AddNode(dependency.Node{
		ID:        "pf:pf-good",
		DependsOn: []dependency.NodeID{"k8s-running"},
	})
	o.depGraph.AddNode(dependency.Node{
		ID:        "pf:pf-bad",
		DependsOn: []dependency.NodeID{"k8s-failed"},
	})

	// Register K8s dependencies - one running, one failed
	o.registry.Register(&mockService{
		label: "k8s-running",
		state: services.StateRunning,
	})
	o.registry.Register(&mockService{
		label: "k8s-failed",
		state: services.StateFailed,
	})

	var mu sync.Mutex
	startedServices := []string{}

	// Register port forward services
	for _, label := range []string{"pf-good", "pf-bad"} {
		labelCopy := label // Capture for closure
		svc := &mockService{
			label:       label,
			serviceType: services.TypePortForward,
			state:       services.StateStopped,
			startFunc: func(ctx context.Context) error {
				mu.Lock()
				startedServices = append(startedServices, labelCopy)
				mu.Unlock()
				return nil
			},
		}
		o.registry.Register(svc)
	}

	servicesToStart := []string{"pf-good", "pf-bad"}

	// Start port forwards in parallel
	err = o.startPortForwardsInParallel(servicesToStart)
	assert.NoError(t, err)

	// Verify only pf-good was started, pf-bad was skipped
	mu.Lock()
	assert.Len(t, startedServices, 1)
	assert.Contains(t, startedServices, "pf-good")
	assert.NotContains(t, startedServices, "pf-bad")
	mu.Unlock()
}

func TestOrchestrator_startMCPServersInParallel(t *testing.T) {
	o := New(Config{})
	ctx := context.Background()
	err := o.Start(ctx)
	require.NoError(t, err)
	defer o.Stop()

	// Setup dependency graph - MCP servers with no dependencies for this test
	o.depGraph = dependency.New()
	o.depGraph.AddNode(dependency.Node{ID: "mcp:mcp1"})
	o.depGraph.AddNode(dependency.Node{ID: "mcp:mcp2"})

	var mu sync.Mutex
	startOrder := []string{}

	// Register MCP server services
	for _, label := range []string{"mcp1", "mcp2"} {
		svc := &mockService{
			label:       label,
			serviceType: services.TypeMCPServer,
			state:       services.StateStopped,
			startFunc: func(ctx context.Context) error {
				mu.Lock()
				startOrder = append(startOrder, label)
				mu.Unlock()
				return nil
			},
		}
		o.registry.Register(svc)
	}

	servicesToStart := []string{"mcp1", "mcp2"}

	// Start MCP servers in parallel
	err = o.startMCPServersInParallel(servicesToStart)
	assert.NoError(t, err)

	// Verify both MCP servers were started
	mu.Lock()
	assert.Len(t, startOrder, 2)
	assert.Contains(t, startOrder, "mcp1")
	assert.Contains(t, startOrder, "mcp2")
	mu.Unlock()
}

func TestOrchestrator_startAggregator(t *testing.T) {
	o := New(Config{})
	ctx := context.Background()
	err := o.Start(ctx)
	require.NoError(t, err)
	defer o.Stop()

	aggregatorStarted := false

	// Register aggregator service
	aggSvc := &mockService{
		label:       "mcp-aggregator",
		serviceType: services.ServiceType("Aggregator"),
		state:       services.StateStopped,
		startFunc: func(ctx context.Context) error {
			aggregatorStarted = true
			return nil
		},
	}
	o.registry.Register(aggSvc)

	// Register non-aggregator service (should be ignored)
	nonAggSvc := &mockService{
		label:       "other-service",
		serviceType: services.TypeMCPServer,
		state:       services.StateStopped,
		startFunc: func(ctx context.Context) error {
			t.Error("non-aggregator service should not be started")
			return nil
		},
	}
	o.registry.Register(nonAggSvc)

	servicesToStart := []string{"mcp-aggregator", "other-service"}

	// Start aggregator
	err = o.startAggregator(servicesToStart)
	assert.NoError(t, err)
	assert.True(t, aggregatorStarted)
}
