package APIMode

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/platform/loader"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/storage"
	"github.com/wallarm/api-firewall/pkg/APIMode/validator"
)

// wrapOASpecErrs wraps errors by the following high level errors ErrSpecValidation, ErrSpecParsing, ErrSpecLoading
func wrapOASpecErrs(err error) error {

	switch {
	case errors.Is(err, loader.ErrOASValidation):
		return fmt.Errorf("%w: %w", validator.ErrSpecValidation, err)
	case errors.Is(err, loader.ErrOASParsing):
		return fmt.Errorf("%w: %w", validator.ErrSpecParsing, err)
	}

	return fmt.Errorf("%w: %w", validator.ErrSpecLoading, err)
}

// getRouters function prepared router.Mux with the routes from OpenAPI specs
func getRouters(specStorage storage.DBOpenAPILoader, parserPool *fastjson.ParserPool, options *Configuration) (map[int]*router.Mux, error) {

	// Init routers
	routers := make(map[int]*router.Mux)
	for _, schemaID := range specStorage.SchemaIDs() {
		routers[schemaID] = router.NewRouter()

		serverURLStr := "/"
		spec := specStorage.Specification(schemaID)
		servers := spec.Servers
		if servers != nil {
			var err error
			if serverURLStr, err = servers.BasePath(); err != nil {
				return nil, fmt.Errorf("getting server URL from OpenAPI specification with ID %d: %w", schemaID, err)
			}
		}

		serverURL, err := url.Parse(serverURLStr)
		if err != nil {
			return nil, fmt.Errorf("parsing server URL from OpenAPI specification with ID %d: %w", schemaID, err)
		}

		if serverURL.Path == "" {
			serverURL.Path = "/"
		}

		// get new router
		newSwagRouter, err := loader.NewRouterDBLoader(specStorage.SpecificationVersion(schemaID), specStorage.Specification(schemaID))
		if err != nil {
			return nil, fmt.Errorf("new router creation failed for specification with ID %d: %w", schemaID, err)
		}

		for i := 0; i < len(newSwagRouter.Routes); i++ {

			s := RequestValidator{
				CustomRoute:   &newSwagRouter.Routes[i],
				ParserPool:    parserPool,
				OpenAPIRouter: newSwagRouter,
				SchemaID:      schemaID,
				Options:       options,
			}
			updRoutePathEsc, err := url.JoinPath(serverURL.Path, newSwagRouter.Routes[i].Path)
			if err != nil {
				return nil, fmt.Errorf("join path error for route %s in specification with ID %d: %w", newSwagRouter.Routes[i].Path, schemaID, err)
			}

			updRoutePath, err := url.PathUnescape(updRoutePathEsc)
			if err != nil {
				return nil, fmt.Errorf("path unescape error for route %s in specification with ID %d: %w", newSwagRouter.Routes[i].Path, schemaID, err)
			}

			if err := routers[schemaID].AddEndpoint(newSwagRouter.Routes[i].Method, updRoutePath, s.APIModeHandler); err != nil {
				return nil, fmt.Errorf("the OAS endpoint registration failed: method %s, path %s: %w", newSwagRouter.Routes[i].Method, updRoutePath, err)
			}
		}
	}

	return routers, nil
}
