package api

import (
	"runtime/debug"
	strconv2 "strconv"

	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"

	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/loader"
	"github.com/wallarm/api-firewall/internal/platform/metrics"
	apiMode "github.com/wallarm/api-firewall/internal/platform/validator"
	"github.com/wallarm/api-firewall/internal/platform/web"
	"github.com/wallarm/api-firewall/pkg/APIMode/validator"
)

type RequestValidator struct {
	CustomRoute   *loader.CustomRoute
	OpenAPIRouter *loader.Router
	Log           zerolog.Logger
	Cfg           *config.APIMode
	ParserPool    *fastjson.ParserPool
	Metrics       metrics.Metrics
	SchemaID      int
}

// Handler validates request and respond with 200, 403 (with error) or 500 status code
func (s *RequestValidator) Handler(ctx *fasthttp.RequestCtx) error {

	// handle panic
	defer func() {
		if r := recover(); r != nil {
			s.Log.Error().Msgf("panic: %v", r)

			// Log the Go stack trace for this panic'd goroutine.
			s.Log.Debug().Msgf("%s", debug.Stack())
			return
		}
	}()

	keyValidationErrors := strconv2.Itoa(s.SchemaID) + validator.APIModePostfixValidationErrors
	keyStatusCode := strconv2.Itoa(s.SchemaID) + validator.APIModePostfixStatusCode

	// Route not found
	if s.CustomRoute == nil {
		s.Log.Debug().
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Bytes("host", ctx.Request.Header.Host()).
			Bytes("path", ctx.Path()).
			Bytes("method", ctx.Request.Header.Method()).
			Msg("method or path were not found")

		ctx.SetUserValue(keyValidationErrors, []*validator.ValidationError{{Message: validator.ErrMethodAndPathNotFound.Error(), Code: validator.ErrCodeMethodAndPathNotFound, SchemaID: &s.SchemaID}})
		ctx.SetUserValue(keyStatusCode, fasthttp.StatusForbidden)
		return nil
	}

	validationErrors, err := apiMode.APIModeValidateRequest(ctx, s.Metrics, s.SchemaID, s.ParserPool, s.CustomRoute, s.Cfg.UnknownParametersDetection)
	if err != nil {
		s.Log.Error().
			Err(err).
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Bytes("host", ctx.Request.Header.Host()).
			Bytes("path", ctx.Path()).
			Bytes("method", ctx.Request.Header.Method()).
			Msg("request validation error")

		ctx.SetUserValue(keyStatusCode, fasthttp.StatusInternalServerError)
		return nil
	}

	// Respond 403 with errors
	if len(validationErrors) > 0 {
		// add schema IDs to the validation error messages
		for _, r := range validationErrors {
			r.SchemaID = &s.SchemaID
			r.SchemaVersion = s.OpenAPIRouter.SchemaVersion
		}

		s.Log.Debug().
			Interface("error", validationErrors).
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Bytes("host", ctx.Request.Header.Host()).
			Bytes("path", ctx.Path()).
			Bytes("method", ctx.Request.Header.Method()).
			Msg("request validation error")

		ctx.SetUserValue(keyValidationErrors, validationErrors)
		ctx.SetUserValue(keyStatusCode, fasthttp.StatusForbidden)
		return nil
	}

	// request successfully validated
	ctx.SetUserValue(keyStatusCode, fasthttp.StatusOK)
	return nil
}
