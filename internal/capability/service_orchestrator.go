package capability

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"envctl/pkg/logging"

	"github.com/google/uuid"
)

// controlLoopMessage represents messages sent to the main control loop
type controlLoopMessage struct {
	messageType controlLoopMessageType
	serviceID   string
	data        interface{}
}

type controlLoopMessageType int

const (
	controlLoopHealthCheck controlLoopMessageType = iota
	controlLoopServiceUpdate
	controlLoopShutdown
)

// ToolCaller represents the interface for calling aggregator tools
// This will be implemented by the aggregator integration in 43.4
type ToolCaller interface {
	CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (map[string]interface{}, error)
}

// ServiceOrchestrator manages dynamic service instances based on capability definitions
type ServiceOrchestrator struct {
	mu sync.RWMutex

	// Core components
	registry   *ServiceCapabilityRegistry
	toolCaller ToolCaller
	config     ServiceOrchestratorConfig

	// Service instance management
	instances map[string]*ServiceInstance // service ID -> instance
	byLabel   map[string]*ServiceInstance // service label -> instance

	// State change notifications
	stateChangeSubscribers []chan<- ServiceInstanceEvent

	// Lifecycle management
	ctx        context.Context
	cancelFunc context.CancelFunc
	stopChan   chan struct{}
	wg         sync.WaitGroup

	// Control loop management
	healthCheckTicker *time.Ticker
	controlLoopChan   chan controlLoopMessage
}

// ServiceOrchestrator configuration
type ServiceOrchestratorConfig struct {
	// How often to check service health
	HealthCheckInterval time.Duration

	// Timeout for service operations
	DefaultCreateTimeout time.Duration
	DefaultDeleteTimeout time.Duration

	// Maximum number of concurrent service operations
	MaxConcurrentOps int

	// For testing: disable background loops
	DisableControlLoops bool
}

// DefaultServiceOrchestratorConfig returns sensible defaults
func DefaultServiceOrchestratorConfig() ServiceOrchestratorConfig {
	return ServiceOrchestratorConfig{
		HealthCheckInterval:  30 * time.Second,
		DefaultCreateTimeout: 60 * time.Second,
		DefaultDeleteTimeout: 30 * time.Second,
		MaxConcurrentOps:     10,
	}
}

// ServiceInstanceEvent represents a service instance state change event
type ServiceInstanceEvent struct {
	ServiceID   string                 `json:"serviceId"`
	Label       string                 `json:"label"`
	ServiceType string                 `json:"serviceType"`
	OldState    ServiceState           `json:"oldState"`
	NewState    ServiceState           `json:"newState"`
	OldHealth   HealthStatus           `json:"oldHealth"`
	NewHealth   HealthStatus           `json:"newHealth"`
	Error       string                 `json:"error,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// CreateServiceRequest represents a request to create a new service instance
type CreateServiceRequest struct {
	// Service capability to use
	CapabilityName string `json:"capabilityName"`

	// Label for the service instance (must be unique)
	Label string `json:"label"`

	// Parameters for service creation
	Parameters map[string]interface{} `json:"parameters"`

	// Override default timeouts
	CreateTimeout *time.Duration `json:"createTimeout,omitempty"`
	DeleteTimeout *time.Duration `json:"deleteTimeout,omitempty"`
}

// ServiceInstanceInfo provides information about a service instance
type ServiceInstanceInfo struct {
	ServiceID          string                 `json:"serviceId"`
	Label              string                 `json:"label"`
	CapabilityName     string                 `json:"capabilityName"`
	CapabilityType     string                 `json:"capabilityType"`
	State              ServiceState           `json:"state"`
	Health             HealthStatus           `json:"health"`
	LastError          string                 `json:"lastError,omitempty"`
	CreatedAt          time.Time              `json:"createdAt"`
	LastChecked        *time.Time             `json:"lastChecked,omitempty"`
	ServiceData        map[string]interface{} `json:"serviceData,omitempty"`
	CreationParameters map[string]interface{} `json:"creationParameters"`
}

// NewServiceOrchestrator creates a new service orchestrator
func NewServiceOrchestrator(registry *ServiceCapabilityRegistry, toolCaller ToolCaller, config ServiceOrchestratorConfig) *ServiceOrchestrator {
	return &ServiceOrchestrator{
		registry:               registry,
		toolCaller:             toolCaller,
		config:                 config,
		instances:              make(map[string]*ServiceInstance),
		byLabel:                make(map[string]*ServiceInstance),
		stateChangeSubscribers: make([]chan<- ServiceInstanceEvent, 0),
		stopChan:               make(chan struct{}),
		controlLoopChan:        make(chan controlLoopMessage, 100),
	}
}

// Start starts the service orchestrator
func (so *ServiceOrchestrator) Start(ctx context.Context) error {
	so.mu.Lock()
	defer so.mu.Unlock()

	so.ctx, so.cancelFunc = context.WithCancel(ctx)

	// Skip control loops if disabled (for testing)
	if !so.config.DisableControlLoops {
		// Start the main control loop
		so.wg.Add(1)
		go so.controlLoop()

		// Start health check loop if enabled
		if so.config.HealthCheckInterval > 0 {
			so.healthCheckTicker = time.NewTicker(so.config.HealthCheckInterval)
			so.wg.Add(1)
			go so.healthCheckLoop()
		}
	}

	logging.Info("ServiceOrchestrator", "Started service orchestrator with health check interval: %v", so.config.HealthCheckInterval)
	return nil
}

// Stop stops the service orchestrator and all managed services
func (so *ServiceOrchestrator) Stop(ctx context.Context) error {
	so.mu.Lock()

	if so.cancelFunc != nil {
		so.cancelFunc()
	}

	// Stop health check ticker
	if so.healthCheckTicker != nil {
		so.healthCheckTicker.Stop()
	}

	// Send shutdown message to control loop
	select {
	case so.controlLoopChan <- controlLoopMessage{messageType: controlLoopShutdown}:
	default:
	}

	// Collect all running services while holding the lock
	var runningInstances []*ServiceInstance
	for _, instance := range so.instances {
		if instance.State == ServiceStateRunning {
			runningInstances = append(runningInstances, instance)
		}
	}
	so.mu.Unlock()

	// Stop all running services (release mutex before starting goroutines)
	var wg sync.WaitGroup
	for _, instance := range runningInstances {
		wg.Add(1)
		go func(inst *ServiceInstance) {
			defer wg.Done()
			if err := so.deleteServiceInstance(ctx, inst); err != nil {
				logging.Error("ServiceOrchestrator", err, "Failed to stop service instance %s during shutdown", inst.Label)
			}
		}(instance)
	}

	// Wait for all services to stop or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logging.Info("ServiceOrchestrator", "All services stopped gracefully")
	case <-time.After(30 * time.Second):
		logging.Warn("ServiceOrchestrator", "Timeout waiting for services to stop")
	}

	// Wait for control loops to finish
	so.wg.Wait()

	close(so.stopChan)
	logging.Info("ServiceOrchestrator", "Stopped service orchestrator")
	return nil
}

// CreateService creates a new service instance based on a capability definition
func (so *ServiceOrchestrator) CreateService(ctx context.Context, req CreateServiceRequest) (*ServiceInstanceInfo, error) {
	// Validate the request
	if err := so.validateCreateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid create request: %w", err)
	}

	// Get the capability definition
	capabilityDef, exists := so.registry.GetServiceCapabilityDefinition(req.CapabilityName)
	if !exists {
		return nil, fmt.Errorf("capability %s not found", req.CapabilityName)
	}

	// Check if capability is available (all required tools present)
	if !so.registry.IsServiceCapabilityAvailable(req.CapabilityName) {
		return nil, fmt.Errorf("capability %s is not available (missing required tools)", req.CapabilityName)
	}

	// Check if label is already in use
	so.mu.Lock()
	if _, exists := so.byLabel[req.Label]; exists {
		so.mu.Unlock()
		return nil, fmt.Errorf("service with label %s already exists", req.Label)
	}

	// Create service instance
	instance := &ServiceInstance{
		ID:                 uuid.New().String(),
		Label:              req.Label,
		CapabilityName:     req.CapabilityName,
		CapabilityType:     capabilityDef.ServiceConfig.ServiceType,
		State:              ServiceStateUnknown,
		Health:             HealthStatusUnknown,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
		CreationParameters: req.Parameters,
		ServiceData:        make(map[string]interface{}),
		Dependencies:       []string{},
	}

	// Store the instance
	so.instances[instance.ID] = instance
	so.byLabel[instance.Label] = instance
	so.mu.Unlock()

	logging.Info("ServiceOrchestrator", "Creating service instance: %s (capability: %s)", req.Label, req.CapabilityName)

	// Create the service instance synchronously for now
	// This will be made async in 43.4 when proper tool integration is added
	if err := so.createServiceInstance(context.Background(), instance, capabilityDef); err != nil {
		logging.Error("ServiceOrchestrator", err, "Failed to create service instance %s", instance.Label)
		so.updateInstanceState(instance, ServiceStateFailed, HealthStatusUnhealthy, err.Error())
		return nil, fmt.Errorf("failed to create service instance: %w", err)
	}

	return so.serviceInstanceToInfo(instance), nil
}

// DeleteService deletes a service instance
func (so *ServiceOrchestrator) DeleteService(ctx context.Context, serviceID string) error {
	so.mu.Lock()
	instance, exists := so.instances[serviceID]
	if !exists {
		so.mu.Unlock()
		return fmt.Errorf("service instance %s not found", serviceID)
	}
	so.mu.Unlock()

	logging.Info("ServiceOrchestrator", "Deleting service instance: %s", instance.Label)

	if err := so.deleteServiceInstance(ctx, instance); err != nil {
		return fmt.Errorf("failed to delete service instance %s: %w", instance.Label, err)
	}

	so.mu.Lock()
	delete(so.instances, instance.ID)
	delete(so.byLabel, instance.Label)
	so.mu.Unlock()

	return nil
}

// GetService returns information about a service instance
func (so *ServiceOrchestrator) GetService(serviceID string) (*ServiceInstanceInfo, error) {
	so.mu.RLock()
	defer so.mu.RUnlock()

	instance, exists := so.instances[serviceID]
	if !exists {
		return nil, fmt.Errorf("service instance %s not found", serviceID)
	}

	return so.serviceInstanceToInfo(instance), nil
}

// GetServiceByLabel returns information about a service instance by label
func (so *ServiceOrchestrator) GetServiceByLabel(label string) (*ServiceInstanceInfo, error) {
	so.mu.RLock()
	defer so.mu.RUnlock()

	instance, exists := so.byLabel[label]
	if !exists {
		return nil, fmt.Errorf("service instance with label %s not found", label)
	}

	return so.serviceInstanceToInfo(instance), nil
}

// ListServices returns information about all service instances
func (so *ServiceOrchestrator) ListServices() []ServiceInstanceInfo {
	so.mu.RLock()
	defer so.mu.RUnlock()

	result := make([]ServiceInstanceInfo, 0, len(so.instances))
	for _, instance := range so.instances {
		result = append(result, *so.serviceInstanceToInfo(instance))
	}

	return result
}

// SubscribeToEvents returns a channel for receiving service instance events
func (so *ServiceOrchestrator) SubscribeToEvents() <-chan ServiceInstanceEvent {
	so.mu.Lock()
	defer so.mu.Unlock()

	eventChan := make(chan ServiceInstanceEvent, 100)
	so.stateChangeSubscribers = append(so.stateChangeSubscribers, eventChan)
	return eventChan
}

// validateCreateRequest validates a service creation request
func (so *ServiceOrchestrator) validateCreateRequest(req CreateServiceRequest) error {
	if req.CapabilityName == "" {
		return fmt.Errorf("capability name is required")
	}
	if req.Label == "" {
		return fmt.Errorf("service label is required")
	}
	if req.Parameters == nil {
		req.Parameters = make(map[string]interface{})
	}
	return nil
}

// serviceInstanceToInfo converts a ServiceInstance to ServiceInstanceInfo
func (so *ServiceOrchestrator) serviceInstanceToInfo(instance *ServiceInstance) *ServiceInstanceInfo {
	return &ServiceInstanceInfo{
		ServiceID:          instance.ID,
		Label:              instance.Label,
		CapabilityName:     instance.CapabilityName,
		CapabilityType:     instance.CapabilityType,
		State:              instance.State,
		Health:             instance.Health,
		LastError:          instance.LastError,
		CreatedAt:          instance.CreatedAt,
		LastChecked:        instance.LastChecked,
		ServiceData:        instance.ServiceData,
		CreationParameters: instance.CreationParameters,
	}
}

// updateInstanceState updates the state of a service instance and publishes events
func (so *ServiceOrchestrator) updateInstanceState(instance *ServiceInstance, newState ServiceState, newHealth HealthStatus, errorMsg string) {
	oldState := instance.State
	oldHealth := instance.Health

	// Update instance state (instance fields are protected by this being the only writer)
	instance.State = newState
	instance.Health = newHealth
	instance.LastError = errorMsg
	instance.UpdatedAt = time.Now()

	// Publish state change event
	event := ServiceInstanceEvent{
		ServiceID:   instance.ID,
		Label:       instance.Label,
		ServiceType: instance.CapabilityType,
		OldState:    oldState,
		NewState:    newState,
		OldHealth:   oldHealth,
		NewHealth:   newHealth,
		Error:       errorMsg,
		Timestamp:   time.Now(),
		Metadata:    instance.ServiceData,
	}

	so.publishEvent(event)

	logging.Debug("ServiceOrchestrator", "Service %s state changed: %s -> %s (health: %s -> %s)",
		instance.Label, oldState, newState, oldHealth, newHealth)
}

// createServiceInstance handles the actual creation of a service instance
func (so *ServiceOrchestrator) createServiceInstance(ctx context.Context, instance *ServiceInstance, capabilityDef *ServiceCapabilityDefinition) error {
	so.updateInstanceState(instance, ServiceStateStarting, HealthStatusUnknown, "")

	logging.Info("ServiceOrchestrator", "Creating service instance %s using capability %s",
		instance.Label, capabilityDef.Name)

	// Check if tool caller is available
	if so.toolCaller == nil {
		return fmt.Errorf("tool caller not available")
	}

	// Get the create tool configuration
	createTool := capabilityDef.ServiceConfig.LifecycleTools.Create
	if createTool.Tool == "" {
		return fmt.Errorf("no create tool specified in capability definition")
	}

	// Prepare the context for template substitution
	templater := NewParameterTemplater()
	templateContext := MergeContexts(
		instance.CreationParameters,
		map[string]interface{}{
			"label":          instance.Label,
			"serviceId":      instance.ID,
			"capabilityName": instance.CapabilityName,
			"capabilityType": instance.CapabilityType,
		},
	)

	// Apply template substitution to tool arguments
	toolArgs, err := templater.ReplaceTemplates(createTool.Arguments, templateContext)
	if err != nil {
		return fmt.Errorf("failed to process tool arguments: %w", err)
	}

	// Ensure tool arguments is a map
	toolArgsMap, ok := toolArgs.(map[string]interface{})
	if !ok {
		return fmt.Errorf("tool arguments must be a map, got %T", toolArgs)
	}

	// Call the create tool
	logging.Debug("ServiceOrchestrator", "Calling create tool %s for service %s", createTool.Tool, instance.Label)
	response, err := so.toolCaller.CallTool(ctx, createTool.Tool, toolArgsMap)
	if err != nil {
		so.updateInstanceState(instance, ServiceStateFailed, HealthStatusUnhealthy, err.Error())
		return fmt.Errorf("create tool failed: %w", err)
	}

	// Process the response
	if err := so.processToolResponse(instance, response, createTool.ResponseMapping); err != nil {
		so.updateInstanceState(instance, ServiceStateFailed, HealthStatusUnhealthy, err.Error())
		return fmt.Errorf("failed to process create tool response: %w", err)
	}

	// Check if tool call was successful
	if success, ok := response["success"].(bool); ok && !success {
		errorMsg := "create tool indicated failure"
		if text, exists := response["text"].(string); exists {
			errorMsg = text
		}
		so.updateInstanceState(instance, ServiceStateFailed, HealthStatusUnhealthy, errorMsg)
		return fmt.Errorf("create tool failed: %s", errorMsg)
	}

	// Mark as running and healthy
	so.updateInstanceState(instance, ServiceStateRunning, HealthStatusHealthy, "")

	logging.Info("ServiceOrchestrator", "Successfully created service instance: %s", instance.Label)
	return nil
}

// deleteServiceInstance handles the actual deletion of a service instance
func (so *ServiceOrchestrator) deleteServiceInstance(ctx context.Context, instance *ServiceInstance) error {
	so.updateInstanceState(instance, ServiceStateStopping, instance.Health, "")

	logging.Info("ServiceOrchestrator", "Deleting service instance %s", instance.Label)

	// Get the capability definition
	capabilityDef, exists := so.registry.GetServiceCapabilityDefinition(instance.CapabilityName)
	if !exists {
		// If capability definition is not found, just mark as stopped
		// This can happen during shutdown or if definitions have changed
		logging.Warn("ServiceOrchestrator", "Capability definition %s not found for service %s, marking as stopped",
			instance.CapabilityName, instance.Label)
		so.updateInstanceState(instance, ServiceStateStopped, HealthStatusUnknown, "")
		return nil
	}

	// Check if tool caller is available
	if so.toolCaller == nil {
		logging.Warn("ServiceOrchestrator", "Tool caller not available for service %s, marking as stopped", instance.Label)
		so.updateInstanceState(instance, ServiceStateStopped, HealthStatusUnknown, "")
		return nil
	}

	// Get the delete tool configuration
	deleteTool := capabilityDef.ServiceConfig.LifecycleTools.Delete
	if deleteTool.Tool == "" {
		// If no delete tool specified, just mark as stopped
		logging.Debug("ServiceOrchestrator", "No delete tool specified for service %s, marking as stopped", instance.Label)
		so.updateInstanceState(instance, ServiceStateStopped, HealthStatusUnknown, "")
		return nil
	}

	// Prepare the context for template substitution
	templater := NewParameterTemplater()
	templateContext := MergeContexts(
		instance.CreationParameters,
		instance.ServiceData, // Include service data that might contain external IDs
		map[string]interface{}{
			"label":          instance.Label,
			"serviceId":      instance.ID,
			"capabilityName": instance.CapabilityName,
			"capabilityType": instance.CapabilityType,
		},
	)

	// Apply template substitution to tool arguments
	toolArgs, err := templater.ReplaceTemplates(deleteTool.Arguments, templateContext)
	if err != nil {
		logging.Error("ServiceOrchestrator", err, "Failed to process delete tool arguments for service %s", instance.Label)
		// Continue with deletion anyway and mark as stopped
		so.updateInstanceState(instance, ServiceStateStopped, HealthStatusUnknown, "")
		return nil
	}

	// Ensure tool arguments is a map
	toolArgsMap, ok := toolArgs.(map[string]interface{})
	if !ok {
		logging.Error("ServiceOrchestrator", nil, "Delete tool arguments must be a map for service %s, got %T", instance.Label, toolArgs)
		so.updateInstanceState(instance, ServiceStateStopped, HealthStatusUnknown, "")
		return nil
	}

	// Call the delete tool
	logging.Debug("ServiceOrchestrator", "Calling delete tool %s for service %s", deleteTool.Tool, instance.Label)
	response, err := so.toolCaller.CallTool(ctx, deleteTool.Tool, toolArgsMap)
	if err != nil {
		logging.Error("ServiceOrchestrator", err, "Delete tool failed for service %s", instance.Label)
		// Even if delete tool fails, mark as stopped to prevent resource leaks in our tracking
		so.updateInstanceState(instance, ServiceStateStopped, HealthStatusUnknown, err.Error())
		return fmt.Errorf("delete tool failed: %w", err)
	}

	// Process the response (but don't fail deletion if this fails)
	if err := so.processToolResponse(instance, response, deleteTool.ResponseMapping); err != nil {
		logging.Warn("ServiceOrchestrator", "Failed to process delete tool response for service %s: %v", instance.Label, err)
	}

	// Mark as stopped
	so.updateInstanceState(instance, ServiceStateStopped, HealthStatusUnknown, "")

	logging.Info("ServiceOrchestrator", "Successfully deleted service instance: %s", instance.Label)
	return nil
}

// controlLoop is the main orchestrator control loop
func (so *ServiceOrchestrator) controlLoop() {
	defer so.wg.Done()

	logging.Debug("ServiceOrchestrator", "Starting main control loop")

	for {
		select {
		case <-so.ctx.Done():
			logging.Debug("ServiceOrchestrator", "Control loop stopped due to context cancellation")
			return
		case msg := <-so.controlLoopChan:
			switch msg.messageType {
			case controlLoopShutdown:
				logging.Debug("ServiceOrchestrator", "Control loop received shutdown message")
				return
			case controlLoopHealthCheck:
				so.processHealthCheck(msg.serviceID)
			case controlLoopServiceUpdate:
				so.processServiceUpdate(msg.serviceID)
			}
		}
	}
}

// healthCheckLoop periodically triggers health checks for all services
func (so *ServiceOrchestrator) healthCheckLoop() {
	defer so.wg.Done()

	logging.Debug("ServiceOrchestrator", "Starting health check loop")

	for {
		select {
		case <-so.ctx.Done():
			logging.Debug("ServiceOrchestrator", "Health check loop stopped due to context cancellation")
			return
		case <-so.healthCheckTicker.C:
			so.triggerHealthChecks()
		}
	}
}

// triggerHealthChecks triggers health checks for all running services
func (so *ServiceOrchestrator) triggerHealthChecks() {
	so.mu.RLock()
	serviceIDs := make([]string, 0, len(so.instances))
	for id, instance := range so.instances {
		if instance.State == ServiceStateRunning {
			serviceIDs = append(serviceIDs, id)
		}
	}
	so.mu.RUnlock()

	for _, serviceID := range serviceIDs {
		select {
		case so.controlLoopChan <- controlLoopMessage{
			messageType: controlLoopHealthCheck,
			serviceID:   serviceID,
		}:
		default:
			logging.Debug("ServiceOrchestrator", "Control loop channel full, skipping health check for service %s", serviceID)
		}
	}
}

// processHealthCheck processes a health check for a specific service
func (so *ServiceOrchestrator) processHealthCheck(serviceID string) {
	so.mu.RLock()
	instance, exists := so.instances[serviceID]
	if !exists {
		so.mu.RUnlock()
		return
	}

	capabilityDef, capExists := so.registry.GetServiceCapabilityDefinition(instance.CapabilityName)
	so.mu.RUnlock()

	if !capExists {
		logging.Warn("ServiceOrchestrator", "Capability definition not found for service %s", instance.Label)
		return
	}

	// Update last checked time
	now := time.Now()
	instance.LastChecked = &now

	// Only perform health checks for running services
	if instance.State != ServiceStateRunning {
		logging.Debug("ServiceOrchestrator", "Skipping health check for service %s (state: %s)", instance.Label, instance.State)
		return
	}

	// Check if health check is enabled in the capability definition
	healthCheckConfig := capabilityDef.ServiceConfig.HealthCheck
	if !healthCheckConfig.Enabled {
		// Health check disabled, maintain current health status
		logging.Debug("ServiceOrchestrator", "Health check disabled for service %s", instance.Label)
		return
	}

	// Check if health check tool is specified
	if capabilityDef.ServiceConfig.LifecycleTools.HealthCheck == nil {
		// No health check tool specified, assume healthy if running
		logging.Debug("ServiceOrchestrator", "No health check tool specified for service %s, assuming healthy", instance.Label)
		so.updateInstanceHealth(instance, HealthStatusHealthy, "")
		return
	}

	// Check if tool caller is available
	if so.toolCaller == nil {
		logging.Debug("ServiceOrchestrator", "Tool caller not available for health check of service %s", instance.Label)
		return
	}

	healthCheckTool := *capabilityDef.ServiceConfig.LifecycleTools.HealthCheck

	// Prepare the context for template substitution
	templater := NewParameterTemplater()
	templateContext := MergeContexts(
		instance.CreationParameters,
		instance.ServiceData, // Include service data that might contain external IDs
		map[string]interface{}{
			"label":          instance.Label,
			"serviceId":      instance.ID,
			"capabilityName": instance.CapabilityName,
			"capabilityType": instance.CapabilityType,
		},
	)

	// Apply template substitution to tool arguments
	toolArgs, err := templater.ReplaceTemplates(healthCheckTool.Arguments, templateContext)
	if err != nil {
		logging.Error("ServiceOrchestrator", err, "Failed to process health check tool arguments for service %s", instance.Label)
		instance.HealthCheckFailures++
		so.updateInstanceHealth(instance, HealthStatusUnhealthy, fmt.Sprintf("health check argument processing failed: %v", err))
		return
	}

	// Ensure tool arguments is a map
	toolArgsMap, ok := toolArgs.(map[string]interface{})
	if !ok {
		logging.Error("ServiceOrchestrator", nil, "Health check tool arguments must be a map for service %s, got %T", instance.Label, toolArgs)
		instance.HealthCheckFailures++
		so.updateInstanceHealth(instance, HealthStatusUnhealthy, "health check argument processing failed")
		return
	}

	// Call the health check tool with a timeout
	healthCheckCtx := context.Background()
	if healthCheckConfig.Interval > 0 {
		var cancel context.CancelFunc
		healthCheckCtx, cancel = context.WithTimeout(context.Background(), healthCheckConfig.Interval/2)
		defer cancel()
	}

	logging.Debug("ServiceOrchestrator", "Calling health check tool %s for service %s", healthCheckTool.Tool, instance.Label)
	response, err := so.toolCaller.CallTool(healthCheckCtx, healthCheckTool.Tool, toolArgsMap)
	if err != nil {
		logging.Debug("ServiceOrchestrator", "Health check tool failed for service %s: %v", instance.Label, err)
		instance.HealthCheckFailures++

		// Check failure threshold
		if instance.HealthCheckFailures >= healthCheckConfig.FailureThreshold {
			so.updateInstanceHealth(instance, HealthStatusUnhealthy, fmt.Sprintf("health check failed: %v", err))
		}
		return
	}

	// Process the response (but don't fail health check if this fails)
	if err := so.processToolResponse(instance, response, healthCheckTool.ResponseMapping); err != nil {
		logging.Warn("ServiceOrchestrator", "Failed to process health check tool response for service %s: %v", instance.Label, err)
	}

	// Check if tool call was successful
	if success, ok := response["success"].(bool); ok && !success {
		logging.Debug("ServiceOrchestrator", "Health check tool indicated failure for service %s", instance.Label)
		instance.HealthCheckFailures++

		// Check failure threshold
		if instance.HealthCheckFailures >= healthCheckConfig.FailureThreshold {
			errorMsg := "health check tool indicated failure"
			if text, exists := response["text"].(string); exists {
				errorMsg = text
			}
			so.updateInstanceHealth(instance, HealthStatusUnhealthy, errorMsg)
		}
		return
	}

	// Health check successful
	instance.HealthCheckSuccesses++
	instance.HealthCheckFailures = 0 // Reset failure count on success

	// Check success threshold
	if instance.HealthCheckSuccesses >= healthCheckConfig.SuccessThreshold {
		so.updateInstanceHealth(instance, HealthStatusHealthy, "")
	}

	logging.Debug("ServiceOrchestrator", "Health check completed for service %s (health: %s)",
		instance.Label, instance.Health)
}

// processServiceUpdate processes a service update message
func (so *ServiceOrchestrator) processServiceUpdate(serviceID string) {
	logging.Debug("ServiceOrchestrator", "Processing service update for %s", serviceID)
	// This will be expanded based on future requirements
}

// updateInstanceHealth updates the health status of a service instance
func (so *ServiceOrchestrator) updateInstanceHealth(instance *ServiceInstance, newHealth HealthStatus, errorMsg string) {
	oldHealth := instance.Health
	instance.Health = newHealth

	if errorMsg != "" {
		instance.LastError = errorMsg
	}

	// Only publish event if health status actually changed
	if oldHealth != newHealth {
		event := ServiceInstanceEvent{
			ServiceID:   instance.ID,
			Label:       instance.Label,
			ServiceType: instance.CapabilityType,
			OldState:    instance.State,
			NewState:    instance.State,
			OldHealth:   oldHealth,
			NewHealth:   newHealth,
			Error:       errorMsg,
			Timestamp:   time.Now(),
			Metadata:    instance.ServiceData,
		}

		so.publishEvent(event)

		logging.Debug("ServiceOrchestrator", "Service %s health changed: %s -> %s",
			instance.Label, oldHealth, newHealth)
	}
}

// publishEvent publishes an event to all subscribers
func (so *ServiceOrchestrator) publishEvent(event ServiceInstanceEvent) {
	so.mu.RLock()
	subscribers := make([]chan<- ServiceInstanceEvent, len(so.stateChangeSubscribers))
	copy(subscribers, so.stateChangeSubscribers)
	so.mu.RUnlock()

	for _, subscriber := range subscribers {
		select {
		case subscriber <- event:
		default:
			logging.Debug("ServiceOrchestrator", "Event subscriber blocked, skipping event for service %s", event.Label)
		}
	}
}

// processToolResponse processes the response from a tool call and updates service data
func (so *ServiceOrchestrator) processToolResponse(instance *ServiceInstance, response map[string]interface{}, responseMapping ResponseMapping) error {
	// Update service data with response
	if instance.ServiceData == nil {
		instance.ServiceData = make(map[string]interface{})
	}

	// Store raw response for debugging
	instance.ServiceData["last_response"] = response

	// Extract specific fields based on response mapping
	if responseMapping.ServiceID != "" {
		if serviceID := so.extractFromResponse(response, responseMapping.ServiceID); serviceID != nil {
			instance.ServiceData["external_service_id"] = serviceID
		}
	}

	if responseMapping.Status != "" {
		if status := so.extractFromResponse(response, responseMapping.Status); status != nil {
			instance.ServiceData["external_status"] = status
		}
	}

	if responseMapping.Health != "" {
		if health := so.extractFromResponse(response, responseMapping.Health); health != nil {
			instance.ServiceData["external_health"] = health
		}
	}

	if responseMapping.Error != "" {
		if errorInfo := so.extractFromResponse(response, responseMapping.Error); errorInfo != nil {
			instance.ServiceData["external_error"] = errorInfo
		}
	}

	// Extract metadata fields
	for key, path := range responseMapping.Metadata {
		if value := so.extractFromResponse(response, path); value != nil {
			instance.ServiceData[key] = value
		}
	}

	logging.Debug("ServiceOrchestrator", "Processed tool response for service %s", instance.Label)
	return nil
}

// extractFromResponse extracts a value from the response using a simple JSONPath-like syntax
func (so *ServiceOrchestrator) extractFromResponse(response map[string]interface{}, path string) interface{} {
	// For now, implement simple dot notation (e.g., "data.port" or "text")
	if path == "" {
		return nil
	}

	// Split path by dots
	parts := strings.Split(path, ".")
	current := response

	for i, part := range parts {
		if current == nil {
			return nil
		}

		if i == len(parts)-1 {
			// Last part - return the value
			return current[part]
		}

		// Navigate deeper
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}

	return nil
}
