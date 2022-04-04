package handlers

import (
	"crypto/rsa"
	"github.com/wallarm/api-firewall/internal/platform/denylist"
	"io/ioutil"
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
	woauth2 "github.com/wallarm/api-firewall/internal/platform/oauth2"
	"github.com/wallarm/api-firewall/internal/platform/openapi3"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

func OpenapiProxy(cfg *config.APIFWConfiguration, serverUrl *url.URL, shutdown chan os.Signal, logger *logrus.Logger, proxy proxy.Pool, swagRouter *router.Router, deniedTokens *denylist.DeniedTokens) fasthttp.RequestHandler {

	var parserPool fastjson.ParserPool

	var oauthValidator woauth2.OAuth2

	switch strings.ToLower(cfg.Server.Oauth.ValidationType) {
	case "jwt":
		var key *rsa.PublicKey
		if strings.HasPrefix(strings.ToLower(cfg.Server.Oauth.JWT.SignatureAlgorithm), "rs") {
			verifyBytes, err := ioutil.ReadFile(cfg.Server.Oauth.JWT.PubCertFile)
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
	app := web.NewApp(shutdown, cfg, logger, mid.Logger(logger), mid.Errors(logger), mid.Panics(logger), mid.Proxy(cfg, serverUrl), mid.Denylist(cfg, deniedTokens, logger))

	for _, route := range swagRouter.Routes {
		pathParamLength := 0
		if getOp := route.Route.PathItem.GetOperation(route.Method); getOp != nil {
			for _, param := range getOp.Parameters {
				if param.Value.In == openapi3.ParameterInPath {
					pathParamLength += 1
				}
			}
		}

		// check common parameters
		if getOp := route.Route.PathItem.Parameters; getOp != nil {
			for _, param := range getOp {
				if param.Value.In == openapi3.ParameterInPath {
					pathParamLength += 1
				}
			}
		}

		s := openapiWaf{
			route:           route.Route,
			proxyPool:       proxy,
			pathParamLength: pathParamLength,
			logger:          logger,
			cfg:             cfg,
			parserPool:      &parserPool,
			oauthValidator:  oauthValidator,
		}
		updRoutePath := path.Join(serverUrl.Path, route.Path)

		s.logger.Debugf("handler: Loaded path : %s - %s", route.Method, updRoutePath)

		app.Handle(route.Method, updRoutePath, s.openapiWafHandler)
	}

	// set handler for default behavior (404, 405)
	s := openapiWaf{
		route:           nil,
		proxyPool:       proxy,
		pathParamLength: 0,
		logger:          logger,
		cfg:             cfg,
		parserPool:      &parserPool,
	}
	app.SetDefaultBehavior(s.openapiWafHandler)

	return app.Router.Handler
}
