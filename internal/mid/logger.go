package mid

import (
	"time"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"

	"github.com/wallarm/api-firewall/internal/platform/web"
)

// Logger writes some information about the request to the logs in the
// format: TraceID : (200) GET /foo -> IP ADDR (latency)
func Logger(logger *logrus.Logger) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before web.Handler) web.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {
			start := time.Now()

			err := before(ctx)

			logger.Infof("(%d) : #%016X : %s %s -> %s (%s)",
				ctx.Response.StatusCode(),
				ctx.ID(),
				ctx.Request.Header.Method(), ctx.Path(),
				ctx.RemoteAddr(), time.Since(start),
			)

			// Return the error so it can be handled further up the chain.
			return err
		}

		return h
	}

	return m
}
