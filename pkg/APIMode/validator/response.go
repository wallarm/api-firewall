package validator

import "math/rand"

type FieldTypeError struct {
	Name         string `json:"name"`
	ExpectedType string `json:"expected_type,omitempty"`
	Pattern      string `json:"pattern,omitempty"`
	CurrentValue string `json:"current_value,omitempty"`
}

type ValidationError struct {
	Message       string           `json:"message"`
	Code          string           `json:"code"`
	SchemaVersion string           `json:"schema_version,omitempty"`
	SchemaID      *int             `json:"schema_id"`
	Fields        []string         `json:"related_fields,omitempty"`
	FieldsDetails []FieldTypeError `json:"related_fields_details,omitempty"`
}

type ValidationResponseSummary struct {
	SchemaID   *int `json:"schema_id"`
	StatusCode *int `json:"status_code"`
}

type ValidationResponse struct {
	Summary []*ValidationResponseSummary `json:"summary"`
	Errors  []*ValidationError           `json:"errors,omitempty"`
}

// SampleSlice function samples data in slice and responds by the subset of slice data
func SampleSlice[T any](rawData []T, limit int) []T {
	if len(rawData) <= limit || limit == 0 {
		return rawData
	}

	indices := rand.Perm(len(rawData))[:limit]

	sampled := make([]T, limit)
	for i, idx := range indices {
		sampled[i] = rawData[idx]
	}
	return sampled
}
