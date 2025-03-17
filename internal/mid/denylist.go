package mid

import (
	"errors"
	"strings"

	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"

	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/denylist"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

type DenylistOptions struct {
	Mode                  string
	Config                *config.Denylist
	CustomBlockStatusCode int
	DeniedTokens          *denylist.DeniedTokens
	Logger                zerolog.Logger
}

var errAccessDenied = errors.New("access denied")

// Denylist forbidden requests with tokens in the blacklist
func Denylist(options *DenylistOptions) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before router.Handler) router.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {

			// check existence and emptiness of the cache
			if options.DeniedTokens != nil && options.DeniedTokens.ElementsNum > 0 {
				if options.Config.Tokens.CookieName != "" {
					token := string(ctx.Request.Header.Cookie(options.Config.Tokens.CookieName))
					if _, found := options.DeniedTokens.Cache.Get(token); found {
						options.Logger.Info().
							Interface("request_id", ctx.UserValue(web.RequestID)).
							Bytes("host", ctx.Request.Header.Host()).
							Bytes("path", ctx.Path()).
							Bytes("method", ctx.Request.Header.Method()).
							Str("token", token).
							Msg("The request with the API token has been blocked")

						if strings.EqualFold(options.Mode, web.GraphQLMode) {
							ctx.Response.SetStatusCode(options.CustomBlockStatusCode)
							return web.RespondGraphQLErrors(&ctx.Response, errAccessDenied)
						}
						return web.RespondError(ctx, options.CustomBlockStatusCode, "")
					}
				}
				if options.Config.Tokens.HeaderName != "" {
					token := string(ctx.Request.Header.Peek(options.Config.Tokens.HeaderName))
					if options.Config.Tokens.TrimBearerPrefix {
						token = strings.TrimPrefix(token, "Bearer ")
					}
					if _, found := options.DeniedTokens.Cache.Get(token); found {

						options.Logger.Info().
							Interface("request_id", ctx.UserValue(web.RequestID)).
							Bytes("host", ctx.Request.Header.Host()).
							Bytes("path", ctx.Path()).
							Bytes("method", ctx.Request.Header.Method()).
							Str("token", token).
							Msg("The request with the API token has been blocked")

						if strings.EqualFold(options.Mode, web.GraphQLMode) {
							ctx.Response.SetStatusCode(options.CustomBlockStatusCode)
							return web.RespondGraphQLErrors(&ctx.Response, errAccessDenied)
						}
						return web.RespondError(ctx, options.CustomBlockStatusCode, "")
					}
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
