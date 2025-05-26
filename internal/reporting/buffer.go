package reporting

import (
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// BufferAction defines what to do when a buffer is full
type BufferAction int

const (
	BufferActionDrop BufferAction = iota
	BufferActionBlock
	BufferActionEvictOldest
)

// String makes BufferAction satisfy the fmt.Stringer interface
func (ba BufferAction) String() string {
	switch ba {
	case BufferActionDrop:
		return "Drop"
	case BufferActionBlock:
		return "Block"
	case BufferActionEvictOldest:
		return "EvictOldest"
	default:
		return "Unknown"
	}
}

// BufferStrategy defines how to handle buffer overflow situations
type BufferStrategy interface {
	OnBufferFull(msg tea.Msg) BufferAction
}

// SimpleBufferStrategy implements a basic buffer strategy
type SimpleBufferStrategy struct {
	Action BufferAction
}

// NewSimpleBufferStrategy creates a buffer strategy with a single action for all messages
func NewSimpleBufferStrategy(action BufferAction) *SimpleBufferStrategy {
	return &SimpleBufferStrategy{Action: action}
}

// OnBufferFull returns the configured action for all messages
func (s *SimpleBufferStrategy) OnBufferFull(msg tea.Msg) BufferAction {
	return s.Action
}

// PriorityBufferStrategy implements a priority-based buffer strategy
type PriorityBufferStrategy struct {
	DefaultAction BufferAction
	PriorityRules map[string]BufferAction // Message type -> action
}

// NewPriorityBufferStrategy creates a priority-based buffer strategy
func NewPriorityBufferStrategy(defaultAction BufferAction) *PriorityBufferStrategy {
	return &PriorityBufferStrategy{
		DefaultAction: defaultAction,
		PriorityRules: make(map[string]BufferAction),
	}
}

// SetPriority sets the action for a specific message type
func (p *PriorityBufferStrategy) SetPriority(msgType string, action BufferAction) {
	p.PriorityRules[msgType] = action
}

// OnBufferFull returns the action based on message type priority
func (p *PriorityBufferStrategy) OnBufferFull(msg tea.Msg) BufferAction {
	msgType := getMessageType(msg)
	if action, exists := p.PriorityRules[msgType]; exists {
		return action
	}
	return p.DefaultAction
}

// getMessageType extracts the message type name for priority matching
func getMessageType(msg tea.Msg) string {
	switch msg.(type) {
	case ReporterUpdateMsg:
		return "ReporterUpdateMsg"
	case HealthStatusMsg:
		return "HealthStatusMsg"
	default:
		return "Unknown"
	}
}

// BufferedChannel wraps a channel with configurable buffer overflow behavior
type BufferedChannel struct {
	ch       chan tea.Msg
	strategy BufferStrategy
	metrics  *ChannelMetrics
	mu       sync.RWMutex
}

// ChannelMetrics tracks channel performance metrics
type ChannelMetrics struct {
	MessagesDropped  int64
	MessagesBlocked  int64
	MessagesEvicted  int64
	MessagesSent     int64
	LastDropTime     time.Time
	LastBlockTime    time.Time
	LastEvictionTime time.Time
	mu               sync.RWMutex
}

// ChannelStats represents a snapshot of channel metrics without the mutex
type ChannelStats struct {
	MessagesDropped  int64
	MessagesBlocked  int64
	MessagesEvicted  int64
	MessagesSent     int64
	LastDropTime     time.Time
	LastBlockTime    time.Time
	LastEvictionTime time.Time
}

// NewChannelMetrics creates a new metrics tracker
func NewChannelMetrics() *ChannelMetrics {
	return &ChannelMetrics{}
}

// IncrementDropped increments the dropped message counter
func (m *ChannelMetrics) IncrementDropped() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesDropped++
	m.LastDropTime = time.Now()
}

// IncrementBlocked increments the blocked message counter
func (m *ChannelMetrics) IncrementBlocked() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesBlocked++
	m.LastBlockTime = time.Now()
}

// IncrementEvicted increments the evicted message counter
func (m *ChannelMetrics) IncrementEvicted() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesEvicted++
	m.LastEvictionTime = time.Now()
}

// IncrementSent increments the sent message counter
func (m *ChannelMetrics) IncrementSent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesSent++
}

// GetStats returns a copy of the current metrics without the mutex
func (m *ChannelMetrics) GetStats() ChannelStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return ChannelStats{
		MessagesDropped:  m.MessagesDropped,
		MessagesBlocked:  m.MessagesBlocked,
		MessagesEvicted:  m.MessagesEvicted,
		MessagesSent:     m.MessagesSent,
		LastDropTime:     m.LastDropTime,
		LastBlockTime:    m.LastBlockTime,
		LastEvictionTime: m.LastEvictionTime,
	}
}

// NewBufferedChannel creates a new buffered channel with the given strategy
func NewBufferedChannel(size int, strategy BufferStrategy) *BufferedChannel {
	return &BufferedChannel{
		ch:       make(chan tea.Msg, size),
		strategy: strategy,
		metrics:  NewChannelMetrics(),
	}
}

// Send attempts to send a message using the configured buffer strategy
func (bc *BufferedChannel) Send(msg tea.Msg) bool {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	select {
	case bc.ch <- msg:
		bc.metrics.IncrementSent()
		return true
	default:
		// Buffer is full, apply strategy
		action := bc.strategy.OnBufferFull(msg)
		switch action {
		case BufferActionDrop:
			bc.metrics.IncrementDropped()
			return false
		case BufferActionBlock:
			bc.metrics.IncrementBlocked()
			// This will block until space is available
			bc.ch <- msg
			bc.metrics.IncrementSent()
			return true
		case BufferActionEvictOldest:
			// Try to evict the oldest message
			select {
			case <-bc.ch:
				bc.metrics.IncrementEvicted()
				// Now send the new message
				bc.ch <- msg
				bc.metrics.IncrementSent()
				return true
			default:
				// Channel is somehow empty now, just send
				bc.ch <- msg
				bc.metrics.IncrementSent()
				return true
			}
		default:
			bc.metrics.IncrementDropped()
			return false
		}
	}
}

// Receive receives a message from the channel
func (bc *BufferedChannel) Receive() tea.Msg {
	return <-bc.ch
}

// TryReceive attempts to receive a message without blocking
func (bc *BufferedChannel) TryReceive() (tea.Msg, bool) {
	select {
	case msg := <-bc.ch:
		return msg, true
	default:
		return nil, false
	}
}

// GetMetrics returns the current channel metrics
func (bc *BufferedChannel) GetMetrics() ChannelStats {
	return bc.metrics.GetStats()
}

// Close closes the underlying channel
func (bc *BufferedChannel) Close() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	close(bc.ch)
}

// Channel returns the underlying channel for compatibility
func (bc *BufferedChannel) Channel() <-chan tea.Msg {
	return bc.ch
}
