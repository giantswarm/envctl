package service

import "envctl/internal/portforwarding"

// pfService is a thin wrapper that forwards all operations to the existing
// portforwarding package.  Having it behind an interface lets the TUI swap in
// a fake during tests.
//
// NOTE: For now Status(id) just returns an empty struct because the underlying
// portforwarding code keeps state in the caller.  We can evolve this once we
// have centralised bookkeeping.
type pfService struct{}

func newPFService() PortForwardService { return &pfService{} }

func (p *pfService) Start(cfg portforwarding.PortForwardConfig, cb portforwarding.PortForwardUpdateFunc) (chan struct{}, error) {
	_, stop, err := portforwarding.StartAndManageIndividualPortForward(cfg, cb)
	return stop, err
}

func (p *pfService) Status(id string) portforwarding.PortForwardProcessUpdate {
	// Real implementation TBD – we would keep a map of last known updates.
	return portforwarding.PortForwardProcessUpdate{InstanceKey: id, StatusMsg: "unknown"}
}

// safeCloseChan is duplicated from tui handlers to avoid an import cycle.  Once
// we have a shared util we can deduplicate.
func safeCloseChan(ch chan struct{}) {
	if ch == nil {
		return
	}
	select {
	case <-ch:
		// already closed
	default:
		close(ch)
	}
}
