package router

import (
	"strings"

	"github.com/valyala/fasthttp"
)

// URLParam returns the url parameter from a fasthttp.Request object.
func URLParam(ctx *fasthttp.RequestCtx, key string) string {
	if rctx := RouteContext(ctx); rctx != nil {
		return rctx.URLParam(key)
	}
	return ""
}

// AllURLParams returns the map of the url parameters from a fasthttp.Request object.
func AllURLParams(ctx *fasthttp.RequestCtx) map[string]string {
	if rctx := RouteContext(ctx); rctx != nil {
		params := make(map[string]string)
		for i := range rctx.URLParams.Keys {
			params[rctx.URLParams.Keys[i]] = rctx.URLParams.Values[i]
		}
		return params
	}

	return nil
}

// RouteContext returns chi's routing Context object from a
// http.Request Context.
func RouteContext(ctx *fasthttp.RequestCtx) *Context {
	val, _ := ctx.Value(RouteCtxKey).(*Context)
	return val
}

// NewRouteContext returns a new routing Context object.
func NewRouteContext() *Context {
	return &Context{}
}

var (
	// RouteCtxKey is the context.Context key to store the request context.
	RouteCtxKey = &contextKey{"RouteContext"}
)

// Context is the default routing context set on the root node of a
// request context to track route patterns, URL parameters and
// an optional routing path.
type Context struct {
	Routes Routes

	// Routing path/method override used during the route search.
	// See Mux#routeHTTP method.
	RoutePath   string
	RouteMethod string

	// URLParams are the stack of routeParams captured during the
	// routing lifecycle across a stack of sub-routers.
	URLParams RouteParams

	// Route parameters matched for the current sub-router. It is
	// intentionally unexported so it can't be tampered.
	routeParams RouteParams

	// The endpoint routing pattern that matched the request URI path
	// or `RoutePath` of the current sub-router. This value will update
	// during the lifecycle of a request passing through a stack of
	// sub-routers.
	routePattern string

	// Routing pattern stack throughout the lifecycle of the request,
	// across all connected routers. It is a record of all matching
	// patterns across a stack of sub-routers.
	RoutePatterns []string

	// methodNotAllowed hint
	methodNotAllowed bool
	methodsAllowed   []methodTyp // allowed methods in case of a 405
}

// Reset a routing context to its initial state.
func (x *Context) Reset() {
	x.Routes = nil
	x.RoutePath = ""
	x.RouteMethod = ""
	x.RoutePatterns = x.RoutePatterns[:0]
	x.URLParams.Keys = x.URLParams.Keys[:0]
	x.URLParams.Values = x.URLParams.Values[:0]

	x.routePattern = ""
	x.routeParams.Keys = x.routeParams.Keys[:0]
	x.routeParams.Values = x.routeParams.Values[:0]
	x.methodNotAllowed = false
	x.methodsAllowed = x.methodsAllowed[:0]
}

// URLParam returns the corresponding URL parameter value from the request
// routing context.
func (x *Context) URLParam(key string) string {
	for k := len(x.URLParams.Keys) - 1; k >= 0; k-- {
		if x.URLParams.Keys[k] == key {
			return x.URLParams.Values[k]
		}
	}
	return ""
}

// RoutePattern builds the routing pattern string for the particular
// request, at the particular point during routing. This means, the value
// will change throughout the execution of a request in a router. That is
// why its advised to only use this value after calling the next handler.
//
// For example,
//
//	func Instrument(next web.Handler) web.Handler {
//		return web.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//			next.ServeHTTP(w, r)
//			routePattern := chi.RouteContext(r.Context()).RoutePattern()
//			measure(w, r, routePattern)
//		})
//	}
func (x *Context) RoutePattern() string {
	routePattern := strings.Join(x.RoutePatterns, "")
	routePattern = replaceWildcards(routePattern)
	if routePattern != "/" {
		routePattern = strings.TrimSuffix(routePattern, "//")
		routePattern = strings.TrimSuffix(routePattern, "/")
	}
	return routePattern
}

// replaceWildcards takes a route pattern and recursively replaces all
// occurrences of "/*/" to "/".
func replaceWildcards(p string) string {
	if strings.Contains(p, "/*/") {
		return replaceWildcards(strings.Replace(p, "/*/", "/", -1))
	}
	return p
}

// RouteParams is a structure to track URL routing parameters efficiently.
type RouteParams struct {
	Keys, Values []string
}

// Add will append a URL parameter to the end of the route param
func (s *RouteParams) Add(key, value string) {
	s.Keys = append(s.Keys, key)
	s.Values = append(s.Values, value)
}

// contextKey is a value for use with context.WithValue. It's used as
// a pointer so it fits in an any without allocation. This technique
// for defining context keys was copied from Go 1.7's new use of context in net/http.
type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return "chi context value " + k.name
}
