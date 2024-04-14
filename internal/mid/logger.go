package mid

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

// Logger writes some information about the request to the logs in the
// format: TraceID : (200) GET /foo -> IP ADDR (latency)
func Logger(logger *logrus.Logger) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before router.Handler) router.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {
			start := time.Now()

			logger.WithFields(logrus.Fields{
				"request_id":     ctx.UserValue(web.RequestID),
				"method":         string(ctx.Request.Header.Method()),
				"path":           string(ctx.Path()),
				"uri":            string(ctx.Request.URI().RequestURI()),
				"client_address": ctx.RemoteAddr(),
			}).Debug("Received request from client")

			err := before(ctx)

			// check method and path
			if isProxyNoRouteValue := ctx.Value(web.RequestProxyNoRoute); isProxyNoRouteValue != nil {
				if isProxyNoRouteValue.(bool) {
					logger.WithFields(logrus.Fields{
						"request_id":      ctx.UserValue(web.RequestID),
						"status_code":     ctx.Response.StatusCode(),
						"response_length": fmt.Sprintf("%d", ctx.Response.Header.ContentLength()),
						"method":          string(ctx.Request.Header.Method()),
						"path":            string(ctx.Path()),
						"uri":             string(ctx.Request.URI().RequestURI()),
						"client_address":  ctx.RemoteAddr(),
					}).Error("Method or path not found in the OpenAPI specification")
				}
			}

			logger.WithFields(logrus.Fields{
				"request_id":      ctx.UserValue(web.RequestID),
				"status_code":     ctx.Response.StatusCode(),
				"method":          string(ctx.Request.Header.Method()),
				"path":            string(ctx.Path()),
				"uri":             string(ctx.Request.URI().RequestURI()),
				"client_address":  ctx.RemoteAddr(),
				"processing_time": time.Since(start),
			}).Debug("Sending response to client")

			// log all information about the request
			web.LogRequestResponseAtTraceLevel(ctx, logger)

			// Return the error, so it can be handled further up the chain.
			return err
		}

		return h
	}

	return m
}
