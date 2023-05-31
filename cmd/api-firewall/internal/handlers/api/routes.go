package api

import (
	"net/url"
	"os"
	"path"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/mid"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

func APIModeHandlers(cfg *config.APIFWConfiguration, serverURL *url.URL, shutdown chan os.Signal, logger *logrus.Logger, swagRouter *router.Router) fasthttp.RequestHandler {

	// define FastJSON parsers pool
	var parserPool fastjson.ParserPool

	// Construct the web.App which holds all routes as well as common Middleware.
	app := web.NewApp(shutdown, cfg, logger, mid.APIHeaders(logger), mid.Logger(logger), mid.Errors(logger), mid.Panics(logger))

	for i := 0; i < len(swagRouter.Routes); i++ {

		s := APIMode{
			CustomRoute:   &swagRouter.Routes[i],
			Log:           logger,
			Cfg:           cfg,
			ParserPool:    &parserPool,
			OpenAPIRouter: swagRouter,
		}
		updRoutePath := path.Join(serverURL.Path, swagRouter.Routes[i].Path)

		s.Log.Debugf("handler: Loaded path : %s - %s", swagRouter.Routes[i].Method, updRoutePath)

		app.Handle(swagRouter.Routes[i].Method, updRoutePath, s.APIModeHandler)
	}

	// set handler for default behavior (404, 405)
	s := APIMode{
		CustomRoute:   nil,
		Log:           logger,
		Cfg:           cfg,
		ParserPool:    &parserPool,
		OpenAPIRouter: swagRouter,
	}
	app.SetDefaultBehavior(s.APIModeHandler)

	return app.Router.Handler
}
