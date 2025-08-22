package api

import (
	"net/url"
	"os"
	"runtime/debug"
	"sync"

	"github.com/corazawaf/coraza/v3"
	"github.com/pb33f/libopenapi"
	oasValidator "github.com/pb33f/libopenapi-validator"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/mid"
	"github.com/wallarm/api-firewall/internal/platform/allowiplist"
	"github.com/wallarm/api-firewall/internal/platform/metrics"
	"github.com/wallarm/api-firewall/internal/platform/storage"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

func Handlers(lock *sync.RWMutex, cfg *config.APIMode, shutdown chan os.Signal, logger zerolog.Logger, metrics metrics.Metrics, storedSpecs storage.DBOpenAPILoader, AllowedIPCache *allowiplist.AllowedIPsType, waf coraza.WAF) fasthttp.RequestHandler {

	// handle panic
	defer func() {
		if r := recover(); r != nil {
			logger.Error().Msgf("panic: %v", r)

			// Log the Go stack trace for this panic'd goroutine.
			logger.Debug().Msgf("%s", debug.Stack())
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
	apps := NewApp(lock, cfg.PassOptionsRequests, cfg.MaxErrorsInResponse, storedSpecs, shutdown, logger, metrics, mid.IPAllowlist(&ipAllowlistOptions), mid.WAFModSecurity(&modSecOptions), mid.Logger(logger), mid.MIMETypeIdentifier(logger), mid.Errors(logger), mid.Panics(logger))

	for _, schemaID := range schemaIDs {

		// todo: fix
		spec := storedSpecs.Specification(schemaID).(libopenapi.Document)
		serverURLStr := spec.GetConfiguration().BasePath
		if serverURLStr == "" {
			serverURLStr = "/"
		}

		serverURL, err := url.Parse(serverURLStr)
		if err != nil {
			logger.Error().Msgf("parsing server URL from OpenAPI specification: %v", err)
		}

		if serverURL.Path == "" {
			serverURL.Path = "/"
		}

		// get new router
		//newSwagRouter, err := loader.NewRouterDBLoader(storedSpecs.SpecificationVersion(schemaID), storedSpecs.Specification(schemaID))
		//if err != nil {
		//	logger.Fatal().Err(err).Msg("new router creation failed")
		//}

		highLevelValidator, validatorErrs := oasValidator.NewValidator(spec)
		if len(validatorErrs) > 0 {
			logger.Error().Msgf("error validator init: %v", validatorErrs)
		}

		//for i := 0; i < len(newSwagRouter.Routes); i++ {
		//
		//	s := RequestValidator{
		//		//CustomRoute:   &newSwagRouter.Routes[i],
		//		Cfg:           cfg,
		//		ParserPool:    &parserPool,
		//		//OpenAPIRouter: newSwagRouter,
		//		SchemaID:      schemaID,
		//		Metrics:       metrics,
		//		OASValidator:  highLevelValidator,
		//	}
		//	updRoutePathEsc, err := url.JoinPath(serverURL.Path, newSwagRouter.Routes[i].Path)
		//	if err != nil {
		//		logger.Error().Msgf("url parse error: Schema ID %d: openAPI version %s: loaded path %s - %v", schemaID, storedSpecs.SpecificationVersion(schemaID), newSwagRouter.Routes[i].Path, err)
		//		continue
		//	}
		//
		//	updRoutePath, err := url.PathUnescape(updRoutePathEsc)
		//	if err != nil {
		//		logger.Error().Msgf("url unescape error: schema ID %d: openAPI version %s: loaded path %s - %v", schemaID, storedSpecs.SpecificationVersion(schemaID), newSwagRouter.Routes[i].Path, err)
		//		continue
		//	}
		//
		//	logger.Debug().Msgf("handler: schema ID %d: openAPI version %s: loaded path %s - %s", schemaID, storedSpecs.SpecificationVersion(schemaID), newSwagRouter.Routes[i].Method, updRoutePath)
		//
		//	if err := apps.Handle(schemaID, newSwagRouter.Routes[i].Method, updRoutePath, s.Handler); err != nil {
		//		logger.Error().Err(err).
		//			Int("schema_id", schemaID).
		//			Msgf("the OAS endpoint registration failed: method %s, path %s", newSwagRouter.Routes[i].Method, updRoutePath)
		//	}
		//}

		model, modelErr := spec.BuildV3Model()
		if err != nil {
			logger.Error().Msgf("error building model: %v", modelErr)
			return nil
		}

		oas := model.Model // *v3.Document

		// Итерируемся по путям в стабильном порядке (порядок вставки)
		for path, pathItem := range oas.Paths.PathItems.FromOldest() {
			//fmt.Println("PATH:", path)

			// Перебор операций у pathItem.
			// Удобно воспользоваться коллекцией операций:

			s := RequestValidator{
				//CustomRoute:   &newSwagRouter.Routes[i],
				Cfg:        cfg,
				ParserPool: &parserPool,
				//OpenAPIRouter: newSwagRouter,
				SchemaID:     schemaID,
				Metrics:      metrics,
				OASValidator: highLevelValidator,
			}
			updRoutePathEsc, err := url.JoinPath(serverURL.Path, path)
			if err != nil {
				logger.Error().Msgf("url parse error: Schema ID %d: openAPI version %s: loaded path %s - %v", schemaID, storedSpecs.SpecificationVersion(schemaID), path, err)
				continue
			}

			updRoutePath, err := url.PathUnescape(updRoutePathEsc)
			if err != nil {
				logger.Error().Msgf("url unescape error: schema ID %d: openAPI version %s: loaded path %s - %v", schemaID, storedSpecs.SpecificationVersion(schemaID), path, err)
				continue
			}

			ops := pathItem.GetOperations() // ordered map с ключами: get, post, put, delete, patch, head, options, trace
			for method, _ := range ops.FromOldest() {

				logger.Debug().Msgf("handler: schema ID %d: openAPI version %s: loaded path %s - %s", schemaID, storedSpecs.SpecificationVersion(schemaID), method, updRoutePath)

				if err := apps.Handle(schemaID, method, updRoutePath, s.Handler); err != nil {
					logger.Error().Err(err).
						Int("schema_id", schemaID).
						Msgf("the OAS endpoint registration failed: method %s, path %s", method, updRoutePath)
				}
			}

		}

	}

	return apps.APIModeMainHandler
}
