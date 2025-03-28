package proxy

import (
	"crypto/rsa"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/corazawaf/coraza/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/karlseguin/ccache/v2"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"

	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/mid"
	"github.com/wallarm/api-firewall/internal/platform/allowiplist"
	"github.com/wallarm/api-firewall/internal/platform/denylist"
	"github.com/wallarm/api-firewall/internal/platform/loader"
	woauth2 "github.com/wallarm/api-firewall/internal/platform/oauth2"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/storage"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

func Handlers(lock *sync.RWMutex, cfg *config.ProxyMode, serverURL *url.URL, shutdown chan os.Signal, logger zerolog.Logger, httpClientsPool proxy.Pool, specStorage storage.DBOpenAPILoader, deniedTokens *denylist.DeniedTokens, allowedIPCache *allowiplist.AllowedIPsType, waf coraza.WAF) fasthttp.RequestHandler {

	// define FastJSON parsers pool
	var parserPool fastjson.ParserPool

	// init OAuth validator
	var oauthValidator woauth2.OAuth2

	switch strings.ToLower(cfg.Server.Oauth.ValidationType) {
	case "jwt":
		var key *rsa.PublicKey
		if strings.HasPrefix(strings.ToLower(cfg.Server.Oauth.JWT.SignatureAlgorithm), "rs") && cfg.Server.Oauth.JWT.PubCertFile != "" {
			verifyBytes, err := os.ReadFile(cfg.Server.Oauth.JWT.PubCertFile)
			if err != nil {
				logger.Error().Msgf("Error reading public key from file: %v", err)
				break
			}

			key, err = jwt.ParseRSAPublicKeyFromPEM(verifyBytes)
			if err != nil {
				logger.Error().Msgf("Error parsing public key: %v", err)
				break
			}

			logger.Info().Msgf("OAuth2: public certificate successfully loaded")
		}

		oauthValidator = &woauth2.JWT{
			Cfg:       &cfg.Server.Oauth,
			Logger:    logger,
			PubKey:    key,
			SecretKey: []byte(cfg.Server.Oauth.JWT.SecretKey),
		}

	case "introspection":
		oauthValidator = &woauth2.Introspection{
			Cfg:    &cfg.Server.Oauth,
			Logger: logger,
			Cache:  ccache.New(ccache.Configure()),
		}
	}

	// define options Handler to handle requests with Options method
	optionsHandler := func(ctx *fasthttp.RequestCtx) {

		// add request ID
		ctx.SetUserValue(web.RequestID, uuid.NewString())

		// log request
		logger.Info().
			Bytes("host", ctx.Request.Header.Host()).
			Bytes("method", ctx.Request.Header.Method()).
			Bytes("path", ctx.Path()).
			Str("client_address", ctx.RemoteAddr().String()).
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Msg("Pass request with OPTIONS method")

		// proxy request
		if err := proxy.Perform(ctx, httpClientsPool, cfg.Server.RequestHostHeader); err != nil {
			logger.Error().
				Err(err).
				Bytes("host", ctx.Request.Header.Host()).
				Bytes("method", ctx.Request.Header.Method()).
				Bytes("path", ctx.Path()).
				Interface("request_id", ctx.UserValue(web.RequestID)).
				Msg("Error while proxying request")
		}
	}

	// set handler for default behavior (404, 405)
	defaultOpenAPIWaf := openapiWaf{
		customRoute: nil,
		proxyPool:   httpClientsPool,
		logger:      logger,
		cfg:         cfg,
		parserPool:  &parserPool,
	}

	// construct the web.App which holds all routes as well as common Middleware.
	options := web.AppAdditionalOptions{
		Mode:                  cfg.Mode,
		PassOptions:           cfg.PassOptionsRequests,
		RequestValidation:     cfg.RequestValidation,
		ResponseValidation:    cfg.ResponseValidation,
		CustomBlockStatusCode: cfg.CustomBlockStatusCode,
		OptionsHandler:        optionsHandler,
		DefaultHandler:        defaultOpenAPIWaf.openapiWafHandler,
		Lock:                  lock,
	}

	proxyOptions := mid.ProxyOptions{
		Mode:                 web.ProxyMode,
		RequestValidation:    cfg.RequestValidation,
		DeleteAcceptEncoding: cfg.Server.DeleteAcceptEncoding,
		ServerURL:            serverURL,
	}

	denylistOptions := mid.DenylistOptions{
		Mode:                  web.ProxyMode,
		Config:                &cfg.Denylist,
		CustomBlockStatusCode: cfg.CustomBlockStatusCode,
		DeniedTokens:          deniedTokens,
		Logger:                logger,
	}

	ipAllowlistOptions := mid.IPAllowListOptions{
		Mode:                  web.ProxyMode,
		Config:                &cfg.AllowIP,
		CustomBlockStatusCode: cfg.CustomBlockStatusCode,
		AllowedIPs:            allowedIPCache,
		Logger:                logger,
	}

	// Use ModSecurity-specific validation settings if defined, otherwise fall back to global settings
	modSecRequestValidation := cfg.ModSecurity.RequestValidation
	if modSecRequestValidation == "" {
		modSecRequestValidation = cfg.RequestValidation
	}
	modSecResponseValidation := cfg.ModSecurity.ResponseValidation
	if modSecResponseValidation == "" {
		modSecResponseValidation = cfg.ResponseValidation
	}

	modSecOptions := mid.ModSecurityOptions{
		Mode:                  web.ProxyMode,
		WAF:                   waf,
		Logger:                logger,
		RequestValidation:     modSecRequestValidation,
		ResponseValidation:    modSecResponseValidation,
		CustomBlockStatusCode: cfg.CustomBlockStatusCode,
	}

	swagRouter, err := loader.NewRouter(specStorage.Specification(0), true)
	if err != nil {
		logger.Error().Msgf("Error parsing OpenAPI specification: %v", err)
		return nil
	}

	app := web.NewApp(&options, shutdown, logger, mid.Logger(logger), mid.Errors(logger), mid.Panics(logger), mid.Proxy(&proxyOptions), mid.IPAllowlist(&ipAllowlistOptions), mid.Denylist(&denylistOptions), mid.WAFModSecurity(&modSecOptions), mid.ShadowAPIMonitor(logger, &cfg.ShadowAPI))

	serverPath := "/"
	if serverURL.Path != "" {
		serverPath = serverURL.Path
	}

	for i := 0; i < len(swagRouter.Routes); i++ {
		s := openapiWaf{
			customRoute:    &swagRouter.Routes[i],
			proxyPool:      httpClientsPool,
			logger:         logger,
			cfg:            cfg,
			parserPool:     &parserPool,
			oauthValidator: oauthValidator,
		}

		updRoutePathEsc, err := url.JoinPath(serverPath, swagRouter.Routes[i].Path)
		if err != nil {
			s.logger.Error().Msgf("url parse error: Loaded path %s - %v", swagRouter.Routes[i].Path, err)
			continue
		}

		updRoutePath, err := url.PathUnescape(updRoutePathEsc)
		if err != nil {
			s.logger.Error().Msgf("url unescape error: Loaded path %s - %v", swagRouter.Routes[i].Path, err)
			continue
		}

		s.logger.Debug().Msgf("handler: Loaded path %s - %s", swagRouter.Routes[i].Method, updRoutePath)

		// set endpoint custom validation modes
		var actions *router.Actions
		for _, endpoint := range cfg.Endpoints {
			if strings.EqualFold(endpoint.Path, updRoutePath) && (endpoint.Method == "" || endpoint.Method != "" && strings.EqualFold(swagRouter.Routes[i].Method, endpoint.Method)) {
				actions = new(router.Actions)
				actions.Request = endpoint.RequestValidation
				actions.Response = endpoint.ResponseValidation

				logger.Debug().
					Str("method", swagRouter.Routes[i].Method).
					Str("path", updRoutePath).
					Str("request_validation_mode", actions.Request).
					Str("response_validation_mode", actions.Response).
					Msgf("handler: custom validation mode applied for %s - %s: request %s, response %s", swagRouter.Routes[i].Method, updRoutePath, actions.Request, actions.Response)
			}
		}

		if err := app.Handle(swagRouter.Routes[i].Method, updRoutePath, actions, s.openapiWafHandler); err != nil {
			logger.Error().Err(err).Msgf("The OAS endpoint registration failed: method %s, path %s", swagRouter.Routes[i].Method, updRoutePath)
		}
	}

	return app.MainHandler
}
