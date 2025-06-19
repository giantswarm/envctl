package testing

import (
	"fmt"
	"os"
	"reflect"
	"time"

	"envctl/internal/template"
)

// MockToolHandler handles mock tool calls with configurable responses
type MockToolHandler struct {
	config         MockToolConfig
	templateEngine *template.Engine
	debug          bool
}

// NewMockToolHandler creates a new mock tool handler
func NewMockToolHandler(config MockToolConfig, templateEngine *template.Engine, debug bool) *MockToolHandler {
	return &MockToolHandler{
		config:         config,
		templateEngine: templateEngine,
		debug:          debug,
	}
}

// HandleCall processes a tool call and returns the configured response
func (h *MockToolHandler) HandleCall(arguments map[string]interface{}) (interface{}, error) {
	if h.debug {
		fmt.Fprintf(os.Stderr, "🔧 Mock tool '%s' called with arguments: %v\n", h.config.Name, arguments)
	}

	// Find the first matching response
	var selectedResponse *MockToolResponse
	for _, response := range h.config.Responses {
		if h.matchesCondition(response.Condition, arguments) {
			selectedResponse = &response
			break
		}
	}

	// If no specific response matched, use the first one as fallback
	if selectedResponse == nil && len(h.config.Responses) > 0 {
		selectedResponse = &h.config.Responses[0]
	}

	if selectedResponse == nil {
		return nil, fmt.Errorf("no response configured for tool %s", h.config.Name)
	}

	// Handle delay if specified
	if selectedResponse.Delay != "" {
		if duration, err := time.ParseDuration(selectedResponse.Delay); err == nil {
			if h.debug {
				fmt.Fprintf(os.Stderr, "⏳ Simulating delay of %s for tool '%s'\n", selectedResponse.Delay, h.config.Name)
			}
			time.Sleep(duration)
		}
	}

	// Handle error response
	if selectedResponse.Error != "" {
		errorMessage, err := h.templateEngine.Replace(selectedResponse.Error, arguments)
		if err != nil {
			if h.debug {
				fmt.Fprintf(os.Stderr, "❌ Failed to render error message for tool '%s': %v\n", h.config.Name, err)
			}
			return nil, fmt.Errorf("failed to render error message: %w", err)
		}
		errorStr, ok := errorMessage.(string)
		if !ok {
			errorStr = fmt.Sprintf("%v", errorMessage)
		}
		if h.debug {
			fmt.Fprintf(os.Stderr, "❌ Mock tool '%s' returning error: %s\n", h.config.Name, errorStr)
		}
		return nil, fmt.Errorf(errorStr)
	}

	// Render the response using the template engine
	renderedResponse, err := h.templateEngine.Replace(selectedResponse.Response, arguments)
	if err != nil {
		if h.debug {
			fmt.Fprintf(os.Stderr, "❌ Failed to render response for tool '%s': %v\n", h.config.Name, err)
		}
		return nil, fmt.Errorf("failed to render response: %w", err)
	}

	if h.debug {
		fmt.Fprintf(os.Stderr, "✅ Mock tool '%s' returning response: %v\n", h.config.Name, renderedResponse)
	}

	return renderedResponse, nil
}

// matchesCondition checks if the given arguments match the response condition
func (h *MockToolHandler) matchesCondition(condition map[string]interface{}, arguments map[string]interface{}) bool {
	if len(condition) == 0 {
		return true // No condition means it matches everything
	}

	for key, expectedValue := range condition {
		actualValue, exists := arguments[key]
		if !exists || actualValue != expectedValue {
			return false
		}
	}

	return true
}

// valuesEqual compares two values for equality, handling type conversions
func (h *MockToolHandler) valuesEqual(expected, actual interface{}) bool {
	// Direct equality check first
	if reflect.DeepEqual(expected, actual) {
		return true
	}

	// Handle string comparisons
	expectedStr := fmt.Sprintf("%v", expected)
	actualStr := fmt.Sprintf("%v", actual)
	if expectedStr == actualStr {
		return true
	}

	return false
}
