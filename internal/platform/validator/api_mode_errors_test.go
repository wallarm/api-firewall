package validator

import (
	"errors"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"

	"github.com/wallarm/api-firewall/pkg/APIMode/validator"
)

func TestGetErrorResponse_MissingRequiredQueryParam(t *testing.T) {
	param := &openapi3.Parameter{
		In:   "query",
		Name: "id",
		Schema: &openapi3.SchemaRef{
			Value: &openapi3.Schema{
				Type: &openapi3.Types{"string"},
			},
		},
	}

	reqErr := &openapi3filter.RequestError{
		Parameter: param,
		Err:       ErrInvalidRequired,
	}

	result, err := GetErrorResponse(reqErr)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 validation error, got %d", len(result))
	}

	if result[0].Code != validator.ErrCodeRequiredQueryParameterMissed {
		t.Errorf("Expected error code %s, got %s", validator.ErrCodeRequiredQueryParameterMissed, result[0].Code)
	}
}

func TestGetErrorResponse_InvalidSyntax(t *testing.T) {
	param := &openapi3.Parameter{
		In:   "path",
		Name: "age",
		Schema: &openapi3.SchemaRef{
			Value: &openapi3.Schema{
				Type: &openapi3.Types{"integer"},
			},
		},
	}

	reqErr := &openapi3filter.RequestError{
		Parameter: param,
		Err:       errors.New("invalid syntax"),
	}

	result, err := GetErrorResponse(reqErr)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result) == 0 {
		t.Fatal("Expected at least one validation error")
	}

	if result[0].Code != validator.ErrCodeRequiredPathParameterInvalidValue {
		t.Errorf("Expected error code %s, got %s", validator.ErrCodeRequiredPathParameterInvalidValue, result[0].Code)
	}
}

func TestGetErrorResponse_RequiredBodyMissed(t *testing.T) {
	reqErr := &openapi3filter.RequestError{
		RequestBody: &openapi3.RequestBody{
			Required: true,
		},
		Err: openapi3filter.ErrInvalidRequired,
	}

	result, err := GetErrorResponse(reqErr)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result) == 0 {
		t.Fatal("Expected validation error for required body field")
	}

	if result[0].Code != validator.ErrCodeRequiredBodyMissed {
		t.Errorf("Expected error code %s, got %s", validator.ErrCodeRequiredBodyMissed, result[0].Code)
	}
}

func TestGetErrorResponse_SecurityError(t *testing.T) {
	secErr := &openapi3filter.SecurityRequirementsError{
		Errors: []error{
			&SecurityRequirementsParameterIsMissingError{
				Field:   "api_key",
				Message: "API key is missing",
			},
		},
	}

	result, err := GetErrorResponse(secErr)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 security validation error, got %d", len(result))
	}

	if result[0].Code != validator.ErrCodeSecRequirementsFailed {
		t.Errorf("Expected security error code %s, got %s", validator.ErrCodeSecRequirementsFailed, result[0].Code)
	}
}

func TestGetErrorResponse_UnknownError(t *testing.T) {
	err := errors.New("some unknown validation error")

	result, retErr := GetErrorResponse(err)
	if retErr != nil {
		t.Fatalf("Unexpected error: %v", retErr)
	}

	if len(result) != 1 {
		t.Fatalf("Expected fallback error response")
	}

	if result[0].Code != validator.ErrCodeUnknownValidationError {
		t.Errorf("Expected fallback error code %s, got %s", validator.ErrCodeUnknownValidationError, result[0].Code)
	}
}
