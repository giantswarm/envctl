package capability

import (
	"fmt"
	"regexp"
	"strings"
)

// ParameterTemplater handles parameter templating for capability operations
type ParameterTemplater struct {
	// Pattern to match template variables like {{ variableName }}
	templatePattern *regexp.Regexp
}

// NewParameterTemplater creates a new parameter templater
func NewParameterTemplater() *ParameterTemplater {
	return &ParameterTemplater{
		templatePattern: regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`),
	}
}

// ReplaceTemplates replaces all template variables in a value with actual values from the context
func (pt *ParameterTemplater) ReplaceTemplates(value interface{}, context map[string]interface{}) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return pt.replaceStringTemplates(v, context)
	case map[string]interface{}:
		return pt.replaceMapTemplates(v, context)
	case []interface{}:
		return pt.replaceSliceTemplates(v, context)
	default:
		// Non-templatable types (numbers, booleans, etc.) are returned as-is
		return value, nil
	}
}

// replaceStringTemplates replaces template variables in a string
func (pt *ParameterTemplater) replaceStringTemplates(template string, context map[string]interface{}) (string, error) {
	// Find all template variables
	matches := pt.templatePattern.FindAllStringSubmatch(template, -1)
	if len(matches) == 0 {
		// No templates found, return original string
		return template, nil
	}

	// Keep track of replacements to avoid duplicate processing
	result := template
	replacements := make(map[string]string)

	for _, match := range matches {
		if len(match) != 2 {
			continue
		}

		fullMatch := match[0]    // e.g., "{{ cluster }}"
		variableName := match[1] // e.g., "cluster"

		// Skip if already processed
		if _, processed := replacements[fullMatch]; processed {
			continue
		}

		// Look up the value in context
		value, exists := context[variableName]
		if !exists {
			return "", fmt.Errorf("template variable '%s' not found in context", variableName)
		}

		// Convert value to string
		stringValue, err := pt.convertToString(value)
		if err != nil {
			return "", fmt.Errorf("cannot convert variable '%s' to string: %w", variableName, err)
		}

		// Replace all occurrences of this template
		result = strings.ReplaceAll(result, fullMatch, stringValue)
		replacements[fullMatch] = stringValue
	}

	return result, nil
}

// replaceMapTemplates recursively replaces templates in a map
func (pt *ParameterTemplater) replaceMapTemplates(m map[string]interface{}, context map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range m {
		// Replace templates in the value
		replacedValue, err := pt.ReplaceTemplates(value, context)
		if err != nil {
			return nil, fmt.Errorf("error in map key '%s': %w", key, err)
		}
		result[key] = replacedValue
	}

	return result, nil
}

// replaceSliceTemplates recursively replaces templates in a slice
func (pt *ParameterTemplater) replaceSliceTemplates(s []interface{}, context map[string]interface{}) ([]interface{}, error) {
	result := make([]interface{}, len(s))

	for i, value := range s {
		replacedValue, err := pt.ReplaceTemplates(value, context)
		if err != nil {
			return nil, fmt.Errorf("error in array index %d: %w", i, err)
		}
		result[i] = replacedValue
	}

	return result, nil
}

// convertToString converts a value to string for template replacement
func (pt *ParameterTemplater) convertToString(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case int, int32, int64:
		return fmt.Sprintf("%d", v), nil
	case float32, float64:
		return fmt.Sprintf("%v", v), nil
	case bool:
		return fmt.Sprintf("%v", v), nil
	case nil:
		return "", nil
	default:
		return "", fmt.Errorf("unsupported type for template conversion: %T", value)
	}
}

// ExtractTemplateVariables extracts all unique template variable names from a value
func (pt *ParameterTemplater) ExtractTemplateVariables(value interface{}) []string {
	variableSet := make(map[string]struct{})
	pt.extractVariablesRecursive(value, variableSet)

	// Convert set to slice
	variables := make([]string, 0, len(variableSet))
	for variable := range variableSet {
		variables = append(variables, variable)
	}

	return variables
}

// extractVariablesRecursive recursively extracts template variables from a value
func (pt *ParameterTemplater) extractVariablesRecursive(value interface{}, variableSet map[string]struct{}) {
	switch v := value.(type) {
	case string:
		matches := pt.templatePattern.FindAllStringSubmatch(v, -1)
		for _, match := range matches {
			if len(match) == 2 {
				variableSet[match[1]] = struct{}{}
			}
		}
	case map[string]interface{}:
		for _, mapValue := range v {
			pt.extractVariablesRecursive(mapValue, variableSet)
		}
	case []interface{}:
		for _, sliceValue := range v {
			pt.extractVariablesRecursive(sliceValue, variableSet)
		}
	}
}

// ValidateTemplateContext validates that all required template variables are present in the context
func (pt *ParameterTemplater) ValidateTemplateContext(value interface{}, context map[string]interface{}) error {
	requiredVariables := pt.ExtractTemplateVariables(value)

	for _, variable := range requiredVariables {
		if _, exists := context[variable]; !exists {
			return fmt.Errorf("required template variable '%s' not found in context", variable)
		}
	}

	return nil
}

// MergeContexts merges multiple contexts, with later contexts overriding earlier ones
func MergeContexts(contexts ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for _, context := range contexts {
		for key, value := range context {
			result[key] = value
		}
	}

	return result
}
