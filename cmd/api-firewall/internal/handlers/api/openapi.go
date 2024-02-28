package api

import (
	"context"
	"fmt"
	"net/http"
	strconv2 "strconv"
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
						Message: fmt.Sprintf("%v: basic authentication is required", ErrAuthHeaderMissed),
					}
				}
			case "bearer":
				bHeader := input.RequestValidationInput.Request.Header.Get("Authorization")
				if bHeader == "" || !strings.HasPrefix(strings.ToLower(bHeader), "bearer ") {
					return &SecurityRequirementsParameterIsMissingError{
						Field:   "Authorization",
						Message: fmt.Sprintf("%v: bearer authentication is required", ErrAuthHeaderMissed),
					}
				}
			}
		case "apiKey":
			switch input.SecurityScheme.In {
			case "header":
				if input.RequestValidationInput.Request.Header.Get(input.SecurityScheme.Name) == "" {
					return &SecurityRequirementsParameterIsMissingError{
						Field:   input.SecurityScheme.Name,
						Message: fmt.Sprintf("%v: missing %s header", ErrAPITokenMissed, input.SecurityScheme.Name),
					}
				}
			case "query":
				if input.RequestValidationInput.Request.URL.Query().Get(input.SecurityScheme.Name) == "" {
					return &SecurityRequirementsParameterIsMissingError{
						Field:   input.SecurityScheme.Name,
						Message: fmt.Sprintf("%v: missing %s query parameter", ErrAPITokenMissed, input.SecurityScheme.Name),
					}
				}
			case "cookie":
				_, err := input.RequestValidationInput.Request.Cookie(input.SecurityScheme.Name)
				if err != nil {
					return &SecurityRequirementsParameterIsMissingError{
						Field:   input.SecurityScheme.Name,
						Message: fmt.Sprintf("%v: missing %s cookie", ErrAPITokenMissed, input.SecurityScheme.Name),
					}
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
	SchemaID      int
}

// APIModeHandler validates request and respond with 200, 403 (with error) or 500 status code
func (s *APIMode) APIModeHandler(ctx *fasthttp.RequestCtx) error {

	keyValidationErrors := strconv2.Itoa(s.SchemaID) + web.APIModePostfixValidationErrors
	keyStatusCode := strconv2.Itoa(s.SchemaID) + web.APIModePostfixStatusCode

	// Route not found
	if s.CustomRoute == nil {
		s.Log.WithFields(logrus.Fields{
			"host":       string(ctx.Request.Header.Host()),
			"path":       string(ctx.Path()),
			"request_id": ctx.UserValue(web.RequestID),
		}).Debug("method or path were not found")
		ctx.SetUserValue(keyValidationErrors, []*web.ValidationError{{Message: ErrMethodAndPathNotFound.Error(), Code: ErrCodeMethodAndPathNotFound, SchemaID: &s.SchemaID}})
		ctx.SetUserValue(keyStatusCode, fasthttp.StatusForbidden)
		return nil
	}

	// Get path parameters
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
			"host":       string(ctx.Request.Header.Host()),
			"path":       string(ctx.Path()),
			"request_id": ctx.UserValue(web.RequestID),
		}).Error("error while converting http request")
		ctx.SetUserValue(keyStatusCode, fasthttp.StatusInternalServerError)
		return nil
	}

	// Decode request body
	requestContentEncoding := strconv.B2S(ctx.Request.Header.ContentEncoding())
	if requestContentEncoding != "" {
		var err error
		if req.Body, err = web.GetDecompressedRequestBody(&ctx.Request, requestContentEncoding); err != nil {
			s.Log.WithFields(logrus.Fields{
				"error":      err,
				"host":       string(ctx.Request.Header.Host()),
				"path":       string(ctx.Path()),
				"request_id": ctx.UserValue(web.RequestID),
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
				// Parse validation error and build the response
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
				"host":       string(ctx.Request.Header.Host()),
				"path":       string(ctx.Path()),
				"request_id": ctx.UserValue(web.RequestID),
			}).Error("request validation error")
		default:
			// Parse validation error and build the response
			parsedValErrs, unknownErr := getErrorResponse(valErr)
			if unknownErr != nil {
				ctx.SetUserValue(keyStatusCode, fasthttp.StatusInternalServerError)
				return nil
			}
			if parsedValErrs != nil {
				s.Log.WithFields(logrus.Fields{
					"error":      valErr,
					"host":       string(ctx.Request.Header.Host()),
					"path":       string(ctx.Path()),
					"request_id": ctx.UserValue(web.RequestID),
				}).Warning("request validation error")

				// Set schema version for each validation
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
				"host":       string(ctx.Request.Header.Host()),
				"path":       string(ctx.Path()),
				"request_id": ctx.UserValue(web.RequestID),
			}).Error("request validation error")

			// validation function returned unknown error
			ctx.SetUserValue(keyStatusCode, fasthttp.StatusInternalServerError)
			return nil
		}
	}

	// Validate unknown parameters
	if s.Cfg.UnknownParametersDetection {

		if valUPReqErrors != nil {
			s.Log.WithFields(logrus.Fields{
				"error":      valUPReqErrors,
				"host":       string(ctx.Request.Header.Host()),
				"path":       string(ctx.Path()),
				"request_id": ctx.UserValue(web.RequestID),
			}).Error("searching for undefined parameters")

			// If it is not a parsing error then return 500
			// If it is a parsing error then it already handled by the request validator
			if _, ok := valUPReqErrors.(*validator.ParseError); !ok {
				ctx.SetUserValue(keyStatusCode, fasthttp.StatusInternalServerError)
				return nil
			}
		}

		if len(upResults) > 0 {
			for _, upResult := range upResults {
				s.Log.WithFields(logrus.Fields{
					"error":      upResult.Err,
					"host":       string(ctx.Request.Header.Host()),
					"path":       string(ctx.Path()),
					"request_id": ctx.UserValue(web.RequestID),
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

	// Respond 403 with errors
	if len(respErrors) > 0 {
		// add schema IDs to the validation error messages
		for _, r := range respErrors {
			r.SchemaID = &s.SchemaID
		}
		ctx.SetUserValue(keyValidationErrors, respErrors)
		ctx.SetUserValue(keyStatusCode, fasthttp.StatusForbidden)
		return nil
	}

	// request successfully validated
	ctx.SetUserValue(keyStatusCode, fasthttp.StatusOK)
	return nil
}
