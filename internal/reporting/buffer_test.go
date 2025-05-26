package reporting

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimpleBufferStrategy(t *testing.T) {
	tests := []struct {
		name     string
		action   BufferAction
		expected BufferAction
	}{
		{"Drop strategy", BufferActionDrop, BufferActionDrop},
		{"Block strategy", BufferActionBlock, BufferActionBlock},
		{"EvictOldest strategy", BufferActionEvictOldest, BufferActionEvictOldest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := NewSimpleBufferStrategy(tt.action)
			result := strategy.OnBufferFull(ReporterUpdateMsg{})
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPriorityBufferStrategy(t *testing.T) {
	strategy := NewPriorityBufferStrategy(BufferActionDrop)
	strategy.SetPriority("ReporterUpdateMsg", BufferActionEvictOldest)

	tests := []struct {
		name           string
		msg            tea.Msg
		expectedAction BufferAction
	}{
		{
			"ReporterUpdateMsg uses priority",
			ReporterUpdateMsg{},
			BufferActionEvictOldest,
		},
		{
			"Unknown message uses default",
			"some string message",
			BufferActionDrop,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.OnBufferFull(tt.msg)
			assert.Equal(t, tt.expectedAction, result)
		})
	}
}

func TestBufferedChannel_Send_Success(t *testing.T) {
	strategy := NewSimpleBufferStrategy(BufferActionDrop)
	bc := NewBufferedChannel(2, strategy)
	defer bc.Close()

	// Should be able to send up to buffer size
	msg1 := ReporterUpdateMsg{Update: NewManagedServiceUpdate(ServiceTypeSystem, "test1", StateRunning)}
	msg2 := ReporterUpdateMsg{Update: NewManagedServiceUpdate(ServiceTypeSystem, "test2", StateRunning)}

	assert.True(t, bc.Send(msg1))
	assert.True(t, bc.Send(msg2))

	metrics := bc.GetMetrics()
	assert.Equal(t, int64(2), metrics.MessagesSent)
	assert.Equal(t, int64(0), metrics.MessagesDropped)
}

func TestBufferedChannel_Send_Drop(t *testing.T) {
	strategy := NewSimpleBufferStrategy(BufferActionDrop)
	bc := NewBufferedChannel(1, strategy)
	defer bc.Close()

	// Fill the buffer
	msg1 := ReporterUpdateMsg{Update: NewManagedServiceUpdate(ServiceTypeSystem, "test1", StateRunning)}
	msg2 := ReporterUpdateMsg{Update: NewManagedServiceUpdate(ServiceTypeSystem, "test2", StateRunning)}

	assert.True(t, bc.Send(msg1))
	assert.False(t, bc.Send(msg2)) // Should be dropped

	metrics := bc.GetMetrics()
	assert.Equal(t, int64(1), metrics.MessagesSent)
	assert.Equal(t, int64(1), metrics.MessagesDropped)
	assert.False(t, metrics.LastDropTime.IsZero())
}

func TestBufferedChannel_Send_EvictOldest(t *testing.T) {
	strategy := NewSimpleBufferStrategy(BufferActionEvictOldest)
	bc := NewBufferedChannel(1, strategy)
	defer bc.Close()

	// Fill the buffer
	msg1 := ReporterUpdateMsg{Update: NewManagedServiceUpdate(ServiceTypeSystem, "test1", StateRunning)}
	msg2 := ReporterUpdateMsg{Update: NewManagedServiceUpdate(ServiceTypeSystem, "test2", StateRunning)}

	assert.True(t, bc.Send(msg1))
	assert.True(t, bc.Send(msg2)) // Should evict msg1 and send msg2

	metrics := bc.GetMetrics()
	assert.Equal(t, int64(2), metrics.MessagesSent)
	assert.Equal(t, int64(1), metrics.MessagesEvicted)
	assert.False(t, metrics.LastEvictionTime.IsZero())

	// Verify msg2 is in the channel
	received := bc.Receive()
	receivedMsg := received.(ReporterUpdateMsg)
	assert.Equal(t, "test2", receivedMsg.Update.SourceLabel)
}

func TestBufferedChannel_Send_Block(t *testing.T) {
	strategy := NewSimpleBufferStrategy(BufferActionBlock)
	bc := NewBufferedChannel(1, strategy)
	defer bc.Close()

	// Fill the buffer
	msg1 := ReporterUpdateMsg{Update: NewManagedServiceUpdate(ServiceTypeSystem, "test1", StateRunning)}
	msg2 := ReporterUpdateMsg{Update: NewManagedServiceUpdate(ServiceTypeSystem, "test2", StateRunning)}

	assert.True(t, bc.Send(msg1))

	// Start a goroutine to receive the first message after a delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		bc.Receive()
	}()

	// This should block until the goroutine receives the first message
	start := time.Now()
	assert.True(t, bc.Send(msg2))
	duration := time.Since(start)

	// Should have taken at least 40ms (allowing for some timing variance)
	assert.Greater(t, duration, 40*time.Millisecond)

	metrics := bc.GetMetrics()
	assert.Equal(t, int64(2), metrics.MessagesSent)
	assert.Equal(t, int64(1), metrics.MessagesBlocked)
	assert.False(t, metrics.LastBlockTime.IsZero())
}

func TestBufferedChannel_TryReceive(t *testing.T) {
	strategy := NewSimpleBufferStrategy(BufferActionDrop)
	bc := NewBufferedChannel(2, strategy)
	defer bc.Close()

	// Empty channel
	msg, ok := bc.TryReceive()
	assert.False(t, ok)
	assert.Nil(t, msg)

	// Send a message
	testMsg := ReporterUpdateMsg{Update: NewManagedServiceUpdate(ServiceTypeSystem, "test", StateRunning)}
	bc.Send(testMsg)

	// Should receive the message
	msg, ok = bc.TryReceive()
	assert.True(t, ok)
	assert.NotNil(t, msg)
	receivedMsg := msg.(ReporterUpdateMsg)
	assert.Equal(t, "test", receivedMsg.Update.SourceLabel)
}

func TestChannelMetrics(t *testing.T) {
	metrics := NewChannelMetrics()

	// Test initial state
	stats := metrics.GetStats()
	assert.Equal(t, int64(0), stats.MessagesSent)
	assert.Equal(t, int64(0), stats.MessagesDropped)
	assert.Equal(t, int64(0), stats.MessagesBlocked)
	assert.Equal(t, int64(0), stats.MessagesEvicted)

	// Test incrementing
	metrics.IncrementSent()
	metrics.IncrementDropped()
	metrics.IncrementBlocked()
	metrics.IncrementEvicted()

	stats = metrics.GetStats()
	assert.Equal(t, int64(1), stats.MessagesSent)
	assert.Equal(t, int64(1), stats.MessagesDropped)
	assert.Equal(t, int64(1), stats.MessagesBlocked)
	assert.Equal(t, int64(1), stats.MessagesEvicted)
	assert.False(t, stats.LastDropTime.IsZero())
	assert.False(t, stats.LastBlockTime.IsZero())
	assert.False(t, stats.LastEvictionTime.IsZero())
}

func TestBufferAction_String(t *testing.T) {
	tests := []struct {
		action   BufferAction
		expected string
	}{
		{BufferActionDrop, "Drop"},
		{BufferActionBlock, "Block"},
		{BufferActionEvictOldest, "EvictOldest"},
		{BufferAction(999), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.action.String())
		})
	}
}

func TestGetMessageType(t *testing.T) {
	tests := []struct {
		name     string
		msg      tea.Msg
		expected string
	}{
		{
			"ReporterUpdateMsg",
			ReporterUpdateMsg{},
			"ReporterUpdateMsg",
		},
		{
			"BackpressureNotificationMsg",
			BackpressureNotificationMsg{},
			"BackpressureNotificationMsg",
		},
		{
			"Unknown message",
			struct{}{},
			"struct {}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMessageType(tt.msg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Integration test that verifies the complete buffer strategy workflow
func TestBufferedChannel_Integration(t *testing.T) {
	// Create a priority strategy
	strategy := NewPriorityBufferStrategy(BufferActionDrop)
	strategy.SetPriority("ReporterUpdateMsg", BufferActionEvictOldest)

	bc := NewBufferedChannel(2, strategy)
	defer bc.Close()

	// Fill buffer with regular messages
	msg1 := ReporterUpdateMsg{Update: NewManagedServiceUpdate(ServiceTypeSystem, "test1", StateRunning)}
	msg2 := ReporterUpdateMsg{Update: NewManagedServiceUpdate(ServiceTypeSystem, "test2", StateRunning)}

	require.True(t, bc.Send(msg1))
	require.True(t, bc.Send(msg2))

	// Try to send another ReporterUpdateMsg - should evict oldest
	msg3 := ReporterUpdateMsg{Update: NewManagedServiceUpdate(ServiceTypeSystem, "test3", StateRunning)}
	require.True(t, bc.Send(msg3))

	// Verify metrics
	metrics := bc.GetMetrics()
	assert.Equal(t, int64(3), metrics.MessagesSent)
	assert.Equal(t, int64(1), metrics.MessagesEvicted)
	assert.Equal(t, int64(0), metrics.MessagesDropped)

	// Verify msg2 and msg3 are in the channel (msg1 was evicted)
	received1 := bc.Receive().(ReporterUpdateMsg)
	received2 := bc.Receive().(ReporterUpdateMsg)

	labels := []string{received1.Update.SourceLabel, received2.Update.SourceLabel}
	assert.Contains(t, labels, "test2")
	assert.Contains(t, labels, "test3")
	assert.NotContains(t, labels, "test1") // This was evicted
}
