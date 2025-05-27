#!/bin/bash

# Test script for v2 architecture
# This script helps verify that MCP server ports are properly displayed

echo "Testing envctl v2 architecture..."
echo "================================"
echo ""
echo "This test will:"
echo "1. Start envctl with v2 architecture enabled"
echo "2. Connect to a test cluster"
echo "3. You should verify that MCP server ports are displayed correctly"
echo ""
echo "Press Ctrl+C to exit the TUI when done testing"
echo ""
echo "Starting in 3 seconds..."
sleep 3

# Run envctl with v2 architecture enabled
ENVCTL_V2=true go run main.go connect golem --debug-tui 