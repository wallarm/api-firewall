package config

import "testing"

func TestEndpointListSet_ValidInputs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected EndpointList
	}{
		{
			name:  "With method",
			input: "GET:/api/v1/resource|BLOCK|LOG_ONLY",
			expected: EndpointList{
				{
					Method: "GET",
					Path:   "/api/v1/resource",
					ValidationMode: ValidationMode{
						RequestValidation:  "BLOCK",
						ResponseValidation: "LOG_ONLY",
					},
				},
			},
		},
		{
			name:  "Without method",
			input: "/api/v1/resource|DISABLE|BLOCK",
			expected: EndpointList{
				{
					Method: "",
					Path:   "/api/v1/resource",
					ValidationMode: ValidationMode{
						RequestValidation:  "DISABLE",
						ResponseValidation: "BLOCK",
					},
				},
			},
		},
		{
			name:  "Multiple entries",
			input: "POST:/create|BLOCK|BLOCK,GET:/list|LOG_ONLY|DISABLE",
			expected: EndpointList{
				{
					Method: "POST",
					Path:   "/create",
					ValidationMode: ValidationMode{
						RequestValidation:  "BLOCK",
						ResponseValidation: "BLOCK",
					},
				},
				{
					Method: "GET",
					Path:   "/list",
					ValidationMode: ValidationMode{
						RequestValidation:  "LOG_ONLY",
						ResponseValidation: "DISABLE",
					},
				},
			},
		},
		{
			name:     "Empty input string",
			input:    "",
			expected: EndpointList{
				// Expect no endpoints
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var epList EndpointList
			err := epList.Set(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(epList) != len(tt.expected) {
				t.Errorf("expected %d endpoints, got %d", len(tt.expected), len(epList))
			}
			for i := range tt.expected {
				if epList[i] != tt.expected[i] {
					t.Errorf("expected endpoint[%d] = %+v, got %+v", i, tt.expected[i], epList[i])
				}
			}
		})
	}
}

func TestEndpointListSet_InvalidInputs(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Missing validation modes",
			input: "GET:/api/v1/resource",
		},
		{
			name:  "Too many parts",
			input: "GET:/api|BLOCK|LOG_ONLY|EXTRA",
		},
		{
			name:  "Empty required segment #1",
			input: "|BLOCK|BLOCK",
		},
		{
			name:  "Empty required segment #2",
			input: "/api|BLOCK|",
		},
		{
			name:  "Empty required segment #3",
			input: "/api||BLOCK",
		},
		{
			name:  "Just delimiter",
			input: "|||",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var epList EndpointList
			err := epList.Set(tt.input)
			if err == nil {
				t.Errorf("expected error for input '%s', got nil", tt.input)
			}
		})
	}
}

func TestEndpointListString(t *testing.T) {
	epList := EndpointList{
		{
			Method: "POST",
			Path:   "/submit",
			ValidationMode: ValidationMode{
				RequestValidation:  "BLOCK",
				ResponseValidation: "DISABLE",
			},
		},
		{
			Method: "GET",
			Path:   "/fetch",
			ValidationMode: ValidationMode{
				RequestValidation:  "LOG_ONLY",
				ResponseValidation: "BLOCK",
			},
		},
	}

	expected := "POST:/submit|BLOCK|DISABLE,GET:/fetch|LOG_ONLY|BLOCK"
	result := epList.String()

	// Strings are expected to be exactly equal
	if result != expected {
		t.Errorf("expected string: %s, got: %s", expected, result)
	}
}
