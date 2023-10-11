package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/validator"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

var (
	ErrAuthHeaderMissed = errors.New("missing Authorization header")
	ErrAPITokenMissed   = errors.New("missing API keys for authorization")
)

var apiModeSecurityRequirementsOptions = &openapi3filter.Options{
	MultiError: true,
	AuthenticationFunc: func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
		switch input.SecurityScheme.Type {
		case "http":
			switch input.SecurityScheme.Scheme {
			case "basic":
				bHeader := input.RequestValidationInput.Request.Header.Get("Authorization")
				if bHeader == "" || !strings.HasPrefix(strings.ToLower(bHeader), "basic ") {
					return fmt.Errorf("%w: basic authentication is required", ErrAuthHeaderMissed)
				}
			case "bearer":
				bHeader := input.RequestValidationInput.Request.Header.Get("Authorization")
				if bHeader == "" || !strings.HasPrefix(strings.ToLower(bHeader), "bearer ") {
					return fmt.Errorf("%w: bearer authentication is required", ErrAuthHeaderMissed)
				}
			}
		case "apiKey":
			switch input.SecurityScheme.In {
			case "header":
				if input.RequestValidationInput.Request.Header.Get(input.SecurityScheme.Name) == "" {
					return fmt.Errorf("%w: missing %s header", ErrAPITokenMissed, input.SecurityScheme.Name)
				}
			case "query":
				if input.RequestValidationInput.Request.URL.Query().Get(input.SecurityScheme.Name) == "" {
					return fmt.Errorf("%w: missing %s query parameter", ErrAPITokenMissed, input.SecurityScheme.Name)
				}
			case "cookie":
				_, err := input.RequestValidationInput.Request.Cookie(input.SecurityScheme.Name)
				if err != nil {
					return fmt.Errorf("%w: missing %s cookie", ErrAPITokenMissed, input.SecurityScheme.Name)
				}
			}
		}
		return nil
	},
}

type APIMode struct {
	CustomRoute   *router.CustomRoute
	OpenAPIRouter *router.Router
	Log           *logrus.Logger
	Cfg           *config.APIMode
	ParserPool    *fastjson.ParserPool
	SchemaID      string
}

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
	ErrMethodAndPathNotFound = errors.New("method and path are not found")

	ErrRequiredBodyIsMissing    = errors.New("required body is missing")
	ErrMissedRequiredParameters = errors.New("required parameters missed")
)

func getErrorResponse(validationError error) ([]*web.ValidationError, error) {
	var responseErrors []*web.ValidationError

	switch err := validationError.(type) {

	case *openapi3filter.RequestError:
		if err.Parameter != nil {

			// required parameter is missed
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
				responseErrors = append(responseErrors, &response)
			}

			// invalid parameter value
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
				responseErrors = append(responseErrors, &response)
			}

			// validation of the required parameter error
			switch multiErrors := err.Err.(type) {
			case openapi3.MultiError:
				for _, multiErr := range multiErrors {
					schemaError, ok := multiErr.(*openapi3.SchemaError)
					if ok {
						response := web.ValidationError{}
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
							responseErrors = append(responseErrors, &response)
						}
					}
				}
			default:
				schemaError, ok := multiErrors.(*openapi3.SchemaError)
				if ok {
					response := web.ValidationError{}
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
						responseErrors = append(responseErrors, &response)
					}
				}
			}

		}

		// validation of the required body error
		switch multiErrors := err.Err.(type) {
		case openapi3.MultiError:
			for _, multiErr := range multiErrors {
				schemaError, ok := multiErr.(*openapi3.SchemaError)
				if ok {
					response := web.ValidationError{}
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
						responseErrors = append(responseErrors, &response)
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
					response.Fields = schemaError.JSONPointer()
					response.Message = schemaError.Error()
					responseErrors = append(responseErrors, &response)
				default:
					response.Code = ErrCodeRequiredBodyParameterInvalidValue
					response.Fields = schemaError.JSONPointer()
					response.Message = schemaError.Error()
					responseErrors = append(responseErrors, &response)
				}
			}
		}

		// handle request body errors
		if err.RequestBody != nil {

			// body required but missed
			if err.RequestBody.Required {
				if err.Err != nil && err.Err.Error() == validator.ErrInvalidRequired.Error() {
					response := web.ValidationError{}
					response.Code = ErrCodeRequiredBodyMissed
					response.Message = ErrRequiredBodyIsMissing.Error()
					responseErrors = append(responseErrors, &response)
				}
			}

			// body parser not found
			if strings.HasPrefix(err.Error(), "request body has an error: failed to decode request body: unsupported content type") {
				return nil, err
			}

			// body parse errors
			_, isParseErr := err.Err.(*validator.ParseError)
			if isParseErr || strings.HasPrefix(err.Error(), "request body has an error: header Content-Type has unexpected value") {
				response := web.ValidationError{}
				response.Code = ErrCodeRequiredBodyParseError
				response.Message = err.Error()
				responseErrors = append(responseErrors, &response)
			}
		}

	case *openapi3filter.SecurityRequirementsError:

		response := web.ValidationError{}

		secErrors := ""
		for _, secError := range err.Errors {
			secErrors += secError.Error() + ","
		}

		response.Code = ErrCodeSecRequirementsFailed
		response.Message = secErrors
		responseErrors = append(responseErrors, &response)
	}

	// set the error as unknown
	if len(responseErrors) == 0 {
		response := web.ValidationError{}
		response.Code = ErrCodeUnknownValidationError
		response.Message = validationError.Error()
		responseErrors = append(responseErrors, &response)
	}

	return responseErrors, nil
}

// APIModeHandler validates request and respond with 200, 403 (with error) or 500 status code
func (s *APIMode) APIModeHandler(ctx *fasthttp.RequestCtx) error {

	keyValidationErrors := s.SchemaID + web.APIModePostfixValidationErrors
	keyStatusCode := s.SchemaID + web.APIModePostfixStatusCode

	// route not found
	if s.CustomRoute == nil {
		s.Log.WithFields(logrus.Fields{
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
		}).Debug("method or path were not found")
		ctx.SetUserValue(keyValidationErrors, []*web.ValidationError{{Message: ErrMethodAndPathNotFound.Error(), Code: ErrCodeMethodAndPathNotFound, SchemaID: s.SchemaID}})
		ctx.SetUserValue(keyStatusCode, fasthttp.StatusForbidden)
		return nil
	}

	// get path parameters
	var pathParams map[string]string

	if s.CustomRoute.ParametersNumberInPath > 0 {
		pathParams = make(map[string]string)

		ctx.VisitUserValues(func(key []byte, value interface{}) {
			pathParams[strconv.B2S(key)] = value.(string)
		})
	}

	// Convert fasthttp request to net/http request
	req := http.Request{}
	if err := fasthttpadaptor.ConvertRequest(ctx, &req, false); err != nil {
		s.Log.WithFields(logrus.Fields{
			"error":      err,
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
		}).Error("error while converting http request")
		ctx.SetUserValue(keyStatusCode, fasthttp.StatusInternalServerError)
		return nil
	}

	// decode request body
	requestContentEncoding := string(ctx.Request.Header.ContentEncoding())
	if requestContentEncoding != "" {
		var err error
		if req.Body, err = web.GetDecompressedRequestBody(&ctx.Request, requestContentEncoding); err != nil {
			s.Log.WithFields(logrus.Fields{
				"error":      err,
				"request_id": fmt.Sprintf("#%016X", ctx.ID()),
			}).Error("request body decompression error")
			ctx.SetUserValue(keyStatusCode, fasthttp.StatusInternalServerError)
			return nil
		}
	}

	// Validate request
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    &req,
		PathParams: pathParams,
		Route:      s.CustomRoute.Route,
		Options:    apiModeSecurityRequirementsOptions,
	}

	var wg sync.WaitGroup

	var valReqErrors error

	wg.Add(1)
	go func() {
		defer wg.Done()

		// Get fastjson parser
		jsonParser := s.ParserPool.Get()
		defer s.ParserPool.Put(jsonParser)

		valReqErrors = validator.ValidateRequest(ctx, requestValidationInput, jsonParser)
	}()

	var valUPReqErrors error
	var upResults []validator.RequestUnknownParameterError

	// Validate unknown parameters
	if s.Cfg.UnknownParametersDetection {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Get fastjson parser
			jsonParser := s.ParserPool.Get()
			defer s.ParserPool.Put(jsonParser)

			upResults, valUPReqErrors = validator.ValidateUnknownRequestParameters(ctx, requestValidationInput.Route, req.Header, jsonParser)
		}()
	}

	wg.Wait()

	var respErrors []*web.ValidationError

	if valReqErrors != nil {

		switch valErr := valReqErrors.(type) {

		case openapi3.MultiError:

			for _, currentErr := range valErr {
				// parse validation error and build the response
				parsedValErrs, unknownErr := getErrorResponse(currentErr)
				if unknownErr != nil {
					ctx.SetUserValue(keyStatusCode, fasthttp.StatusInternalServerError)
					return nil
				}

				if len(parsedValErrs) > 0 {
					for i := range parsedValErrs {
						parsedValErrs[i].SchemaVersion = s.OpenAPIRouter.SchemaVersion
					}
					respErrors = append(respErrors, parsedValErrs...)
				}
			}

			s.Log.WithFields(logrus.Fields{
				"error":      valReqErrors,
				"request_id": fmt.Sprintf("#%016X", ctx.ID()),
			}).Error("request validation error")
		default:
			// parse validation error and build the response
			parsedValErrs, unknownErr := getErrorResponse(valErr)
			if unknownErr != nil {
				ctx.SetUserValue(keyStatusCode, fasthttp.StatusInternalServerError)
				return nil
			}
			if parsedValErrs != nil {
				s.Log.WithFields(logrus.Fields{
					"error":      valErr,
					"request_id": fmt.Sprintf("#%016X", ctx.ID()),
				}).Warning("request validation error")

				// set schema version for each validation
				if len(parsedValErrs) > 0 {
					for i := range parsedValErrs {
						parsedValErrs[i].SchemaVersion = s.OpenAPIRouter.SchemaVersion
					}
				}
				respErrors = append(respErrors, parsedValErrs...)
			}
		}

		if len(respErrors) == 0 {
			s.Log.WithFields(logrus.Fields{
				"error":      valReqErrors,
				"request_id": fmt.Sprintf("#%016X", ctx.ID()),
			}).Error("request validation error")

			// validation function returned unknown error
			ctx.SetUserValue(keyStatusCode, fasthttp.StatusInternalServerError)
			return nil
		}
	}

	// validate unknown parameters
	if s.Cfg.UnknownParametersDetection {

		if valUPReqErrors != nil {
			s.Log.WithFields(logrus.Fields{
				"error":      valUPReqErrors,
				"request_id": fmt.Sprintf("#%016X", ctx.ID()),
			}).Error("searching for undefined parameters")

			// if it is not a parsing error then return 500
			// if it is a parsing error then it already handled by the request validator
			if _, ok := valUPReqErrors.(*validator.ParseError); !ok {
				ctx.SetUserValue(keyStatusCode, fasthttp.StatusInternalServerError)
				return nil
			}
		}

		if len(upResults) > 0 {
			for _, upResult := range upResults {
				s.Log.WithFields(logrus.Fields{
					"error":      upResult.Err,
					"request_id": fmt.Sprintf("#%016X", ctx.ID()),
				}).Error("searching for undefined parameters")

				response := web.ValidationError{}
				response.SchemaVersion = s.OpenAPIRouter.SchemaVersion
				response.Message = upResult.Err.Error()
				response.Code = ErrCodeUnknownParameterFound
				response.Fields = upResult.Parameters
				respErrors = append(respErrors, &response)
			}
		}
	}

	// respond 403 with errors
	if len(respErrors) > 0 {
		// add schema IDs to the validation error messages
		for _, r := range respErrors {
			r.SchemaID = s.SchemaID
		}
		ctx.SetUserValue(keyValidationErrors, respErrors)
		ctx.SetUserValue(keyStatusCode, fasthttp.StatusForbidden)
		return nil
	}

	// request successfully validated
	ctx.SetUserValue(keyStatusCode, fasthttp.StatusOK)
	return nil
}
