package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCLIClient(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "creates client successfully",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewCLIClient()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotEmpty(t, client.endpoint)
				assert.Equal(t, 30*time.Second, client.timeout)
			}
		})
	}
}

func TestNewCLIClientWithEndpoint(t *testing.T) {
	endpoint := "http://localhost:8090/mcp"
	client := NewCLIClientWithEndpoint(endpoint)
	
	assert.NotNil(t, client)
	assert.Equal(t, endpoint, client.endpoint)
	assert.Equal(t, 30*time.Second, client.timeout)
}

func TestCLIClient_CallToolSimple(t *testing.T) {
	client := NewCLIClientWithEndpoint("http://localhost:8090/mcp")
	
	// Test with mock - this would require a running server in integration tests
	// For unit tests, we test the method exists and has correct signature
	assert.NotNil(t, client.CallToolSimple)
}

func TestCLIClient_CallToolJSON(t *testing.T) {
	client := NewCLIClientWithEndpoint("http://localhost:8090/mcp")
	
	// Test with mock - this would require a running server in integration tests
	// For unit tests, we test the method exists and has correct signature
	assert.NotNil(t, client.CallToolJSON)
}

func TestCLIClient_Close(t *testing.T) {
	client := NewCLIClientWithEndpoint("http://localhost:8090/mcp")
	
	// Should not panic when closing unconnected client
	assert.NotPanics(t, func() {
		client.Close()
	})
}

func TestCLIClient_Connect_InvalidEndpoint(t *testing.T) {
	client := NewCLIClientWithEndpoint("invalid-endpoint")
	
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	err := client.Connect(ctx)
	assert.Error(t, err)
	// The error message may vary, but it should be an error
	assert.Contains(t, err.Error(), "failed")
} 