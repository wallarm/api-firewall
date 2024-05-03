package APImode

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/pkg/errors"
	"github.com/wallarm/api-firewall/internal/platform/validator"
	"github.com/wallarm/api-firewall/internal/platform/web"
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

type SecurityRequirementsParameterIsMissingError struct {
	Field   string
	Message string
}

func (e *SecurityRequirementsParameterIsMissingError) Error() string {
	return e.Message
}

func checkParseErr(reqErr *openapi3filter.RequestError) *web.FieldTypeError {
	if parseErr, ok := reqErr.Err.(*validator.ParseError); ok {
		return &web.FieldTypeError{
			Name:         reqErr.Parameter.Name,
			ExpectedType: parseErr.ExpectedType,
			CurrentValue: parseErr.ValueStr,
		}
	}
	return nil
}

func checkRequiredFields(reqErr *openapi3filter.RequestError, schemaError *openapi3.SchemaError) []*web.ValidationError {
	totalResponse := []*web.ValidationError{}
	switch schemaError.SchemaField {
	case "required":

		response := web.ValidationError{}

		switch reqErr.Parameter.In {
		case "query":
			response.Code = ErrCodeRequiredQueryParameterMissed
		case "cookie":
			response.Code = ErrCodeRequiredCookieParameterMissed
		case "path":
			response.Code = ErrCodeRequiredPathParameterMissed
		case "header":
			response.Code = ErrCodeRequiredHeaderMissed
		}

		for _, field := range schemaError.JSONPointer() {

			response.Fields = []string{field}
			response.Message = ErrMissedRequiredParameters.Error()

			for _, t := range schemaError.Schema.Type.Slice() {
				details := web.FieldTypeError{
					Name:         reqErr.Parameter.Name,
					ExpectedType: t,
				}

				response.FieldsDetails = append(response.FieldsDetails, details)
			}

			totalResponse = append(totalResponse, &response)
		}

		return totalResponse
	default:

		response := web.ValidationError{}

		switch reqErr.Parameter.In {
		case "query":
			response.Code = ErrCodeRequiredQueryParameterInvalidValue
		case "cookie":
			response.Code = ErrCodeRequiredCookieParameterInvalidValue
		case "path":
			response.Code = ErrCodeRequiredPathParameterInvalidValue
		case "header":
			response.Code = ErrCodeRequiredHeaderInvalidValue
		}
		response.Fields = []string{reqErr.Parameter.Name}
		response.Message = schemaError.Error()

		// handle parse error case
		if fieldTypeErr := checkParseErr(reqErr); fieldTypeErr != nil {
			response.FieldsDetails = append(response.FieldsDetails, *fieldTypeErr)
			return totalResponse
		}

		for _, t := range schemaError.Schema.Type.Slice() {
			details := web.FieldTypeError{
				Name:         reqErr.Parameter.Name,
				ExpectedType: t,
				CurrentValue: fmt.Sprintf("%v", schemaError.Value),
			}

			// handle max, min and pattern cases
			switch schemaError.SchemaField {
			case "maximum":
				details.Pattern = fmt.Sprintf("<=%0.4f", *schemaError.Schema.Max)
			case "minimum":
				details.Pattern = fmt.Sprintf(">=%0.4f", *schemaError.Schema.Min)
			case "pattern":
				details.Pattern = schemaError.Schema.Pattern
			}

			response.FieldsDetails = append(response.FieldsDetails, details)
		}
		totalResponse = append(totalResponse, &response)
	}

	return totalResponse
}

func GetErrorResponse(validationError error) ([]*web.ValidationError, error) {
	var responseErrors []*web.ValidationError

	switch err := validationError.(type) {

	case *openapi3filter.RequestError:
		if err.Parameter != nil {

			// Required parameter is missed
			if errors.Is(err, validator.ErrInvalidRequired) || errors.Is(err, validator.ErrInvalidEmptyValue) {
				response := web.ValidationError{}
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

				for _, t := range err.Parameter.Schema.Value.Type.Slice() {
					details := web.FieldTypeError{
						Name:         err.Parameter.Name,
						ExpectedType: t,
					}
					response.FieldsDetails = append(response.FieldsDetails, details)
				}

				responseErrors = append(responseErrors, &response)
			}

			// Invalid parameter value
			if strings.HasSuffix(err.Error(), "invalid syntax") {
				response := web.ValidationError{}
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

				if fieldTypeErr := checkParseErr(err); fieldTypeErr != nil {
					response.FieldsDetails = append(response.FieldsDetails, *fieldTypeErr)
				}
				schemaError, ok := err.Err.(*openapi3.SchemaError)
				if ok {
					if schemaError.SchemaField == "pattern" {
						for _, t := range schemaError.Schema.Type.Slice() {
							response.FieldsDetails = append(response.FieldsDetails, web.FieldTypeError{
								Name:         err.Parameter.Name,
								ExpectedType: t,
								Pattern:      schemaError.Schema.Pattern,
								CurrentValue: fmt.Sprintf("%v", schemaError.Value),
							})
						}
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
						response := checkRequiredFields(err, schemaError)
						responseErrors = append(responseErrors, response...)
					}
				}
			default:
				schemaError, ok := multiErrors.(*openapi3.SchemaError)
				if ok {
					response := checkRequiredFields(err, schemaError)
					responseErrors = append(responseErrors, response...)
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
					response := web.ValidationError{}
					switch schemaError.SchemaField {
					case "required":
						response.Code = ErrCodeRequiredBodyParameterMissed

						for _, field := range schemaError.JSONPointer() {

							response.Fields = []string{field}
							response.Message = schemaError.Error()

							for _, f := range response.Fields {
								if p, lookupErr := schemaError.Schema.Properties.JSONLookup(f); lookupErr == nil {
									for _, t := range p.(*openapi3.Schema).Type.Slice() {
										details := web.FieldTypeError{
											Name:         f,
											ExpectedType: t,
										}
										response.FieldsDetails = append(response.FieldsDetails, details)
									}
								}
							}

							responseErrors = append(responseErrors, &response)
						}
					default:
						response.Code = ErrCodeRequiredBodyParameterInvalidValue
						response.Message = schemaError.Error()

						for _, field := range schemaError.JSONPointer() {

							response.Fields = []string{field}

							if len(response.Fields) > 0 {
								parseErr, ok := err.Err.(*validator.ParseError)
								if ok {
									response.FieldsDetails = append(response.FieldsDetails, web.FieldTypeError{
										Name:         response.Fields[0],
										ExpectedType: parseErr.ExpectedType,
										CurrentValue: parseErr.ValueStr,
									})
								} else {
									for _, t := range schemaError.Schema.Type.Slice() {
										details := web.FieldTypeError{
											Name:         response.Fields[0],
											ExpectedType: t,
											CurrentValue: fmt.Sprintf("%v", schemaError.Value),
										}
										switch schemaError.SchemaField {
										case "pattern":
											details.Pattern = schemaError.Schema.Pattern
										case "maximum":
											details.Pattern = fmt.Sprintf("<=%0.4f", *schemaError.Schema.Max)
										case "minimum":
											details.Pattern = fmt.Sprintf(">=%0.4f", *schemaError.Schema.Min)
										}

										response.FieldsDetails = append(response.FieldsDetails, details)
									}
								}
								responseErrors = append(responseErrors, &response)
							}
						}
					}
				}
			}
		default:
			schemaError, ok := multiErrors.(*openapi3.SchemaError)
			if ok {
				response := web.ValidationError{}
				switch schemaError.SchemaField {
				case "required":
					response.Code = ErrCodeRequiredBodyParameterMissed
					for _, field := range schemaError.JSONPointer() {

						response.Fields = []string{field}
						response.Message = schemaError.Error()

						for _, f := range response.Fields {
							if p, lookupErr := schemaError.Schema.Properties.JSONLookup(f); lookupErr == nil {
								for _, t := range p.(*openapi3.Schema).Type.Slice() {
									details := web.FieldTypeError{
										Name:         f,
										ExpectedType: t,
									}
									response.FieldsDetails = append(response.FieldsDetails, details)
								}
							}
						}

						responseErrors = append(responseErrors, &response)
					}
				default:
					response.Code = ErrCodeRequiredBodyParameterInvalidValue
					for _, field := range schemaError.JSONPointer() {

						response.Fields = []string{field}
						response.Message = schemaError.Error()

						if len(response.Fields) > 0 {
							parseErr, ok := err.Err.(*validator.ParseError)
							if ok {
								response.FieldsDetails = append(response.FieldsDetails, web.FieldTypeError{
									Name:         response.Fields[0],
									ExpectedType: parseErr.ExpectedType,
									CurrentValue: parseErr.ValueStr,
								})
							} else {

								for _, t := range schemaError.Schema.Type.Slice() {
									details := web.FieldTypeError{
										Name:         response.Fields[0],
										ExpectedType: t,
										CurrentValue: fmt.Sprintf("%v", schemaError.Value),
									}
									switch schemaError.SchemaField {
									case "pattern":
										details.Pattern = schemaError.Schema.Pattern
									case "maximum":
										details.Pattern = fmt.Sprintf("<=%0.4f", *schemaError.Schema.Max)
									case "minimum":
										details.Pattern = fmt.Sprintf(">=%0.4f", *schemaError.Schema.Min)
									}

									response.FieldsDetails = append(response.FieldsDetails, details)
								}

							}
							responseErrors = append(responseErrors, &response)
						}
					}
				}
			}
		}

		// Handle request body errors
		if err.RequestBody != nil {

			// Body required but missed
			if err.RequestBody.Required {
				if err.Err != nil && err.Err.Error() == validator.ErrInvalidRequired.Error() {
					response := web.ValidationError{}
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
				response := web.ValidationError{}
				response.Code = ErrCodeRequiredBodyParseError
				response.Message = err.Error()
				responseErrors = append(responseErrors, &response)
			}
		}

	case *openapi3filter.SecurityRequirementsError:

		for _, secError := range err.Errors {
			response := web.ValidationError{}
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
		response := web.ValidationError{}
		response.Code = ErrCodeUnknownValidationError
		response.Message = validationError.Error()
		responseErrors = append(responseErrors, &response)
	}

	return responseErrors, nil
}
