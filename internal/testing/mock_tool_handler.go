package testing

import (
	"fmt"
	"reflect"
	"time"

	"envctl/internal/template"
)

// MockToolHandler handles calls to a specific mock tool
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

// HandleCall processes a tool call and returns the appropriate response
func (h *MockToolHandler) HandleCall(parameters map[string]interface{}) (interface{}, error) {
	if h.debug {
		fmt.Printf("🔧 MockToolHandler.HandleCall for tool '%s' with parameters: %v\n", h.config.Name, parameters)
	}

	// Find matching response based on conditions
	var selectedResponse *MockToolResponse
	for i := range h.config.Responses {
		response := &h.config.Responses[i]
		if h.matchesCondition(response.Condition, parameters) {
			selectedResponse = response
			break
		}
	}

	// If no conditional response matched, use fallback (response with no condition)
	if selectedResponse == nil {
		for i := range h.config.Responses {
			response := &h.config.Responses[i]
			if len(response.Condition) == 0 {
				selectedResponse = response
				break
			}
		}
	}

	if selectedResponse == nil {
		return nil, fmt.Errorf("no matching response found for tool '%s' with parameters: %v", h.config.Name, parameters)
	}

	// Apply delay if configured
	if selectedResponse.Delay != "" {
		delay, err := time.ParseDuration(selectedResponse.Delay)
		if err != nil {
			if h.debug {
				fmt.Printf("⚠️  Invalid delay format '%s', ignoring\n", selectedResponse.Delay)
			}
		} else {
			if h.debug {
				fmt.Printf("⏳ Applying delay of %v for tool '%s'\n", delay, h.config.Name)
			}
			time.Sleep(delay)
		}
	}

	// Return error if configured
	if selectedResponse.Error != "" {
		return nil, fmt.Errorf("%s", selectedResponse.Error)
	}

	// Process response with template engine
	context := map[string]interface{}{
		"parameters": parameters,
	}

	// Add individual parameters to context for easier access
	for key, value := range parameters {
		context[key] = value
	}

	processedResponse, err := h.templateEngine.Replace(selectedResponse.Response, context)
	if err != nil {
		return nil, fmt.Errorf("template processing failed for tool '%s': %w", h.config.Name, err)
	}

	if h.debug {
		fmt.Printf("✅ Tool '%s' returning response: %v\n", h.config.Name, processedResponse)
	}

	return processedResponse, nil
}

// matchesCondition checks if the given parameters match the condition
func (h *MockToolHandler) matchesCondition(condition map[string]interface{}, parameters map[string]interface{}) bool {
	if len(condition) == 0 {
		return false // Empty condition doesn't match (it's a fallback)
	}

	for key, expectedValue := range condition {
		actualValue, exists := parameters[key]
		if !exists {
			if h.debug {
				fmt.Printf("🔍 Condition check: parameter '%s' not found in call\n", key)
			}
			return false
		}

		if !h.valuesEqual(expectedValue, actualValue) {
			if h.debug {
				fmt.Printf("🔍 Condition check: parameter '%s' expected '%v', got '%v'\n", key, expectedValue, actualValue)
			}
			return false
		}
	}

	if h.debug {
		fmt.Printf("✅ Condition matched for tool '%s'\n", h.config.Name)
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
