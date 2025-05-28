// Package config provides configuration management for envctl.
//
// This package implements a layered configuration system that allows users to
// customize envctl's behavior through YAML files. Configuration is loaded from
// multiple sources and merged in a specific order, with later sources overriding
// earlier ones.
//
// # Configuration Layers
//
// Configuration is loaded and merged in the following order:
//
//  1. Default Configuration (embedded in binary)
//     - Provides sensible defaults for all settings
//     - Ensures envctl works out-of-the-box
//
//  2. User Configuration (~/.config/envctl/config.yaml)
//     - User-specific settings that apply to all projects
//     - Useful for personal preferences and common overrides
//
//  3. Project Configuration (./.envctl/config.yaml)
//     - Project-specific settings in the current directory
//     - Allows teams to share configuration via version control
//
// # Configuration Structure
//
// The configuration file uses YAML format with the following main sections:
//
//	portForwards:
//	  - name: "mc-prometheus"
//	    enabled: true
//	    namespace: "mimir"
//	    service: "prometheus-operated"
//	    localPort: 8080
//	    remotePort: 9090
//	    kubeContextTarget: "mc"  # Can be "mc", "wc", or explicit context
//
//	mcpServers:
//	  - name: "kubernetes"
//	    enabled: true
//	    type: "localCommand"  # or "container"
//	    command: ["mcp-server-kubernetes"]
//	    proxyPort: 8001
//	    requiresK8sConnection: "mc"
//	    requiresPortForwards: []
//	    env:
//	      KEY: "value"
//
// # Port Forward Configuration
//
// Port forwards define kubectl port-forward tunnels to expose cluster services locally:
//
//   - name: Unique identifier for the port forward
//   - enabled: Whether this port forward should be created
//   - namespace: Kubernetes namespace containing the service
//   - service: Name of the Kubernetes service to forward
//   - localPort: Local port to bind to
//   - remotePort: Remote port on the service
//   - kubeContextTarget: Which context to use ("mc", "wc", or explicit)
//
// # MCP Server Configuration
//
// MCP servers can be configured as local commands or Docker containers:
//
// Local Command:
//   - type: "localCommand"
//   - command: Array of command and arguments
//   - env: Environment variables to set
//
// Container:
//   - type: "container"
//   - image: Docker image to use
//   - ports: Port mappings (container format)
//   - volumes: Volume mounts
//   - env: Environment variables
//
// # Environment Variable Expansion
//
// Configuration values support environment variable expansion:
//
//	env:
//	  API_KEY: "${MY_API_KEY}"
//	  HOME_DIR: "${HOME}/data"
//	  WITH_DEFAULT: "${MISSING:-default_value}"
//
// # Dynamic Context Targeting
//
// The kubeContextTarget field supports dynamic resolution:
//
//   - "mc": Uses the management cluster context
//   - "wc": Uses the workload cluster context (if available)
//   - "dynamic": Automatically selects WC if available, otherwise MC
//   - Explicit context name: Uses the specified context directly
//
// # Usage Example
//
//	// Load configuration for a specific cluster setup
//	cfg, err := config.LoadConfig("myinstallation", "myworkload")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Access port forward configurations
//	for _, pf := range cfg.PortForwards {
//	    if pf.Enabled {
//	        fmt.Printf("Port forward %s: %d -> %d\n",
//	            pf.Name, pf.LocalPort, pf.RemotePort)
//	    }
//	}
//
//	// Access MCP server configurations
//	for _, mcp := range cfg.MCPServers {
//	    if mcp.Enabled {
//	        fmt.Printf("MCP server %s on port %d\n",
//	            mcp.Name, mcp.ProxyPort)
//	    }
//	}
package config
