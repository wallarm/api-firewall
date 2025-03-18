package router

// NewRouter returns a new Mux object that implements the Router interface.
func NewRouter() *Mux {
	return NewMux()
}

// Router consisting of the core routing methods used by chi's Mux,
// using only the standard net/http.
type Router interface {
	Routes

	// AddEndpoint adds routes for `pattern` that matches
	// the `method` HTTP method.
	AddEndpoint(method, pattern string, handler Handler) error
}

// Routes interface adds two methods for router traversal, which is also
// used by the `docgen` subpackage to generation documentation for Routers.
type Routes interface {
	// Find searches the routing tree for a handler that matches
	// the method/path - similar to routing a http request, but without
	// executing the handler thereafter.
	Find(rctx *Context, method, path string) Handler

	// FindWithActions searches the routing tree for a handler and actions that matches
	// the method/path - similar to routing a http request, but without
	// executing the handler thereafter.
	FindWithActions(rctx *Context, method, path string) (Handler, *Actions)
}
