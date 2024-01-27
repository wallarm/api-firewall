package mid

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/allowiplist"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

type IPAllowListOptions struct {
	Mode                  string
	Config                *config.AllowIP
	CustomBlockStatusCode int
	AllowedIPs            *allowiplist.AllowedIPsType
	Logger                *logrus.Logger
}

var errAccessDeniedIP = errors.New("access denied to this IP")

// This function checks if an IP is allowed else gives error.
func IPAllowlist(options *IPAllowListOptions) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before web.Handler) web.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {
			addr := ctx.RemoteAddr()
			ipAddr, _ := addr.(*net.TCPAddr)
			ipToCheck := ipAddr.IP.String()
			if options.Config.HeaderName != "" {
				ipFromHeader := string(ctx.Request.Header.Peek(options.Config.HeaderName))
				if ipFromHeader != "" {
					ipToCheck = ipFromHeader
				}

			}

			ipToCheck = strings.TrimSpace(ipToCheck)

			if options.AllowedIPs != nil && options.AllowedIPs.ElementsNum > 0 {
				_, presentbool := options.AllowedIPs.Cache.Get(ipToCheck)

				if !presentbool {
					options.Logger.WithFields(logrus.Fields{
						"request_id": fmt.Sprintf("#%016X", ctx.ID()),
						"IP Address": ipToCheck,
					}).Info("the request with the IP has been blocked")
					if strings.EqualFold(options.Mode, web.GraphQLMode) {
						ctx.Response.SetStatusCode(options.CustomBlockStatusCode)
						return web.RespondGraphQLErrors(&ctx.Response, errAccessDeniedIP)
					}
					return web.RespondError(ctx, options.CustomBlockStatusCode, "")
				}

			}

			err := before(ctx)

			// Return the error, so it can be handled further up the chain.
			return err
		}

		return h
	}

	return m
}
