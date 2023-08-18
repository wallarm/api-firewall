package graphql

import (
	"github.com/savsgio/gotils/strconv"
	"github.com/savsgio/gotils/strings"
	"net/url"
	"os"
	"sync"

	"github.com/fasthttp/websocket"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/mid"
	"github.com/wallarm/api-firewall/internal/platform/denylist"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/web"
	"github.com/wundergraph/graphql-go-tools/pkg/graphql"
	"github.com/wundergraph/graphql-go-tools/pkg/playground"
)

func Handlers(cfg *config.GraphQLMode, schema *graphql.Schema, serverURL *url.URL, shutdown chan os.Signal, logger *logrus.Logger, proxy proxy.Pool, wsClient proxy.WebSocketClient, deniedTokens *denylist.DeniedTokens) fasthttp.RequestHandler {

	// Construct the web.App which holds all routes as well as common Middleware.
	appOptions := web.AppAdditionalOptions{
		Mode:        cfg.Mode,
		PassOptions: false,
	}

	proxyOptions := mid.ProxyOptions{
		Mode:                 web.GraphQLMode,
		RequestValidation:    cfg.Graphql.RequestValidation,
		DeleteAcceptEncoding: cfg.Server.DeleteAcceptEncoding,
		ServerUrl:            serverURL,
	}

	denylistOptions := mid.DenylistOptions{
		Mode:                  web.GraphQLMode,
		Config:                &cfg.Denylist,
		CustomBlockStatusCode: fasthttp.StatusUnauthorized,
		DeniedTokens:          deniedTokens,
		Logger:                logger,
	}
	app := web.NewApp(&appOptions, shutdown, logger, mid.Logger(logger), mid.Errors(logger), mid.Panics(logger), mid.Proxy(&proxyOptions), mid.Denylist(&denylistOptions))

	// define FastJSON parsers pool
	var parserPool fastjson.ParserPool

	var upgrader = websocket.FastHTTPUpgrader{
		Subprotocols: []string{"graphql-ws"},
		CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
			if !cfg.Graphql.WSCheckOrigin {
				return true
			}
			return strings.Include(cfg.Graphql.WSOrigin, strconv.B2S(ctx.Request.Header.Peek("Origin")))
		},
	}

	s := Handler{
		cfg:        cfg,
		serverURL:  serverURL,
		proxyPool:  proxy,
		logger:     logger,
		schema:     schema,
		parserPool: &parserPool,
		wsClient:   wsClient,
		upgrader:   &upgrader,
		mu:         sync.Mutex{},
	}

	graphqlPath := serverURL.Path
	if graphqlPath == "" {
		graphqlPath = "/"
	}

	app.Handle(fasthttp.MethodGet, graphqlPath, s.GraphQLHandle)
	app.Handle(fasthttp.MethodPost, graphqlPath, s.GraphQLHandle)

	// enable playground
	if cfg.Graphql.Playground {
		p := playground.New(playground.Config{
			PathPrefix:                      "",
			PlaygroundPath:                  cfg.Graphql.PlaygroundPath,
			GraphqlEndpointPath:             graphqlPath,
			GraphQLSubscriptionEndpointPath: graphqlPath,
		})

		handlers, err := p.Handlers()
		if err != nil {
			logger.Fatalf("playground handlers error: %v", err)
			return nil
		}

		for i := range handlers {
			app.Handle(fasthttp.MethodGet, handlers[i].Path, web.NewFastHTTPHandler(handlers[i].Handler, true))
		}
	}

	return app.Router.Handler
}
