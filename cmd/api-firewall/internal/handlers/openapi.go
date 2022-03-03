package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"

	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/oauth2"
	"github.com/wallarm/api-firewall/internal/platform/openapi3"
	"github.com/wallarm/api-firewall/internal/platform/openapi3filter"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/routers"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

type openapiWaf struct {
	route           *routers.Route
	proxyPool       proxy.Pool
	logger          *logrus.Logger
	cfg             *config.APIFWConfiguration
	pathParamLength int
	parserPool      *fastjson.ParserPool
	oauthValidator  oauth2.OAuth2
}

// EXPERIMENTAL feature
// returns APIFW-Validation-Status header value
func getValidationHeader(ctx *fasthttp.RequestCtx, err error) *string {
	var reason = "unknown"

	switch err.(type) {

	case *openapi3filter.ResponseError:
		responseError, ok := err.(*openapi3filter.ResponseError)

		if ok && responseError.Reason != "" {
			reason = responseError.Reason
		}

		id := fmt.Sprintf("response-%d-%s", ctx.Response.StatusCode(), strings.Split(string(ctx.Response.Header.ContentType()), ";")[0])
		value := fmt.Sprintf("%s:%s:response", id, reason)
		return &value

	case *openapi3filter.RequestError:

		requestError, ok := err.(*openapi3filter.RequestError)
		if !ok {
			return nil
		}

		if requestError.Reason != "" {
			reason = requestError.Reason
		}

		if requestError.Parameter != nil {
			paramName := "request-parameter"

			if requestError.Reason == "" {
				schemaError, ok := requestError.Err.(*openapi3.SchemaError)
				if ok && schemaError.Reason != "" {
					reason = schemaError.Reason
				}
				paramName = requestError.Parameter.Name
			}

			value := fmt.Sprintf("request-parameter:%s:%s", reason, paramName)
			return &value
		}

		if requestError.RequestBody != nil {
			id := fmt.Sprintf("request-body-%s", strings.Split(string(ctx.Request.Header.ContentType()), ";")[0])
			value := fmt.Sprintf("%s:%s:request-body", id, reason)
			return &value
		}
	case *openapi3filter.SecurityRequirementsError:

		secSchemeName := ""
		for _, scheme := range err.(*openapi3filter.SecurityRequirementsError).SecurityRequirements {
			for key := range scheme {
				secSchemeName += key + ","
			}
		}

		secErrors := ""
		for _, secError := range err.(*openapi3filter.SecurityRequirementsError).Errors {
			secErrors += secError.Error() + ","
		}

		value := fmt.Sprintf("security-requirements-%s:%s:%s", strings.TrimSuffix(secSchemeName, ","), strings.TrimSuffix(secErrors, ","), strings.TrimSuffix(secSchemeName, ","))
		return &value
	}

	return nil
}

func (s *openapiWaf) openapiWafHandler(ctx *fasthttp.RequestCtx) error {
	s.logger.Debugf("New Request: #%016X : %s -> %s %s (%s)",
		ctx.ID(),
		ctx.Request.Header.Method(),
		ctx.RemoteAddr(), ctx.Path(),
		time.Since(ctx.Time()),
	)

	client, err := s.proxyPool.Get()
	if err != nil {
		s.logger.Errorf("#%016X : error while proxying request: %s", ctx.ID(), strings.Replace(err.Error(), "\n", " ", -1))
		return web.RespondError(ctx, fasthttp.StatusServiceUnavailable, nil)
	}
	defer s.proxyPool.Put(client)

	// Handle request if Validation Disabled for request and response
	if s.route == nil || (s.cfg.RequestValidation == web.ValidationDisable && s.cfg.ResponseValidation == web.ValidationDisable) {

		if err := client.Do(&ctx.Request, &ctx.Response); err != nil {
			s.logger.Errorf("#%016X : error while proxying request: %s", ctx.ID(), strings.Replace(err.Error(), "\n", " ", -1))
			switch err {
			case fasthttp.ErrDialTimeout:
				return web.RespondError(ctx, fasthttp.StatusGatewayTimeout, nil)
			case fasthttp.ErrNoFreeConns:
				return web.RespondError(ctx, fasthttp.StatusServiceUnavailable, nil)
			default:
				return web.RespondError(ctx, fasthttp.StatusBadGateway, nil)
			}
		}

		// check shadow api if path or method are not found and validation mode is LOG_ONLY
		if s.route == nil && (s.cfg.RequestValidation == web.ValidationLog || s.cfg.ResponseValidation == web.ValidationLog) {
			web.ShadowAPIChecks(ctx, s.logger, &s.cfg.ShadowAPI)
		}

		return nil
	}

	var pathParams map[string]string

	if s.pathParamLength > 0 {
		pathParams = make(map[string]string, s.pathParamLength)

		ctx.VisitUserValues(func(key []byte, value interface{}) {
			keyStr := strconv.B2S(key)
			pathParams[keyStr] = value.(string)
		})
	}

	// Validate request
	requestValidationInput := &openapi3filter.RequestValidationInput{
		RequestCtx: ctx,
		PathParams: pathParams,
		Route:      s.route,
		ParserJson: s.parserPool,
		Options: &openapi3filter.Options{
			AuthenticationFunc: func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
				switch input.SecurityScheme.Type {
				case "http":
					switch input.SecurityScheme.Scheme {
					case "basic":
						bHeader := input.RequestValidationInput.RequestCtx.Request.Header.Peek("Authorization")
						if bHeader == nil || !strings.HasPrefix(strings.ToLower(strconv.B2S(bHeader)), "basic ") {
							return errors.New("missing basic authorization header")
						}
					case "bearer":
						bHeader := input.RequestValidationInput.RequestCtx.Request.Header.Peek("Authorization")
						if bHeader == nil || !strings.HasPrefix(strings.ToLower(strconv.B2S(bHeader)), "bearer ") {
							return errors.New("missing bearer authorization header")
						}
					}
				case "oauth2", "openIdConnect":
					if s.oauthValidator == nil {
						return errors.New("oauth2 validator not configured")
					}
					if err := s.oauthValidator.Validate(ctx, string(input.RequestValidationInput.RequestCtx.Request.Header.Peek("Authorization")), input.Scopes); err != nil {
						return fmt.Errorf("oauth2 error: %s", err)
					}

				case "apiKey":
					switch input.SecurityScheme.In {
					case "header":
						if input.RequestValidationInput.RequestCtx.Request.Header.Peek(input.SecurityScheme.Name) == nil {
							return fmt.Errorf("missing %s header", input.SecurityScheme.Name)
						}
					case "query":
						if input.RequestValidationInput.RequestCtx.URI().QueryArgs().Peek(input.SecurityScheme.Name) == nil {
							return fmt.Errorf("missing %s query parameter", input.SecurityScheme.Name)
						}
					case "cookie":
						if input.RequestValidationInput.RequestCtx.Request.Header.Cookie(input.SecurityScheme.Name) == nil {
							return fmt.Errorf("missing %s cookie", input.SecurityScheme.Name)
						}
					}
				}
				return nil
			},
		},
	}

	switch s.cfg.RequestValidation {
	case web.ValidationBlock:
		if err := openapi3filter.ValidateRequest(ctx, requestValidationInput); err != nil {
			s.logger.Errorf("#%016X : request validation error: %s", ctx.ID(), strings.Replace(err.Error(), "\n", " ", -1))
			if s.cfg.AddValidationStatusHeader {
				if vh := getValidationHeader(ctx, err); vh != nil {
					s.logger.Errorf("add header %s: %s", web.ValidationStatus, *vh)
					ctx.Request.Header.Add(web.ValidationStatus, *vh)
					return web.RespondError(ctx, s.cfg.CustomBlockStatusCode, vh)
				}
			}
			return web.RespondError(ctx, s.cfg.CustomBlockStatusCode, nil)
		}
	case web.ValidationLog:
		if err := openapi3filter.ValidateRequest(ctx, requestValidationInput); err != nil {
			s.logger.Errorf("#%016X : request validation error: %s", ctx.ID(), strings.Replace(err.Error(), "\n", " ", -1))
		}
	}

	if err := client.Do(&ctx.Request, &ctx.Response); err != nil {
		s.logger.Errorf("#%016X : error while proxying request: %s", ctx.ID(), strings.Replace(err.Error(), "\n", " ", -1))
		switch err {
		case fasthttp.ErrDialTimeout:
			return web.RespondError(ctx, fasthttp.StatusGatewayTimeout, nil)
		case fasthttp.ErrNoFreeConns:
			return web.RespondError(ctx, fasthttp.StatusServiceUnavailable, nil)
		default:
			return web.RespondError(ctx, fasthttp.StatusBadGateway, nil)
		}
	}

	responseValidationInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestValidationInput,
		Status:                 ctx.Response.StatusCode(),
		ResponseHeader:         &ctx.Response.Header,
		Body:                   ioutil.NopCloser(bytes.NewReader(ctx.Response.Body())),
		Options: &openapi3filter.Options{
			ExcludeRequestBody:    false,
			ExcludeResponseBody:   false,
			IncludeResponseStatus: true,
			MultiError:            false,
			AuthenticationFunc:    nil,
		},
	}

	// Validate response
	switch s.cfg.ResponseValidation {
	case web.ValidationBlock:
		if err := openapi3filter.ValidateResponse(responseValidationInput); err != nil {
			s.logger.Errorf("#%016X : response validation error :  %s", ctx.ID(), strings.Replace(err.Error(), "\n", " ", -1))
			if s.cfg.AddValidationStatusHeader {
				if vh := getValidationHeader(ctx, err); vh != nil {
					s.logger.Errorf("add header %s: %s", web.ValidationStatus, *vh)
					ctx.Response.Header.Add(web.ValidationStatus, *vh)
					return web.RespondError(ctx, s.cfg.CustomBlockStatusCode, vh)
				}
			}
			return web.RespondError(ctx, s.cfg.CustomBlockStatusCode, nil)
		}
	case web.ValidationLog:
		if err := openapi3filter.ValidateResponse(responseValidationInput); err != nil {
			s.logger.Errorf("#%016X : response validation error :  %s", ctx.ID(), strings.Replace(err.Error(), "\n", " ", -1))
		}
	}

	return nil
}
