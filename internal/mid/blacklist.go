package mid

import (
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/blacklist"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

// Blacklist forbidden requests with tokens in the blacklist
func Blacklist(cfg *config.APIFWConfiguration, blacklistedTokens *blacklist.BlacklistedTokens, logger *logrus.Logger) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before web.Handler) web.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {

			// check existence and emptiness of the cache
			if blacklistedTokens != nil && blacklistedTokens.ElementsNum > 0 {
				//TODO: update getting token
				if cfg.Blacklist.Tokens.CookieName != "" {
					token := string(ctx.Request.Header.Cookie(cfg.Blacklist.Tokens.CookieName))
					_, found := blacklistedTokens.Cache.Get(token)
					if found {
						return web.RespondError(ctx, cfg.CustomBlockStatusCode, nil)
					}
				}
				if cfg.Blacklist.Tokens.HeaderName != "" {
					token := string(ctx.Request.Header.Peek(cfg.Blacklist.Tokens.HeaderName))
					if cfg.Blacklist.Tokens.TrimBearerPrefix {
						token = strings.TrimPrefix(token, "Bearer ")
					}
					_, found := blacklistedTokens.Cache.Get(token)
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
