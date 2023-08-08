package router

import (
	"context"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
	"github.com/wallarm/api-firewall/internal/platform/database"
)

// Router helps link http.Request.s and an OpenAPIv3 spec
type Router struct {
	Routes        []CustomRoute
	SchemaVersion string
}

type CustomRoute struct {
	Route                  *routers.Route
	Path                   string
	Method                 string
	ParametersNumberInPath int
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

			// count number of parameters in the path
			pathParamLength := 0
			if getOp := pathItem.GetOperation(route.Method); getOp != nil {
				for _, param := range getOp.Parameters {
					if param.Value.In == openapi3.ParameterInPath {
						pathParamLength += 1
					}
				}
			}

			// check common parameters
			if getOp := pathItem.Parameters; getOp != nil {
				for _, param := range getOp {
					if param.Value.In == openapi3.ParameterInPath {
						pathParamLength += 1
					}
				}
			}

			router.Routes = append(router.Routes, CustomRoute{
				Route:                  &route,
				Path:                   path,
				Method:                 method,
				ParametersNumberInPath: pathParamLength,
			})
		}
	}

	return &router, nil
}

// NewRouterDBLoader creates a new router based on DB OpenAPI loader.
func NewRouterDBLoader(schemaID int, openAPISpec database.DBOpenAPILoader) (*Router, error) {
	doc := openAPISpec.Specification(schemaID)

	router, err := NewRouter(doc)
	if err != nil {
		return nil, err
	}

	router.SchemaVersion = openAPISpec.SpecificationVersion(schemaID)

	return router, nil
}
