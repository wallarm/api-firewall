package mid

import (
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"

	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

// Errors handles errors coming out of the call chain. It detects normal
// application errors which are used to respond to the client in a uniform way.
// Unexpected errors (status >= 500) are logged.
func Errors(logger zerolog.Logger) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before router.Handler) router.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {

			// Run the handler chain and catch any propagated error.
			if err := before(ctx); err != nil {

				// Log the error.
				logger.Error().Err(err).
					Interface("request_id", ctx.UserValue(web.RequestID)).
					Bytes("host", ctx.Request.Header.Host()).
					Bytes("path", ctx.Path()).
					Bytes("method", ctx.Request.Header.Method()).
					Msg("common error")

				// Respond to the error.
				if err := web.RespondError(ctx, fasthttp.StatusInternalServerError, ""); err != nil {
					return err
				}

				// If we receive the shutdown err we need to return it
				// back to the base handler to shutdown the service.
				if ok := web.IsShutdown(err); ok {
					return err
				}
			}

			// The error has been handled so we can stop propagating it.
			return nil
		}

		return h
	}

	return m
}
