# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- Kubernetes connections are now modeled as dependencies in the service dependency graph
- Cascading stop functionality: stopping a service automatically stops all dependent services
- K8s connection health monitoring with automatic service lifecycle management
- Port forwards now depend on their kubernetes context being authenticated and healthy
- The kubernetes MCP server depends on the management cluster connection
- When k8s connections become unhealthy, dependent services are automatically stopped
- Manual stop (x key) now uses cascading stop to cleanly shut down dependent services
- New `StartServicesDependingOn` method in ServiceManager to restart services when dependencies recover
- New `orchestrator` package that manages application state and service lifecycle for both TUI and non-TUI modes
- New `HealthStatusUpdate` and `ReportHealth` for proper health status reporting

### Changed
- Dependency graph now includes K8sConnection nodes as fundamental dependencies
- Service manager's StopServiceWithDependents method handles cascading stops
- Health check failures trigger automatic cleanup of dependent services
- Non-TUI mode now uses the orchestrator for health monitoring and dependency management
- TUI mode no longer performs its own health checks - the orchestrator handles all health monitoring and the TUI only displays results
- Proper separation of concerns: orchestrator manages health checks and service lifecycle, TUI only displays status

### Fixed
- Services no longer continue running with broken k8s dependencies
- Port forwards and MCP servers properly shut down when k8s connection is lost
- Services are now properly restarted when k8s connection is restored after a network failure
- TUI correctly updates service states when they are stopped due to k8s connection loss
- Fixed issue where port forwards were not stopped when k8s connection failed (k8s nodes are now skipped in StopServiceWithDependents)
- Both TUI and non-TUI modes now have consistent service lifecycle management
- Removed duplicated health check logic from TUI - orchestrator is the single source of truth

## [Previous]

### Added
- Support for containerized MCP servers (#41)
  - New `container` type for MCP server configuration
  - Docker-based execution with automatic container lifecycle management
  - Container-specific configuration fields: image, ports, volumes, environment
  - Automatic port detection from container logs
  - Health check support for containers
  - Example Dockerfiles for kubernetes, prometheus, and grafana MCP servers
  - GitHub Actions workflow for building and publishing container images

### Changed
- MCP server configuration now supports both `localCommand` and `container` types
- Updated documentation with containerized MCP server guide

### Technical Details
- Added `containerizer` package for container runtime abstraction
- Implemented Docker runtime with support for pull, start, stop, logs operations
- Extended MCP server startup logic to handle containerized servers
- Added container ID tracking in managed server info 