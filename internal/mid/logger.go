package mid

import (
	"fmt"
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

			logger.WithFields(logrus.Fields{
				"request_id":      fmt.Sprintf("#%016X", ctx.ID()),
				"status_code":     ctx.Response.StatusCode(),
				"method":          fmt.Sprintf("%s", ctx.Request.Header.Method()),
				"path":            fmt.Sprintf("%s", ctx.Path()),
				"client_address":  ctx.RemoteAddr(),
				"processing_time": time.Since(start),
			}).Debug("new request")

			// Return the error, so it can be handled further up the chain.
			return err
		}

		return h
	}

	return m
}
