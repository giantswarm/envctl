package mcpserver

import (
	"testing"
)

func TestPredefinedMcpServers_NotEmpty(t *testing.T) {
	if len(PredefinedMcpServers) == 0 {
		t.Error("PredefinedMcpServers slice should not be empty")
	}
}

func TestPredefinedMcpServers_ContainsExpectedServers(t *testing.T) {
	expectedServers := map[string]struct{
		proxyPort int
		command   string
	}{
		"kubernetes": {proxyPort: 8001, command: "npx"},
		"prometheus": {proxyPort: 8002, command: "uvx"},
		"grafana":    {proxyPort: 8003, command: "uvx"},
	}

	if len(PredefinedMcpServers) < len(expectedServers) {
		t.Errorf("Expected at least %d predefined servers, got %d", len(expectedServers), len(PredefinedMcpServers))
	}

	foundCount := 0
	for _, server := range PredefinedMcpServers {
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