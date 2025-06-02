package aggregator

import (
	"envctl/internal/aggregator"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAggregatorService(t *testing.T) {
	config := aggregator.AggregatorConfig{
		Host: "localhost",
		Port: 8080,
	}

	service := NewAggregatorService(config, nil)

	assert.NotNil(t, service)
	assert.Equal(t, "mcp-aggregator", service.GetLabel())
	assert.Equal(t, 0, len(service.GetDependencies()), "Should have no dependencies by default")
}

func TestNewAggregatorServiceWithDependencies(t *testing.T) {
	config := aggregator.AggregatorConfig{
		Host: "localhost",
		Port: 8080,
	}

	dependencies := []string{"mcp-server1", "mcp-server2", "mcp-server3"}
	service := NewAggregatorServiceWithDependencies(config, nil, dependencies)

	assert.NotNil(t, service)
	assert.Equal(t, "mcp-aggregator", service.GetLabel())
	assert.Equal(t, dependencies, service.GetDependencies(), "Should have the specified dependencies")
}
