package api

import (
	"context"
	"envctl/internal/kube"
	"envctl/internal/reporting"
	"envctl/pkg/logging"
	"fmt"
	"strings"
	"time"
)

// kubernetesAPI implements the KubernetesAPI interface
type kubernetesAPI struct {
	eventBus   reporting.EventBus
	stateStore reporting.StateStore
	kubeMgr    kube.Manager
}

// NewKubernetesAPI creates a new Kubernetes API implementation
func NewKubernetesAPI(eventBus reporting.EventBus, stateStore reporting.StateStore, kubeMgr kube.Manager) KubernetesAPI {
	return &kubernetesAPI{
		eventBus:   eventBus,
		stateStore: stateStore,
		kubeMgr:    kubeMgr,
	}
}

// GetClusterHealth returns health information for a cluster
func (k *kubernetesAPI) GetClusterHealth(ctx context.Context, contextName string) (*ClusterHealth, error) {
	// Find the K8s service in the state store by context name
	var serviceLabel string

	// Check all K8s services to find the one with matching context
	k8sServices := k.stateStore.GetServicesByType(reporting.ServiceTypeKube)
	for label, snapshot := range k8sServices {
		// Check if this service is for the requested context
		if snapshot.K8sHealth != nil {
			// For MC, we can match by context name
			if snapshot.K8sHealth.IsMC && contextName == k.kubeMgr.BuildMcContextName(strings.TrimPrefix(label, "k8s-mc-")) {
				serviceLabel = label
				break
			}
			// For WC, matching is more complex as we need both MC and WC names
			// This is a limitation of the current design
			if !snapshot.K8sHealth.IsMC && strings.Contains(contextName, strings.TrimPrefix(label, "k8s-wc-")) {
				serviceLabel = label
				break
			}
		}
	}

	if serviceLabel == "" {
		// If not found in state store, try to get health directly
		clientset, err := kube.GetClientsetForContext(ctx, contextName)
		if err != nil {
			return nil, fmt.Errorf("failed to create clientset: %w", err)
		}

		// Get node status
		readyNodes, totalNodes, err := kube.GetNodeStatus(clientset)
		if err != nil {
			return nil, fmt.Errorf("failed to get node status: %w", err)
		}

		return &ClusterHealth{
			ContextName:    contextName,
			IsHealthy:      readyNodes == totalNodes && totalNodes > 0,
			NodeCount:      totalNodes,
			ReadyNodeCount: readyNodes,
			LastCheck:      time.Now(),
		}, nil
	}

	// Get health from state store
	snapshot, exists := k.stateStore.GetServiceState(serviceLabel)
	if !exists {
		return nil, fmt.Errorf("service %s not found in state store", serviceLabel)
	}

	health := &ClusterHealth{
		ContextName: contextName,
		IsHealthy:   snapshot.State == reporting.StateRunning,
		LastCheck:   snapshot.LastUpdated,
	}

	if snapshot.K8sHealth != nil {
		health.NodeCount = snapshot.K8sHealth.TotalNodes
		health.ReadyNodeCount = snapshot.K8sHealth.ReadyNodes
		health.IsHealthy = health.ReadyNodeCount == health.NodeCount && health.NodeCount > 0
	}

	if snapshot.ErrorDetail != nil {
		health.Error = snapshot.ErrorDetail
		health.IsHealthy = false
	}

	return health, nil
}

// GetNamespaces returns list of namespaces
func (k *kubernetesAPI) GetNamespaces(ctx context.Context, contextName string) ([]string, error) {
	// TODO: Implement namespace listing
	return nil, fmt.Errorf("not yet implemented")
}

// GetResources returns resources in a namespace
func (k *kubernetesAPI) GetResources(ctx context.Context, contextName string, namespace string, resourceType string) ([]Resource, error) {
	// TODO: Implement resource listing
	return nil, fmt.Errorf("not yet implemented")
}

// SubscribeToHealthUpdates subscribes to cluster health changes
func (k *kubernetesAPI) SubscribeToHealthUpdates(contextName string) <-chan ClusterHealthEvent {
	ch := make(chan ClusterHealthEvent, 10)

	// Subscribe to K8s service state changes
	filter := reporting.CombineFilters(
		reporting.FilterByType(reporting.EventTypeServiceRunning, reporting.EventTypeServiceFailed, reporting.EventTypeServiceStopped),
		func(event reporting.Event) bool {
			// Filter for K8s service events
			if serviceEvent, ok := event.(*reporting.ServiceStateEvent); ok {
				return serviceEvent.ServiceType == reporting.ServiceTypeKube
			}
			return false
		},
	)

	subscription := k.eventBus.Subscribe(filter, func(event reporting.Event) {
		if serviceEvent, ok := event.(*reporting.ServiceStateEvent); ok {
			// Get the current health from state store
			snapshot, exists := k.stateStore.GetServiceState(serviceEvent.SourceLabel)
			if !exists {
				return
			}

			// Check if this event is for the requested context
			// This is a simplified check - in production you'd need better context matching
			if snapshot.K8sHealth != nil {
				var eventContext string
				if snapshot.K8sHealth.IsMC {
					eventContext = k.kubeMgr.BuildMcContextName(strings.TrimPrefix(serviceEvent.SourceLabel, "k8s-mc-"))
				} else {
					// For WC, this is more complex - we'd need to parse the label
					eventContext = contextName // Simplified
				}

				if eventContext != contextName {
					return
				}

				// Create health event
				healthEvent := ClusterHealthEvent{
					ContextName: contextName,
					Timestamp:   time.Now(),
				}

				// Set new health
				healthEvent.NewHealth = ClusterHealth{
					ContextName:    contextName,
					IsHealthy:      serviceEvent.NewState == reporting.StateRunning,
					NodeCount:      snapshot.K8sHealth.TotalNodes,
					ReadyNodeCount: snapshot.K8sHealth.ReadyNodes,
					LastCheck:      snapshot.LastUpdated,
					Error:          snapshot.ErrorDetail,
				}

				// Send event
				select {
				case ch <- healthEvent:
					logging.Debug("KubernetesAPI", "Sent health update for context %s", contextName)
				default:
					logging.Warn("KubernetesAPI", "Health update channel full for context %s", contextName)
				}
			}
		}
	})

	// Clean up subscription when channel is closed
	go func() {
		// This goroutine will block until the channel is closed by the consumer
		for range ch {
			// Drain channel
		}
		k.eventBus.Unsubscribe(subscription)
	}()

	// Send initial health if available
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if health, err := k.GetClusterHealth(ctx, contextName); err == nil {
			select {
			case ch <- ClusterHealthEvent{
				ContextName: contextName,
				NewHealth:   *health,
				Timestamp:   time.Now(),
			}:
			default:
			}
		}
	}()

	return ch
}

// StartHealthMonitoring is a no-op since health monitoring is handled by K8s connection services
func (k *kubernetesAPI) StartHealthMonitoring(contextName string, interval time.Duration) error {
	logging.Debug("KubernetesAPI", "StartHealthMonitoring called for %s - health monitoring is handled by K8s connection services", contextName)
	return nil
}

// StopHealthMonitoring is a no-op since health monitoring is handled by K8s connection services
func (k *kubernetesAPI) StopHealthMonitoring(contextName string) error {
	logging.Debug("KubernetesAPI", "StopHealthMonitoring called for %s - health monitoring is handled by K8s connection services", contextName)
	return nil
}
