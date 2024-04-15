package mid

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

const apifwHeaderName = "APIFW-Request-Id"

// Hop-by-hop headers. These are removed when sent to the backend.
// As of RFC 7230, hop-by-hop headers are required to appear in the
// Connection header field. These are the headers defined by the
// obsoleted RFC 2616 (section 13.5.1) and are used for backward
// compatibility.
var (
	hopHeaders = []string{
		"Connection",          // Connection
		"Proxy-Connection",    // non-standard but still sent by libcurl and rejected by e.g. google
		"Keep-Alive",          // Keep-Alive
		"Proxy-Authenticate",  // Proxy-Authenticate
		"Proxy-Authorization", // Proxy-Authorization
		"Te",                  // canonicalized version of "TE"
		"Trailer",             // not Trailers per URL above; https://www.rfc-editor.org/errata_search.php?eid=4522
		"Transfer-Encoding",   // Transfer-Encoding
		"Upgrade",             // Upgrade
	}
	acHeader = http.CanonicalHeaderKey("Accept-Encoding")
)

type ProxyOptions struct {
	Mode                 string
	RequestValidation    string
	DeleteAcceptEncoding bool
	ServerURL            *url.URL
}

// Proxy changes request scheme before request
func Proxy(options *ProxyOptions) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before router.Handler) router.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {

			for _, h := range hopHeaders {

				if options.Mode == web.GraphQLMode {
					// skip (not delete) ws required headers
					if h == "Connection" && ctx.Request.Header.ConnectionUpgrade() {
						continue
					}
					if h == "Upgrade" && strings.EqualFold(string(ctx.Request.Header.Peek("Upgrade")), "websocket") {
						continue
					}
				}
				ctx.Request.Header.Del(h)
			}

			if strings.EqualFold(options.RequestValidation, web.ValidationBlock) {
				// add apifw header to the request
				ctx.Request.Header.Add(apifwHeaderName, ctx.UserValue(web.RequestID).(string))
			}

			if !bytes.Equal([]byte(options.ServerURL.Scheme), ctx.Request.URI().Scheme()) {
				ctx.Request.URI().SetSchemeBytes([]byte(options.ServerURL.Scheme))
			}

			if !bytes.Equal([]byte(options.ServerURL.Host), ctx.Request.URI().Host()) {
				ctx.Request.URI().SetHostBytes([]byte(options.ServerURL.Host))
			}

			// update or set x-forwarded-for header
			switch xffValueb := ctx.Request.Header.Peek("X-Forwarded-For"); {
			case xffValueb != nil:
				ctx.Request.Header.Set("X-Forwarded-For",
					fmt.Sprintf("%s, %s", strconv.B2S(xffValueb), ctx.RemoteIP().String()),
				)
			default:
				ctx.Request.Header.Set("X-Forwarded-For", ctx.RemoteIP().String())
			}

			// delete Accept-Encoding header
			if options.DeleteAcceptEncoding {
				ctx.Request.Header.Del(acHeader)
			}

			err := before(ctx)

			for _, h := range hopHeaders {

				if options.Mode == web.GraphQLMode {
					// skip (not delete) ws required headers
					if h == "Connection" && ctx.Response.Header.ConnectionUpgrade() {
						continue
					}
					if h == "Upgrade" && strings.EqualFold(string(ctx.Response.Header.Peek("Upgrade")), "websocket") {
						continue
					}
				}
				ctx.Response.Header.Del(h)
			}

			// Return the error, so it can be handled further up the chain.
			return err
		}

		return h
	}

	return m
}
