package api

import (
	"context"
	"envctl/internal/reporting"
	"fmt"
)

// portForwardAPI implements the PortForwardAPI interface
type portForwardAPI struct {
	eventBus   reporting.EventBus
	stateStore reporting.StateStore
}

// NewPortForwardAPI creates a new Port Forward API implementation
func NewPortForwardAPI(eventBus reporting.EventBus, stateStore reporting.StateStore) PortForwardAPI {
	return &portForwardAPI{
		eventBus:   eventBus,
		stateStore: stateStore,
	}
}

// GetActiveForwards returns all active port forwards
func (p *portForwardAPI) GetActiveForwards() []PortForwardInfo {
	// TODO: Implement active forwards listing
	return []PortForwardInfo{}
}

// GetForwardMetrics returns metrics for a port forward
func (p *portForwardAPI) GetForwardMetrics(name string) (*PortForwardMetrics, error) {
	// TODO: Implement metrics retrieval
	return nil, fmt.Errorf("not yet implemented")
}

// TestConnection tests if a port forward is working
func (p *portForwardAPI) TestConnection(ctx context.Context, name string) error {
	// TODO: Implement connection testing
	return fmt.Errorf("not yet implemented")
}
