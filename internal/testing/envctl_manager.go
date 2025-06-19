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

	instance := &EnvCtlInstance{
		ID:         instanceID,
		ConfigPath: configPath,
		Port:       port,
		Endpoint:   fmt.Sprintf("http://localhost:%d/mcp", port),
		Process:    managedProc.cmd.Process,
		StartTime:  time.Now(),
		Logs:       nil, // Will be populated when destroying
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

// WaitForReady waits for an instance to be ready to accept connections
func (m *envCtlInstanceManager) WaitForReady(ctx context.Context, instance *EnvCtlInstance) error {
	if m.debug {
		fmt.Printf("⏳ Waiting for envctl instance %s to be ready at %s\n", instance.ID, instance.Endpoint)
	}

	timeout := 30 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	readyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Give the process a moment to start
	time.Sleep(2 * time.Second)

	for {
		select {
		case <-readyCtx.Done():
			// Before giving up, try to show logs if available
			if m.debug {
				m.showLogs(instance)
			}
			return fmt.Errorf("timeout waiting for envctl instance to be ready")
		case <-ticker.C:
			// Check if port is accepting connections
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", instance.Port), 1*time.Second)
			if err == nil {
				conn.Close()
				if m.debug {
					fmt.Printf("✅ envctl instance %s is ready\n", instance.ID)
				}
				return nil
			}

			if m.debug {
				fmt.Printf("🔍 Port %d not ready yet: %v\n", instance.Port, err)
			}
		}
	}
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

	// Create command
	args := []string{
		"serve",
		"--no-tui",
		"--config-path", configPath,
		"--debug",
	}

	cmd := exec.CommandContext(ctx, envctlPath, args...)

	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("ENVCTL_PORT=%d", port),
		"ENVCTL_HOST=localhost",
	)

	if m.debug {
		fmt.Printf("🚀 Starting command: %s %v\n", envctlPath, args)
		fmt.Printf("🌍 Environment: ENVCTL_PORT=%d ENVCTL_HOST=localhost\n", port)
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
	// Create subdirectories
	dirs := []string{"mcpservers", "workflows", "capabilities", "serviceclasses", "services"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(configPath, dir), 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", dir, err)
		}
	}

	// Generate main config.yaml
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

	configFile := filepath.Join(configPath, "config.yaml")
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
			filename := filepath.Join(configPath, "mcpservers", mcpServer.Name+".yaml")

			// For mock servers, we need to create the full server definition
			if mcpServer.Type == "mock" && mcpServer.MockConfig != nil {
				serverDef := map[string]interface{}{
					"name":             mcpServer.Name,
					"type":             mcpServer.Type,
					"enabledByDefault": true,
					"mock_config":      mcpServer.MockConfig,
				}
				if err := m.writeYAMLFile(filename, serverDef); err != nil {
					return fmt.Errorf("failed to write mock MCP server config %s: %w", mcpServer.Name, err)
				}
			} else {
				// For regular servers, use the Config field
				if err := m.writeYAMLFile(filename, mcpServer.Config); err != nil {
					return fmt.Errorf("failed to write MCP server config %s: %w", mcpServer.Name, err)
				}
			}
		}

		// Generate workflow configs
		for _, workflow := range config.Workflows {
			filename := filepath.Join(configPath, "workflows", workflow.Name+".yaml")
			if err := m.writeYAMLFile(filename, workflow.Config); err != nil {
				return fmt.Errorf("failed to write workflow config %s: %w", workflow.Name, err)
			}
		}

		// Generate capability configs
		for _, capability := range config.Capabilities {
			filename := filepath.Join(configPath, "capabilities", capability.Name+".yaml")
			if err := m.writeYAMLFile(filename, capability.Config); err != nil {
				return fmt.Errorf("failed to write capability config %s: %w", capability.Name, err)
			}
		}

		// Generate service class configs
		for _, serviceClass := range config.ServiceClasses {
			filename := filepath.Join(configPath, "serviceclasses", serviceClass.Name+".yaml")
			if err := m.writeYAMLFile(filename, serviceClass.Config); err != nil {
				return fmt.Errorf("failed to write service class config %s: %w", serviceClass.Name, err)
			}
		}

		// Generate service configs
		for _, service := range config.Services {
			filename := filepath.Join(configPath, "services", service.Name+".yaml")
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
