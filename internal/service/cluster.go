package service

import (
	"context"
	"fmt"
	"time"

	"envctl/internal/kube"
	"envctl/internal/utils"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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
	return kube.GetCurrentKubeContext()
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
	return kube.SwitchKubeContext(target)
}

func (c *clusterService) Health(ctx context.Context, clusterContextName string) (ClusterHealthInfo, error) {
	// Create clientset here
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{CurrentContext: clusterContextName}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		// Ensure ClusterHealthInfo is returned correctly on error
		return ClusterHealthInfo{Error: fmt.Errorf("REST config error for %s: %w", clusterContextName, err)}, err
	}
	restConfig.Timeout = 15 * time.Second
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		// Ensure ClusterHealthInfo is returned correctly on error
		return ClusterHealthInfo{Error: fmt.Errorf("clientset error for %s: %w", clusterContextName, err)}, err
	}

	// Call GetNodeStatusClientGo, but its ready/total are not part of service.ClusterHealthInfo yet.
	_, _, err = kube.GetNodeStatusClientGo(clientset)

	// The IsLoading field should ideally be set by the caller or a wrapper if the operation is async.
	// For this direct synchronous call, it's false upon return.
	return ClusterHealthInfo{IsLoading: false, Error: err}, err
}
