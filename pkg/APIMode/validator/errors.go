package validator

import (
	"fmt"

	"github.com/pkg/errors"
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
	ErrMethodAndPathNotFound    = errors.New("method and path are not found")
	ErrAuthHeaderMissed         = errors.New("missing Authorization header")
	ErrAPITokenMissed           = errors.New("missing API keys for authorization")
	ErrRequiredBodyIsMissing    = errors.New("required body is missing")
	ErrMissedRequiredParameters = errors.New("required parameters missed")

	ErrSchemaNotFound = fmt.Errorf("schema not found")
	ErrRequestParsing = fmt.Errorf("request parsing error")
	ErrSpecParsing    = fmt.Errorf("OpenAPI specification parsing error")
	ErrSpecValidation = fmt.Errorf("OpenAPI specification validator error")
	ErrSpecLoading    = fmt.Errorf("OpenAPI specifications reading from database error")
	ErrHandlersInit   = fmt.Errorf("handlers initialization error")
)
