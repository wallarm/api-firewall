package api

import (
	"github.com/rs/zerolog"
	"net/url"
	"os"
	"runtime/debug"
	"sync"

	"github.com/corazawaf/coraza/v3"
	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"

	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/mid"
	"github.com/wallarm/api-firewall/internal/platform/allowiplist"
	"github.com/wallarm/api-firewall/internal/platform/loader"
	"github.com/wallarm/api-firewall/internal/platform/storage"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

func Handlers(lock *sync.RWMutex, cfg *config.APIMode, shutdown chan os.Signal, logger zerolog.Logger, storedSpecs storage.DBOpenAPILoader, AllowedIPCache *allowiplist.AllowedIPsType, waf coraza.WAF) fasthttp.RequestHandler {

	// handle panic
	defer func() {
		if r := recover(); r != nil {
			log.Error().Msgf("panic: %v", r)

			// Log the Go stack trace for this panic'd goroutine.
			log.Debug().Msgf("%s", debug.Stack())
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
				log.Error().Msgf("getting server URL from OpenAPI specification: %v", err)
			}
		}

		serverURL, err := url.Parse(serverURLStr)
		if err != nil {
			log.Error().Msgf("parsing server URL from OpenAPI specification: %v", err)
		}

		if serverURL.Path == "" {
			serverURL.Path = "/"
		}

		// get new router
		newSwagRouter, err := loader.NewRouterDBLoader(storedSpecs.SpecificationVersion(schemaID), storedSpecs.Specification(schemaID))
		if err != nil {
			log.Fatal().Err(err).Msg("new router creation failed")
		}

		for i := 0; i < len(newSwagRouter.Routes); i++ {

			s := RequestValidator{
				CustomRoute:   &newSwagRouter.Routes[i],
				Cfg:           cfg,
				ParserPool:    &parserPool,
				OpenAPIRouter: newSwagRouter,
				SchemaID:      schemaID,
			}
			updRoutePathEsc, err := url.JoinPath(serverURL.Path, newSwagRouter.Routes[i].Path)
			if err != nil {
				log.Error().Msgf("url parse error: Schema ID %d: openAPI version %s: loaded path %s - %v", schemaID, storedSpecs.SpecificationVersion(schemaID), newSwagRouter.Routes[i].Path, err)
				continue
			}

			updRoutePath, err := url.PathUnescape(updRoutePathEsc)
			if err != nil {
				log.Error().Msgf("url unescape error: schema ID %d: openAPI version %s: loaded path %s - %v", schemaID, storedSpecs.SpecificationVersion(schemaID), newSwagRouter.Routes[i].Path, err)
				continue
			}

			log.Debug().Msgf("handler: schema ID %d: openAPI version %s: loaded path %s - %s", schemaID, storedSpecs.SpecificationVersion(schemaID), newSwagRouter.Routes[i].Method, updRoutePath)

			if err := apps.Handle(schemaID, newSwagRouter.Routes[i].Method, updRoutePath, s.Handler); err != nil {
				log.Error().Err(err).
					Int("schema_id", schemaID).
					Msgf("the OAS endpoint registration failed: method %s, path %s", newSwagRouter.Routes[i].Method, updRoutePath)
			}
		}

	}

	return apps.APIModeMainHandler
}
