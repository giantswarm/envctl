package tui

import "testing"

// Tests for the unexported helper nextFocus located in handler_key_global.go.
// The helper is responsible for wrapping focus navigation across panels.
func TestNextFocus(t *testing.T) {
	order := []string{"a", "b", "c"}

	// Empty order should return current unchanged.
	if got := nextFocus([]string{}, "x", 1); got != "x" {
		t.Errorf("empty order: expected current unchanged, got %q", got)
	}

	// Forward movement.
	if got := nextFocus(order, "a", 1); got != "b" {
		t.Errorf("forward: expected 'b', got %q", got)
	}

	// Backward movement.
	if got := nextFocus(order, "b", -1); got != "a" {
		t.Errorf("backward: expected 'a', got %q", got)
	}

	// Wrap-around forward.
	if got := nextFocus(order, "c", 1); got != "a" {
		t.Errorf("wrap forward: expected 'a', got %q", got)
	}

	// Wrap-around backward.
	if got := nextFocus(order, "a", -1); got != "c" {
		t.Errorf("wrap backward: expected 'c', got %q", got)
	}

	// Current not found – forward.
	if got := nextFocus(order, "x", 1); got != "a" {
		t.Errorf("not found forward: expected 'a', got %q", got)
	}

	// Current not found – backward.
	if got := nextFocus(order, "x", -1); got != "c" {
		t.Errorf("not found backward: expected 'c', got %q", got)
	}
}
