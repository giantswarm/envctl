package capability

import (
	"fmt"
	"strings"
	"time"
)

// ValidationError represents a validation error with context
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error in field '%s': %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}

	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}

	return fmt.Sprintf("validation failed: %s", strings.Join(messages, "; "))
}

// ValidateServiceCapabilityDefinition validates a service capability definition
func ValidateServiceCapabilityDefinition(def *ServiceCapabilityDefinition) error {
	var errors ValidationErrors

	// Validate base fields
	if def.Name == "" {
		errors = append(errors, ValidationError{Field: "name", Message: "name is required"})
	}

	if def.Type == "" {
		errors = append(errors, ValidationError{Field: "type", Message: "type is required"})
	}

	if def.Version == "" {
		errors = append(errors, ValidationError{Field: "version", Message: "version is required"})
	}

	if def.Description == "" {
		errors = append(errors, ValidationError{Field: "description", Message: "description is required"})
	}

	// Validate service config
	if err := validateServiceConfig(&def.ServiceConfig); err != nil {
		if validationErrs, ok := err.(ValidationErrors); ok {
			for _, validationErr := range validationErrs {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("serviceConfig.%s", validationErr.Field),
					Message: validationErr.Message,
				})
			}
		} else {
			errors = append(errors, ValidationError{Field: "serviceConfig", Message: err.Error()})
		}
	}

	// Validate operations (optional but if present must be valid)
	for opName, op := range def.Operations {
		if err := validateOperationDefinition(&op); err != nil {
			if validationErrs, ok := err.(ValidationErrors); ok {
				for _, validationErr := range validationErrs {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("operations.%s.%s", opName, validationErr.Field),
						Message: validationErr.Message,
					})
				}
			} else {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("operations.%s", opName),
					Message: err.Error(),
				})
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateServiceConfig validates the service configuration section
func validateServiceConfig(config *ServiceConfig) error {
	var errors ValidationErrors

	if config.ServiceType == "" {
		errors = append(errors, ValidationError{Field: "serviceType", Message: "serviceType is required"})
	}

	if config.DefaultLabel == "" {
		errors = append(errors, ValidationError{Field: "defaultLabel", Message: "defaultLabel is required"})
	}

	// Validate lifecycle tools
	if err := validateLifecycleTools(&config.LifecycleTools); err != nil {
		if validationErrs, ok := err.(ValidationErrors); ok {
			for _, validationErr := range validationErrs {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("lifecycleTools.%s", validationErr.Field),
					Message: validationErr.Message,
				})
			}
		} else {
			errors = append(errors, ValidationError{Field: "lifecycleTools", Message: err.Error()})
		}
	}

	// Validate health check config
	if err := validateHealthCheckConfig(&config.HealthCheck); err != nil {
		if validationErrs, ok := err.(ValidationErrors); ok {
			for _, validationErr := range validationErrs {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("healthCheck.%s", validationErr.Field),
					Message: validationErr.Message,
				})
			}
		} else {
			errors = append(errors, ValidationError{Field: "healthCheck", Message: err.Error()})
		}
	}

	// Validate timeout config
	if err := validateTimeoutConfig(&config.Timeout); err != nil {
		if validationErrs, ok := err.(ValidationErrors); ok {
			for _, validationErr := range validationErrs {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("timeout.%s", validationErr.Field),
					Message: validationErr.Message,
				})
			}
		} else {
			errors = append(errors, ValidationError{Field: "timeout", Message: err.Error()})
		}
	}

	// Validate create parameters
	for paramName, param := range config.CreateParameters {
		if err := validateParameterMapping(&param); err != nil {
			if validationErrs, ok := err.(ValidationErrors); ok {
				for _, validationErr := range validationErrs {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("createParameters.%s.%s", paramName, validationErr.Field),
						Message: validationErr.Message,
					})
				}
			} else {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("createParameters.%s", paramName),
					Message: err.Error(),
				})
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateLifecycleTools validates the lifecycle tools configuration
func validateLifecycleTools(tools *LifecycleTools) error {
	var errors ValidationErrors

	// Create tool is required
	if err := validateToolCall(&tools.Create, "create"); err != nil {
		if validationErrs, ok := err.(ValidationErrors); ok {
			for _, validationErr := range validationErrs {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("create.%s", validationErr.Field),
					Message: validationErr.Message,
				})
			}
		} else {
			errors = append(errors, ValidationError{Field: "create", Message: err.Error()})
		}
	}

	// Delete tool is required
	if err := validateToolCall(&tools.Delete, "delete"); err != nil {
		if validationErrs, ok := err.(ValidationErrors); ok {
			for _, validationErr := range validationErrs {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("delete.%s", validationErr.Field),
					Message: validationErr.Message,
				})
			}
		} else {
			errors = append(errors, ValidationError{Field: "delete", Message: err.Error()})
		}
	}

	// Health check tool is optional but if present must be valid
	if tools.HealthCheck != nil {
		if err := validateToolCall(tools.HealthCheck, "healthCheck"); err != nil {
			if validationErrs, ok := err.(ValidationErrors); ok {
				for _, validationErr := range validationErrs {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("healthCheck.%s", validationErr.Field),
						Message: validationErr.Message,
					})
				}
			} else {
				errors = append(errors, ValidationError{Field: "healthCheck", Message: err.Error()})
			}
		}
	}

	// Status tool is optional but if present must be valid
	if tools.Status != nil {
		if err := validateToolCall(tools.Status, "status"); err != nil {
			if validationErrs, ok := err.(ValidationErrors); ok {
				for _, validationErr := range validationErrs {
					errors = append(errors, ValidationError{
						Field:   fmt.Sprintf("status.%s", validationErr.Field),
						Message: validationErr.Message,
					})
				}
			} else {
				errors = append(errors, ValidationError{Field: "status", Message: err.Error()})
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateToolCall validates a tool call configuration
func validateToolCall(toolCall *ToolCall, context string) error {
	var errors ValidationErrors

	if toolCall.Tool == "" {
		errors = append(errors, ValidationError{Field: "tool", Message: "tool name is required"})
	} else {
		// Validate tool name format (should start with x_ for aggregator tools)
		if !strings.HasPrefix(toolCall.Tool, "x_") {
			errors = append(errors, ValidationError{
				Field:   "tool",
				Message: "tool name should start with 'x_' for aggregator tools",
			})
		}
	}

	// Arguments can be empty but if present should be valid
	if toolCall.Arguments == nil {
		toolCall.Arguments = make(map[string]interface{})
	}

	// Validate response mapping
	if err := validateResponseMapping(&toolCall.ResponseMapping); err != nil {
		if validationErrs, ok := err.(ValidationErrors); ok {
			for _, validationErr := range validationErrs {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("responseMapping.%s", validationErr.Field),
					Message: validationErr.Message,
				})
			}
		} else {
			errors = append(errors, ValidationError{Field: "responseMapping", Message: err.Error()})
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateResponseMapping validates response mapping configuration
func validateResponseMapping(mapping *ResponseMapping) error {
	var errors ValidationErrors

	// All fields are optional, but if JSON paths are provided, they should be valid format
	// For now, we'll just check they're not empty strings (empty means not used)

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateHealthCheckConfig validates health check configuration
func validateHealthCheckConfig(config *HealthCheckConfig) error {
	var errors ValidationErrors

	if config.Enabled {
		if config.Interval <= 0 {
			errors = append(errors, ValidationError{
				Field:   "interval",
				Message: "interval must be positive when health check is enabled",
			})
		}

		if config.FailureThreshold <= 0 {
			errors = append(errors, ValidationError{
				Field:   "failureThreshold",
				Message: "failureThreshold must be positive",
			})
		}

		if config.SuccessThreshold <= 0 {
			errors = append(errors, ValidationError{
				Field:   "successThreshold",
				Message: "successThreshold must be positive",
			})
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateTimeoutConfig validates timeout configuration
func validateTimeoutConfig(config *TimeoutConfig) error {
	var errors ValidationErrors

	if config.Create <= 0 {
		errors = append(errors, ValidationError{
			Field:   "create",
			Message: "create timeout must be positive",
		})
	}

	if config.Delete <= 0 {
		errors = append(errors, ValidationError{
			Field:   "delete",
			Message: "delete timeout must be positive",
		})
	}

	if config.HealthCheck <= 0 {
		errors = append(errors, ValidationError{
			Field:   "healthCheck",
			Message: "healthCheck timeout must be positive",
		})
	}

	// Reasonable timeout limits (adjust as needed)
	maxTimeout := 10 * time.Minute

	if config.Create > maxTimeout {
		errors = append(errors, ValidationError{
			Field:   "create",
			Message: fmt.Sprintf("create timeout cannot exceed %v", maxTimeout),
		})
	}

	if config.Delete > maxTimeout {
		errors = append(errors, ValidationError{
			Field:   "delete",
			Message: fmt.Sprintf("delete timeout cannot exceed %v", maxTimeout),
		})
	}

	if config.HealthCheck > maxTimeout {
		errors = append(errors, ValidationError{
			Field:   "healthCheck",
			Message: fmt.Sprintf("healthCheck timeout cannot exceed %v", maxTimeout),
		})
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateParameterMapping validates parameter mapping configuration
func validateParameterMapping(mapping *ParameterMapping) error {
	var errors ValidationErrors

	if mapping.ToolParameter == "" {
		errors = append(errors, ValidationError{
			Field:   "toolParameter",
			Message: "toolParameter is required",
		})
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateOperationDefinition validates an operation definition (from existing capability system)
func validateOperationDefinition(op *OperationDefinition) error {
	var errors ValidationErrors

	if op.Description == "" {
		errors = append(errors, ValidationError{Field: "description", Message: "description is required"})
	}

	// Workflow validation is complex and depends on the existing workflow system
	// For now, we'll just check that if workflow is specified, it's not nil
	if op.Workflow == nil {
		errors = append(errors, ValidationError{Field: "workflow", Message: "workflow is required"})
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}
