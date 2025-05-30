package validator

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/pkg/errors"
	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/platform/metrics"

	"github.com/wallarm/api-firewall/internal/platform/loader"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/web"
	"github.com/wallarm/api-firewall/pkg/APIMode/validator"
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
					return &SecurityRequirementsParameterIsMissingError{
						Field:   "Authorization",
						Message: fmt.Sprintf("%v: basic authentication is required", validator.ErrAuthHeaderMissed),
					}
				}
			case "bearer":
				bHeader := input.RequestValidationInput.Request.Header.Get("Authorization")
				if bHeader == "" || !strings.HasPrefix(strings.ToLower(bHeader), "bearer ") {
					return &SecurityRequirementsParameterIsMissingError{
						Field:   "Authorization",
						Message: fmt.Sprintf("%v: bearer authentication is required", validator.ErrAuthHeaderMissed),
					}
				}
			}
		case "apiKey":
			switch input.SecurityScheme.In {
			case "header":
				if input.RequestValidationInput.Request.Header.Get(input.SecurityScheme.Name) == "" {
					return &SecurityRequirementsParameterIsMissingError{
						Field:   input.SecurityScheme.Name,
						Message: fmt.Sprintf("%v: missing %s header", validator.ErrAPITokenMissed, input.SecurityScheme.Name),
					}
				}
			case "query":
				if input.RequestValidationInput.Request.URL.Query().Get(input.SecurityScheme.Name) == "" {
					return &SecurityRequirementsParameterIsMissingError{
						Field:   input.SecurityScheme.Name,
						Message: fmt.Sprintf("%v: missing %s query parameter", validator.ErrAPITokenMissed, input.SecurityScheme.Name),
					}
				}
			case "cookie":
				_, err := input.RequestValidationInput.Request.Cookie(input.SecurityScheme.Name)
				if err != nil {
					return &SecurityRequirementsParameterIsMissingError{
						Field:   input.SecurityScheme.Name,
						Message: fmt.Sprintf("%v: missing %s cookie", validator.ErrAPITokenMissed, input.SecurityScheme.Name),
					}
				}
			}
		}
		return nil
	},
}

// APIModeValidateRequest validates request and respond with 200, 403 (with error) or 500 status code
func APIModeValidateRequest(ctx *fasthttp.RequestCtx, schemaID int, jsonParserPool *fastjson.ParserPool, openAPI *loader.CustomRoute, unknownParametersDetection bool) (validationErrs []*validator.ValidationError, err error) {

	// handle panic
	defer func() {
		if r := recover(); r != nil {

			switch e := r.(type) {
			case error:
				err = e
			default:
				err = fmt.Errorf("panic: %v", r)
			}

			return
		}
	}()

	// Get path parameters
	var pathParams map[string]string

	if openAPI.ParametersNumberInPath > 0 {
		pathParams = router.AllURLParams(ctx)
	}

	// Convert fasthttp request to net/http request
	req := http.Request{}
	if err := fasthttpadaptor.ConvertRequest(ctx, &req, false); err != nil {
		metrics.IncErrorTypeCounter("request conversion error", schemaID)
		return nil, errors.Wrap(err, "request conversion error")
	}

	// Decode request body
	requestContentEncoding := strconv.B2S(ctx.Request.Header.ContentEncoding())
	if requestContentEncoding != "" {
		var err error
		if req.Body, err = web.GetDecompressedRequestBody(&ctx.Request, requestContentEncoding); err != nil {
			metrics.IncErrorTypeCounter("request body decompression error", schemaID)
			return nil, errors.Wrap(err, "request body decompression error")
		}
	}

	// Validate request
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:     &req,
		PathParams:  pathParams,
		Route:       openAPI.Route,
		QueryParams: req.URL.Query(),
		Options:     apiModeSecurityRequirementsOptions,
	}

	var wg sync.WaitGroup

	var valReqErrors error
	var valUPReqErrors error
	var upResults []RequestUnknownParameterError
	var respErrors []*validator.ValidationError

	wg.Add(1)
	go func() {
		defer wg.Done()

		// handle panic
		defer func() {
			if r := recover(); r != nil {
				return
			}
		}()

		// Get fastjson parser
		jsonParser := jsonParserPool.Get()
		defer jsonParserPool.Put(jsonParser)

		valReqErrors = ValidateRequest(ctx, requestValidationInput, jsonParser)
	}()

	// Validate unknown parameters
	if unknownParametersDetection {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// handle panic
			defer func() {
				if r := recover(); r != nil {
					return
				}
			}()

			// Get fastjson parser
			jsonParser := jsonParserPool.Get()
			defer jsonParserPool.Put(jsonParser)

			upResults, valUPReqErrors = ValidateUnknownRequestParameters(ctx, requestValidationInput.Route, req.Header, jsonParser)
		}()
	}

	wg.Wait()

	if valReqErrors != nil {
		switch valErr := valReqErrors.(type) {

		case openapi3.MultiError:

			for _, currentErr := range valErr {
				// Parse validator error and build the response
				parsedValErrs, unknownErr := GetErrorResponse(currentErr)
				if unknownErr != nil {
					metrics.IncErrorTypeCounter("request body decode error: unsupported content type", schemaID)
					return nil, errors.Wrap(unknownErr, "request body decode error: unsupported content type")
				}

				if len(parsedValErrs) > 0 {
					respErrors = append(respErrors, parsedValErrs...)
				}
			}

		default:
			// Parse validator error and build the response
			parsedValErrs, unknownErr := GetErrorResponse(valErr)
			if unknownErr != nil {
				metrics.IncErrorTypeCounter("request body decode error: unsupported content type", schemaID)
				return nil, errors.Wrap(unknownErr, "request body decode error: unsupported content type")
			}
			if parsedValErrs != nil {
				respErrors = append(respErrors, parsedValErrs...)
			}
		}
	}

	if unknownParametersDetection {
		if valUPReqErrors != nil {

			// If it is not a parsing error then return 500
			// If it is a parsing error then it already handled by the request validator
			var parseError *ParseError
			if !errors.As(valUPReqErrors, &parseError) {
				metrics.IncErrorTypeCounter("unknown parameter detection error", schemaID)
				return nil, errors.Wrap(valUPReqErrors, "unknown parameter detection error")
			}
		}

		if len(upResults) > 0 {
			for _, upResult := range upResults {
				for _, f := range upResult.Parameters {
					response := validator.ValidationError{}
					response.Message = upResult.Message
					response.Code = validator.ErrCodeUnknownParameterFound
					response.Fields = []string{f.Name}
					respErrors = append(respErrors, &response)
				}
			}
		}
	}

	return respErrors, nil
}
