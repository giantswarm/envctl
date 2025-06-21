package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"envctl/internal/api"
	"envctl/internal/containerizer"
	"envctl/pkg/logging"
)

// ContainerRunner handles container-based MCP servers
type ContainerRunner struct {
	containerizer containerizer.ContainerRuntime
	definition    *api.MCPServer
	containerID   string
	stopChan      chan struct{}
}

// NewContainerRunner creates a new container runner
func NewContainerRunner(definition *api.MCPServer) (*ContainerRunner, error) {
	containerRuntime, err := containerizer.NewContainerRuntime("docker")
	if err != nil {
		return nil, fmt.Errorf("failed to create container runtime: %w", err)
	}

	return &ContainerRunner{
		containerizer: containerRuntime,
		definition:    definition,
		stopChan:      make(chan struct{}),
	}, nil
}

// Start starts the container
func (cr *ContainerRunner) Start(ctx context.Context) error {
	logging.Info("ContainerRunner", "Starting container for MCP server: %s", cr.definition.Name)

	// Validate container configuration
	if cr.definition.Image == "" {
		return fmt.Errorf("container image is required for container-type MCP server")
	}

	// Prepare container config
	config := containerizer.ContainerConfig{
		Name:        fmt.Sprintf("envctl-mcp-%s-%d", cr.definition.Name, time.Now().Unix()),
		Image:       cr.definition.Image,
		Env:         cr.convertEnvMap(),
		Ports:       cr.definition.ContainerPorts,
		Volumes:     cr.definition.ContainerVolumes,
		Entrypoint:  cr.definition.Entrypoint,
		User:        cr.definition.ContainerUser,
		HealthCheck: cr.definition.HealthCheckCmd,
	}

	// Pull image first
	if err := cr.containerizer.PullImage(ctx, cr.definition.Image); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// Start the container
	containerID, err := cr.containerizer.StartContainer(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	cr.containerID = containerID
	logging.Info("ContainerRunner", "Container started with ID: %s", containerID[:12])

	// Start health monitoring if configured
	if len(cr.definition.HealthCheckCmd) > 0 && cr.definition.HealthCheckInterval > 0 {
		go cr.startHealthMonitoring(ctx)
	}

	return nil
}

// Stop stops the container
func (cr *ContainerRunner) Stop(ctx context.Context) error {
	if cr.containerID == "" {
		return nil
	}

	logging.Info("ContainerRunner", "Stopping container: %s", cr.containerID[:12])

	// Signal health monitoring to stop
	select {
	case <-cr.stopChan:
		// Already closed
	default:
		close(cr.stopChan)
	}

	// Stop the container
	if err := cr.containerizer.StopContainer(ctx, cr.containerID); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Remove the container
	if err := cr.containerizer.RemoveContainer(ctx, cr.containerID); err != nil {
		logging.Warn("ContainerRunner", "Failed to remove container: %v", err)
	}

	cr.containerID = ""
	return nil
}

// IsRunning checks if the container is running
func (cr *ContainerRunner) IsRunning(ctx context.Context) bool {
	if cr.containerID == "" {
		return false
	}

	running, err := cr.containerizer.IsContainerRunning(ctx, cr.containerID)
	if err != nil {
		logging.Warn("ContainerRunner", "Failed to check container status: %v", err)
		return false
	}
	return running
}

// GetPorts returns the exposed ports of the container
func (cr *ContainerRunner) GetPorts(ctx context.Context) (map[string]string, error) {
	if cr.containerID == "" {
		return nil, fmt.Errorf("container not started")
	}

	ports := make(map[string]string)
	for _, portSpec := range cr.definition.ContainerPorts {
		parts := strings.Split(portSpec, ":")
		if len(parts) == 2 {
			containerPort := parts[1]
			hostPort, err := cr.containerizer.GetContainerPort(ctx, cr.containerID, containerPort)
			if err != nil {
				logging.Warn("ContainerRunner", "Failed to get port mapping for %s: %v", containerPort, err)
				continue
			}
			ports[containerPort] = hostPort
		}
	}

	return ports, nil
}

// GetContainerID returns the container ID
func (cr *ContainerRunner) GetContainerID() string {
	return cr.containerID
}

// convertEnvMap converts the environment map to the format expected by containerizer
func (cr *ContainerRunner) convertEnvMap() map[string]string {
	env := make(map[string]string)

	// Add local env variables
	for k, v := range cr.definition.Env {
		env[k] = v
	}

	// Add container-specific env variables
	for k, v := range cr.definition.ContainerEnv {
		env[k] = v
	}

	return env
}

// startHealthMonitoring starts health monitoring for the container
func (cr *ContainerRunner) startHealthMonitoring(ctx context.Context) {
	ticker := time.NewTicker(cr.definition.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-cr.stopChan:
			return
		case <-ticker.C:
			if !cr.IsRunning(ctx) {
				logging.Warn("ContainerRunner", "Container %s is not running", cr.definition.Name)
			}
		}
	}
}

// GetLogs retrieves container logs
func (cr *ContainerRunner) GetLogs(ctx context.Context) (string, error) {
	if cr.containerID == "" {
		return "", fmt.Errorf("container not started")
	}

	logsReader, err := cr.containerizer.GetContainerLogs(ctx, cr.containerID)
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}
	defer logsReader.Close()

	// Read a limited amount of logs
	buffer := make([]byte, 4096)
	n, err := logsReader.Read(buffer)
	if err != nil && err.Error() != "EOF" {
		return "", fmt.Errorf("failed to read container logs: %w", err)
	}

	return string(buffer[:n]), nil
}
