package orchestrator

import (
	"envctl/internal/api"
	"envctl/internal/config"
	"envctl/internal/dependency"
	"envctl/internal/kube"
	"envctl/internal/managers"
	"envctl/internal/reporting"
	"strings"
	"sync"
	"time"
)

// StopReason tracks why a service was stopped
type StopReason int

const (
	StopReasonManual     StopReason = iota // User explicitly stopped the service
	StopReasonDependency                   // Service stopped due to dependency failure
)

// Orchestrator manages the lifecycle of all services and their dependencies
type Orchestrator struct {
	serviceMgr managers.ServiceManagerAPI
	kubeMgr    kube.Manager
	kubeAPI    api.KubernetesAPI
	depGraph   *dependency.Graph
	reporter   reporting.ServiceReporter

	// Configuration
	mcName       string
	wcName       string
	portForwards []config.PortForwardDefinition
	mcpServers   []config.MCPServerDefinition

	// Health monitoring
	healthCheckInterval time.Duration

	// Service state tracking
	stopReasons     map[string]StopReason                    // Track why services were stopped
	pendingRestarts map[string]bool                          // Track services pending restart
	serviceConfigs  map[string]managers.ManagedServiceConfig // Store all service configs
	activeWaitGroup *sync.WaitGroup                          // Track active services

	mu sync.RWMutex
}

// Config holds the configuration for the orchestrator
type Config struct {
	MCName              string
	WCName              string
	PortForwards        []config.PortForwardDefinition
	MCPServers          []config.MCPServerDefinition
	HealthCheckInterval time.Duration
}

// New creates a new Orchestrator
func New(
	serviceMgr managers.ServiceManagerAPI,
	reporter reporting.ServiceReporter,
	cfg Config,
) *Orchestrator {
	if cfg.HealthCheckInterval == 0 {
		cfg.HealthCheckInterval = 15 * time.Second
	}

	// Create kube manager
	kubeMgr := kube.NewManager(reporter)

	// Create event bus and state store for API layer
	eventBus := reporting.NewEventBus()
	stateStore := reporting.NewStateStore()

	// Create Kubernetes API
	kubeAPI := api.NewKubernetesAPI(eventBus, stateStore, kubeMgr)

	return &Orchestrator{
		serviceMgr:          serviceMgr,
		kubeMgr:             kubeMgr,
		kubeAPI:             kubeAPI,
		reporter:            reporter,
		mcName:              cfg.MCName,
		wcName:              cfg.WCName,
		portForwards:        cfg.PortForwards,
		mcpServers:          cfg.MCPServers,
		healthCheckInterval: cfg.HealthCheckInterval,
		stopReasons:         make(map[string]StopReason),
		pendingRestarts:     make(map[string]bool),
		serviceConfigs:      make(map[string]managers.ManagedServiceConfig),
		activeWaitGroup:     &sync.WaitGroup{},
	}
}

// GetDependencyGraph returns the current dependency graph
func (o *Orchestrator) GetDependencyGraph() *dependency.Graph {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.depGraph
}

// getNodeIDForService converts a service label to a dependency graph node ID
func (o *Orchestrator) getNodeIDForService(label string, serviceType reporting.ServiceType) string {
	switch serviceType {
	case reporting.ServiceTypePortForward:
		return "pf:" + label
	case reporting.ServiceTypeMCPServer:
		return "mcp:" + label
	case reporting.ServiceTypeKube:
		// K8s services use their label directly as the node ID
		return label
	default:
		return label
	}
}

// getLabelFromNodeID extracts the service label from a node ID
func (o *Orchestrator) getLabelFromNodeID(nodeID string) string {
	if strings.HasPrefix(nodeID, "pf:") {
		return strings.TrimPrefix(nodeID, "pf:")
	} else if strings.HasPrefix(nodeID, "mcp:") {
		return strings.TrimPrefix(nodeID, "mcp:")
	} else if strings.HasPrefix(nodeID, "k8s-") {
		// K8s services use their label directly as the node ID
		return nodeID
	}
	return nodeID
}
