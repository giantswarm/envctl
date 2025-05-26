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
- Health-aware startup: Services now wait for their K8s dependencies to be healthy before starting
- Add comprehensive dependency management system for services
  - Services now track why they were stopped (manual vs dependency cascade)
  - Automatically restart services when their dependencies recover
  - Ensure correct startup order based on dependency graph
  - Prevent manually stopped services from auto-restarting
- **Phase 1 of Issue #45: Message Handling Architecture Improvements**
  - Added correlation ID support to `ManagedServiceUpdate` for tracing related messages and cascading effects
  - Implemented configurable buffer strategies for TUI message channels:
    - `BufferActionDrop`: Drop messages when buffer is full
    - `BufferActionBlock`: Block until space is available
    - `BufferActionEvictOldest`: Remove oldest message to make room for new ones
  - Added priority-based buffer strategies to handle different message types differently
  - Introduced `BufferedChannel` with metrics tracking (messages sent, dropped, blocked, evicted)
  - Enhanced orchestrator with correlation tracking for health checks and cascading operations
  - Updated service manager to use new correlation ID system for better debugging
  - Added comprehensive test coverage for buffer strategies and correlation tracking
- **Phase 2 of Issue #45: State Consolidation**
  - Implemented centralized `StateStore` as single source of truth for all service states
  - Added `ServiceStateSnapshot` for complete state information with correlation tracking
  - Introduced state change subscriptions with `StateSubscription` for reactive updates
  - Enhanced `ServiceReporter` interface with `GetStateStore()` method for direct state access
  - Updated `TUIReporter` and `ConsoleReporter` to use centralized state management
  - Migrated `ServiceManager` from local state tracking to centralized `StateStore`
  - Added comprehensive metrics tracking for state changes and subscription performance
  - Implemented state change event system with old/new state tracking
  - Added support for filtering services by type and state
  - Maintained full backwards compatibility while eliminating state duplication
- **Phase 3 of Issue #45: Structured Event System**
  - Implemented comprehensive event hierarchy with semantic event types:
    - `ServiceStateEvent` for service lifecycle changes with old/new state tracking
    - `HealthEvent` for cluster health status updates
    - `DependencyEvent` for cascade start/stop operations
    - `UserActionEvent` for user-initiated actions
    - `SystemEvent` for system-level operations
  - Added `EventBus` interface with publish/subscribe functionality
  - Implemented flexible event filtering system with composable filters:
    - Filter by event type, source, severity, correlation ID
    - Combine filters with AND/OR logic for complex subscriptions
  - Created `EventBusAdapter` for backwards compatibility with existing `ServiceReporter` interface
  - Added comprehensive event metrics tracking (published, delivered, dropped events)
  - Implemented both handler-based and channel-based event subscriptions
  - Added event severity levels (trace, debug, info, warn, error, fatal) for better categorization
  - Enhanced correlation tracking with event metadata support
  - Provided thread-safe concurrent event publishing and subscription management
  - Added extensive test coverage for all event types and bus functionality
- **Phase 4 of Issue #45: Testing and Polish**
  - Added comprehensive integration tests covering end-to-end event flows
  - Implemented performance monitoring utilities with `PerformanceMonitor` and metrics tracking
  - Created event batching system with `EventBatchProcessor` for high-volume scenarios
  - Built `OptimizedEventBus` with configurable performance optimizations
  - Added object pooling system with `EventPoolManager` to reduce GC pressure
  - Implemented extensive error recovery testing including panic handling
  - Added memory usage monitoring and subscription cleanup verification
  - Created comprehensive documentation covering architecture, usage, and best practices
  - Fixed race conditions in event bus concurrent access patterns
  - Enhanced thread safety across all components with proper synchronization
  - Provided migration guides and troubleshooting documentation
  - Achieved high test coverage with robust integration and unit tests

### Changed
- Dependency graph now includes K8sConnection nodes as fundamental dependencies
- Service manager's StopServiceWithDependents method handles cascading stops
- Health check failures trigger automatic cleanup of dependent services
- Non-TUI mode now uses the orchestrator for health monitoring and dependency management
- TUI mode no longer performs its own health checks - the orchestrator handles all health monitoring and the TUI only displays results
- Proper separation of concerns: orchestrator manages health checks and service lifecycle, TUI only displays status
- Orchestrator now performs initial health check before starting services
- Refactored TUI message handling system
  - Introduced specialized controller/dispatcher for better separation of concerns
  - Controllers now focus on single responsibilities
  - Better error handling and logging throughout the message flow
- Improved startup behavior - the UI now shows loading state until all clusters are fully loaded
- Port forwards no longer start before K8s health checks pass - orchestrator now checks K8s health before starting dependent services
- `ManagedServiceUpdate` now includes `CorrelationID`, `CausedBy`, and `ParentID` fields for tracing
- `TUIReporter` now uses configurable buffered channels instead of simple channels
- Service state updates now include correlation information in logs
- Orchestrator operations (stop/restart) now generate and track correlation IDs

### Fixed
- Services no longer continue running with broken k8s dependencies
- Port forwards and MCP servers properly shut down when k8s connection is lost
- Services are now properly restarted when k8s connection is restored after a network failure
- TUI correctly updates service states when they are stopped due to k8s connection loss
- Fixed issue where port forwards were not stopped when k8s connection failed (k8s nodes are now skipped in StopServiceWithDependents)
- Both TUI and non-TUI modes now have consistent service lifecycle management
- Removed duplicated health check logic from TUI - orchestrator is the single source of truth
- Fixed services not restarting after K8s connection recovery - `StartServicesDependingOn` now finds and restarts all transitive dependencies, not just direct ones
- Fixed service configs being lost - `StartServicesWithDependencyOrder` now properly stores service configurations for recovery after health failures
- Manually stopped services are correctly excluded from automatic restart when K8s connections recover
- Fixed race condition in restart logic that could cause services to get stuck in "Stopping" state
- Fix issue where MCPs did not restart when their required port forwards restarted
- Fix issue where MCPs would start before their required port forwards
- Fix transitive dependency restart - all dependent services now restart when K8s connections recover
- Fix port forwards starting immediately without waiting for K8s health checks
- Fix service getting stuck in "Stopping" state after restart and stop sequence
  - Resolved cascade logic incorrectly including initiating service in its own cascade
- Fixed issue where MCPs would not restart when their dependent port forwarding was restarted
- Fixed issue where only port forwardings would restart after K8s recovery, not their dependent MCPs
- Fixed issue where MCPs would start before their required port forwardings
- Fixed issue where services would get stuck in "Stopping" state after restart and stop operations

### Technical Details
- New helper functions: `NewManagedServiceUpdate()`, `WithCause()`, `WithError()`, `WithServiceData()`
- New types: `BufferStrategy`, `BufferedChannel`, `ChannelMetrics`, `ChannelStats`
- Backwards compatibility maintained for existing interfaces
- All existing tests updated and new comprehensive test suite added

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

## [0.6.0] - 2025-01-15 