package proxy

import (
	"crypto/rsa"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/golang-jwt/jwt"
	"github.com/karlseguin/ccache/v2"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/mid"
	"github.com/wallarm/api-firewall/internal/platform/denylist"
	woauth2 "github.com/wallarm/api-firewall/internal/platform/oauth2"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

func Handlers(cfg *config.APIFWConfiguration, serverURL *url.URL, shutdown chan os.Signal, logger *logrus.Logger, proxy proxy.Pool, swagRouter *router.Router, deniedTokens *denylist.DeniedTokens) fasthttp.RequestHandler {

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

	// Construct the web.App which holds all routes as well as common Middleware.
	app := web.NewApp(shutdown, cfg, logger, mid.Logger(logger), mid.Errors(logger), mid.Panics(logger), mid.Proxy(cfg, serverURL), mid.Denylist(cfg, deniedTokens, logger), mid.ShadowAPIMonitor(logger, &cfg.ShadowAPI))

	for i := 0; i < len(swagRouter.Routes); i++ {
		s := openapiWaf{
			customRoute:    &swagRouter.Routes[i],
			proxyPool:      proxy,
			logger:         logger,
			cfg:            cfg,
			parserPool:     &parserPool,
			oauthValidator: oauthValidator,
		}
		updRoutePath := path.Join(serverURL.Path, swagRouter.Routes[i].Path)

		s.logger.Debugf("handler: Loaded path %s - %s", swagRouter.Routes[i].Method, updRoutePath)

		app.Handle(swagRouter.Routes[i].Method, updRoutePath, s.openapiWafHandler)
	}

	// set handler for default behavior (404, 405)
	s := openapiWaf{
		customRoute: nil,
		proxyPool:   proxy,
		logger:      logger,
		cfg:         cfg,
		parserPool:  &parserPool,
	}
	app.SetDefaultBehavior(s.openapiWafHandler)

	return app.Router.Handler
}
