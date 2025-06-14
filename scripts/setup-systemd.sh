#!/bin/bash

# Setup script for envctl systemd user service
# This script installs and enables the envctl systemd service

set -e

echo "🔧 Setting up envctl systemd user service..."

# Create systemd user directory if it doesn't exist
mkdir -p ~/.config/systemd/user

# Copy service file
echo "📁 Installing service file..."
cp envctl.service ~/.config/systemd/user/

# Reload systemd
echo "🔄 Reloading systemd daemon..."
systemctl --user daemon-reload

# Enable the service
echo "✅ Enabling envctl service..."
systemctl --user enable envctl.service

echo "📦 Building and installing envctl..."
go install .

echo "🚀 Starting envctl service..."
systemctl --user start envctl.service

echo "📊 Service status:"
systemctl --user status envctl.service --no-pager

echo ""
echo "✅ envctl systemd service setup complete!"
echo ""
echo "💡 Development workflow:"
echo "  ./scripts/dev-restart.sh                   # Build, install & restart"
echo "  systemctl --user status envctl.service     # Check status"
echo "  journalctl --user -u envctl.service -f     # Follow logs"
echo "  systemctl --user stop envctl.service       # Stop service"
echo "  systemctl --user disable envctl.service    # Disable auto-start" 