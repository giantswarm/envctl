package reporting

import (
	"fmt"
	"sort"
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

// getMessageType returns a string representation of the message type for metrics
func getMessageType(msg tea.Msg) string {
	switch msg.(type) {
	case ReporterUpdateMsg:
		return "ReporterUpdateMsg"
	case BackpressureNotificationMsg:
		return "BackpressureNotificationMsg"
	default:
		return fmt.Sprintf("%T", msg)
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

// SequencedMessage wraps a message with sequence information for ordering
type SequencedMessage struct {
	Message  tea.Msg
	Sequence int64
}

// MessageBuffer handles message ordering and buffering
type MessageBuffer struct {
	buffer         []SequencedMessage
	expectedSeq    int64
	maxBufferSize  int
	reorderTimeout time.Duration
	mu             sync.Mutex
}

// NewMessageBuffer creates a new message buffer for handling out-of-order messages
func NewMessageBuffer(maxSize int, timeout time.Duration) *MessageBuffer {
	return &MessageBuffer{
		buffer:         make([]SequencedMessage, 0, maxSize),
		expectedSeq:    1,
		maxBufferSize:  maxSize,
		reorderTimeout: timeout,
	}
}

// AddMessage adds a message to the buffer and returns any messages that can be delivered in order
func (mb *MessageBuffer) AddMessage(msg tea.Msg, sequence int64) []tea.Msg {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	// If this is the expected sequence, we can deliver it immediately
	if sequence == mb.expectedSeq {
		mb.expectedSeq++
		result := []tea.Msg{msg}

		// Check if we can deliver any buffered messages
		result = append(result, mb.deliverBufferedMessages()...)
		return result
	}

	// If sequence is older than expected, it's a duplicate or very late message - ignore it
	if sequence < mb.expectedSeq {
		return nil
	}

	// Buffer the message for later delivery
	mb.buffer = append(mb.buffer, SequencedMessage{
		Message:  msg,
		Sequence: sequence,
	})

	// Sort buffer by sequence number
	sort.Slice(mb.buffer, func(i, j int) bool {
		return mb.buffer[i].Sequence < mb.buffer[j].Sequence
	})

	// Limit buffer size to prevent memory issues
	if len(mb.buffer) > mb.maxBufferSize {
		// Remove oldest messages that are too far ahead
		mb.buffer = mb.buffer[len(mb.buffer)-mb.maxBufferSize:]
	}

	// Try to deliver any messages that are now in order
	return mb.deliverBufferedMessages()
}

// deliverBufferedMessages delivers any messages that are now in the correct sequence
func (mb *MessageBuffer) deliverBufferedMessages() []tea.Msg {
	var result []tea.Msg

	for len(mb.buffer) > 0 && mb.buffer[0].Sequence == mb.expectedSeq {
		result = append(result, mb.buffer[0].Message)
		mb.expectedSeq++
		mb.buffer = mb.buffer[1:]
	}

	return result
}

// ForceDeliverOldMessages delivers messages that have been waiting too long
func (mb *MessageBuffer) ForceDeliverOldMessages(cutoffTime time.Time) []tea.Msg {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	var result []tea.Msg
	var remaining []SequencedMessage

	for _, buffered := range mb.buffer {
		// For simplicity, we'll deliver all buffered messages when forcing
		// In a real implementation, you'd check timestamps
		result = append(result, buffered.Message)
		if buffered.Sequence >= mb.expectedSeq {
			mb.expectedSeq = buffered.Sequence + 1
		}
	}

	mb.buffer = remaining
	return result
}
