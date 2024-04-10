package mid

import (
	"errors"
	"net"
	"strings"

	"github.com/savsgio/gotils/strconv"
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

// The IPAllowlist function checks if an IP is allowed else gives error
func IPAllowlist(options *IPAllowListOptions) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before web.Handler) web.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {

			if options.AllowedIPs != nil && options.AllowedIPs.ElementsNum > 0 {

				// get header or remote addr
				var ipToCheck string

				switch strings.ToLower(options.Config.HeaderName) {
				case "":
					addr := ctx.RemoteAddr()
					ipAddr, ok := addr.(*net.TCPAddr)
					if !ok {
						options.Logger.WithFields(logrus.Fields{
							"request_id": ctx.UserValue(web.RequestID),
							"host":       string(ctx.Request.Header.Host()),
							"path":       string(ctx.Path()),
						}).Error("allow IP: can't get client IP address")
						break
					}
					ipToCheck = ipAddr.IP.String()
				case "x-forwarded-for":
					ipToCheck = strconv.B2S(ctx.Request.Header.Peek(options.Config.HeaderName))
					ipToCheck = strings.Split(ipToCheck, ",")[0]
				default:
					ipToCheck = strconv.B2S(ctx.Request.Header.Peek(options.Config.HeaderName))
				}

				ipToCheck = strings.TrimSpace(ipToCheck)

				ip := net.ParseIP(ipToCheck)
				if ip == nil {
					options.Logger.WithFields(logrus.Fields{
						"request_id":        ctx.UserValue(web.RequestID),
						"host":              string(ctx.Request.Header.Host()),
						"path":              string(ctx.Path()),
						"source_ip_address": ipToCheck,
					}).Info("allow IP: could not parse source IP address")

					switch options.Mode {
					case web.APIMode:
						ctx.SetUserValue(web.GlobalResponseStatusCodeKey, options.CustomBlockStatusCode)
						return nil
					case web.GraphQLMode:
						ctx.Response.SetStatusCode(options.CustomBlockStatusCode)
						return web.RespondGraphQLErrors(&ctx.Response, errAccessDeniedIP)
					}
					return web.RespondError(ctx, options.CustomBlockStatusCode, "")
				}

				if _, found := options.AllowedIPs.Cache.Get(ip.String()); !found {
					options.Logger.WithFields(logrus.Fields{
						"request_id":        ctx.UserValue(web.RequestID),
						"host":              string(ctx.Request.Header.Host()),
						"path":              string(ctx.Path()),
						"source_ip_address": ipToCheck,
					}).Info("allow IP: requests from the source IP address are not allowed")

					switch options.Mode {
					case web.APIMode:
						ctx.SetUserValue(web.GlobalResponseStatusCodeKey, options.CustomBlockStatusCode)
						return nil
					case web.GraphQLMode:
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
