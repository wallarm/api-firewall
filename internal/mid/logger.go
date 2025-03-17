package mid

import (
	"time"

	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

// Logger writes some information about the request to the logs in the
// format: TraceID : (200) GET /foo -> IP ADDR (latency)
func Logger(logger zerolog.Logger) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before router.Handler) router.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {
			start := time.Now()

			logger.Debug().
				Interface("request_id", ctx.UserValue(web.RequestID)).
				Bytes("uri", ctx.Request.URI().RequestURI()).
				Bytes("path", ctx.Path()).
				Bytes("method", ctx.Request.Header.Method()).
				Str("client_address", ctx.RemoteAddr().String()).
				Msg("request received")

			err := before(ctx)

			// check method and path
			if isProxyNoRouteValue := ctx.Value(web.RequestProxyNoRoute); isProxyNoRouteValue != nil {
				if isProxyNoRouteValue.(bool) {
					logger.Error().
						Interface("request_id", ctx.UserValue(web.RequestID)).
						Int("status_code", ctx.Response.StatusCode()).
						Int("response_length", ctx.Response.Header.ContentLength()).
						Bytes("method", ctx.Request.Header.Method()).
						Bytes("path", ctx.Path()).
						Bytes("uri", ctx.Request.URI().RequestURI()).
						Str("client_address", ctx.RemoteAddr().String()).
						Msg("method or path not found in the OpenAPI specification")
				}
			}

			logger.Debug().
				Interface("request_id", ctx.UserValue(web.RequestID)).
				Int("status_code", ctx.Response.StatusCode()).
				Bytes("method", ctx.Request.Header.Method()).
				Bytes("path", ctx.Path()).
				Bytes("uri", ctx.Request.URI().RequestURI()).
				Str("client_address", ctx.RemoteAddr().String()).
				Str("processing_time", time.Since(start).String()).
				Msg("request processed")

			// log all information about the request
			web.LogRequestResponseAtTraceLevel(ctx, logger)

			// Return the error, so it can be handled further up the chain.
			return err
		}

		return h
	}

	return m
}
