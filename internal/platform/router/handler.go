package router

import "github.com/valyala/fasthttp"

// A Handler is a type that handles an http request within our own little mini
// framework.
type Handler func(ctx *fasthttp.RequestCtx) error
