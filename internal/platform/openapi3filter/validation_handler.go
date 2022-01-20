package openapi3filter

import (
	"context"
	"net/http"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"

	//legacyrouter "github.com/getkin/kin-openapi/routers/legacy"
	"github.com/wallarm/api-firewall/internal/platform/openapi3"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/routers"
	"github.com/wallarm/api-firewall/internal/platform/routers/legacy"
)

type AuthenticationFunc func(context.Context, *AuthenticationInput) error

func NoopAuthenticationFunc(context.Context, *AuthenticationInput) error { return nil }

var _ AuthenticationFunc = NoopAuthenticationFunc

type ValidationHandler struct {
	Handler            http.Handler
	AuthenticationFunc AuthenticationFunc
	SwaggerFile        string
	ErrorEncoder       ErrorEncoder
	router             routers.Router
}

func (h *ValidationHandler) Load() error {
	loader := openapi3.NewSwaggerLoader()
	doc, err := loader.LoadSwaggerFromFile(h.SwaggerFile)
	if err != nil {
		return err
	}
	if err := doc.Validate(loader.Context); err != nil {
		return err
	}
	if h.router, err = legacy.NewRouter(doc); err != nil {
		return err
	}

	// set defaults
	if h.Handler == nil {
		h.Handler = http.DefaultServeMux
	}
	if h.AuthenticationFunc == nil {
		h.AuthenticationFunc = NoopAuthenticationFunc
	}
	if h.ErrorEncoder == nil {
		h.ErrorEncoder = DefaultErrorEncoder
	}

	return nil
}

func (h *ValidationHandler) validateRequest(ctx *fasthttp.RequestCtx) error {
	r, err := router.NewReqB2S(ctx)
	if err != nil {
		return err
	}

	// Find route
	route, pathParams, err := h.router.FindRoute(r)
	if err != nil {
		return err
	}

	options := &Options{
		AuthenticationFunc: h.AuthenticationFunc,
	}

	var parserPool fastjson.ParserPool

	// Validate request
	requestValidationInput := &RequestValidationInput{
		RequestCtx: ctx,
		PathParams: pathParams,
		Route:      route,
		ParserJson: &parserPool,
		Options:    options,
	}
	if err = ValidateRequest(r.Context(), requestValidationInput); err != nil {
		return err
	}

	return nil
}
