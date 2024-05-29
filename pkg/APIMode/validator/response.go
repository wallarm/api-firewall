package validator

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
