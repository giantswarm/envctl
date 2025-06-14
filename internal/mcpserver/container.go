package mcpserver

import (
	"bufio"
	"context"
	"envctl/internal/containerizer"
	"envctl/pkg/logging"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// StartAndManageContainerizedMcpServer starts and manages a containerized MCP server
func StartAndManageContainerizedMcpServer(
	serverConfig MCPServerDefinition,
	runtime containerizer.ContainerRuntime,
	updateFn McpUpdateFunc,
	wg *sync.WaitGroup,
) (containerID string, stopChan chan struct{}, initialError error) {
	label := serverConfig.Name
	subsystem := "MCPContainer-" + label

	logging.Info(subsystem, "Initializing containerized MCP server %s (image: %s)", label, serverConfig.Image)
	if updateFn != nil {
		updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ContainerInitializing"})
	}

	// Validate configuration
	if serverConfig.Image == "" {
		errMsg := fmt.Errorf("container image not defined for MCP server %s", label)
		logging.Error(subsystem, errMsg, "Cannot start containerized MCP server")
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ContainerStartFailed", ProcessErr: errMsg})
		}
		return "", nil, errMsg
	}

	ctx := context.Background()
	currentStopChan := make(chan struct{})

	// Pull the image if needed
	if err := runtime.PullImage(ctx, serverConfig.Image); err != nil {
		errMsg := fmt.Errorf("failed to pull image for %s: %w", label, err)
		logging.Error(subsystem, err, "Failed to pull container image")
		close(currentStopChan)
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ContainerStartFailed", ProcessErr: errMsg})
		}
		return "", nil, errMsg
	}

	// Prepare container configuration
	containerConfig := containerizer.ContainerConfig{
		Name:        fmt.Sprintf("envctl-mcp-%s-%d", label, time.Now().Unix()),
		Image:       serverConfig.Image,
		Env:         mergeEnvMaps(serverConfig.Env, serverConfig.ContainerEnv),
		Ports:       serverConfig.ContainerPorts,
		Volumes:     serverConfig.ContainerVolumes,
		Entrypoint:  serverConfig.Entrypoint,
		User:        serverConfig.ContainerUser,
		HealthCheck: serverConfig.HealthCheckCmd,
	}

	// Start the container
	cID, err := runtime.StartContainer(ctx, containerConfig)
	if err != nil {
		errMsg := fmt.Errorf("failed to start container for %s: %w", label, err)
		logging.Error(subsystem, err, "Failed to start container")
		close(currentStopChan)
		if updateFn != nil {
			updateFn(McpDiscreteStatusUpdate{Label: label, ProcessStatus: "ContainerStartFailed", ProcessErr: err})
		}
		return "", nil, errMsg
	}

	containerID = cID
	shortID := containerID
	if len(containerID) > 12 {
		shortID = containerID[:12]
	}
	logging.Debug(subsystem, "Container started successfully with ID %s", shortID)

	// Get logs reader
	logsReader, err := runtime.GetContainerLogs(ctx, containerID)
	if err != nil {
		logging.Error(subsystem, err, "Failed to get container logs, continuing anyway")
	}

	// Initialize with configured port or 0
	actualPort := 0

	if updateFn != nil {
		updateFn(McpDiscreteStatusUpdate{
			Label:         label,
			ProcessStatus: "ContainerRunning",
		})
	}

	// Start goroutine to manage the container
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		if logsReader != nil {
			defer logsReader.Close()
		}

		// Start log processing if we have a reader
		if logsReader != nil {
			go func() {
				scanner := bufio.NewScanner(logsReader)
				for scanner.Scan() {
					line := scanner.Text()

					// Log the line
					if strings.Contains(line, "ERROR") || strings.Contains(line, "FATAL") {
						logging.Error(subsystem+"-logs", nil, "%s", line)
					} else if strings.Contains(line, "WARN") {
						logging.Warn(subsystem+"-logs", "%s", line)
					} else {
						logging.Info(subsystem+"-logs", "%s", line)
					}

					// Try to detect the actual port if not set
					if actualPort == 0 {
						if detectedPort := detectPortFromLog(line); detectedPort > 0 {
							actualPort = detectedPort
							logging.Info(subsystem, "Detected container listening on port %d", actualPort)
							if updateFn != nil {
								updateFn(McpDiscreteStatusUpdate{
									Label:         label,
									ProcessStatus: "ContainerRunning",
								})
							}
						}
					}
				}
			}()
		}

		// If we still don't have a port and container has port mappings, try to get it from Docker
		if actualPort == 0 && len(serverConfig.ContainerPorts) > 0 {
			// Create a channel to wait for port detection
			portDetected := make(chan int, 1)

			// Try to get port mapping in a goroutine
			go func() {
				// Give container a moment to fully initialize port bindings
				retries := 10
				for i := 0; i < retries && actualPort == 0; i++ {
					// Try to get the first container port mapping
					for _, portMapping := range serverConfig.ContainerPorts {
						parts := strings.Split(portMapping, ":")
						if len(parts) >= 2 {
							containerPort := parts[len(parts)-1]
							if hostPort, err := runtime.GetContainerPort(ctx, containerID, containerPort); err == nil {
								if port, err := strconv.Atoi(hostPort); err == nil {
									portDetected <- port
									return
								}
							}
						}
					}
					// Small delay between retries
					select {
					case <-time.After(200 * time.Millisecond):
					case <-currentStopChan:
						return
					}
				}
				portDetected <- 0
			}()

			// Wait for port detection with timeout
			select {
			case detectedPort := <-portDetected:
				if detectedPort > 0 {
					actualPort = detectedPort
					logging.Info(subsystem, "Got container port mapping: %d", actualPort)
					if updateFn != nil {
						updateFn(McpDiscreteStatusUpdate{
							Label:         label,
							ProcessStatus: "ContainerRunning",
						})
					}
				}
			case <-time.After(3 * time.Second):
				logging.Warn(subsystem, "Timeout waiting for container port detection")
			case <-currentStopChan:
				logging.Debug(subsystem, "Stopped while waiting for port detection")
			}
		}

		// Monitor container status
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check if container is still running
				running, err := runtime.IsContainerRunning(ctx, containerID)
				if err != nil {
					logging.Error(subsystem, err, "Failed to check container status")
					if updateFn != nil {
						updateFn(McpDiscreteStatusUpdate{
							Label:         label,
							ProcessStatus: "ContainerStatusCheckFailed",
							ProcessErr:    err,
						})
					}
				} else if !running {
					logging.Info(subsystem, "Container has stopped")
					if updateFn != nil {
						updateFn(McpDiscreteStatusUpdate{
							Label:         label,
							ProcessStatus: "ContainerExited",
						})
					}
					return
				}

			case <-currentStopChan:
				shortID := containerID
				if len(containerID) > 12 {
					shortID = containerID[:12]
				}
				logging.Debug(subsystem, "Received stop signal for container %s", shortID)
				finalProcessStatus := "ContainerStoppedByUser"
				var stopErr error

				if err := runtime.StopContainer(ctx, containerID); err != nil {
					logging.Error(subsystem, err, "Failed to stop container")
					finalProcessStatus = "ContainerStopFailed"
					stopErr = err
				} else {
					logging.Info(subsystem, "Container stopped successfully")
					// Clean up by removing the container
					if err := runtime.RemoveContainer(ctx, containerID); err != nil {
						logging.Warn(subsystem, "Failed to remove container: %v", err)
					}
				}

				if updateFn != nil {
					updateFn(McpDiscreteStatusUpdate{
						Label:         label,
						ProcessStatus: finalProcessStatus,
						ProcessErr:    stopErr,
					})
				}
				return
			}
		}
	}()

	return containerID, currentStopChan, nil
}

// mergeEnvMaps merges two environment variable maps, with the second taking precedence
func mergeEnvMaps(env1, env2 map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range env1 {
		result[k] = v
	}
	for k, v := range env2 {
		result[k] = v
	}
	return result
}

// containsPort checks if a port mapping is already in the list
func containsPort(ports []string, portMapping string) bool {
	for _, p := range ports {
		if p == portMapping {
			return true
		}
	}
	return false
}

// detectPortFromLog tries to detect port information from log lines
func detectPortFromLog(line string) int {
	// Common patterns for port announcements
	patterns := []string{
		"Starting MCP SSE server on port",
		"MCP server running on",
		"Server running on port",
		"Listening on port",
		"listening on :",
		"Started server on :",
	}

	for _, pattern := range patterns {
		if strings.Contains(line, pattern) {
			// Try to extract port number
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "port" && i+1 < len(parts) {
					portStr := strings.TrimSuffix(parts[i+1], ",")
					portStr = strings.TrimSuffix(portStr, ".")
					if port, err := strconv.Atoi(portStr); err == nil {
						return port
					}
				}
				// Also check for :PORT pattern
				if strings.Contains(part, ":") {
					subparts := strings.Split(part, ":")
					if len(subparts) >= 2 {
						portStr := strings.TrimSuffix(subparts[len(subparts)-1], ",")
						portStr = strings.TrimSuffix(portStr, ".")
						if port, err := strconv.Atoi(portStr); err == nil {
							return port
						}
					}
				}
			}
		}
	}

	return 0
}
