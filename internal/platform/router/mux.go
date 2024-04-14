package router

import (
	"fmt"
	"strings"
)

var _ Router = &Mux{}

// Mux is a simple fastHTTP route multiplexer that parses a request path,
// records any URL params, and searched for the appropriate web.Handler. It implements
// the web.Handler interface and is friendly with the standard library.
type Mux struct {
	// The radix trie router
	tree *node
}

// NewMux returns a newly initialized Mux object that implements the Router
// interface.
func NewMux() *Mux {
	mux := &Mux{tree: &node{}}
	return mux
}

// AddEndpoint adds the route `pattern` that matches `method` http method to
// execute the `handler` web.Handler.
func (mx *Mux) AddEndpoint(method, pattern string, handler Handler) error {
	m, ok := methodMap[strings.ToUpper(method)]
	if !ok {
		return fmt.Errorf("'%s' http method is not supported", method)
	}

	if _, err := mx.handle(m, pattern, handler); err != nil {
		return err
	}

	return nil
}

// Routes returns a slice of routing information from the tree,
// useful for traversing available routes of a router.
func (mx *Mux) Routes() []Route {
	return mx.tree.routes()
}

func (mx *Mux) Find(rctx *Context, method, path string) Handler {
	m, ok := methodMap[method]
	if !ok {
		return nil
	}

	node, _, h := mx.tree.FindRoute(rctx, m, path)

	if node != nil && node.subroutes != nil {
		rctx.RoutePath = mx.nextRoutePath(rctx)
		return node.subroutes.Find(rctx, method, rctx.RoutePath)
	}

	return h
}

// handle registers a web.Handler in the routing tree for a particular http method
// and routing pattern.
func (mx *Mux) handle(method methodTyp, pattern string, handler Handler) (*node, error) {
	if len(pattern) == 0 || pattern[0] != '/' {
		return nil, fmt.Errorf("routing pattern must begin with '/' in '%s'", pattern)
	}

	// Add the endpoint to the tree and return the node
	return mx.tree.InsertRoute(method, pattern, handler)
}

func (mx *Mux) nextRoutePath(rctx *Context) string {
	routePath := "/"
	nx := len(rctx.routeParams.Keys) - 1 // index of last param in list
	if nx >= 0 && rctx.routeParams.Keys[nx] == "*" && len(rctx.routeParams.Values) > nx {
		routePath = "/" + rctx.routeParams.Values[nx]
	}
	return routePath
}
