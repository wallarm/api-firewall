package validator

import (
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/pkg/errors"
	"github.com/wallarm/api-firewall/pkg/APIMode/validator"
)

type SecurityRequirementsParameterIsMissingError struct {
	Field   string
	Message string
}

func (e *SecurityRequirementsParameterIsMissingError) Error() string {
	return e.Message
}

func checkParseErr(reqErr *openapi3filter.RequestError) *validator.FieldTypeError {
	if parseErr, ok := reqErr.Err.(*ParseError); ok {
		return &validator.FieldTypeError{
			Name:         reqErr.Parameter.Name,
			ExpectedType: parseErr.ExpectedType,
			CurrentValue: parseErr.ValueStr,
		}
	}
	return nil
}

func checkRequiredFields(reqErr *openapi3filter.RequestError, schemaError *openapi3.SchemaError) []*validator.ValidationError {
	totalResponse := []*validator.ValidationError{}
	switch schemaError.SchemaField {
	case "required":

		response := validator.ValidationError{}

		switch reqErr.Parameter.In {
		case "query":
			response.Code = validator.ErrCodeRequiredQueryParameterMissed
		case "cookie":
			response.Code = validator.ErrCodeRequiredCookieParameterMissed
		case "path":
			response.Code = validator.ErrCodeRequiredPathParameterMissed
		case "header":
			response.Code = validator.ErrCodeRequiredHeaderMissed
		}

		for _, field := range schemaError.JSONPointer() {

			response.Fields = []string{field}
			response.Message = validator.ErrMissedRequiredParameters.Error()

			for _, t := range schemaError.Schema.Type.Slice() {
				details := validator.FieldTypeError{
					Name:         reqErr.Parameter.Name,
					ExpectedType: t,
				}

				response.FieldsDetails = append(response.FieldsDetails, details)
			}

			totalResponse = append(totalResponse, &response)
		}

		return totalResponse
	default:

		response := validator.ValidationError{}

		switch reqErr.Parameter.In {
		case "query":
			response.Code = validator.ErrCodeRequiredQueryParameterInvalidValue
		case "cookie":
			response.Code = validator.ErrCodeRequiredCookieParameterInvalidValue
		case "path":
			response.Code = validator.ErrCodeRequiredPathParameterInvalidValue
		case "header":
			response.Code = validator.ErrCodeRequiredHeaderInvalidValue
		}
		response.Fields = []string{reqErr.Parameter.Name}
		response.Message = schemaError.Error()

		// handle parse error case
		if fieldTypeErr := checkParseErr(reqErr); fieldTypeErr != nil {
			response.FieldsDetails = append(response.FieldsDetails, *fieldTypeErr)
			return totalResponse
		}

		for _, t := range schemaError.Schema.Type.Slice() {
			details := validator.FieldTypeError{
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

func GetErrorResponse(validationError error) ([]*validator.ValidationError, error) {
	var responseErrors []*validator.ValidationError

	switch err := validationError.(type) {

	case *openapi3filter.RequestError:
		if err.Parameter != nil {

			// Required parameter is missed
			if errors.Is(err, ErrInvalidRequired) || errors.Is(err, ErrInvalidEmptyValue) {
				response := validator.ValidationError{}
				switch err.Parameter.In {
				case "path":
					response.Code = validator.ErrCodeRequiredPathParameterMissed
				case "query":
					response.Code = validator.ErrCodeRequiredQueryParameterMissed
				case "cookie":
					response.Code = validator.ErrCodeRequiredCookieParameterMissed
				case "header":
					response.Code = validator.ErrCodeRequiredHeaderMissed
				}
				response.Message = err.Error()
				response.Fields = []string{err.Parameter.Name}

				for _, t := range err.Parameter.Schema.Value.Type.Slice() {
					details := validator.FieldTypeError{
						Name:         err.Parameter.Name,
						ExpectedType: t,
					}
					response.FieldsDetails = append(response.FieldsDetails, details)
				}

				responseErrors = append(responseErrors, &response)
			}

			// Invalid parameter value
			if strings.HasSuffix(err.Error(), "invalid syntax") {
				response := validator.ValidationError{}
				switch err.Parameter.In {
				case "path":
					response.Code = validator.ErrCodeRequiredPathParameterInvalidValue
				case "query":
					response.Code = validator.ErrCodeRequiredQueryParameterInvalidValue
				case "cookie":
					response.Code = validator.ErrCodeRequiredCookieParameterInvalidValue
				case "header":
					response.Code = validator.ErrCodeRequiredHeaderInvalidValue
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
							response.FieldsDetails = append(response.FieldsDetails, validator.FieldTypeError{
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
					response := validator.ValidationError{}
					switch schemaError.SchemaField {
					case "required":
						response.Code = validator.ErrCodeRequiredBodyParameterMissed

						for _, field := range schemaError.JSONPointer() {

							response.Fields = []string{field}
							response.Message = schemaError.Error()

							for _, f := range response.Fields {
								if p, lookupErr := schemaError.Schema.Properties.JSONLookup(f); lookupErr == nil {
									for _, t := range p.(*openapi3.Schema).Type.Slice() {
										details := validator.FieldTypeError{
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
						response.Code = validator.ErrCodeRequiredBodyParameterInvalidValue
						response.Message = schemaError.Error()

						for _, field := range schemaError.JSONPointer() {

							response.Fields = []string{field}

							if len(response.Fields) > 0 {
								parseErr, ok := err.Err.(*ParseError)
								if ok {
									response.FieldsDetails = append(response.FieldsDetails, validator.FieldTypeError{
										Name:         response.Fields[0],
										ExpectedType: parseErr.ExpectedType,
										CurrentValue: parseErr.ValueStr,
									})
								} else {
									for _, t := range schemaError.Schema.Type.Slice() {
										details := validator.FieldTypeError{
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
				response := validator.ValidationError{}
				switch schemaError.SchemaField {
				case "required":
					response.Code = validator.ErrCodeRequiredBodyParameterMissed
					for _, field := range schemaError.JSONPointer() {

						response.Fields = []string{field}
						response.Message = schemaError.Error()

						for _, f := range response.Fields {
							if p, lookupErr := schemaError.Schema.Properties.JSONLookup(f); lookupErr == nil {
								for _, t := range p.(*openapi3.Schema).Type.Slice() {
									details := validator.FieldTypeError{
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
					response.Code = validator.ErrCodeRequiredBodyParameterInvalidValue
					for _, field := range schemaError.JSONPointer() {

						response.Fields = []string{field}
						response.Message = schemaError.Error()

						if len(response.Fields) > 0 {
							parseErr, ok := err.Err.(*ParseError)
							if ok {
								response.FieldsDetails = append(response.FieldsDetails, validator.FieldTypeError{
									Name:         response.Fields[0],
									ExpectedType: parseErr.ExpectedType,
									CurrentValue: parseErr.ValueStr,
								})
							} else {

								for _, t := range schemaError.Schema.Type.Slice() {
									details := validator.FieldTypeError{
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
				if err.Err != nil && err.Err.Error() == ErrInvalidRequired.Error() {
					response := validator.ValidationError{}
					response.Code = validator.ErrCodeRequiredBodyMissed
					response.Message = validator.ErrRequiredBodyIsMissing.Error()
					responseErrors = append(responseErrors, &response)
				}
			}

			// Body parser not found
			if strings.HasPrefix(err.Error(), "request body has an error: failed to decode request body: unsupported content type") {
				return nil, err
			}

			// Body parse errors
			_, isParseErr := err.Err.(*ParseError)
			if isParseErr || strings.HasPrefix(err.Error(), "request body has an error: header Content-Type has unexpected value") {
				response := validator.ValidationError{}
				response.Code = validator.ErrCodeRequiredBodyParseError
				response.Message = err.Error()
				responseErrors = append(responseErrors, &response)
			}
		}

	case *openapi3filter.SecurityRequirementsError:

		for _, secError := range err.Errors {
			response := validator.ValidationError{}
			if err, ok := secError.(*SecurityRequirementsParameterIsMissingError); ok {
				response.Code = validator.ErrCodeSecRequirementsFailed
				response.Message = err.Message
				response.Fields = []string{err.Field}
				responseErrors = append(responseErrors, &response)
				continue
			}
			// In case of security requirement err is unknown
			response.Code = validator.ErrCodeSecRequirementsFailed
			response.Message = secError.Error()
			responseErrors = append(responseErrors, &response)
		}
	}

	// Set the error as unknown
	if len(responseErrors) == 0 {
		response := validator.ValidationError{}
		response.Code = validator.ErrCodeUnknownValidationError
		response.Message = validationError.Error()
		responseErrors = append(responseErrors, &response)
	}

	return responseErrors, nil
}
