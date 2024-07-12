package api

import (
	"net/url"
	"os"
	"runtime/debug"
	"sync"

	"github.com/corazawaf/coraza/v3"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"

	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/mid"
	"github.com/wallarm/api-firewall/internal/platform/allowiplist"
	"github.com/wallarm/api-firewall/internal/platform/loader"
	"github.com/wallarm/api-firewall/internal/platform/storage"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

func Handlers(lock *sync.RWMutex, cfg *config.APIMode, shutdown chan os.Signal, logger *logrus.Logger, storedSpecs storage.DBOpenAPILoader, AllowedIPCache *allowiplist.AllowedIPsType, waf coraza.WAF) fasthttp.RequestHandler {

	// handle panic
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("panic: %v", r)

			// Log the Go stack trace for this panic'd goroutine.
			logger.Debugf("%s", debug.Stack())
			return
		}
	}()

	// define FastJSON parsers pool
	var parserPool fastjson.ParserPool
	schemaIDs := storedSpecs.SchemaIDs()

	ipAllowlistOptions := mid.IPAllowListOptions{
		Mode:                  web.APIMode,
		Config:                &cfg.AllowIP,
		CustomBlockStatusCode: fasthttp.StatusForbidden,
		AllowedIPs:            AllowedIPCache,
		Logger:                logger,
	}

	modSecOptions := mid.ModSecurityOptions{
		Mode:   web.APIMode,
		WAF:    waf,
		Logger: logger,
	}

	// Construct the App which holds all routes as well as common Middleware.
	apps := NewApp(lock, cfg.PassOptionsRequests, storedSpecs, shutdown, logger, mid.IPAllowlist(&ipAllowlistOptions), mid.WAFModSecurity(&modSecOptions), mid.Logger(logger), mid.MIMETypeIdentifier(logger), mid.Errors(logger), mid.Panics(logger))

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

		if serverURL.Path == "" {
			serverURL.Path = "/"
		}

		// get new router
		newSwagRouter, err := loader.NewRouterDBLoader(storedSpecs.SpecificationVersion(schemaID), storedSpecs.Specification(schemaID))
		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("New router creation failed")
		}

		for i := 0; i < len(newSwagRouter.Routes); i++ {

			s := RequestValidator{
				CustomRoute:   &newSwagRouter.Routes[i],
				Log:           logger,
				Cfg:           cfg,
				ParserPool:    &parserPool,
				OpenAPIRouter: newSwagRouter,
				SchemaID:      schemaID,
			}
			updRoutePathEsc, err := url.JoinPath(serverURL.Path, newSwagRouter.Routes[i].Path)
			if err != nil {
				s.Log.Errorf("url parse error: Schema ID %d: OpenAPI version %s: Loaded path %s - %v", schemaID, storedSpecs.SpecificationVersion(schemaID), newSwagRouter.Routes[i].Path, err)
				continue
			}

			updRoutePath, err := url.PathUnescape(updRoutePathEsc)
			if err != nil {
				s.Log.Errorf("url unescape error: Schema ID %d: OpenAPI version %s: Loaded path %s - %v", schemaID, storedSpecs.SpecificationVersion(schemaID), newSwagRouter.Routes[i].Path, err)
				continue
			}

			s.Log.Debugf("handler: Schema ID %d: OpenAPI version %s: Loaded path %s - %s", schemaID, storedSpecs.SpecificationVersion(schemaID), newSwagRouter.Routes[i].Method, updRoutePath)

			if err := apps.Handle(schemaID, newSwagRouter.Routes[i].Method, updRoutePath, s.Handler); err != nil {
				logger.WithFields(logrus.Fields{"error": err, "schema_id": schemaID}).Errorf("The OAS endpoint registration failed: method %s, path %s", newSwagRouter.Routes[i].Method, updRoutePath)
			}
		}

	}

	return apps.APIModeMainHandler
}
