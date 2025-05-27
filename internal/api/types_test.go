package api

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestMCPTool(t *testing.T) {
	// Test MCPTool structure
	tool := MCPTool{
		Name:        "test-tool",
		Description: "A test tool for testing",
	}

	if tool.Name != "test-tool" {
		t.Errorf("Expected Name to be 'test-tool', got %s", tool.Name)
	}

	if tool.Description != "A test tool for testing" {
		t.Errorf("Expected Description to be 'A test tool for testing', got %s", tool.Description)
	}
}

func TestMCPToolJSON(t *testing.T) {
	// Test JSON marshaling/unmarshaling
	tool := MCPTool{
		Name:        "json-tool",
		Description: "Tool for JSON testing",
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("Error marshaling MCPTool to JSON: %v", err)
	}

	// Unmarshal from JSON
	var unmarshaledTool MCPTool
	err = json.Unmarshal(jsonData, &unmarshaledTool)
	if err != nil {
		t.Fatalf("Error unmarshaling MCPTool from JSON: %v", err)
	}

	// Verify fields
	if unmarshaledTool.Name != tool.Name {
		t.Errorf("Expected Name to be %s, got %s", tool.Name, unmarshaledTool.Name)
	}

	if unmarshaledTool.Description != tool.Description {
		t.Errorf("Expected Description to be %s, got %s", tool.Description, unmarshaledTool.Description)
	}
}

func TestMCPToolUpdateEvent(t *testing.T) {
	// Test MCPToolUpdateEvent structure
	tools := []MCPTool{
		{Name: "tool1", Description: "First tool"},
		{Name: "tool2", Description: "Second tool"},
	}

	testErr := errors.New("test error")

	event := MCPToolUpdateEvent{
		ServerName: "test-server",
		Tools:      tools,
		Error:      testErr,
	}

	if event.ServerName != "test-server" {
		t.Errorf("Expected ServerName to be 'test-server', got %s", event.ServerName)
	}

	if len(event.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(event.Tools))
	}

	if event.Tools[0].Name != "tool1" {
		t.Errorf("Expected first tool name to be 'tool1', got %s", event.Tools[0].Name)
	}

	if event.Tools[1].Name != "tool2" {
		t.Errorf("Expected second tool name to be 'tool2', got %s", event.Tools[1].Name)
	}

	if event.Error != testErr {
		t.Errorf("Expected error to be %v, got %v", testErr, event.Error)
	}
}

func TestMCPToolUpdateEventWithoutError(t *testing.T) {
	// Test MCPToolUpdateEvent without error
	tools := []MCPTool{
		{Name: "success-tool", Description: "Successful tool"},
	}

	event := MCPToolUpdateEvent{
		ServerName: "success-server",
		Tools:      tools,
		Error:      nil,
	}

	if event.ServerName != "success-server" {
		t.Errorf("Expected ServerName to be 'success-server', got %s", event.ServerName)
	}

	if len(event.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(event.Tools))
	}

	if event.Error != nil {
		t.Errorf("Expected no error, got %v", event.Error)
	}
}

func TestEmptyMCPToolUpdateEvent(t *testing.T) {
	// Test empty MCPToolUpdateEvent
	event := MCPToolUpdateEvent{}

	if event.ServerName != "" {
		t.Errorf("Expected empty ServerName, got %s", event.ServerName)
	}

	if event.Tools != nil {
		t.Errorf("Expected nil Tools, got %v", event.Tools)
	}

	if event.Error != nil {
		t.Errorf("Expected nil Error, got %v", event.Error)
	}
}
