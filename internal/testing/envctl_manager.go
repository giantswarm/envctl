package testing

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

// logCapture captures stdout and stderr from a process
type logCapture struct {
	stdoutBuf    *bytes.Buffer
	stderrBuf    *bytes.Buffer
	stdoutReader *io.PipeReader
	stderrReader *io.PipeReader
	stdoutWriter *io.PipeWriter
	stderrWriter *io.PipeWriter
	wg           sync.WaitGroup
	mu           sync.RWMutex
}

// newLogCapture creates a new log capture instance
func newLogCapture() *logCapture {
	lc := &logCapture{
		stdoutBuf: &bytes.Buffer{},
		stderrBuf: &bytes.Buffer{},
	}

	lc.stdoutReader, lc.stdoutWriter = io.Pipe()
	lc.stderrReader, lc.stderrWriter = io.Pipe()

	// Start goroutines to capture output
	lc.wg.Add(2)
	go lc.captureOutput(lc.stdoutReader, lc.stdoutBuf)
	go lc.captureOutput(lc.stderrReader, lc.stderrBuf)

	return lc
}

// captureOutput captures output from a reader to a buffer
func (lc *logCapture) captureOutput(reader io.Reader, buffer *bytes.Buffer) {
	defer lc.wg.Done()

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		lc.mu.Lock()
		buffer.WriteString(line + "\n")
		lc.mu.Unlock()
	}
}

// close closes the capture pipes and waits for completion
func (lc *logCapture) close() {
	lc.stdoutWriter.Close()
	lc.stderrWriter.Close()
	lc.wg.Wait()
}

// getLogs returns the captured logs
func (lc *logCapture) getLogs() *InstanceLogs {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	stdout := lc.stdoutBuf.String()
	stderr := lc.stderrBuf.String()

	// Create combined log with simple interleaving
	combined := ""
	if stdout != "" {
		combined += "=== STDOUT ===\n" + stdout
	}
	if stderr != "" {
		if combined != "" {
			combined += "\n"
		}
		combined += "=== STDERR ===\n" + stderr
	}

	return &InstanceLogs{
		Stdout:   stdout,
		Stderr:   stderr,
		Combined: combined,
	}
}

// managedProcess represents a managed envctl process with its command and log capture
type managedProcess struct {
	cmd        *exec.Cmd
	logCapture *logCapture
}

// envCtlInstanceManager implements the EnvCtlInstanceManager interface
type envCtlInstanceManager struct {
	debug      bool
	basePort   int
	portOffset int
	tempDir    string
	processes  map[string]*managedProcess // Track processes by instance ID
	mu         sync.RWMutex
}

// NewEnvCtlInstanceManager creates a new envctl instance manager
func NewEnvCtlInstanceManager(debug bool, basePort int) (EnvCtlInstanceManager, error) {
	// Create temporary directory for test configurations
	tempDir, err := os.MkdirTemp("", "envctl-test-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	return &envCtlInstanceManager{
		debug:     debug,
		basePort:  basePort,
		tempDir:   tempDir,
		processes: make(map[string]*managedProcess),
	}, nil
}

// CreateInstance creates a new envctl serve instance with the given configuration
func (m *envCtlInstanceManager) CreateInstance(ctx context.Context, scenarioName string, config *EnvCtlPreConfiguration) (*EnvCtlInstance, error) {
	// Generate unique instance ID
	instanceID := fmt.Sprintf("test-%s-%d", sanitizeFileName(scenarioName), time.Now().UnixNano())

	// Find available port
	port, err := m.findAvailablePort()
	if err != nil {
		return nil, fmt.Errorf("failed to find available port: %w", err)
	}

	// Create instance configuration directory
	configPath := filepath.Join(m.tempDir, instanceID)
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	if m.debug {
		fmt.Printf("🏗️  Creating envctl instance %s with config at %s\n", instanceID, configPath)
	}

	// Generate configuration files
	if err := m.generateConfigFiles(configPath, config, port); err != nil {
		return nil, fmt.Errorf("failed to generate config files: %w", err)
	}

	// Start envctl serve process with log capture
	managedProc, err := m.startEnvCtlProcess(ctx, configPath, port)
	if err != nil {
		// Clean up on failure
		os.RemoveAll(configPath)
		return nil, fmt.Errorf("failed to start envctl process: %w", err)
	}

	// Store the managed process
	m.mu.Lock()
	m.processes[instanceID] = managedProc
	m.mu.Unlock()

	// Extract expected tools from configuration
	expectedTools := m.extractExpectedTools(config)

	instance := &EnvCtlInstance{
		ID:            instanceID,
		ConfigPath:    configPath,
		Port:          port,
		Endpoint:      fmt.Sprintf("http://localhost:%d/mcp", port),
		Process:       managedProc.cmd.Process,
		StartTime:     time.Now(),
		Logs:          nil, // Will be populated when destroying
		ExpectedTools: expectedTools,
	}

	if m.debug {
		fmt.Printf("🚀 Started envctl instance %s on port %d (PID: %d)\n", instanceID, port, managedProc.cmd.Process.Pid)
	}

	return instance, nil
}

// DestroyInstance stops and cleans up an envctl serve instance
func (m *envCtlInstanceManager) DestroyInstance(ctx context.Context, instance *EnvCtlInstance) error {
	if m.debug {
		fmt.Printf("🛑 Destroying envctl instance %s (PID: %d)\n", instance.ID, instance.Process.Pid)
	}

	// Get the managed process
	m.mu.RLock()
	managedProc, exists := m.processes[instance.ID]
	m.mu.RUnlock()

	if exists && managedProc != nil {
		// Attempt graceful shutdown first
		if err := m.gracefulShutdown(managedProc, instance.ID); err != nil {
			if m.debug {
				fmt.Printf("⚠️  Graceful shutdown failed for %s: %v, forcing termination\n", instance.ID, err)
			}
		}

		// Collect logs before cleanup
		if managedProc.logCapture != nil {
			managedProc.logCapture.close()
			instance.Logs = managedProc.logCapture.getLogs()
		}

		// Clean up from processes map
		m.mu.Lock()
		delete(m.processes, instance.ID)
		m.mu.Unlock()
	}

	// Clean up configuration directory
	if err := os.RemoveAll(instance.ConfigPath); err != nil {
		if m.debug {
			fmt.Printf("⚠️  Failed to remove config directory %s: %v\n", instance.ConfigPath, err)
		}
		return fmt.Errorf("failed to remove config directory: %w", err)
	}

	if m.debug {
		fmt.Printf("✅ Destroyed envctl instance %s\n", instance.ID)
	}

	return nil
}

// gracefulShutdown attempts to gracefully shutdown an envctl process
func (m *envCtlInstanceManager) gracefulShutdown(managedProc *managedProcess, instanceID string) error {
	if managedProc.cmd == nil || managedProc.cmd.Process == nil {
		return fmt.Errorf("no process to shutdown")
	}

	process := managedProc.cmd.Process

	// Send SIGTERM for graceful shutdown
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// If SIGTERM fails, try SIGKILL
		if m.debug {
			fmt.Printf("🔄 SIGTERM failed for %s, using SIGKILL: %v\n", instanceID, err)
		}
		return process.Kill()
	}

	// Wait for graceful shutdown with timeout
	shutdownTimeout := 10 * time.Second
	done := make(chan error, 1)

	go func() {
		err := managedProc.cmd.Wait()
		done <- err
	}()

	select {
	case err := <-done:
		if m.debug {
			if err != nil {
				fmt.Printf("✅ Process %s exited with: %v\n", instanceID, err)
			} else {
				fmt.Printf("✅ Process %s exited gracefully\n", instanceID)
			}
		}
		return nil
	case <-time.After(shutdownTimeout):
		if m.debug {
			fmt.Printf("⏰ Graceful shutdown timeout for %s, forcing kill\n", instanceID)
		}
		return process.Kill()
	}
}

// WaitForReady waits for an instance to be ready to accept connections and has all expected tools available
func (m *envCtlInstanceManager) WaitForReady(ctx context.Context, instance *EnvCtlInstance) error {
	if m.debug {
		fmt.Printf("⏳ Waiting for envctl instance %s to be ready at %s\n", instance.ID, instance.Endpoint)
	}

	timeout := 60 * time.Second // Increased timeout for more complex setups
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	readyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Give the process a moment to start
	time.Sleep(2 * time.Second)

	// First wait for port to be available
	portReady := false
	for !portReady {
		select {
		case <-readyCtx.Done():
			if m.debug {
				m.showLogs(instance)
			}
			return fmt.Errorf("timeout waiting for envctl instance port to be ready")
		case <-ticker.C:
			// Check if port is accepting connections
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", instance.Port), 1*time.Second)
			if err == nil {
				conn.Close()
				portReady = true
				if m.debug {
					fmt.Printf("✅ Port %d is ready\n", instance.Port)
				}
			} else if m.debug {
				fmt.Printf("🔍 Port %d not ready yet: %v\n", instance.Port, err)
			}
		}
	}

	// Now wait for services to be fully initialized and tools to be available
	if m.debug {
		fmt.Printf("⏳ Waiting for services to be fully initialized and tools to be available...\n")
	}

	// Create MCP client to check tool availability
	mcpClient := NewMCPTestClient(m.debug)
	defer mcpClient.Close()

	// Connect to the MCP aggregator
	connectCtx, connectCancel := context.WithTimeout(readyCtx, 30*time.Second)
	defer connectCancel()

	// Retry connection until successful or timeout
	var connected bool
	for !connected {
		select {
		case <-connectCtx.Done():
			if m.debug {
				fmt.Printf("⚠️  Failed to connect to MCP aggregator, proceeding anyway\n")
			}
			// If we can't connect to MCP, fall back to the old behavior
			time.Sleep(3 * time.Second)
			return nil
		case <-time.After(1 * time.Second):
			err := mcpClient.Connect(connectCtx, instance.Endpoint)
			if err == nil {
				connected = true
				if m.debug {
					fmt.Printf("✅ Connected to MCP aggregator\n")
				}
			} else if m.debug {
				fmt.Printf("🔍 Waiting for MCP connection: %v\n", err)
			}
		}
	}

	// Extract expected tools from the pre-configuration if available
	expectedTools := m.extractExpectedToolsFromInstance(instance)
	
	if len(expectedTools) == 0 {
		if m.debug {
			fmt.Printf("ℹ️  No expected tools specified, waiting for basic service readiness\n")
		}
		// If no specific tools expected, wait a bit longer for general readiness
		time.Sleep(5 * time.Second)
		return nil
	}

	if m.debug {
		fmt.Printf("🎯 Waiting for %d expected tools to be available: %v\n", len(expectedTools), expectedTools)
	}

	// Wait for all expected tools to be available
	toolTimeout := 45 * time.Second
	toolCtx, toolCancel := context.WithTimeout(readyCtx, toolTimeout)
	defer toolCancel()

	toolTicker := time.NewTicker(2 * time.Second)
	defer toolTicker.Stop()

	for {
		select {
		case <-toolCtx.Done():
			if m.debug {
				fmt.Printf("⚠️  Tool availability check timed out, checking what's available...\n")
				// Show what tools are available for debugging
				if availableTools, err := mcpClient.ListTools(context.Background()); err == nil {
					fmt.Printf("🛠️  Available tools: %v\n", availableTools)
					fmt.Printf("🎯 Expected tools: %v\n", expectedTools)
				}
			}
			return fmt.Errorf("timeout waiting for all expected tools to be available")
		case <-toolTicker.C:
			// Check if all expected tools are available
			availableTools, err := mcpClient.ListTools(toolCtx)
			if err != nil {
				if m.debug {
					fmt.Printf("🔍 Failed to list tools: %v\n", err)
				}
				continue
			}

			if m.debug && len(availableTools) > 0 {
				fmt.Printf("🛠️  Currently available tools (%d): %v\n", len(availableTools), availableTools)
			}

			// Check if all expected tools are present
			missingTools := m.findMissingTools(expectedTools, availableTools)
			if len(missingTools) == 0 {
				if m.debug {
					fmt.Printf("✅ All expected tools are available!\n")
				}
				// Wait a little bit more to ensure tool registration is fully complete
				time.Sleep(2 * time.Second)
				return nil
			}

			if m.debug {
				fmt.Printf("⏳ Still waiting for tools: %v\n", missingTools)
			}
		}
	}
}

// extractExpectedTools extracts expected tool names from the configuration during instance creation
func (m *envCtlInstanceManager) extractExpectedTools(config *EnvCtlPreConfiguration) []string {
	if config == nil {
		return []string{}
	}
	
	var expectedTools []string
	
	// Extract tools from MCP server configurations
	for _, mcpServer := range config.MCPServers {
		if tools, hasTools := mcpServer.Config["tools"]; hasTools {
			if toolsList, ok := tools.([]interface{}); ok {
				for _, tool := range toolsList {
					if toolMap, ok := tool.(map[string]interface{}); ok {
						if name, ok := toolMap["name"].(string); ok {
							expectedTools = append(expectedTools, name)
						}
					}
				}
			}
		}
	}
	
	if m.debug && len(expectedTools) > 0 {
		fmt.Printf("🎯 Extracted expected tools from configuration: %v\n", expectedTools)
	}
	
	return expectedTools
}

// extractExpectedToolsFromInstance gets the expected tools stored in the instance
func (m *envCtlInstanceManager) extractExpectedToolsFromInstance(instance *EnvCtlInstance) []string {
	return instance.ExpectedTools
}

// findMissingTools returns tools that are expected but not found in available tools
func (m *envCtlInstanceManager) findMissingTools(expectedTools, availableTools []string) []string {
	var missing []string
	
	for _, expected := range expectedTools {
		found := false
		for _, available := range availableTools {
			// Check for exact match or suffix match (for prefixed tools)
			if available == expected || m.isToolMatch(available, expected) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, expected)
		}
	}
	
	return missing
}

// isToolMatch checks if an available tool matches an expected tool name
// This handles cases where tools might have prefixes from MCP server names
func (m *envCtlInstanceManager) isToolMatch(availableTool, expectedTool string) bool {
	// Check exact match
	if availableTool == expectedTool {
		return true
	}
	
	// Check for suffix match with underscore (server_tool format)
	suffix := "_" + expectedTool
	if len(availableTool) > len(suffix) && availableTool[len(availableTool)-len(suffix):] == suffix {
		return true
	}
	
	// Check for suffix match with dash (server-tool format)
	dashSuffix := "-" + expectedTool
	if len(availableTool) > len(dashSuffix) && availableTool[len(availableTool)-len(dashSuffix):] == dashSuffix {
		return true
	}
	
	return false
}

// showLogs displays the recent logs from an envctl instance
func (m *envCtlInstanceManager) showLogs(instance *EnvCtlInstance) {
	logDir := filepath.Join(instance.ConfigPath, "logs")

	// Show stdout logs
	stdoutPath := filepath.Join(logDir, "stdout.log")
	if content, err := os.ReadFile(stdoutPath); err == nil && len(content) > 0 {
		fmt.Printf("📄 Instance %s stdout logs:\n%s\n", instance.ID, string(content))
	}

	// Show stderr logs
	stderrPath := filepath.Join(logDir, "stderr.log")
	if content, err := os.ReadFile(stderrPath); err == nil && len(content) > 0 {
		fmt.Printf("🚨 Instance %s stderr logs:\n%s\n", instance.ID, string(content))
	}
}

// findAvailablePort finds an available port starting from the base port
func (m *envCtlInstanceManager) findAvailablePort() (int, error) {
	for i := 0; i < 100; i++ { // Try up to 100 ports
		port := m.basePort + m.portOffset + i

		// Check if port is available
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			m.portOffset = i + 1 // Next search starts from next port
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports found starting from %d", m.basePort)
}

// startEnvCtlProcess starts an envctl serve process
func (m *envCtlInstanceManager) startEnvCtlProcess(ctx context.Context, configPath string, port int) (*managedProcess, error) {
	// Get the path to the envctl binary
	envctlPath, err := m.getEnvCtlBinaryPath()
	if err != nil {
		return nil, fmt.Errorf("failed to find envctl binary: %w", err)
	}

	// envctl serve should use the envctl subdirectory as config path
	envctlConfigPath := filepath.Join(configPath, "envctl")

	// Create command
	args := []string{
		"serve",
		"--no-tui",
		"--config-path", envctlConfigPath,
		"--debug",
	}

	cmd := exec.CommandContext(ctx, envctlPath, args...)

	if m.debug {
		fmt.Printf("🚀 Starting command: %s %v\n", envctlPath, args)
	}

	// Create log capture
	logCapture := newLogCapture()

	// Set up stdout/stderr capture
	cmd.Stdout = logCapture.stdoutWriter
	cmd.Stderr = logCapture.stderrWriter

	// Start the process
	if err := cmd.Start(); err != nil {
		logCapture.close()
		return nil, fmt.Errorf("failed to start envctl process: %w", err)
	}

	managedProc := &managedProcess{
		cmd:        cmd,
		logCapture: logCapture,
	}

	return managedProc, nil
}

// getEnvCtlBinaryPath returns the path to the envctl binary
func (m *envCtlInstanceManager) getEnvCtlBinaryPath() (string, error) {
	// First try to find in PATH
	if path, err := exec.LookPath("envctl"); err == nil {
		return path, nil
	}

	// Try common locations relative to current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if we're in the envctl project root
	possiblePaths := []string{
		filepath.Join(cwd, "envctl"),
		filepath.Join(cwd, "bin", "envctl"),
		filepath.Join(cwd, "..", "envctl"),
		filepath.Join(cwd, "..", "bin", "envctl"),
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Try to build envctl if we're in the source directory
	if m.isInEnvCtlSource(cwd) {
		if m.debug {
			fmt.Printf("🔨 Building envctl binary from source\n")
		}

		buildCmd := exec.Command("go", "build", "-o", "envctl", ".")
		buildCmd.Dir = cwd
		if err := buildCmd.Run(); err != nil {
			return "", fmt.Errorf("failed to build envctl: %w", err)
		}

		builtPath := filepath.Join(cwd, "envctl")
		if _, err := os.Stat(builtPath); err == nil {
			return builtPath, nil
		}
	}

	return "", fmt.Errorf("envctl binary not found")
}

// isInEnvCtlSource checks if we're in the envctl source directory
func (m *envCtlInstanceManager) isInEnvCtlSource(dir string) bool {
	// Check for key files that indicate we're in the envctl source
	markers := []string{"main.go", "go.mod", "cmd/serve.go"}

	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(dir, marker)); err != nil {
			return false
		}
	}

	return true
}

// generateConfigFiles generates configuration files for the envctl instance
func (m *envCtlInstanceManager) generateConfigFiles(configPath string, config *EnvCtlPreConfiguration, port int) error {
	// Create envctl subdirectory - this is where envctl serve will look for configs
	envctlConfigPath := filepath.Join(configPath, "envctl")
	
	// Create subdirectories under envctl
	dirs := []string{"mcpservers", "workflows", "capabilities", "serviceclasses", "services"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(envctlConfigPath, dir), 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", dir, err)
		}
	}
	
	// Create mocks directory for mock configurations
	if err := os.MkdirAll(filepath.Join(configPath, "mocks"), 0755); err != nil {
		return fmt.Errorf("failed to create mocks directory: %w", err)
	}

	// Generate main config.yaml in envctl subdirectory
	mainConfig := map[string]interface{}{
		"aggregator": map[string]interface{}{
			"host":      "localhost",
			"port":      port,
			"transport": "streamable-http",
			"enabled":   true,
		},
		"logging": map[string]interface{}{
			"level": "debug",
		},
	}

	// Apply custom main config if provided
	if config != nil && config.MainConfig != nil {
		for key, value := range config.MainConfig.Config {
			mainConfig[key] = value
		}
	}

	configFile := filepath.Join(envctlConfigPath, "config.yaml")
	if err := m.writeYAMLFile(configFile, mainConfig); err != nil {
		return fmt.Errorf("failed to write main config: %w", err)
	}

	if m.debug {
		// Show the generated config
		configContent, _ := os.ReadFile(configFile)
		fmt.Printf("📋 Generated config.yaml:\n%s\n", string(configContent))
	}

	// Generate configuration files if config is provided
	if config != nil {
		// Generate MCP server configs
		for _, mcpServer := range config.MCPServers {
			// Check if this is a mock server (has tools in config)
			if tools, hasMockTools := mcpServer.Config["tools"]; hasMockTools {
				// Get the current working directory to build the envctl command path
				envctlPath, err := m.getEnvCtlBinaryPath()
				if err != nil {
					return fmt.Errorf("failed to get envctl binary path: %w", err)
				}
				
				// Create the localCommand server definition for envctl serve
				serverDef := map[string]interface{}{
					"name":             mcpServer.Name,
					"type":             "localCommand",
					"enabledByDefault": true,
					"icon":             "m",
					"category":         "Test",
					"command":          []string{envctlPath, "test", "--mock-mcp-server", "--mock-config", filepath.Join(configPath, "mocks", mcpServer.Name+".yaml")},
				}
				
				if m.debug {
					fmt.Printf("🧪 ServerDef for %s: %+v\n", mcpServer.Name, serverDef)
					fmt.Printf("🧪 Tools config for %s: %+v\n", mcpServer.Name, mcpServer.Config)
				}
				
				// Save server definition to mcpservers directory (what envctl serve loads)
				filename := filepath.Join(envctlConfigPath, "mcpservers", mcpServer.Name+".yaml")
				if err := m.writeYAMLFile(filename, serverDef); err != nil {
					return fmt.Errorf("failed to write mock MCP server config %s: %w", mcpServer.Name, err)
				}

				// Save mock tools config to mocks directory (what mock server reads)
				mockConfigFile := filepath.Join(configPath, "mocks", mcpServer.Name+".yaml")
				if err := m.writeYAMLFile(mockConfigFile, mcpServer.Config); err != nil {
					return fmt.Errorf("failed to write mock config %s: %w", mcpServer.Name, err)
				}
				
				if m.debug {
					fmt.Printf("🧪 Created mock server %s with %d tools\n", mcpServer.Name, len(tools.([]interface{})))
				}
			} else {
				// For regular servers, use the Config field directly
				filename := filepath.Join(envctlConfigPath, "mcpservers", mcpServer.Name+".yaml")
				if err := m.writeYAMLFile(filename, mcpServer.Config); err != nil {
					return fmt.Errorf("failed to write MCP server config %s: %w", mcpServer.Name, err)
				}
			}
		}

		// Generate workflow configs in envctl subdirectory
		for _, workflow := range config.Workflows {
			filename := filepath.Join(envctlConfigPath, "workflows", workflow.Name+".yaml")
			if err := m.writeYAMLFile(filename, workflow.Config); err != nil {
				return fmt.Errorf("failed to write workflow config %s: %w", workflow.Name, err)
			}
		}

		// Generate capability configs in envctl subdirectory
		for _, capability := range config.Capabilities {
			filename := filepath.Join(envctlConfigPath, "capabilities", capability.Name+".yaml")
			if err := m.writeYAMLFile(filename, capability.Config); err != nil {
				return fmt.Errorf("failed to write capability config %s: %w", capability.Name, err)
			}
		}

		// Generate service class configs in envctl subdirectory
		for _, serviceClass := range config.ServiceClasses {
			filename := filepath.Join(envctlConfigPath, "serviceclasses", serviceClass.Name+".yaml")
			if err := m.writeYAMLFile(filename, serviceClass.Config); err != nil {
				return fmt.Errorf("failed to write service class config %s: %w", serviceClass.Name, err)
			}
		}

		// Generate service configs in envctl subdirectory
		for _, service := range config.Services {
			filename := filepath.Join(envctlConfigPath, "services", service.Name+".yaml")
			if err := m.writeYAMLFile(filename, service.Config); err != nil {
				return fmt.Errorf("failed to write service config %s: %w", service.Name, err)
			}
		}
	}

	return nil
}

// writeYAMLFile writes data to a YAML file
func (m *envCtlInstanceManager) writeYAMLFile(filename string, data interface{}) error {
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	if err := os.WriteFile(filename, yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	if m.debug {
		fmt.Printf("📝 Generated config file: %s\n", filename)
		fmt.Printf("📄 Content:\n%s\n", string(yamlData))
	}

	return nil
}

// sanitizeFileName sanitizes a string to be safe for use as a filename
func sanitizeFileName(name string) string {
	// Replace invalid characters with underscores
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)

	sanitized := replacer.Replace(name)

	// Limit length
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}

	return sanitized
}

// Cleanup cleans up all temporary directories created by this manager
func (m *envCtlInstanceManager) Cleanup() error {
	if m.tempDir != "" {
		return os.RemoveAll(m.tempDir)
	}
	return nil
}
