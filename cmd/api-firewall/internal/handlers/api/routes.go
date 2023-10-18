package api

import (
	"net/url"
	"os"
	"path"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/mid"
	"github.com/wallarm/api-firewall/internal/platform/database"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

func Handlers(lock *sync.RWMutex, cfg *config.APIMode, shutdown chan os.Signal, logger *logrus.Logger, storedSpecs database.DBOpenAPILoader) fasthttp.RequestHandler {

	// define FastJSON parsers pool
	var parserPool fastjson.ParserPool
	schemaIDs := storedSpecs.SchemaIDs()

	// Construct the web.App which holds all routes as well as common Middleware.
	apps := web.NewAPIModeApp(lock, cfg.PassOptionsRequests, storedSpecs, shutdown, logger, mid.Logger(logger), mid.MIMETypeIdentifier(logger), mid.Errors(logger), mid.Panics(logger))

	for _, schemaID := range schemaIDs {

		serverURLStr := "/"
		spec := storedSpecs.Specification(schemaID)
		servers := spec.Servers
		if servers != nil {
			var err error
			if serverURLStr, err = servers.BasePath(); err != nil {
				logger.Errorf("getting server URL from OpenAPI specification: %v", err)
			}
		}

		serverURL, err := url.Parse(serverURLStr)
		if err != nil {
			logger.Errorf("parsing server URL from OpenAPI specification: %v", err)
		}

		// get new router
		newSwagRouter, err := router.NewRouterDBLoader(schemaID, storedSpecs)
		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("new router creation failed")
		}

		for i := 0; i < len(newSwagRouter.Routes); i++ {

			s := APIMode{
				CustomRoute:   &newSwagRouter.Routes[i],
				Log:           logger,
				Cfg:           cfg,
				ParserPool:    &parserPool,
				OpenAPIRouter: newSwagRouter,
				SchemaID:      schemaID,
			}
			updRoutePath := path.Join(serverURL.Path, newSwagRouter.Routes[i].Path)

			s.Log.Debugf("handler: Schema ID %d: OpenAPI version %s: Loaded path %s - %s", schemaID, storedSpecs.SpecificationVersion(schemaID), newSwagRouter.Routes[i].Method, updRoutePath)

			apps.Handle(schemaID, newSwagRouter.Routes[i].Method, updRoutePath, s.APIModeHandler)
		}

		//set handler for default behavior (404, 405)
		s := APIMode{
			CustomRoute:   nil,
			Log:           logger,
			Cfg:           cfg,
			ParserPool:    &parserPool,
			OpenAPIRouter: newSwagRouter,
			SchemaID:      schemaID,
		}
		apps.SetDefaultBehavior(schemaID, s.APIModeHandler)
	}

	return apps.APIModeHandler
}
