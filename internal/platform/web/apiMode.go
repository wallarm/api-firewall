package web

const (
	APIModePostfixStatusCode       = "_status_code"
	APIModePostfixValidationErrors = "_validation_errors"

	GlobalResponseStatusCodeKey = "global_response_status_code"

	RequestSchemaID = "__wallarm_apifw_request_schema_id"
)

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

type APIModeResponseSummary struct {
	SchemaID   *int `json:"schema_id"`
	StatusCode *int `json:"status_code"`
}

type APIModeResponse struct {
	Summary []*APIModeResponseSummary `json:"summary"`
	Errors  []*ValidationError        `json:"errors,omitempty"`
}
