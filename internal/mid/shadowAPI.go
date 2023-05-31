package mid

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/shadowAPI"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

// ShadowAPIMonitor check each request for the params, methods or paths that are not specified
// in the OpenAPI specification and log each violation
func ShadowAPIMonitor(logger *logrus.Logger, checker shadowAPI.Checker) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before web.Handler) web.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {

			err := before(ctx)

			if isProxyFailedValue := ctx.Value("proxy_failed"); isProxyFailedValue != nil {
				if isProxyFailedValue.(bool) {
					return err
				}
			}

			// skip check if request has been blocked
			if isBlockedValue := ctx.Value("blocked"); isBlockedValue != nil {
				if isBlockedValue.(bool) {
					return err
				}
			}

			if err := checker.Check(ctx); err != nil {
				logger.WithFields(logrus.Fields{
					"error":      err,
					"request_id": fmt.Sprintf("#%016X", ctx.ID()),
				}).Error("Shadow API check error")
			}

			// Return the error, so it can be handled further up the chain.
			return err
		}

		return h
	}

	return m
}
