package aggregator

import (
	"envctl/internal/api"
)

// APIAdapter adapts the AggregatorService to implement api.AggregatorHandler
type APIAdapter struct {
	service *AggregatorService
}

// NewAPIAdapter creates a new aggregator API adapter
func NewAPIAdapter(s *AggregatorService) *APIAdapter {
	return &APIAdapter{service: s}
}

// GetServiceData returns aggregator service data
func (a *APIAdapter) GetServiceData() map[string]interface{} {
	if a.service == nil {
		return nil
	}
	return a.service.GetServiceData()
}

// GetEndpoint returns the aggregator's SSE endpoint URL
func (a *APIAdapter) GetEndpoint() string {
	if a.service == nil {
		return ""
	}
	return a.service.GetEndpoint()
}

// GetPort returns the aggregator port
func (a *APIAdapter) GetPort() int {
	if a.service == nil {
		return 0
	}
	// Extract port from service data
	data := a.service.GetServiceData()
	if port, ok := data["port"].(int); ok {
		return port
	}
	return 0
}

// Register registers this adapter with the API package
func (a *APIAdapter) Register() {
	api.RegisterAggregator(a)
}
