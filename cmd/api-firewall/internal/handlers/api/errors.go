package api

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/pkg/errors"
	"github.com/wallarm/api-firewall/internal/platform/validator"
)

const (
	ErrCodeMethodAndPathNotFound               = "method_and_path_not_found"
	ErrCodeRequiredBodyMissed                  = "required_body_missed"
	ErrCodeRequiredBodyParseError              = "required_body_parse_error"
	ErrCodeRequiredBodyParameterMissed         = "required_body_parameter_missed"
	ErrCodeRequiredBodyParameterInvalidValue   = "required_body_parameter_invalid_value"
	ErrCodeRequiredPathParameterMissed         = "required_path_parameter_missed"
	ErrCodeRequiredPathParameterInvalidValue   = "required_path_parameter_invalid_value"
	ErrCodeRequiredQueryParameterMissed        = "required_query_parameter_missed"
	ErrCodeRequiredQueryParameterInvalidValue  = "required_query_parameter_invalid_value"
	ErrCodeRequiredCookieParameterMissed       = "required_cookie_parameter_missed"
	ErrCodeRequiredCookieParameterInvalidValue = "required_cookie_parameter_invalid_value"
	ErrCodeRequiredHeaderMissed                = "required_header_missed"
	ErrCodeRequiredHeaderInvalidValue          = "required_header_invalid_value"

	ErrCodeSecRequirementsFailed = "required_security_requirements_failed"

	ErrCodeUnknownParameterFound = "unknown_parameter_found"

	ErrCodeUnknownValidationError = "unknown_validation_error"
)

var (
	ErrAuthHeaderMissed = errors.New("missing Authorization header")
	ErrAPITokenMissed   = errors.New("missing API keys for authorization")

	ErrMethodAndPathNotFound    = errors.New("method and path are not found")
	ErrRequiredBodyIsMissing    = errors.New("required body is missing")
	ErrMissedRequiredParameters = errors.New("required parameters missed")
)

type FieldTypeError struct {
	Name         string `json:"name"`
	ExpectedType string `json:"expected_type,omitempty"`
	Pattern      string `json:"pattern,omitempty"`
	CurrentValue string `json:"current_value"`
}

type ValidationError struct {
	Message       string           `json:"message"`
	Code          string           `json:"code"`
	SchemaVersion string           `json:"schema_version,omitempty"`
	Fields        []string         `json:"related_fields,omitempty"`
	FieldsDetails []FieldTypeError `json:"related_fields_details,omitempty"`
}

type SecurityRequirementsParameterIsMissingError struct {
	Field   string
	Message string
}

func (e *SecurityRequirementsParameterIsMissingError) Error() string {
	return e.Message
}

func getErrorResponse(validationError error) ([]*ValidationError, error) {
	var responseErrors []*ValidationError

	switch err := validationError.(type) {

	case *openapi3filter.RequestError:
		if err.Parameter != nil {

			// Required parameter is missed
			if errors.Is(err, validator.ErrInvalidRequired) || errors.Is(err, validator.ErrInvalidEmptyValue) {
				response := ValidationError{}
				switch err.Parameter.In {
				case "path":
					response.Code = ErrCodeRequiredPathParameterMissed
				case "query":
					response.Code = ErrCodeRequiredQueryParameterMissed
				case "cookie":
					response.Code = ErrCodeRequiredCookieParameterMissed
				case "header":
					response.Code = ErrCodeRequiredHeaderMissed
				}
				response.Message = err.Error()
				response.Fields = []string{err.Parameter.Name}
				responseErrors = append(responseErrors, &response)
			}

			// Invalid parameter value
			if strings.HasSuffix(err.Error(), "invalid syntax") {
				response := ValidationError{}
				switch err.Parameter.In {
				case "path":
					response.Code = ErrCodeRequiredPathParameterInvalidValue
				case "query":
					response.Code = ErrCodeRequiredQueryParameterInvalidValue
				case "cookie":
					response.Code = ErrCodeRequiredCookieParameterInvalidValue
				case "header":
					response.Code = ErrCodeRequiredHeaderInvalidValue
				}
				response.Message = err.Error()
				response.Fields = []string{err.Parameter.Name}
				if parseErr, ok := err.Err.(*validator.ParseError); ok {
					response.FieldsDetails = append(response.FieldsDetails, FieldTypeError{
						Name:         err.Parameter.Name,
						ExpectedType: parseErr.ExpectedType,
						CurrentValue: parseErr.ValueStr,
					})
				}
				schemaError, ok := err.Err.(*openapi3.SchemaError)
				if ok {
					if schemaError.SchemaField == "pattern" {
						response.FieldsDetails = append(response.FieldsDetails, FieldTypeError{
							Name:         err.Parameter.Name,
							ExpectedType: schemaError.Schema.Type,
							Pattern:      schemaError.Schema.Pattern,
							CurrentValue: fmt.Sprintf("%v", schemaError.Value),
						})
					}
				}

				responseErrors = append(responseErrors, &response)
			}

			// Validation of the required parameter error
			switch multiErrors := err.Err.(type) {
			case openapi3.MultiError:
				for _, multiErr := range multiErrors {
					schemaError, ok := multiErr.(*openapi3.SchemaError)
					if ok {
						response := ValidationError{}
						switch schemaError.SchemaField {
						case "required":
							switch err.Parameter.In {
							case "query":
								response.Code = ErrCodeRequiredQueryParameterMissed
							case "cookie":
								response.Code = ErrCodeRequiredCookieParameterMissed
							case "header":
								response.Code = ErrCodeRequiredHeaderMissed
							}
							response.Fields = schemaError.JSONPointer()
							response.Message = ErrMissedRequiredParameters.Error()
							responseErrors = append(responseErrors, &response)
						default:
							switch err.Parameter.In {
							case "query":
								response.Code = ErrCodeRequiredQueryParameterInvalidValue
							case "cookie":
								response.Code = ErrCodeRequiredCookieParameterInvalidValue
							case "header":
								response.Code = ErrCodeRequiredHeaderInvalidValue
							}
							response.Fields = []string{err.Parameter.Name}
							response.Message = schemaError.Error()
							if parseErr, ok := err.Err.(*validator.ParseError); ok {
								response.FieldsDetails = append(response.FieldsDetails, FieldTypeError{
									Name:         err.Parameter.Name,
									ExpectedType: parseErr.ExpectedType,
									CurrentValue: parseErr.ValueStr,
								})
							}
							if schemaError.SchemaField == "pattern" {
								response.FieldsDetails = append(response.FieldsDetails, FieldTypeError{
									Name:         err.Parameter.Name,
									ExpectedType: schemaError.Schema.Type,
									Pattern:      schemaError.Schema.Pattern,
									CurrentValue: fmt.Sprintf("%v", schemaError.Value),
								})
							}
							responseErrors = append(responseErrors, &response)
						}
					}
				}
			default:
				schemaError, ok := multiErrors.(*openapi3.SchemaError)
				if ok {
					response := ValidationError{}
					switch schemaError.SchemaField {
					case "required":
						switch err.Parameter.In {
						case "query":
							response.Code = ErrCodeRequiredQueryParameterMissed
						case "cookie":
							response.Code = ErrCodeRequiredCookieParameterMissed
						case "header":
							response.Code = ErrCodeRequiredHeaderMissed
						}
						response.Fields = schemaError.JSONPointer()
						response.Message = ErrMissedRequiredParameters.Error()
						responseErrors = append(responseErrors, &response)
					default:
						switch err.Parameter.In {
						case "query":
							response.Code = ErrCodeRequiredQueryParameterInvalidValue
						case "cookie":
							response.Code = ErrCodeRequiredCookieParameterInvalidValue
						case "header":
							response.Code = ErrCodeRequiredHeaderInvalidValue
						}
						response.Fields = []string{err.Parameter.Name}
						response.Message = schemaError.Error()
						if parseErr, ok := err.Err.(*validator.ParseError); ok {
							response.FieldsDetails = append(response.FieldsDetails, FieldTypeError{
								Name:         err.Parameter.Name,
								ExpectedType: parseErr.ExpectedType,
								CurrentValue: parseErr.ValueStr,
							})
						}
						if schemaError.SchemaField == "pattern" {
							response.FieldsDetails = append(response.FieldsDetails, FieldTypeError{
								Name:         err.Parameter.Name,
								ExpectedType: schemaError.Schema.Type,
								Pattern:      schemaError.Schema.Pattern,
								CurrentValue: fmt.Sprintf("%v", schemaError.Value),
							})
						}
						responseErrors = append(responseErrors, &response)
					}
				}
			}

		}

		if len(responseErrors) > 0 {
			return responseErrors, nil
		}

		// Validation of the required body error
		switch multiErrors := err.Err.(type) {
		case openapi3.MultiError:
			for _, multiErr := range multiErrors {
				schemaError, ok := multiErr.(*openapi3.SchemaError)
				if ok {
					response := ValidationError{}
					switch schemaError.SchemaField {
					case "required":
						response.Code = ErrCodeRequiredBodyParameterMissed
						response.Fields = schemaError.JSONPointer()
						response.Message = schemaError.Error()
						responseErrors = append(responseErrors, &response)
					default:
						response.Code = ErrCodeRequiredBodyParameterInvalidValue
						response.Fields = schemaError.JSONPointer()
						response.Message = schemaError.Error()
						if parseErr, ok := err.Err.(*validator.ParseError); ok && len(response.Fields) > 0 {
							response.FieldsDetails = append(response.FieldsDetails, FieldTypeError{
								Name:         response.Fields[0],
								ExpectedType: parseErr.ExpectedType,
								CurrentValue: parseErr.ValueStr,
							})
						}
						if schemaError.SchemaField == "pattern" && len(response.Fields) > 0 {
							response.FieldsDetails = append(response.FieldsDetails, FieldTypeError{
								Name:         response.Fields[0],
								ExpectedType: schemaError.Schema.Type,
								Pattern:      schemaError.Schema.Pattern,
								CurrentValue: fmt.Sprintf("%v", schemaError.Value),
							})
						}
						responseErrors = append(responseErrors, &response)
					}
				}
			}
		default:
			schemaError, ok := multiErrors.(*openapi3.SchemaError)
			if ok {
				response := ValidationError{}
				switch schemaError.SchemaField {
				case "required":
					response.Code = ErrCodeRequiredBodyParameterMissed
					response.Fields = schemaError.JSONPointer()
					response.Message = schemaError.Error()
					responseErrors = append(responseErrors, &response)
				default:
					response.Code = ErrCodeRequiredBodyParameterInvalidValue
					response.Fields = schemaError.JSONPointer()
					response.Message = schemaError.Error()
					if parseErr, ok := err.Err.(*validator.ParseError); ok && len(response.Fields) > 0 {
						response.FieldsDetails = append(response.FieldsDetails, FieldTypeError{
							Name:         response.Fields[0],
							ExpectedType: parseErr.ExpectedType,
							CurrentValue: parseErr.ValueStr,
						})
					}
					if schemaError.SchemaField == "pattern" && len(response.Fields) > 0 {
						response.FieldsDetails = append(response.FieldsDetails, FieldTypeError{
							Name:         response.Fields[0],
							ExpectedType: schemaError.Schema.Type,
							Pattern:      schemaError.Schema.Pattern,
							CurrentValue: fmt.Sprintf("%v", schemaError.Value),
						})
					}
					responseErrors = append(responseErrors, &response)
				}
			}
		}

		// Handle request body errors
		if err.RequestBody != nil {

			// Body required but missed
			if err.RequestBody.Required {
				if err.Err != nil && err.Err.Error() == validator.ErrInvalidRequired.Error() {
					response := ValidationError{}
					response.Code = ErrCodeRequiredBodyMissed
					response.Message = ErrRequiredBodyIsMissing.Error()
					responseErrors = append(responseErrors, &response)
				}
			}

			// Body parser not found
			if strings.HasPrefix(err.Error(), "request body has an error: failed to decode request body: unsupported content type") {
				return nil, err
			}

			// Body parse errors
			_, isParseErr := err.Err.(*validator.ParseError)
			if isParseErr || strings.HasPrefix(err.Error(), "request body has an error: header Content-Type has unexpected value") {
				response := ValidationError{}
				response.Code = ErrCodeRequiredBodyParseError
				response.Message = err.Error()
				responseErrors = append(responseErrors, &response)
			}
		}

	case *openapi3filter.SecurityRequirementsError:

		for _, secError := range err.Errors {
			response := ValidationError{}
			if err, ok := secError.(*SecurityRequirementsParameterIsMissingError); ok {
				response.Code = ErrCodeSecRequirementsFailed
				response.Message = err.Message
				response.Fields = []string{err.Field}
				responseErrors = append(responseErrors, &response)
				continue
			}
			// In case of security requirement err is unknown
			response.Code = ErrCodeSecRequirementsFailed
			response.Message = secError.Error()
			responseErrors = append(responseErrors, &response)
		}
	}

	// Set the error as unknown
	if len(responseErrors) == 0 {
		response := ValidationError{}
		response.Code = ErrCodeUnknownValidationError
		response.Message = validationError.Error()
		responseErrors = append(responseErrors, &response)
	}

	return responseErrors, nil
}
