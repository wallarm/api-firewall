package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/rs/zerolog"
	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"github.com/valyala/fastjson"

	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/loader"
	"github.com/wallarm/api-firewall/internal/platform/oauth2"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/validator"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

type openapiWaf struct {
	customRoute    *loader.CustomRoute
	proxyPool      proxy.Pool
	logger         zerolog.Logger
	cfg            *config.ProxyMode
	parserPool     *fastjson.ParserPool
	oauthValidator oauth2.OAuth2
}

// EXPERIMENTAL feature
// returns APIFW-Validation-Status header value
func getValidationHeader(ctx *fasthttp.RequestCtx, err error) *string {
	var reason = "unknown"

	switch err := err.(type) {
	case *openapi3filter.ResponseError:
		if err.Reason != "" {
			reason = err.Reason
		}

		id := fmt.Sprintf("response-%d-%s", ctx.Response.StatusCode(), strings.Split(strconv.B2S(ctx.Response.Header.ContentType()), ";")[0])
		value := fmt.Sprintf("%s:%s:response", id, reason)
		return &value

	case *openapi3filter.RequestError:

		if err.Reason != "" {
			reason = err.Reason
		}

		if err.Parameter != nil {
			paramName := "request-parameter"

			if err.Reason == "" {
				schemaError, ok := err.Err.(*openapi3.SchemaError)
				if ok && schemaError.Reason != "" {
					reason = schemaError.Reason
				}
				paramName = err.Parameter.Name
			}

			value := fmt.Sprintf("request-parameter:%s:%s", reason, paramName)
			return &value
		}

		if err.RequestBody != nil {
			id := fmt.Sprintf("request-body-%s", strings.Split(strconv.B2S(ctx.Request.Header.ContentType()), ";")[0])
			value := fmt.Sprintf("%s:%s:request-body", id, reason)
			return &value
		}
	case *openapi3filter.SecurityRequirementsError:

		secRequirements := err.SecurityRequirements
		secSchemeName := ""
		for _, scheme := range secRequirements {
			for key := range scheme {
				secSchemeName += key + ","
			}
		}

		secErrors := ""
		for _, secError := range err.Errors {
			secErrors += secError.Error() + ","
		}

		value := fmt.Sprintf("security-requirements-%s:%s:%s", strings.TrimSuffix(secSchemeName, ","), strings.TrimSuffix(secErrors, ","), strings.TrimSuffix(secSchemeName, ","))
		return &value
	}

	return nil
}

func (s *openapiWaf) openapiWafHandler(ctx *fasthttp.RequestCtx) error {

	// pass OPTIONS if the feature is enabled
	var isOptionsReq, ok bool
	if isOptionsReq, ok = ctx.UserValue(web.PassRequestOPTIONS).(bool); !ok {
		isOptionsReq = false
	}

	// set Request/Response validation mode
	var RequestValidationMode = s.cfg.RequestValidation
	var ResponseValidationMode = s.cfg.ResponseValidation
	if actions, ok := ctx.UserValue(web.EndpointActions).(*router.Actions); ok {
		RequestValidationMode = actions.Request
		ResponseValidationMode = actions.Response
	}

	// proxy request if APIFW is disabled
	if isOptionsReq ||
		strings.EqualFold(RequestValidationMode, web.ValidationDisable) && strings.EqualFold(ResponseValidationMode, web.ValidationDisable) {
		if err := proxy.Perform(ctx, s.proxyPool, s.cfg.Server.RequestHostHeader); err != nil {
			s.logger.Error().
				Err(err).
				Interface("request_id", ctx.UserValue(web.RequestID)).
				Bytes("host", ctx.Request.Header.Host()).
				Bytes("path", ctx.Path()).
				Bytes("method", ctx.Request.Header.Method()).
				Msg("Error while proxying request")
		}
		return nil
	}

	// if Validation is BLOCK for request and response then respond by CustomBlockStatusCode
	if s.customRoute == nil {
		// route for the request not found
		ctx.SetUserValue(web.RequestProxyNoRoute, true)

		if strings.EqualFold(RequestValidationMode, web.ValidationBlock) || strings.EqualFold(ResponseValidationMode, web.ValidationBlock) {
			if s.cfg.AddValidationStatusHeader {
				vh := "request: customRoute not found"
				return web.RespondError(ctx, s.cfg.CustomBlockStatusCode, vh)
			}
			return web.RespondError(ctx, s.cfg.CustomBlockStatusCode, "")
		}

		if err := proxy.Perform(ctx, s.proxyPool, s.cfg.Server.RequestHostHeader); err != nil {
			s.logger.Error().
				Err(err).
				Interface("request_id", ctx.UserValue(web.RequestID)).
				Bytes("host", ctx.Request.Header.Host()).
				Bytes("path", ctx.Path()).
				Bytes("method", ctx.Request.Header.Method()).
				Msg("Error while proxying request")
		}
		return nil
	}

	var pathParams map[string]string

	if s.customRoute.ParametersNumberInPath > 0 {
		pathParams = router.AllURLParams(ctx)
	}

	// convert fasthttp request to net/http request
	req := http.Request{}
	if err := fasthttpadaptor.ConvertRequest(ctx, &req, false); err != nil {
		s.logger.Error().
			Err(err).
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Bytes("host", ctx.Request.Header.Host()).
			Bytes("path", ctx.Path()).
			Bytes("method", ctx.Request.Header.Method()).
			Msg("Error while converting http request")
		return web.RespondError(ctx, fasthttp.StatusBadRequest, "")
	}

	// decode request body
	requestContentEncoding := strconv.B2S(ctx.Request.Header.ContentEncoding())
	if requestContentEncoding != "" {
		var err error
		req.Body, err = web.GetDecompressedRequestBody(&ctx.Request, requestContentEncoding)
		if err != nil {
			s.logger.Error().
				Err(err).
				Interface("request_id", ctx.UserValue(web.RequestID)).
				Bytes("host", ctx.Request.Header.Host()).
				Bytes("path", ctx.Path()).
				Bytes("method", ctx.Request.Header.Method()).
				Msg("Request body decompression error")
			return err
		}
	}

	// validate request
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:     &req,
		PathParams:  pathParams,
		Route:       s.customRoute.Route,
		QueryParams: req.URL.Query(),
		Options: &openapi3filter.Options{
			AuthenticationFunc: func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
				switch input.SecurityScheme.Type {
				case "http":
					switch input.SecurityScheme.Scheme {
					case "basic":
						bHeader := input.RequestValidationInput.Request.Header.Get("Authorization")
						if bHeader == "" || !strings.HasPrefix(strings.ToLower(bHeader), "basic ") {
							return errors.New("missing basic authorization header")
						}
					case "bearer":
						bHeader := input.RequestValidationInput.Request.Header.Get("Authorization")
						if bHeader == "" || !strings.HasPrefix(strings.ToLower(bHeader), "bearer ") {
							return errors.New("missing bearer authorization header")
						}
					}
				case "oauth2", "openIdConnect":
					if s.oauthValidator == nil {
						return errors.New("oauth2 validator not configured")
					}
					if err := s.oauthValidator.Validate(ctx, input.RequestValidationInput.Request.Header.Get("Authorization"), input.Scopes); err != nil {
						return fmt.Errorf("oauth2 error: %s", err)
					}

				case "apiKey":
					switch input.SecurityScheme.In {
					case "header":
						if input.RequestValidationInput.Request.Header.Get(input.SecurityScheme.Name) == "" {
							return fmt.Errorf("missing %s header", input.SecurityScheme.Name)
						}
					case "query":
						if input.RequestValidationInput.Request.URL.Query().Get(input.SecurityScheme.Name) == "" {
							return fmt.Errorf("missing %s query parameter", input.SecurityScheme.Name)
						}
					case "cookie":
						_, err := input.RequestValidationInput.Request.Cookie(input.SecurityScheme.Name)
						if err != nil {
							return fmt.Errorf("missing %s cookie", input.SecurityScheme.Name)
						}
					}
				}
				return nil
			},
		},
	}

	// get fastjson parser
	jsonParser := s.parserPool.Get()
	defer s.parserPool.Put(jsonParser)

	switch strings.ToLower(RequestValidationMode) {
	case web.ValidationBlock:
		if err := validator.ValidateRequest(ctx, requestValidationInput, jsonParser); err != nil {

			isRequestBlocked := true
			if requestErr, ok := err.(*openapi3filter.RequestError); ok {

				// body parser not found
				if strings.HasPrefix(requestErr.Error(), "request body has an error: failed to decode request body: unsupported content type") {
					s.logger.Error().
						Err(err).
						Interface("request_id", ctx.UserValue(web.RequestID)).
						Bytes("host", ctx.Request.Header.Host()).
						Bytes("path", ctx.Path()).
						Bytes("method", ctx.Request.Header.Method()).
						Msg("Request body parsing error: request passed")
					isRequestBlocked = false
				}
			}

			if isRequestBlocked {
				// request has been blocked
				ctx.SetUserValue(web.RequestBlocked, true)

				s.logger.Error().
					Err(err).
					Interface("request_id", ctx.UserValue(web.RequestID)).
					Bytes("host", ctx.Request.Header.Host()).
					Bytes("path", ctx.Path()).
					Bytes("method", ctx.Request.Header.Method()).
					Msg("Request validation error: request blocked")

				if s.cfg.AddValidationStatusHeader {
					if vh := getValidationHeader(ctx, err); vh != nil {
						s.logger.Error().
							Err(err).
							Interface("request_id", ctx.UserValue(web.RequestID)).
							Msgf("add header %s: %s", web.ValidationStatus, *vh)
						ctx.Request.Header.Add(web.ValidationStatus, *vh)
						return web.RespondError(ctx, s.cfg.CustomBlockStatusCode, *vh)
					}
				}

				return web.RespondError(ctx, s.cfg.CustomBlockStatusCode, "")
			}
		}

		if s.cfg.ShadowAPI.UnknownParametersDetection {
			upResults, valUPReqErrors := validator.ValidateUnknownRequestParameters(ctx, requestValidationInput.Route, req.Header, jsonParser)
			// log only error and pass request if unknown params module can't parse it
			if valUPReqErrors != nil {
				s.logger.Warn().
					Err(valUPReqErrors).
					Interface("request_id", ctx.UserValue(web.RequestID)).
					Msg("Shadow API: searching for undefined parameters")
			}

			if len(upResults) > 0 {
				unknownParameters, _ := json.Marshal(upResults)
				s.logger.Error().
					Bytes("errors", unknownParameters).
					Interface("request_id", ctx.UserValue(web.RequestID)).
					Bytes("host", ctx.Request.Header.Host()).
					Bytes("path", ctx.Path()).
					Bytes("method", ctx.Request.Header.Method()).
					Msg("Shadow API: undefined parameters found")

				// request has been blocked
				ctx.SetUserValue(web.RequestBlocked, true)
				return web.RespondError(ctx, s.cfg.CustomBlockStatusCode, "")
			}
		}
	case web.ValidationLog:
		if err := validator.ValidateRequest(ctx, requestValidationInput, jsonParser); err != nil {
			s.logger.Error().
				Err(err).
				Interface("request_id", ctx.UserValue(web.RequestID)).
				Bytes("host", ctx.Request.Header.Host()).
				Bytes("path", ctx.Path()).
				Bytes("method", ctx.Request.Header.Method()).
				Msg("Request validation error")
		}

		if s.cfg.ShadowAPI.UnknownParametersDetection {
			upResults, valUPReqErrors := validator.ValidateUnknownRequestParameters(ctx, requestValidationInput.Route, req.Header, jsonParser)
			// log only error and pass request if unknown params module can't parse it
			if valUPReqErrors != nil {
				s.logger.Warn().
					Err(valUPReqErrors).
					Interface("request_id", ctx.UserValue(web.RequestID)).
					Msg("Shadow API: searching for undefined parameters")
			}

			if len(upResults) > 0 {
				unknownParameters, _ := json.Marshal(upResults)
				s.logger.Error().
					Bytes("errors", unknownParameters).
					Interface("request_id", ctx.UserValue(web.RequestID)).
					Bytes("host", ctx.Request.Header.Host()).
					Bytes("path", ctx.Path()).
					Bytes("method", ctx.Request.Header.Method()).
					Msg("Shadow API: undefined parameters found")
			}
		}
	}

	if err := proxy.Perform(ctx, s.proxyPool, s.cfg.Server.RequestHostHeader); err != nil {
		s.logger.Error().
			Err(err).
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Bytes("host", ctx.Request.Header.Host()).
			Bytes("path", ctx.Path()).
			Bytes("method", ctx.Request.Header.Method()).
			Msg("Error while proxying request")
		return nil
	}

	// prepare http response headers
	respHeader := http.Header{}
	ctx.Response.Header.VisitAll(func(k, v []byte) {
		sk := strconv.B2S(k)
		sv := strconv.B2S(v)

		respHeader.Set(sk, sv)
	})

	// decode response body
	responseBodyReader, err := web.GetDecompressedResponseBody(&ctx.Response, strconv.B2S(ctx.Response.Header.ContentEncoding()))
	if err != nil {
		s.logger.Error().
			Err(err).
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Bytes("host", ctx.Request.Header.Host()).
			Bytes("path", ctx.Path()).
			Bytes("method", ctx.Request.Header.Method()).
			Msg("Response body decompression error")
		return err
	}

	responseValidationInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestValidationInput,
		Status:                 ctx.Response.StatusCode(),
		Header:                 respHeader,
		Body:                   responseBodyReader,
		Options: &openapi3filter.Options{
			ExcludeRequestBody:    false,
			ExcludeResponseBody:   false,
			IncludeResponseStatus: true,
			MultiError:            false,
			AuthenticationFunc:    nil,
		},
	}

	// validate response
	switch strings.ToLower(ResponseValidationMode) {
	case web.ValidationBlock:
		if err := validator.ValidateResponse(ctx, responseValidationInput, jsonParser); err != nil {
			// response has been blocked
			ctx.SetUserValue(web.ResponseBlocked, true)
			s.logger.Error().
				Err(err).
				Interface("request_id", ctx.UserValue(web.RequestID)).
				Bytes("host", ctx.Request.Header.Host()).
				Bytes("path", ctx.Path()).
				Bytes("method", ctx.Request.Header.Method()).
				Msg("Response validation error")

			if s.cfg.AddValidationStatusHeader {
				if vh := getValidationHeader(ctx, err); vh != nil {
					s.logger.Error().
						Err(err).
						Interface("request_id", ctx.UserValue(web.RequestID)).
						Msgf("Add header %s: %s", web.ValidationStatus, *vh)
					ctx.Response.Header.Add(web.ValidationStatus, *vh)
					return web.RespondError(ctx, s.cfg.CustomBlockStatusCode, *vh)
				}
			}
			return web.RespondError(ctx, s.cfg.CustomBlockStatusCode, "")
		}
	case web.ValidationLog:
		if err := validator.ValidateResponse(ctx, responseValidationInput, jsonParser); err != nil {
			if respErr, ok := err.(*openapi3filter.ResponseError); ok {
				// body parser not found
				if respErr.Reason == "status is not supported" {
					// received response status was not found in the OpenAPI spec
					ctx.SetUserValue(web.ResponseStatusNotFound, true)
				}
				return nil
			}

			s.logger.Error().
				Err(err).
				Interface("request_id", ctx.UserValue(web.RequestID)).
				Bytes("host", ctx.Request.Header.Host()).
				Bytes("path", ctx.Path()).
				Bytes("method", ctx.Request.Header.Method()).
				Msg("Response validation error")
		}
	}

	return nil
}
