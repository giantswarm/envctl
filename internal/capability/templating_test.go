package capability

import (
	"reflect"
	"testing"
)

func TestParameterTemplater_ReplaceStringTemplates(t *testing.T) {
	pt := NewParameterTemplater()

	tests := []struct {
		name     string
		template string
		context  map[string]interface{}
		want     string
		wantErr  bool
	}{
		{
			name:     "simple template replacement",
			template: "Hello {{ name }}!",
			context:  map[string]interface{}{"name": "World"},
			want:     "Hello World!",
			wantErr:  false,
		},
		{
			name:     "multiple templates",
			template: "{{ greeting }} {{ name }}!",
			context:  map[string]interface{}{"greeting": "Hello", "name": "Alice"},
			want:     "Hello Alice!",
			wantErr:  false,
		},
		{
			name:     "template with spaces",
			template: "Value: {{   spacedVar   }}",
			context:  map[string]interface{}{"spacedVar": "test"},
			want:     "Value: test",
			wantErr:  false,
		},
		{
			name:     "no templates",
			template: "Just plain text",
			context:  map[string]interface{}{},
			want:     "Just plain text",
			wantErr:  false,
		},
		{
			name:     "missing variable",
			template: "{{ missing }}",
			context:  map[string]interface{}{},
			want:     "",
			wantErr:  true,
		},
		{
			name:     "numeric value",
			template: "Port: {{ port }}",
			context:  map[string]interface{}{"port": 8080},
			want:     "Port: 8080",
			wantErr:  false,
		},
		{
			name:     "boolean value",
			template: "Enabled: {{ enabled }}",
			context:  map[string]interface{}{"enabled": true},
			want:     "Enabled: true",
			wantErr:  false,
		},
		{
			name:     "repeated template",
			template: "{{ value }} and {{ value }} again",
			context:  map[string]interface{}{"value": "test"},
			want:     "test and test again",
			wantErr:  false,
		},
		{
			name:     "complex path template",
			template: "teleport.giantswarm.io-{{ cluster }}",
			context:  map[string]interface{}{"cluster": "mymc-cluster1"},
			want:     "teleport.giantswarm.io-mymc-cluster1",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pt.ReplaceTemplates(tt.template, tt.context)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReplaceTemplates() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ReplaceTemplates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParameterTemplater_ReplaceMapTemplates(t *testing.T) {
	pt := NewParameterTemplater()

	tests := []struct {
		name    string
		input   map[string]interface{}
		context map[string]interface{}
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "simple map replacement",
			input: map[string]interface{}{
				"cluster": "{{ clusterName }}",
				"port":    8080,
			},
			context: map[string]interface{}{"clusterName": "test-cluster"},
			want: map[string]interface{}{
				"cluster": "test-cluster",
				"port":    8080,
			},
			wantErr: false,
		},
		{
			name: "nested map replacement",
			input: map[string]interface{}{
				"config": map[string]interface{}{
					"host": "{{ hostname }}",
					"port": "{{ port }}",
				},
			},
			context: map[string]interface{}{"hostname": "localhost", "port": 9090},
			want: map[string]interface{}{
				"config": map[string]interface{}{
					"host": "localhost",
					"port": "9090",
				},
			},
			wantErr: false,
		},
		{
			name: "map with slice",
			input: map[string]interface{}{
				"commands": []interface{}{
					"login",
					"{{ cluster }}",
				},
			},
			context: map[string]interface{}{"cluster": "prod-cluster"},
			want: map[string]interface{}{
				"commands": []interface{}{
					"login",
					"prod-cluster",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pt.ReplaceTemplates(tt.input, tt.context)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReplaceTemplates() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReplaceTemplates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParameterTemplater_ExtractTemplateVariables(t *testing.T) {
	pt := NewParameterTemplater()

	tests := []struct {
		name  string
		input interface{}
		want  []string
	}{
		{
			name:  "simple string",
			input: "Hello {{ name }}!",
			want:  []string{"name"},
		},
		{
			name:  "multiple variables",
			input: "{{ greeting }} {{ name }}, port {{ port }}",
			want:  []string{"greeting", "name", "port"},
		},
		{
			name:  "repeated variable",
			input: "{{ value }} and {{ value }} again",
			want:  []string{"value"},
		},
		{
			name: "map with templates",
			input: map[string]interface{}{
				"cluster": "{{ clusterName }}",
				"context": "teleport-{{ env }}",
			},
			want: []string{"clusterName", "env"},
		},
		{
			name: "nested structure",
			input: map[string]interface{}{
				"tools": []interface{}{
					map[string]interface{}{
						"tool": "x_teleport_kube",
						"params": map[string]interface{}{
							"command": "login",
							"cluster": "{{ cluster }}",
						},
					},
				},
			},
			want: []string{"cluster"},
		},
		{
			name:  "no templates",
			input: "Just plain text",
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pt.ExtractTemplateVariables(tt.input)

			// Convert to map for order-independent comparison
			gotMap := make(map[string]bool)
			for _, v := range got {
				gotMap[v] = true
			}
			wantMap := make(map[string]bool)
			for _, v := range tt.want {
				wantMap[v] = true
			}

			if !reflect.DeepEqual(gotMap, wantMap) {
				t.Errorf("ExtractTemplateVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParameterTemplater_ValidateTemplateContext(t *testing.T) {
	pt := NewParameterTemplater()

	tests := []struct {
		name    string
		value   interface{}
		context map[string]interface{}
		wantErr bool
	}{
		{
			name:    "all variables present",
			value:   "{{ name }} on {{ cluster }}",
			context: map[string]interface{}{"name": "test", "cluster": "prod"},
			wantErr: false,
		},
		{
			name:    "missing variable",
			value:   "{{ name }} on {{ cluster }}",
			context: map[string]interface{}{"name": "test"},
			wantErr: true,
		},
		{
			name:    "no templates",
			value:   "plain text",
			context: map[string]interface{}{},
			wantErr: false,
		},
		{
			name: "nested structure all present",
			value: map[string]interface{}{
				"config": map[string]interface{}{
					"host": "{{ hostname }}",
					"port": "{{ port }}",
				},
			},
			context: map[string]interface{}{"hostname": "localhost", "port": 8080},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pt.ValidateTemplateContext(tt.value, tt.context)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTemplateContext() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMergeContexts(t *testing.T) {
	tests := []struct {
		name     string
		contexts []map[string]interface{}
		want     map[string]interface{}
	}{
		{
			name: "merge two contexts",
			contexts: []map[string]interface{}{
				{"a": 1, "b": 2},
				{"b": 3, "c": 4},
			},
			want: map[string]interface{}{"a": 1, "b": 3, "c": 4},
		},
		{
			name: "merge three contexts",
			contexts: []map[string]interface{}{
				{"cluster": "default"},
				{"cluster": "prod", "env": "production"},
				{"port": 8080},
			},
			want: map[string]interface{}{"cluster": "prod", "env": "production", "port": 8080},
		},
		{
			name:     "single context",
			contexts: []map[string]interface{}{{"key": "value"}},
			want:     map[string]interface{}{"key": "value"},
		},
		{
			name:     "empty contexts",
			contexts: []map[string]interface{}{},
			want:     map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeContexts(tt.contexts...)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergeContexts() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParameterTemplater_ComplexScenarios(t *testing.T) {
	pt := NewParameterTemplater()

	// Test a complex operation specification scenario
	operationSpec := map[string]interface{}{
		"tools": []interface{}{
			map[string]interface{}{
				"tool": "x_teleport_kube",
				"params": map[string]interface{}{
					"command": "login",
					"cluster": "{{ cluster }}",
					"ttl":     "{{ ttl }}",
				},
			},
			map[string]interface{}{
				"tool": "x_auth_validate",
				"params": map[string]interface{}{
					"context": "teleport.giantswarm.io-{{ cluster }}",
				},
			},
		},
	}

	context := map[string]interface{}{
		"cluster": "mymc-cluster1",
		"ttl":     "8h",
	}

	result, err := pt.ReplaceTemplates(operationSpec, context)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify the result
	resultMap := result.(map[string]interface{})
	tools := resultMap["tools"].([]interface{})

	// Check first tool
	tool1 := tools[0].(map[string]interface{})
	params1 := tool1["params"].(map[string]interface{})
	if params1["cluster"] != "mymc-cluster1" {
		t.Errorf("Expected cluster to be 'mymc-cluster1', got %v", params1["cluster"])
	}
	if params1["ttl"] != "8h" {
		t.Errorf("Expected ttl to be '8h', got %v", params1["ttl"])
	}

	// Check second tool
	tool2 := tools[1].(map[string]interface{})
	params2 := tool2["params"].(map[string]interface{})
	if params2["context"] != "teleport.giantswarm.io-mymc-cluster1" {
		t.Errorf("Expected context to be 'teleport.giantswarm.io-mymc-cluster1', got %v", params2["context"])
	}
}
