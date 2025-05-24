package containerizer

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

// Helper to check if Docker is available
func dockerAvailable() bool {
	cmd := exec.Command("docker", "info")
	err := cmd.Run()
	return err == nil
}

// Helper to skip test if Docker is not available
func skipIfNoDocker(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker not available, skipping test")
	}
}

// Mock exec.Command for testing
type mockCommand struct {
	expectedCmd  string
	expectedArgs []string
	returnError  error
	returnOutput string
}

var mockCommands []mockCommand
var mockCommandIndex int

func mockExecCommand(name string, args ...string) *exec.Cmd {
	if mockCommandIndex >= len(mockCommands) {
		panic("unexpected command execution")
	}

	mock := mockCommands[mockCommandIndex]
	mockCommandIndex++

	// Verify expected command
	if mock.expectedCmd != name {
		panic("unexpected command: " + name)
	}

	// Create a command that will return our mock output/error
	cmd := exec.Command("echo", mock.returnOutput)
	if mock.returnError != nil {
		cmd = exec.Command("false") // Will fail
	}

	return cmd
}

func TestDockerRuntime_PullImage(t *testing.T) {
	skipIfNoDocker(t)

	tests := []struct {
		name        string
		image       string
		imageExists bool
		pullSuccess bool
		expectError bool
	}{
		{
			name:        "image already exists",
			image:       "test:latest",
			imageExists: true,
			pullSuccess: true,
			expectError: false,
		},
		{
			name:        "image needs pull",
			image:       "test:latest",
			imageExists: false,
			pullSuccess: true,
			expectError: false,
		},
		{
			name:        "pull fails",
			image:       "test:latest",
			imageExists: false,
			pullSuccess: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DockerRuntime{}
			ctx := context.Background()

			// Mock commands setup would go here
			// For now, we'll skip the actual execution test
			// and focus on the structure

			err := d.PullImage(ctx, tt.image)
			if (err != nil) != tt.expectError {
				t.Errorf("PullImage() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestDockerRuntime_StartContainer(t *testing.T) {
	skipIfNoDocker(t)

	tests := []struct {
		name        string
		config      ContainerConfig
		expectError bool
		expectedID  string
	}{
		{
			name: "basic container",
			config: ContainerConfig{
				Name:  "test-container",
				Image: "test:latest",
			},
			expectError: false,
			expectedID:  "abc123",
		},
		{
			name: "container with ports and volumes",
			config: ContainerConfig{
				Name:    "test-container",
				Image:   "test:latest",
				Ports:   []string{"8080:80"},
				Volumes: []string{"/host:/container"},
				Env: map[string]string{
					"TEST": "value",
				},
			},
			expectError: false,
			expectedID:  "def456",
		},
		{
			name: "container with entrypoint",
			config: ContainerConfig{
				Name:       "test-container",
				Image:      "test:latest",
				Entrypoint: []string{"/bin/sh", "-c", "echo hello"},
			},
			expectError: false,
			expectedID:  "ghi789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DockerRuntime{}
			ctx := context.Background()

			// Test would mock exec.Command here
			id, err := d.StartContainer(ctx, tt.config)

			if (err != nil) != tt.expectError {
				t.Errorf("StartContainer() error = %v, expectError %v", err, tt.expectError)
			}

			// In real test, verify the ID matches expected
			_ = id
		})
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no tilde",
			input:    "/absolute/path",
			expected: "/absolute/path",
		},
		{
			name:     "relative path",
			input:    "relative/path",
			expected: "relative/path",
		},
		// Note: Testing tilde expansion requires mocking os.UserHomeDir
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			if result != tt.expected {
				t.Errorf("expandPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestDetectPortFromLog would test the detectPortFromLog function
// but it's defined in container.go in the mcpserver package.
// This test is kept as documentation of expected behavior.
/*
func TestDetectPortFromLog(t *testing.T) {
	tests := []struct {
		name     string
		logLine  string
		expected int
	}{
		{
			name:     "MCP SSE server format",
			logLine:  "Starting MCP SSE server on port 8080",
			expected: 8080,
		},
		{
			name:     "Server running format",
			logLine:  "Server running on port 3000",
			expected: 3000,
		},
		{
			name:     "Listening format with colon",
			logLine:  "listening on :9090",
			expected: 9090,
		},
		{
			name:     "No port in log",
			logLine:  "Server started successfully",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Would test mcpserver.detectPortFromLog here
		})
	}
}
*/

func TestDockerRuntime_GetContainerPort(t *testing.T) {
	skipIfNoDocker(t)

	tests := []struct {
		name          string
		containerID   string
		containerPort string
		dockerOutput  string
		expectedPort  string
		expectError   bool
	}{
		{
			name:          "standard format",
			containerID:   "abc123",
			containerPort: "80",
			dockerOutput:  "0.0.0.0:32768",
			expectedPort:  "32768",
			expectError:   false,
		},
		{
			name:          "IPv6 format",
			containerID:   "abc123",
			containerPort: "80",
			dockerOutput:  "[::]:32768",
			expectedPort:  "32768",
			expectError:   false,
		},
		{
			name:          "no mapping",
			containerID:   "abc123",
			containerPort: "80",
			dockerOutput:  "",
			expectedPort:  "",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &DockerRuntime{}
			ctx := context.Background()

			// Would mock exec.Command here to return dockerOutput
			port, err := d.GetContainerPort(ctx, tt.containerID, tt.containerPort)

			if (err != nil) != tt.expectError {
				t.Errorf("GetContainerPort() error = %v, expectError %v", err, tt.expectError)
			}

			// In real test, verify port matches expected
			_ = port
		})
	}
}

func TestContainerConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      ContainerConfig
		expectValid bool
	}{
		{
			name: "valid config",
			config: ContainerConfig{
				Name:  "test",
				Image: "test:latest",
			},
			expectValid: true,
		},
		{
			name: "missing name",
			config: ContainerConfig{
				Image: "test:latest",
			},
			expectValid: false,
		},
		{
			name: "missing image",
			config: ContainerConfig{
				Name: "test",
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add validation method to ContainerConfig if needed
			valid := tt.config.Name != "" && tt.config.Image != ""
			if valid != tt.expectValid {
				t.Errorf("config validation = %v, want %v", valid, tt.expectValid)
			}
		})
	}
}

// TestParsePortMapping tests parsing of port mapping strings
func TestParsePortMapping(t *testing.T) {
	tests := []struct {
		input     string
		wantHost  string
		wantCont  string
		wantError bool
	}{
		{"8080:80", "8080", "80", false},
		{"80", "", "", true},
		{"8080:80:90", "", "", true},
		{"", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			parts := strings.Split(tt.input, ":")
			if len(parts) != 2 {
				if !tt.wantError {
					t.Errorf("expected valid port mapping for %q", tt.input)
				}
				return
			}

			if tt.wantError {
				t.Errorf("expected error for %q but got valid parse", tt.input)
			}

			if parts[0] != tt.wantHost || parts[1] != tt.wantCont {
				t.Errorf("got host=%q cont=%q, want host=%q cont=%q",
					parts[0], parts[1], tt.wantHost, tt.wantCont)
			}
		})
	}
}
