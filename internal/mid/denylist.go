package mid

import (
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/denylist"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

// Denylist forbidden requests with tokens in the blacklist
func Denylist(cfg *config.APIFWConfiguration, deniedTokens *denylist.DeniedTokens, logger *logrus.Logger) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before web.Handler) web.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {

			// check existence and emptiness of the cache
			if deniedTokens != nil && deniedTokens.ElementsNum > 0 {
				//TODO: update getting token
				if cfg.Denylist.Tokens.CookieName != "" {
					token := string(ctx.Request.Header.Cookie(cfg.Denylist.Tokens.CookieName))
					_, found := deniedTokens.Cache.Get(token)
					if found {
						return web.RespondError(ctx, cfg.CustomBlockStatusCode, nil)
					}
				}
				if cfg.Denylist.Tokens.HeaderName != "" {
					token := string(ctx.Request.Header.Peek(cfg.Denylist.Tokens.HeaderName))
					if cfg.Denylist.Tokens.TrimBearerPrefix {
						token = strings.TrimPrefix(token, "Bearer ")
					}
					_, found := deniedTokens.Cache.Get(token)
					if found {
						return web.RespondError(ctx, cfg.CustomBlockStatusCode, nil)
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
