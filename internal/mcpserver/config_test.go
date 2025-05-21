package mcpserver

import (
	"testing"
)

func TestGetMCPServerConfig_NotEmpty(t *testing.T) {
	if len(GetMCPServerConfig()) == 0 {
		t.Error("GetMCPServerConfig() slice should not be empty")
	}
}

func TestGetMCPServerConfig_ContainsExpectedServers(t *testing.T) {
	expectedServers := map[string]struct {
		proxyPort int
		command   string
	}{
		"kubernetes": {proxyPort: 8001, command: "npx"},
		"prometheus": {proxyPort: 8002, command: "uvx"},
		"grafana":    {proxyPort: 8003, command: "uvx"},
	}

	servers := GetMCPServerConfig()
	if len(servers) < len(expectedServers) {
		t.Errorf("Expected at least %d predefined servers, got %d", len(expectedServers), len(servers))
	}

	foundCount := 0
	for _, server := range servers {
		if expected, ok := expectedServers[server.Name]; ok {
			foundCount++
			if server.ProxyPort != expected.proxyPort {
				t.Errorf("Server %s: expected ProxyPort %d, got %d", server.Name, expected.proxyPort, server.ProxyPort)
			}
			if server.Command != expected.command {
				t.Errorf("Server %s: expected Command %s, got %s", server.Name, expected.command, server.Command)
			}
			// Could also check Args and Env if they are critical and stable
		}
	}

	if foundCount != len(expectedServers) {
		t.Errorf("Did not find all expected servers. Found %d out of %d.", foundCount, len(expectedServers))
		// For more detail, could list which ones were not found
	}
}
