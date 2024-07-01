package graphql

import (
	"net/url"
	"os"
	"sync"

	"github.com/fasthttp/websocket"
	"github.com/savsgio/gotils/strconv"
	"github.com/savsgio/gotils/strings"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/mid"
	"github.com/wallarm/api-firewall/internal/platform/allowiplist"
	"github.com/wallarm/api-firewall/internal/platform/denylist"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/web"
	"github.com/wundergraph/graphql-go-tools/pkg/graphql"
	"github.com/wundergraph/graphql-go-tools/pkg/playground"
)

func Handlers(cfg *config.GraphQLMode, schema *graphql.Schema, serverURL *url.URL, shutdown chan os.Signal, logger *logrus.Logger, proxy proxy.Pool, wsClient proxy.WebSocketClient, deniedTokens *denylist.DeniedTokens, AllowedIPCache *allowiplist.AllowedIPsType) fasthttp.RequestHandler {

	// Construct the web.App which holds all routes as well as common Middleware.
	appOptions := web.AppAdditionalOptions{
		Mode:        web.GraphQLMode,
		PassOptions: false,
	}

	proxyOptions := mid.ProxyOptions{
		Mode:                 web.GraphQLMode,
		RequestValidation:    cfg.Graphql.RequestValidation,
		DeleteAcceptEncoding: cfg.Server.DeleteAcceptEncoding,
		ServerURL:            serverURL,
	}

	denylistOptions := mid.DenylistOptions{
		Mode:                  web.GraphQLMode,
		Config:                &cfg.Denylist,
		CustomBlockStatusCode: fasthttp.StatusUnauthorized,
		DeniedTokens:          deniedTokens,
		Logger:                logger,
	}

	ipAllowlistOptions := mid.IPAllowListOptions{
		Mode:                  web.GraphQLMode,
		Config:                &cfg.AllowIP,
		CustomBlockStatusCode: fasthttp.StatusUnauthorized,
		AllowedIPs:            AllowedIPCache,
		Logger:                logger,
	}

	app := web.NewApp(&appOptions, shutdown, logger, mid.Logger(logger), mid.Errors(logger), mid.Panics(logger), mid.Proxy(&proxyOptions), mid.IPAllowlist(&ipAllowlistOptions), mid.Denylist(&denylistOptions))

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

	// use API Host env var to take path
	apiHost, err := url.ParseRequestURI(cfg.APIHost)
	if err != nil {
		logger.Fatalf("parsing API Host URL: %v", err)
		return nil
	}

	graphqlPath := apiHost.Path
	if graphqlPath == "" {
		graphqlPath = "/"
	}

	if err := app.Handle(fasthttp.MethodGet, graphqlPath, s.GraphQLHandle); err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("GraphQL GET endpoint registration failed")
	}
	if err := app.Handle(fasthttp.MethodPost, graphqlPath, s.GraphQLHandle); err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("GraphQL POST endpoint registration failed")
	}

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
			if err := app.Handle(fasthttp.MethodGet, handlers[i].Path, web.NewFastHTTPHandler(handlers[i].Handler, true)); err != nil {
				logger.WithFields(logrus.Fields{"error": err}).Error("GraphQL Playground endpoint registration failed")
			}
		}
	}

	return app.MainHandler
}
