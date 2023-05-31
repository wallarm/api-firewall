package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/mid"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/validator"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

var (
	ErrAuthHeaderMissed = errors.New("missing Authorization header")
	ErrAPITokenMissed   = errors.New("missing API keys for authorization")
)

var apiModeSecurityRequirementsOptions = &openapi3filter.Options{
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
	Cfg           *config.APIFWConfiguration
	ParserPool    *fastjson.ParserPool
}

const (
	ErrCodeMethodAndPathNotFound               = "method_and_path_not_found"
	ErrCodeRequiredBodyMissed                  = "required_body_missed"
	ErrCodeRequiredBodyParameterMissed         = "required_body_parameter_missed"
	ErrCodeRequiredBodyParameterInvalidValue   = "required_body_parameter_invalid_value"
	ErrCodeRequiredQueryParameterMissed        = "required_query_parameter_missed"
	ErrCodeRequiredQueryParameterInvalidValue  = "required_query_parameter_invalid_value"
	ErrCodeRequiredCookieParameterMissed       = "required_cookie_parameter_missed"
	ErrCodeRequiredCookieParameterInvalidValue = "required_cookie_parameter_invalid_value"
	ErrCodeRequiredHeaderMissed                = "required_header_missed"
	ErrCodeRequiredHeaderInvalidValue          = "required_header_invalid_value"

	ErrCodeSecRequirementsFailed = "required_security_requirements_failed"
)

var (
	ErrMethodAndPathNotFound = errors.New("method and path are not found")

	ErrRequiredBodyIsMissing    = errors.New("required body is missing")
	ErrMissedRequiredParameters = errors.New("required parameters missed")
)

type ResponseWithError struct {
	Message       string   `json:"message"`
	Code          string   `json:"code"`
	SchemaVersion string   `json:"schema_version"`
	Fields        []string `json:"related_fields,omitempty"`
}

func getErrorResponse(err error) *ResponseWithError {
	response := ResponseWithError{}

	switch err.(type) {

	case *openapi3filter.RequestError:

		requestError, ok := err.(*openapi3filter.RequestError)
		if !ok {
			return nil
		}

		if requestError.Parameter != nil {

			if errors.Is(requestError, validator.ErrInvalidRequired) {
				switch requestError.Parameter.In {
				case "query":
					response.Code = ErrCodeRequiredQueryParameterMissed
				case "cookie":
					response.Code = ErrCodeRequiredCookieParameterMissed
				case "header":
					response.Code = ErrCodeRequiredHeaderMissed
				}
				response.Message = requestError.Error()
				response.Fields = []string{requestError.Parameter.Name}
				return &response
			}

			schemaError, ok := requestError.Err.(*openapi3.SchemaError)
			if ok {
				switch schemaError.SchemaField {
				case "required":
					switch requestError.Parameter.In {
					case "query":
						response.Code = ErrCodeRequiredQueryParameterMissed
					case "cookie":
						response.Code = ErrCodeRequiredCookieParameterMissed
					case "header":
						response.Code = ErrCodeRequiredHeaderMissed
					}
					response.Fields = schemaError.JSONPointer()
					response.Message = ErrMissedRequiredParameters.Error()
					return &response
				default:
					switch requestError.Parameter.In {
					case "query":
						response.Code = ErrCodeRequiredQueryParameterInvalidValue
					case "cookie":
						response.Code = ErrCodeRequiredCookieParameterInvalidValue
					case "header":
						response.Code = ErrCodeRequiredHeaderInvalidValue
					}
					response.Fields = []string{requestError.Parameter.Name}
					response.Message = schemaError.Error()
					return &response
				}
			}

		}

		schemaError, ok := requestError.Err.(*openapi3.SchemaError)
		if ok {
			switch schemaError.SchemaField {
			case "required":
				response.Code = ErrCodeRequiredBodyParameterMissed
				response.Fields = schemaError.JSONPointer()
				response.Message = schemaError.Error()
				return &response
			default:
				response.Code = ErrCodeRequiredBodyParameterInvalidValue
				response.Fields = schemaError.JSONPointer()
				response.Message = schemaError.Error()
				return &response
			}
		}

		if requestError.RequestBody.Required {
			if requestError.Err.Error() == validator.ErrInvalidRequired.Error() {
				response.Code = ErrCodeRequiredBodyMissed
				response.Message = ErrRequiredBodyIsMissing.Error()
				return &response
			}
		}

	case *openapi3filter.SecurityRequirementsError:

		secErrors := ""
		for _, secError := range err.(*openapi3filter.SecurityRequirementsError).Errors {
			secErrors += secError.Error() + ","
		}

		response.Code = ErrCodeSecRequirementsFailed
		response.Message = secErrors
		return &response
	}

	return nil
}

func (s *APIMode) APIModeHandler(ctx *fasthttp.RequestCtx) error {

	// route not found
	if s.CustomRoute == nil {
		return web.Respond(ctx, ResponseWithError{Message: ErrMethodAndPathNotFound.Error(), Code: ErrCodeMethodAndPathNotFound}, fasthttp.StatusForbidden)
	}

	// get path parameters
	var pathParams map[string]string

	if s.CustomRoute.ParametersNumberInPath > 0 {
		pathParams = make(map[string]string)

		ctx.VisitUserValues(func(key []byte, value interface{}) {
			keyStr := strconv.B2S(key)
			if keyStr != mid.WallarmSchemaID {
				pathParams[keyStr] = value.(string)
			}
		})
	}

	// Convert fasthttp request to net/http request
	req := http.Request{}
	if err := fasthttpadaptor.ConvertRequest(ctx, &req, false); err != nil {
		s.Log.WithFields(logrus.Fields{
			"error":      err,
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
		}).Error("error while converting http request")
		return web.RespondError(ctx, fasthttp.StatusBadRequest, "")
	}

	// get current Wallarm Schema ID
	schemaID := ctx.Value(mid.WallarmSchemaID).(int)

	if schemaID != s.OpenAPIRouter.SchemaID {
		s.Log.WithFields(logrus.Fields{
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
		}).Error("schema version mismatch")
		return web.RespondError(ctx, fasthttp.StatusInternalServerError, "")
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
			return err
		}
	}

	// Validate request
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    &req,
		PathParams: pathParams,
		Route:      s.CustomRoute.Route,
		Options:    apiModeSecurityRequirementsOptions,
	}

	// Get fastjson parser
	jsonParser := s.ParserPool.Get()
	defer s.ParserPool.Put(jsonParser)

	if err := validator.ValidateRequest(ctx, requestValidationInput, jsonParser); err != nil {

		s.Log.WithFields(logrus.Fields{
			"error":      err,
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
		}).Error("request validation error")

		if response := getErrorResponse(err); response != nil {
			response.SchemaVersion = s.OpenAPIRouter.SpecificationVersion
			return web.Respond(ctx, response, fasthttp.StatusForbidden)
		}

		return web.RespondError(ctx, fasthttp.StatusInternalServerError, "")
	}

	return web.RespondError(ctx, fasthttp.StatusOK, "")
}
