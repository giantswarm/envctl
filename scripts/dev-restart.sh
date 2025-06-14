#!/bin/bash

# Development restart script for envctl
# This script builds, installs, and restarts the envctl systemd service

set -e

echo "🔨 Building envctl..."
go build -o envctl .

echo "📦 Installing envctl to $(go env GOPATH)/bin..."
go install .

echo "🔄 Restarting envctl service..."
systemctl --user restart envctl.service

echo "📊 Checking service status..."
systemctl --user status envctl.service --no-pager

echo "📝 Recent logs:"
journalctl --user -u envctl.service --no-pager -n 10

echo "✅ envctl restarted successfully!"
echo ""
echo "💡 Useful commands:"
echo "  systemctl --user status envctl.service     # Check status"
echo "  journalctl --user -u envctl.service -f     # Follow logs"
echo "  systemctl --user stop envctl.service       # Stop service"
echo "  systemctl --user start envctl.service      # Start service" 