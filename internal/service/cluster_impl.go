package service

import (
	"context"

	"envctl/internal/utils"
)

// clusterService is the concrete implementation of ClusterService that simply
// delegates to the existing helpers in internal/utils.  It contains no state –
// it is safe to share across goroutines.
//
// NOTE: We keep the type unexported.  Construction happens via newClusterService
// and via the Default() bundle.
type clusterService struct{}

func newClusterService() ClusterService { return &clusterService{} }

func (c *clusterService) CurrentContext() (string, error) {
	return utils.GetCurrentKubeContext()
}

func (c *clusterService) SwitchContext(mc, wc string) error {
	// mc may contain full context name already (e.g. teleport prefix) if wc is empty.
	// When wc is non-empty, utils.BuildMc/WcContext helpers are expected to be used
	// by the caller before invoking SwitchContext.  Therefore we just pass mc
	// through to utils.SwitchKubeContext.
	target := mc
	if wc != "" {
		target = utils.BuildWcContext(mc, wc)
	}
	return utils.SwitchKubeContext(target)
}

func (c *clusterService) Health(ctx context.Context, cluster string) (ClusterHealthInfo, error) {
	ready, total, err := utils.GetNodeStatusClientGo(cluster)
	// We currently ignore ready/total counts; they will be added to the return
	// struct once the TUI consumes them via the service interface.
	_ = ready
	_ = total
	return ClusterHealthInfo{IsLoading: false, Error: err}, nil
}
