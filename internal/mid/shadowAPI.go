package mid

import (
	"fmt"

	"golang.org/x/exp/slices"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

// ShadowAPIMonitor check each request for the params, methods or paths that are not specified
// in the OpenAPI specification and log each violation
func ShadowAPIMonitor(logger *logrus.Logger, config *config.ShadowAPI) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before web.Handler) web.Handler {

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
			idx := slices.IndexFunc(config.ExcludeList, func(c int) bool { return c == statusCode })
			// if response status code not found in the OpenAPI spec AND the code not in the exclude list
			if isProxyStatusCodeNotFound && idx < 0 {
				logger.WithFields(logrus.Fields{
					"request_id":      fmt.Sprintf("#%016X", ctx.ID()),
					"status_code":     ctx.Response.StatusCode(),
					"response_length": fmt.Sprintf("%d", ctx.Response.Header.ContentLength()),
					"method":          currentMethod,
					"path":            currentPath,
					"client_address":  ctx.RemoteAddr(),
					"violation":       "shadow_api",
				}).Error("Shadow API detected: response status code not found in the OpenAPI specification")
			}

			// Return the error, so it can be handled further up the chain.
			return err
		}

		return h
	}

	return m
}
