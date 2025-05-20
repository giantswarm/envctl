// Package service defines small interfaces that wrap the side-effecting operations
// (kube-context switching, port-forwarding, MCP proxy management, …).  The goal is
// to let the TUI depend only on these abstractions so that the UI can be unit-
// tested without touching the network or spawning child processes.
//
// NOTE: These interfaces are intentionally minimal.  They will very likely evolve
// once we start migrating call-sites from the existing utils / portforwarding
// code.  For now they are just enough to compile and to express the design
// described in GitHub issue #27.
package service

import (
	"context"

	"envctl/internal/mcpserver"
	"envctl/internal/portforwarding"
)

// ClusterHealthInfo synthesises the basic information the TUI needs to render a
// health indicator.  Once the real implementation is plugged in we will extend
// the struct as required.
//
// Placing the type in this package avoids a dependency edge from service → tui.
// If that becomes awkward we can move it into its own internal/health package in
// a follow-up.
//
// For now only a very small subset of fields is defined.
//
// TODO: Reconcile with the existing clusterHealthInfo definition in the TUI.
//       (Will be done when usages are migrated.)
//
// nolint:revive // we intentionally use initialisms like ID, CPU.
type ClusterHealthInfo struct {
    IsLoading bool
    Error     error
}

// ClusterService groups operations that query or mutate Kubernetes/Teleport
// contexts *per cluster*.
//
// All methods must be thread-safe – they will be called from concurrent
// goroutines that emit Bubble Tea messages.
type ClusterService interface {
    CurrentContext() (string, error)
    SwitchContext(mc, wc string) error
    Health(ctx context.Context, cluster string) (ClusterHealthInfo, error)
}

// PortForwardService exposes a minimal API for creating and inspecting
// long-running port-forward processes.
//
// The returned stop() func must be idempotent and safe to call from any goroutine.
// The Start call should be asynchronous, returning as soon as the port-forward is
// *initiated* – status updates go through the provided callback inside
// portforwarding.PortForwardConfig.
type PortForwardService interface {
    Start(cfg portforwarding.PortForwardConfig, cb portforwarding.PortForwardUpdateFunc) (stopChan chan struct{}, err error)
    Status(id string) portforwarding.PortForwardProcessUpdate
}

// MCPProxyService abstracts the MCP side-car processes (k8s-api, etcd, etc.).
// We deliberately keep the interface similar to PortForwardService so that the
// TUI can handle both uniformly.
//
// TODO: Once we move the implementation out of internal/mcpserver we may want to
// share common behaviour in an internal/processutil helper.
type MCPProxyService interface {
    // Start launches the MCP proxy defined by cfg and streams async updates via
    // updateFn.  It returns the stop channel, PID of the spawned process (or 0
    // if not applicable) and an error for immediate failures.
    Start(cfg mcpserver.PredefinedMcpServer, updateFn func(mcpserver.McpProcessUpdate)) (stopChan chan struct{}, pid int, err error)
    Status(name string) (running bool, err error)
}

// Services is a small struct used for dependency-injection into the Bubble Tea
// model.  It is easier to pass around than three individual interfaces.
//
// We keep the fields exported so that test code can replace selected services
// with fakes while keeping the rest on the real implementation.
type Services struct {
    Cluster ClusterService
    PF      PortForwardService
    Proxy   MCPProxyService
}

// Default returns a Services bundle backed by the current concrete
// implementations.  For now we only provide stubs so that the refactor compiles –
// the real logic will be hooked up in follow-up commits of issue #27.
func Default() Services {
    return Services{
        Cluster: newClusterService(),
        PF:      newPFService(),
        Proxy:   newProxyService(),
    }
}

// ---------------------------------------------------------------------------
// Temporary no-op implementations for PortForward- and Proxy-services.  They
// will be replaced in a later commit of Issue #27.
// ---------------------------------------------------------------------------

type noopPFService struct{}

func (n *noopPFService) Start(cfg portforwarding.PortForwardConfig, cb portforwarding.PortForwardUpdateFunc) (chan struct{}, error) {
    return make(chan struct{}), nil
}
func (n *noopPFService) Status(id string) portforwarding.PortForwardProcessUpdate {
    return portforwarding.PortForwardProcessUpdate{StatusMsg: "noop", Running: false}
}

type noopProxyService struct{}

func (n *noopProxyService) Start(cfg mcpserver.PredefinedMcpServer, updateFn func(mcpserver.McpProcessUpdate)) (chan struct{}, int, error) {
    return make(chan struct{}), 0, nil
}
func (n *noopProxyService) Status(name string) (bool, error) { return false, nil } 