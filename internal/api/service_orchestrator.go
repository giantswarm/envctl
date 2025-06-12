package api

import (
	"context"
	"fmt"
	"time"
)

// ServiceCapabilityInfo provides information about a service capability
type ServiceCapabilityInfo struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Version     string                 `json:"version"`
	Description string                 `json:"description"`
	ServiceType string                 `json:"serviceType"`
	Available   bool                   `json:"available"`
	Operations  []OperationInfo        `json:"operations"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ServiceInstanceInfo provides information about a service instance
type ServiceInstanceInfo struct {
	ServiceID          string                 `json:"serviceId"`
	Label              string                 `json:"label"`
	CapabilityName     string                 `json:"capabilityName"`
	CapabilityType     string                 `json:"capabilityType"`
	State              string                 `json:"state"`
	Health             string                 `json:"health"`
	LastError          string                 `json:"lastError,omitempty"`
	CreatedAt          time.Time              `json:"createdAt"`
	LastChecked        *time.Time             `json:"lastChecked,omitempty"`
	ServiceData        map[string]interface{} `json:"serviceData,omitempty"`
	CreationParameters map[string]interface{} `json:"creationParameters"`
}

// CreateServiceRequest represents a request to create a new service instance
type CreateServiceRequest struct {
	CapabilityName string                 `json:"capabilityName"`
	Label          string                 `json:"label"`
	Parameters     map[string]interface{} `json:"parameters"`
}

// ServiceInstanceListResponse represents the response for listing service instances
type ServiceInstanceListResponse struct {
	Services []ServiceInstanceInfo `json:"services"`
	Total    int                   `json:"total"`
}

// ServiceCapabilityListResponse represents the response for listing service capabilities
type ServiceCapabilityListResponse struct {
	Capabilities []ServiceCapabilityInfo `json:"capabilities"`
	Total        int                     `json:"total"`
}

// ServiceInstanceEvent represents a service instance state change event
type ServiceInstanceEvent struct {
	ServiceID   string                 `json:"serviceId"`
	Label       string                 `json:"label"`
	ServiceType string                 `json:"serviceType"`
	OldState    string                 `json:"oldState"`
	NewState    string                 `json:"newState"`
	OldHealth   string                 `json:"oldHealth"`
	NewHealth   string                 `json:"newHealth"`
	Error       string                 `json:"error,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ServiceOrchestratorAPI defines the interface for the service orchestrator API
type ServiceOrchestratorAPI interface {
	// Service capability operations
	ListServiceCapabilities() ([]ServiceCapabilityInfo, error)
	GetServiceCapability(name string) (*ServiceCapabilityInfo, error)
	IsServiceCapabilityAvailable(name string) bool

	// Service instance operations
	CreateService(ctx context.Context, req CreateServiceRequest) (*ServiceInstanceInfo, error)
	DeleteService(ctx context.Context, serviceID string) error
	GetService(serviceID string) (*ServiceInstanceInfo, error)
	GetServiceByLabel(label string) (*ServiceInstanceInfo, error)
	ListServices() ([]ServiceInstanceInfo, error)

	// Service events
	SubscribeToServiceEvents() (<-chan ServiceInstanceEvent, error)
}

// serviceOrchestratorAPI implements the ServiceOrchestratorAPI interface
type serviceOrchestratorAPI struct {
	// No fields - uses handlers from registry
}

// NewServiceOrchestratorAPI creates a new ServiceOrchestratorAPI instance
func NewServiceOrchestratorAPI() ServiceOrchestratorAPI {
	return &serviceOrchestratorAPI{}
}

// ListServiceCapabilities returns all available service capabilities
func (s *serviceOrchestratorAPI) ListServiceCapabilities() ([]ServiceCapabilityInfo, error) {
	handler := GetServiceOrchestrator()
	if handler == nil {
		return nil, fmt.Errorf("service orchestrator handler not registered")
	}
	return handler.ListServiceCapabilities(), nil
}

// GetServiceCapability returns information about a specific service capability
func (s *serviceOrchestratorAPI) GetServiceCapability(name string) (*ServiceCapabilityInfo, error) {
	handler := GetServiceOrchestrator()
	if handler == nil {
		return nil, fmt.Errorf("service orchestrator handler not registered")
	}
	return handler.GetServiceCapability(name)
}

// IsServiceCapabilityAvailable checks if a service capability is available
func (s *serviceOrchestratorAPI) IsServiceCapabilityAvailable(name string) bool {
	handler := GetServiceOrchestrator()
	if handler == nil {
		return false
	}
	return handler.IsServiceCapabilityAvailable(name)
}

// CreateService creates a new service instance
func (s *serviceOrchestratorAPI) CreateService(ctx context.Context, req CreateServiceRequest) (*ServiceInstanceInfo, error) {
	handler := GetServiceOrchestrator()
	if handler == nil {
		return nil, fmt.Errorf("service orchestrator handler not registered")
	}
	return handler.CreateService(ctx, req)
}

// DeleteService deletes a service instance
func (s *serviceOrchestratorAPI) DeleteService(ctx context.Context, serviceID string) error {
	handler := GetServiceOrchestrator()
	if handler == nil {
		return fmt.Errorf("service orchestrator handler not registered")
	}
	return handler.DeleteService(ctx, serviceID)
}

// GetService returns information about a service instance
func (s *serviceOrchestratorAPI) GetService(serviceID string) (*ServiceInstanceInfo, error) {
	handler := GetServiceOrchestrator()
	if handler == nil {
		return nil, fmt.Errorf("service orchestrator handler not registered")
	}
	return handler.GetService(serviceID)
}

// GetServiceByLabel returns information about a service instance by label
func (s *serviceOrchestratorAPI) GetServiceByLabel(label string) (*ServiceInstanceInfo, error) {
	handler := GetServiceOrchestrator()
	if handler == nil {
		return nil, fmt.Errorf("service orchestrator handler not registered")
	}
	return handler.GetServiceByLabel(label)
}

// ListServices returns all service instances
func (s *serviceOrchestratorAPI) ListServices() ([]ServiceInstanceInfo, error) {
	handler := GetServiceOrchestrator()
	if handler == nil {
		return nil, fmt.Errorf("service orchestrator handler not registered")
	}
	return handler.ListServices(), nil
}

// SubscribeToServiceEvents returns a channel for receiving service instance events
func (s *serviceOrchestratorAPI) SubscribeToServiceEvents() (<-chan ServiceInstanceEvent, error) {
	handler := GetServiceOrchestrator()
	if handler == nil {
		return nil, fmt.Errorf("service orchestrator handler not registered")
	}
	return handler.SubscribeToServiceEvents(), nil
}

// Convenience functions for direct API access

// ListServiceCapabilities returns all available service capabilities
func ListServiceCapabilities() ([]ServiceCapabilityInfo, error) {
	api := NewServiceOrchestratorAPI()
	return api.ListServiceCapabilities()
}

// GetServiceCapability returns information about a specific service capability
func GetServiceCapability(name string) (*ServiceCapabilityInfo, error) {
	api := NewServiceOrchestratorAPI()
	return api.GetServiceCapability(name)
}

// IsServiceCapabilityAvailable checks if a service capability is available
func IsServiceCapabilityAvailable(name string) bool {
	api := NewServiceOrchestratorAPI()
	return api.IsServiceCapabilityAvailable(name)
}

// CreateService creates a new service instance
func CreateService(ctx context.Context, req CreateServiceRequest) (*ServiceInstanceInfo, error) {
	api := NewServiceOrchestratorAPI()
	return api.CreateService(ctx, req)
}

// DeleteService deletes a service instance
func DeleteService(ctx context.Context, serviceID string) error {
	api := NewServiceOrchestratorAPI()
	return api.DeleteService(ctx, serviceID)
}

// GetService returns information about a service instance
func GetService(serviceID string) (*ServiceInstanceInfo, error) {
	api := NewServiceOrchestratorAPI()
	return api.GetService(serviceID)
}

// GetServiceByLabel returns information about a service instance by label
func GetServiceByLabel(label string) (*ServiceInstanceInfo, error) {
	api := NewServiceOrchestratorAPI()
	return api.GetServiceByLabel(label)
}

// ListServices returns all service instances
func ListServices() ([]ServiceInstanceInfo, error) {
	api := NewServiceOrchestratorAPI()
	return api.ListServices()
}

// SubscribeToServiceEvents returns a channel for receiving service instance events
func SubscribeToServiceEvents() (<-chan ServiceInstanceEvent, error) {
	api := NewServiceOrchestratorAPI()
	return api.SubscribeToServiceEvents()
}
