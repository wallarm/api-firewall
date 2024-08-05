package proxy

import (
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

// Perform function proxies the request to the backend server
func Perform(ctx *fasthttp.RequestCtx, proxyPool Pool, customHostHeader string) error {

	client, ip, err := proxyPool.Get()
	if err != nil {
		return err
	}
	defer proxyPool.Put(ip, client)

	if customHostHeader != "" {
		ctx.Request.Header.SetHost(customHostHeader)
		ctx.Request.URI().SetHost(customHostHeader)
	}

	if err := client.Do(&ctx.Request, &ctx.Response); err != nil {
		// request proxy has been failed
		ctx.SetUserValue(web.RequestProxyFailed, true)

		switch err {
		case fasthttp.ErrDialTimeout:
			if err := web.RespondError(ctx, fasthttp.StatusGatewayTimeout, ""); err != nil {
				return err
			}
		case fasthttp.ErrNoFreeConns:
			if err := web.RespondError(ctx, fasthttp.StatusServiceUnavailable, ""); err != nil {
				return err
			}
		default:
			if err := web.RespondError(ctx, fasthttp.StatusBadGateway, ""); err != nil {
				return err
			}
		}

		// The error has been handled so we can stop propagating it
		return err
	}

	return nil
}
