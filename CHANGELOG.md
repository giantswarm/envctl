# Changelog

## [Unreleased]

### Added
- Support for containerized MCP servers (#41)
  - New `type: container` option for MCP server definitions
  - Docker runtime implementation in `internal/containerizer` package
  - Container lifecycle management (pull, start, stop, logs, port detection)
  - Example Dockerfiles for kubernetes, prometheus, and grafana MCP servers
  - GitHub Actions workflow for building and publishing container images
  - Documentation for containerized MCP server configuration

### Changed
- Updated `MCPServerDefinition` to support container-specific fields
- Modified `StartAllMCPServers` to handle both local command and container types
- Enhanced startup logic to initialize container runtime when needed

## [Previous versions...] 