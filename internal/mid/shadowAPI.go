package mid

import (
	"slices"

	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"

	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

// ShadowAPIMonitor check each request for the params, methods or paths that are not specified
// in the OpenAPI specification and log each violation
func ShadowAPIMonitor(logger zerolog.Logger, cfg *config.ShadowAPI) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before router.Handler) router.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {

			err := before(ctx)

			if isProxyFailedValue := ctx.Value(web.RequestProxyFailed); isProxyFailedValue != nil {
				if isProxyFailedValue.(bool) {
					return err
				}
			}

			// skip check if request has been blocked
			if isBlockedValue := ctx.Value(web.RequestBlocked); isBlockedValue != nil {
				if isBlockedValue.(bool) {
					return err
				}
			}

			currentMethod := string(ctx.Request.Header.Method())
			currentPath := string(ctx.Path())

			// get the response status code presence in the OpenAPI status
			isProxyStatusCodeNotFound := false
			statusCodeNotFoundValue := ctx.Value(web.ResponseStatusNotFound)
			if statusCodeNotFoundValue != nil {
				isProxyStatusCodeNotFound = statusCodeNotFoundValue.(bool)
			}

			// check response status code
			statusCode := ctx.Response.StatusCode()
			idx := slices.IndexFunc(cfg.ExcludeList, func(c int) bool { return c == statusCode })

			// if response status code not found in the OpenAPI spec AND the code not in the exclude list
			if isProxyStatusCodeNotFound && idx < 0 {
				logger.Error().
					Interface("request_id", ctx.UserValue(web.RequestID)).
					Int("status_code", ctx.Response.StatusCode()).
					Int("response_length", ctx.Response.Header.ContentLength()).
					Str("method", currentMethod).
					Str("path", currentPath).
					Str("client_address", ctx.RemoteAddr().String()).
					Str("violation", "shadow_api").
					Msg("Shadow API detected: response status code not found in the OpenAPI specification")
			}

			// Return the error, so it can be handled further up the chain.
			return err
		}

		return h
	}

	return m
}
