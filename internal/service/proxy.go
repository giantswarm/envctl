package service

import "envctl/internal/mcpserver"

// proxyService provides the actual wiring to mcpserver package.
type proxyService struct{}

func newProxyService() MCPProxyService { return &proxyService{} }

func (p *proxyService) Start(cfg mcpserver.PredefinedMcpServer, updateFn func(mcpserver.McpProcessUpdate)) (chan struct{}, int, error) {
	pid, stop, err := mcpserver.StartAndManageIndividualMcpServer(cfg, updateFn, nil)
	return stop, pid, err
}

func (p *proxyService) Status(name string) (bool, error) {
	// TODO: keep status map; for now unknown
	return false, nil
}
