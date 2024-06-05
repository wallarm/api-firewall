package proxy

import (
	"bytes"
	"crypto/rsa"
	"net/url"
	"os"
	"strings"

	"github.com/corazawaf/coraza/v3"
	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/karlseguin/ccache/v2"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/mid"
	"github.com/wallarm/api-firewall/internal/platform/allowiplist"
	"github.com/wallarm/api-firewall/internal/platform/denylist"
	"github.com/wallarm/api-firewall/internal/platform/loader"
	woauth2 "github.com/wallarm/api-firewall/internal/platform/oauth2"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

func Handlers(cfg *config.ProxyMode, serverURL *url.URL, shutdown chan os.Signal, logger *logrus.Logger, httpClientsPool proxy.Pool, swagRouter *loader.Router, deniedTokens *denylist.DeniedTokens, AllowedIPCache *allowiplist.AllowedIPsType, waf coraza.WAF) fasthttp.RequestHandler {

	// define FastJSON parsers pool
	var parserPool fastjson.ParserPool

	// Init OAuth validator
	var oauthValidator woauth2.OAuth2

	switch strings.ToLower(cfg.Server.Oauth.ValidationType) {
	case "jwt":
		var key *rsa.PublicKey
		if strings.HasPrefix(strings.ToLower(cfg.Server.Oauth.JWT.SignatureAlgorithm), "rs") && cfg.Server.Oauth.JWT.PubCertFile != "" {
			verifyBytes, err := os.ReadFile(cfg.Server.Oauth.JWT.PubCertFile)
			if err != nil {
				logger.Errorf("Error reading public key from file: %s", err)
				break
			}

			key, err = jwt.ParseRSAPublicKeyFromPEM(verifyBytes)
			if err != nil {
				logger.Errorf("Error parsing public key: %s", err)
				break
			}

			logger.Infof("OAuth2: public certificate successfully loaded")
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

	// Define options Handler to handle requests with Options method
	optionsHandler := func(ctx *fasthttp.RequestCtx) {

		// Add request ID
		ctx.SetUserValue(web.RequestID, uuid.NewString())

		// Log request
		logger.WithFields(logrus.Fields{
			"host":           string(ctx.Request.Header.Host()),
			"method":         bytes.NewBuffer(ctx.Request.Header.Method()).String(),
			"path":           string(ctx.Path()),
			"client_address": ctx.RemoteAddr(),
			"request_id":     ctx.UserValue(web.RequestID),
		}).Info("Pass request with OPTIONS method")

		// Proxy request
		if err := proxy.Perform(ctx, httpClientsPool, cfg.Server.Host); err != nil {
			logger.WithFields(logrus.Fields{
				"error":      err,
				"host":       string(ctx.Request.Header.Host()),
				"path":       string(ctx.Path()),
				"method":     string(ctx.Request.Header.Method()),
				"request_id": ctx.UserValue(web.RequestID),
			}).Error("Error while proxying request")
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

	// Construct the web.App which holds all routes as well as common Middleware.
	options := web.AppAdditionalOptions{
		Mode:                  cfg.Mode,
		PassOptions:           cfg.PassOptionsRequests,
		RequestValidation:     cfg.RequestValidation,
		ResponseValidation:    cfg.ResponseValidation,
		CustomBlockStatusCode: cfg.CustomBlockStatusCode,
		OptionsHandler:        optionsHandler,
		DefaultHandler:        defaultOpenAPIWaf.openapiWafHandler,
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
		AllowedIPs:            AllowedIPCache,
		Logger:                logger,
	}

	modSecOptions := mid.ModSecurityOptions{
		Mode:                  web.ProxyMode,
		WAF:                   waf,
		Logger:                logger,
		RequestValidation:     cfg.RequestValidation,
		ResponseValidation:    cfg.ResponseValidation,
		CustomBlockStatusCode: cfg.CustomBlockStatusCode,
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
			s.logger.Errorf("url parse error: Loaded path %s - %v", swagRouter.Routes[i].Path, err)
			continue
		}

		updRoutePath, err := url.PathUnescape(updRoutePathEsc)
		if err != nil {
			s.logger.Errorf("url unescape error: Loaded path %s - %v", swagRouter.Routes[i].Path, err)
			continue
		}

		s.logger.Debugf("handler: Loaded path %s - %s", swagRouter.Routes[i].Method, updRoutePath)

		if err := app.Handle(swagRouter.Routes[i].Method, updRoutePath, s.openapiWafHandler); err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Errorf("The OAS endpoint registration failed: method %s, path %s", swagRouter.Routes[i].Method, updRoutePath)
		}
	}

	return app.MainHandler
}
