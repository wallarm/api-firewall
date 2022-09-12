package router

import (
	"context"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
)

// Router helps link http.Request.s and an OpenAPIv3 spec
type Router struct {
	Routes []Route
}

type Route struct {
	Route  *routers.Route
	Path   string
	Method string
}

// NewRouter creates a new router.
//
// If the given Swagger has servers, router will use them.
// All operations of the Swagger will be added to the router.
func NewRouter(doc *openapi3.T) (*Router, error) {
	if err := doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("validating OpenAPI failed: %v", err)
	}
	var router Router

	for path, pathItem := range doc.Paths {
		for method, operation := range pathItem.Operations() {
			method = strings.ToUpper(method)
			route := routers.Route{
				Spec:      doc,
				Path:      path,
				PathItem:  pathItem,
				Method:    method,
				Operation: operation,
			}
			router.Routes = append(router.Routes, Route{
				Route:  &route,
				Path:   path,
				Method: method,
			})
		}
	}
	return &router, nil
}
