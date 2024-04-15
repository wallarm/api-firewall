//go:build go1.22
// +build go1.22

package router

import (
	"github.com/valyala/fasthttp"
)

// supportsPathValue is true if the Go version is 1.22 and above.
//
// If this is true, `net/http.Request` has methods `SetPathValue` and `PathValue`.
const supportsPathValue = true

// setPathValue sets the path values in the Request value
// based on the provided request context.
func setPathValue(rctx *Context, ctx *fasthttp.RequestCtx) {
	for i, key := range rctx.URLParams.Keys {
		value := rctx.URLParams.Values[i]
		ctx.SetUserValue(key, value)
	}
}
