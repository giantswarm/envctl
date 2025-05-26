package reporting

import (
	"context"
	"sync"
	"time"
)

// PerformanceMonitor tracks performance metrics for the event system
type PerformanceMonitor struct {
	eventBus   EventBus
	stateStore StateStore
	metrics    PerformanceMetrics
	mu         sync.RWMutex
	stopChan   chan struct{}
	running    bool
}

// PerformanceMetrics contains detailed performance information
type PerformanceMetrics struct {
	// Event processing metrics
	EventsPerSecond       float64
	AverageEventLatency   time.Duration
	MaxEventLatency       time.Duration
	EventProcessingErrors int64

	// Memory usage metrics
	ActiveSubscriptions int
	TotalEventsSent     int64
	TotalEventsDropped  int64
	MemoryUsageBytes    int64

	// State store metrics
	StateStoreOperations int64
	StateStoreLatency    time.Duration
	StateStoreSize       int

	// System health
	LastUpdateTime   time.Time
	SystemHealthy    bool
	PerformanceScore float64 // 0-100 score
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(eventBus EventBus, stateStore StateStore) *PerformanceMonitor {
	return &PerformanceMonitor{
		eventBus:   eventBus,
		stateStore: stateStore,
		stopChan:   make(chan struct{}),
	}
}

// Start begins performance monitoring
func (pm *PerformanceMonitor) Start(ctx context.Context, interval time.Duration) {
	pm.mu.Lock()
	if pm.running {
		pm.mu.Unlock()
		return
	}
	pm.running = true
	pm.mu.Unlock()

	go pm.monitorLoop(ctx, interval)
}

// Stop stops performance monitoring
func (pm *PerformanceMonitor) Stop() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		close(pm.stopChan)
		pm.running = false
	}
}

// GetMetrics returns current performance metrics
func (pm *PerformanceMonitor) GetMetrics() PerformanceMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.metrics
}

// monitorLoop runs the monitoring loop
func (pm *PerformanceMonitor) monitorLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastEventCount int64
	var lastTime time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopChan:
			return
		case <-ticker.C:
			pm.updateMetrics(&lastEventCount, &lastTime)
		}
	}
}

// updateMetrics calculates and updates performance metrics
func (pm *PerformanceMonitor) updateMetrics(lastEventCount *int64, lastTime *time.Time) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()

	// Get event bus metrics
	busMetrics := pm.eventBus.GetMetrics()

	// Calculate events per second
	if !lastTime.IsZero() {
		timeDiff := now.Sub(*lastTime).Seconds()
		eventDiff := float64(busMetrics.EventsPublished - *lastEventCount)
		if timeDiff > 0 {
			pm.metrics.EventsPerSecond = eventDiff / timeDiff
		}
	}

	*lastEventCount = busMetrics.EventsPublished
	*lastTime = now

	// Update metrics from event bus
	pm.metrics.ActiveSubscriptions = busMetrics.ActiveSubscriptions
	pm.metrics.TotalEventsSent = busMetrics.EventsDelivered
	pm.metrics.TotalEventsDropped = busMetrics.EventsDropped
	pm.metrics.AverageEventLatency = busMetrics.AverageDeliveryTime

	// Get state store metrics
	if pm.stateStore != nil {
		storeMetrics := pm.stateStore.GetMetrics()
		pm.metrics.StateStoreSize = storeMetrics.TotalServices
		pm.metrics.StateStoreOperations = storeMetrics.StateChanges
	}

	// Calculate performance score
	pm.metrics.PerformanceScore = pm.calculatePerformanceScore(busMetrics)
	pm.metrics.SystemHealthy = pm.metrics.PerformanceScore > 70.0
	pm.metrics.LastUpdateTime = now
}

// calculatePerformanceScore calculates a 0-100 performance score
func (pm *PerformanceMonitor) calculatePerformanceScore(busMetrics EventBusMetrics) float64 {
	score := 100.0

	// Penalize for dropped events
	if busMetrics.EventsPublished > 0 {
		dropRate := float64(busMetrics.EventsDropped) / float64(busMetrics.EventsPublished)
		score -= dropRate * 30.0 // Up to 30 points penalty for drops
	}

	// Penalize for high latency
	if busMetrics.AverageDeliveryTime > 10*time.Millisecond {
		latencyPenalty := float64(busMetrics.AverageDeliveryTime/time.Millisecond) / 10.0
		score -= latencyPenalty * 20.0 // Up to 20 points penalty for latency
	}

	// Penalize for too many subscriptions (memory usage)
	if busMetrics.ActiveSubscriptions > 1000 {
		subscriptionPenalty := float64(busMetrics.ActiveSubscriptions-1000) / 1000.0
		score -= subscriptionPenalty * 10.0 // Up to 10 points penalty for subscriptions
	}

	if score < 0 {
		score = 0
	}
	return score
}

// EventBatchProcessor processes events in batches for better performance
type EventBatchProcessor struct {
	eventBus   EventBus
	batchSize  int
	flushTime  time.Duration
	eventQueue []Event
	mu         sync.Mutex
	stopChan   chan struct{}
	running    bool
}

// NewEventBatchProcessor creates a new batch processor
func NewEventBatchProcessor(eventBus EventBus, batchSize int, flushTime time.Duration) *EventBatchProcessor {
	return &EventBatchProcessor{
		eventBus:   eventBus,
		batchSize:  batchSize,
		flushTime:  flushTime,
		eventQueue: make([]Event, 0, batchSize),
		stopChan:   make(chan struct{}),
	}
}

// Start begins batch processing
func (bp *EventBatchProcessor) Start() {
	bp.mu.Lock()
	if bp.running {
		bp.mu.Unlock()
		return
	}
	bp.running = true
	bp.mu.Unlock()

	go bp.processLoop()
}

// Stop stops batch processing and flushes remaining events
func (bp *EventBatchProcessor) Stop() {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.running {
		close(bp.stopChan)
		bp.running = false
		bp.flush() // Flush remaining events
	}
}

// QueueEvent adds an event to the batch queue
func (bp *EventBatchProcessor) QueueEvent(event Event) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if !bp.running {
		// If not running, publish immediately
		bp.eventBus.Publish(event)
		return
	}

	bp.eventQueue = append(bp.eventQueue, event)

	// Flush if batch is full
	if len(bp.eventQueue) >= bp.batchSize {
		bp.flush()
	}
}

// processLoop runs the batch processing loop
func (bp *EventBatchProcessor) processLoop() {
	ticker := time.NewTicker(bp.flushTime)
	defer ticker.Stop()

	for {
		select {
		case <-bp.stopChan:
			return
		case <-ticker.C:
			bp.mu.Lock()
			bp.flush()
			bp.mu.Unlock()
		}
	}
}

// flush publishes all queued events (must be called with mutex held)
func (bp *EventBatchProcessor) flush() {
	if len(bp.eventQueue) == 0 {
		return
	}

	// Publish all events in the queue
	for _, event := range bp.eventQueue {
		bp.eventBus.Publish(event)
	}

	// Clear the queue
	bp.eventQueue = bp.eventQueue[:0]
}

// OptimizedEventBus is a wrapper around EventBus with performance optimizations
type OptimizedEventBus struct {
	EventBus
	monitor          *PerformanceMonitor
	batchProcessor   *EventBatchProcessor
	enableBatching   bool
	enableMonitoring bool
}

// NewOptimizedEventBus creates an optimized event bus
func NewOptimizedEventBus(config OptimizedEventBusConfig) *OptimizedEventBus {
	eventBus := NewEventBus()

	optimized := &OptimizedEventBus{
		EventBus:         eventBus,
		enableBatching:   config.EnableBatching,
		enableMonitoring: config.EnableMonitoring,
	}

	if config.EnableMonitoring {
		optimized.monitor = NewPerformanceMonitor(eventBus, config.StateStore)
		optimized.monitor.Start(context.Background(), config.MonitoringInterval)
	}

	if config.EnableBatching {
		optimized.batchProcessor = NewEventBatchProcessor(
			eventBus,
			config.BatchSize,
			config.BatchFlushTime,
		)
		optimized.batchProcessor.Start()
	}

	return optimized
}

// OptimizedEventBusConfig configures the optimized event bus
type OptimizedEventBusConfig struct {
	EnableBatching     bool
	EnableMonitoring   bool
	BatchSize          int
	BatchFlushTime     time.Duration
	MonitoringInterval time.Duration
	StateStore         StateStore
}

// DefaultOptimizedEventBusConfig returns sensible defaults
func DefaultOptimizedEventBusConfig() OptimizedEventBusConfig {
	return OptimizedEventBusConfig{
		EnableBatching:     true,
		EnableMonitoring:   true,
		BatchSize:          50,
		BatchFlushTime:     10 * time.Millisecond,
		MonitoringInterval: 1 * time.Second,
		StateStore:         nil, // Will be created if needed
	}
}

// Publish publishes an event, using batching if enabled
func (oeb *OptimizedEventBus) Publish(event Event) {
	if oeb.enableBatching && oeb.batchProcessor != nil {
		oeb.batchProcessor.QueueEvent(event)
	} else {
		oeb.EventBus.Publish(event)
	}
}

// GetPerformanceMetrics returns performance metrics if monitoring is enabled
func (oeb *OptimizedEventBus) GetPerformanceMetrics() *PerformanceMetrics {
	if oeb.enableMonitoring && oeb.monitor != nil {
		metrics := oeb.monitor.GetMetrics()
		return &metrics
	}
	return nil
}

// Close closes the optimized event bus and all its components
func (oeb *OptimizedEventBus) Close() {
	if oeb.batchProcessor != nil {
		oeb.batchProcessor.Stop()
	}
	if oeb.monitor != nil {
		oeb.monitor.Stop()
	}
	oeb.EventBus.Close()
}

// EventPoolManager manages object pools for events to reduce GC pressure
type EventPoolManager struct {
	serviceEventPool *sync.Pool
	healthEventPool  *sync.Pool
	depEventPool     *sync.Pool
	userEventPool    *sync.Pool
	systemEventPool  *sync.Pool
}

// NewEventPoolManager creates a new event pool manager
func NewEventPoolManager() *EventPoolManager {
	return &EventPoolManager{
		serviceEventPool: &sync.Pool{
			New: func() interface{} {
				return &ServiceStateEvent{}
			},
		},
		healthEventPool: &sync.Pool{
			New: func() interface{} {
				return &HealthEvent{}
			},
		},
		depEventPool: &sync.Pool{
			New: func() interface{} {
				return &DependencyEvent{}
			},
		},
		userEventPool: &sync.Pool{
			New: func() interface{} {
				return &UserActionEvent{}
			},
		},
		systemEventPool: &sync.Pool{
			New: func() interface{} {
				return &SystemEvent{}
			},
		},
	}
}

// GetServiceStateEvent gets a pooled ServiceStateEvent
func (epm *EventPoolManager) GetServiceStateEvent() *ServiceStateEvent {
	event := epm.serviceEventPool.Get().(*ServiceStateEvent)
	// Reset the event
	*event = ServiceStateEvent{}
	return event
}

// PutServiceStateEvent returns a ServiceStateEvent to the pool
func (epm *EventPoolManager) PutServiceStateEvent(event *ServiceStateEvent) {
	if event != nil {
		epm.serviceEventPool.Put(event)
	}
}

// GetHealthEvent gets a pooled HealthEvent
func (epm *EventPoolManager) GetHealthEvent() *HealthEvent {
	event := epm.healthEventPool.Get().(*HealthEvent)
	*event = HealthEvent{}
	return event
}

// PutHealthEvent returns a HealthEvent to the pool
func (epm *EventPoolManager) PutHealthEvent(event *HealthEvent) {
	if event != nil {
		epm.healthEventPool.Put(event)
	}
}

// GetDependencyEvent gets a pooled DependencyEvent
func (epm *EventPoolManager) GetDependencyEvent() *DependencyEvent {
	event := epm.depEventPool.Get().(*DependencyEvent)
	*event = DependencyEvent{}
	return event
}

// PutDependencyEvent returns a DependencyEvent to the pool
func (epm *EventPoolManager) PutDependencyEvent(event *DependencyEvent) {
	if event != nil {
		epm.depEventPool.Put(event)
	}
}

// GetUserActionEvent gets a pooled UserActionEvent
func (epm *EventPoolManager) GetUserActionEvent() *UserActionEvent {
	event := epm.userEventPool.Get().(*UserActionEvent)
	*event = UserActionEvent{}
	return event
}

// PutUserActionEvent returns a UserActionEvent to the pool
func (epm *EventPoolManager) PutUserActionEvent(event *UserActionEvent) {
	if event != nil {
		epm.userEventPool.Put(event)
	}
}

// GetSystemEvent gets a pooled SystemEvent
func (epm *EventPoolManager) GetSystemEvent() *SystemEvent {
	event := epm.systemEventPool.Get().(*SystemEvent)
	*event = SystemEvent{}
	return event
}

// PutSystemEvent returns a SystemEvent to the pool
func (epm *EventPoolManager) PutSystemEvent(event *SystemEvent) {
	if event != nil {
		epm.systemEventPool.Put(event)
	}
}
