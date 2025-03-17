package graphql

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/ardanlabs/conf"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
	"github.com/wundergraph/graphql-go-tools/pkg/graphql"

	handlersProxy "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/proxy"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/allowiplist"
	"github.com/wallarm/api-firewall/internal/platform/denylist"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/version"
)

const (
	logPrefix = "main"

	initialPoolCapacity = 100
	livenessEndpoint    = "/v1/liveness"
	readinessEndpoint   = "/v1/readiness"
)

func Run(logger zerolog.Logger) error {

	// =========================================================================
	// Configuration

	var cfg config.GraphQLMode
	cfg.Version.SVN = version.Version
	cfg.Version.Desc = version.ProjectName

	if err := conf.Parse(os.Args[1:], version.Namespace, &cfg); err != nil {
		switch err {
		case conf.ErrHelpWanted:
			usage, err := conf.Usage(version.Namespace, &cfg)
			if err != nil {
				return errors.Wrap(err, "generating config usage")
			}
			fmt.Println(usage)
			return nil
		case conf.ErrVersionWanted:
			version, err := conf.VersionString(version.Namespace, &cfg)
			if err != nil {
				return errors.Wrap(err, "generating config version")
			}
			fmt.Println(version)
			return nil
		}
		return errors.Wrap(err, "parsing config")
	}

	// =========================================================================
	// Init Logger

	logger.Info().Msgf("%s : Started : Application initializing : version %q", logPrefix, version.Version)
	defer logger.Info().Msgf("%s: Completed", logPrefix)

	out, err := conf.String(&cfg)
	if err != nil {
		return errors.Wrap(err, "generating config for output")
	}
	logger.Info().Msgf("%s: Configuration Loaded :\n%v\n", logPrefix, out)

	// Make a channel to listen for an interrupt or terminate signal from the OS.
	// Use a buffered channel because the signal package requires it.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Make a channel to listen for errors coming from the listener. Use a
	// buffered channel so the goroutine can exit if we don't collect this error.
	serverErrors := make(chan error, 1)

	// =========================================================================
	// Init GraphQL schema

	// load file with GraphQL schema
	f, err := os.Open(cfg.Graphql.Schema)
	if err != nil {
		logger.Fatal().Msgf("Loading GraphQL Schema error: %v", err)
		return err
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromReader(f)
	if err != nil {
		logger.Fatal().Msgf("Loading GraphQL Schema error: %v", err)
		return err
	}

	validationRes, err := schema.Validate()
	if err != nil {
		logger.Fatal().Msgf("GraphQL Schema validator error: %v", err)
		return err
	}

	if !validationRes.Valid {
		logger.Fatal().Msgf("GraphQL Schema validator error: %v", validationRes.Errors)
		return validationRes.Errors
	}

	if err := f.Close(); err != nil {
		logger.Fatal().Msgf("Loading GraphQL Schema error: %v", err)
		return err
	}

	// =========================================================================
	// Init Proxy Pool

	serverURL, err := url.ParseRequestURI(cfg.Server.URL)
	if err != nil {
		return errors.Wrap(err, "parsing proxy URL")
	}
	host := serverURL.Host
	if serverURL.Port() == "" {
		switch serverURL.Scheme {
		case "https":
			host += ":443"
		case "http":
			host += ":80"
		}
	}

	initialCap := initialPoolCapacity

	if cfg.Server.ClientPoolCapacity < initialPoolCapacity {
		initialCap = 1
	}

	options := proxy.Options{
		InitialPoolCapacity: initialCap,
		ClientPoolCapacity:  cfg.Server.ClientPoolCapacity,
		InsecureConnection:  cfg.Server.InsecureConnection,
		RootCA:              cfg.Server.RootCA,
		MaxConnsPerHost:     cfg.Server.MaxConnsPerHost,
		ReadTimeout:         cfg.Server.ReadTimeout,
		WriteTimeout:        cfg.Server.WriteTimeout,
		ReadBufferSize:      cfg.Server.ReadBufferSize,
		WriteBufferSize:     cfg.Server.WriteBufferSize,
		MaxResponseBodySize: cfg.Server.MaxResponseBodySize,
		DialTimeout:         cfg.Server.DialTimeout,
		Logger:              logger,
	}

	pool, err := proxy.NewChanPool(host, &options)
	if err != nil {
		return errors.Wrap(err, "proxy pool init")
	}

	// =========================================================================
	// Init WS Pool

	wsScheme := "ws"
	if serverURL.Scheme == "https" {
		wsScheme = "wss"
	}
	wsConnPoolOptions := &proxy.WSClientOptions{
		Scheme:             wsScheme,
		Host:               serverURL.Host,
		Path:               serverURL.Path,
		InsecureConnection: cfg.Server.InsecureConnection,
		RootCA:             cfg.Server.RootCA,
		DialTimeout:        cfg.Server.DialTimeout,
	}

	wsPool, err := proxy.NewWSClient(logger, wsConnPoolOptions)
	if err != nil {
		return errors.Wrap(err, "ws connections pool init")
	}

	// =========================================================================
	// Init Cache

	logger.Info().Msgf("%s: Initializing DenyList Cache", logPrefix)

	deniedTokens, err := denylist.New(&cfg.Denylist, logger)
	if err != nil {
		return errors.Wrap(err, "denylist init error")
	}

	switch deniedTokens {
	case nil:
		logger.Info().Msgf("%s: Denylist not configured", logPrefix)
	default:
		logger.Info().Msgf("%s: Loaded %d tokens to the cache", logPrefix, deniedTokens.ElementsNum)
	}

	// =========================================================================
	// Init Allow IP List

	logger.Info().Msgf("%s: Initializing IP Whitelist Cache", logPrefix)

	allowedIPCache, err := allowiplist.New(&cfg.AllowIP, logger)
	if err != nil {
		return errors.Wrap(err, "The allow IP list init error")
	}

	switch allowedIPCache {
	case nil:
		logger.Info().Msgf("%s: The allow ip list is not configured", logPrefix)
	default:
		logger.Info().Msgf("%s: Loaded %d Whitelisted IP's to the cache", logPrefix, allowedIPCache.ElementsNum)
	}

	// =========================================================================
	// Init ZeroLogger

	zeroLogger := &config.ZerologAdapter{Logger: logger}

	// =========================================================================
	// Init Handlers

	requestHandlers := Handlers(&cfg, schema, serverURL, shutdown, logger, pool, wsPool, deniedTokens, allowedIPCache)

	// =========================================================================
	// Start Health API Service

	healthData := handlersProxy.Health{
		Logger: logger,
		Pool:   pool,
	}

	// health service handler
	healthHandler := func(ctx *fasthttp.RequestCtx) {
		switch string(ctx.Path()) {
		case livenessEndpoint:
			if err := healthData.Liveness(ctx); err != nil {
				healthData.Logger.Error().Msgf("%s: liveness: %s", logPrefix, err.Error())
			}
		case readinessEndpoint:
			if err := healthData.Readiness(ctx); err != nil {
				healthData.Logger.Error().Msgf("%s: readiness: %s", logPrefix, err.Error())
			}
		default:
			ctx.Error("Unsupported path", fasthttp.StatusNotFound)
		}
	}

	healthAPI := fasthttp.Server{
		Handler:               healthHandler,
		ReadTimeout:           cfg.ReadTimeout,
		WriteTimeout:          cfg.WriteTimeout,
		Logger:                zeroLogger,
		NoDefaultServerHeader: true,
	}

	// Start the service listening for requests.
	go func() {
		logger.Info().Msgf("%s: Health API listening on %s", logPrefix, cfg.HealthAPIHost)
		serverErrors <- healthAPI.ListenAndServe(cfg.HealthAPIHost)
	}()

	// =========================================================================
	// Start API Service

	logger.Info().Msgf("%s: Initializing API support", logPrefix)

	apiHost, err := url.ParseRequestURI(cfg.APIHost)
	if err != nil {
		return errors.Wrap(err, "parsing API Host URL")
	}

	var isTLS bool

	switch apiHost.Scheme {
	case "http":
		isTLS = false
	case "https":
		isTLS = true
	}

	api := fasthttp.Server{
		Handler:            requestHandlers,
		ReadTimeout:        cfg.ReadTimeout,
		WriteTimeout:       cfg.WriteTimeout,
		ReadBufferSize:     cfg.ReadBufferSize,
		WriteBufferSize:    cfg.WriteBufferSize,
		MaxRequestBodySize: cfg.MaxRequestBodySize,
		DisableKeepalive:   cfg.DisableKeepalive,
		MaxConnsPerIP:      cfg.MaxConnsPerIP,
		MaxRequestsPerConn: cfg.MaxRequestsPerConn,
		ErrorHandler: func(ctx *fasthttp.RequestCtx, err error) {
			logger.Error().
				Err(err).
				Msg("request processing error")

			ctx.Error("", fasthttp.StatusForbidden)
		},
		Logger:                zeroLogger,
		NoDefaultServerHeader: true,
	}

	// Start the service listening for requests.
	go func() {
		logger.Info().Msgf("%s: API listening on %s", logPrefix, cfg.APIHost)
		switch isTLS {
		case false:
			serverErrors <- api.ListenAndServe(apiHost.Host)
		case true:
			serverErrors <- api.ListenAndServeTLS(apiHost.Host, path.Join(cfg.TLS.CertsPath, cfg.TLS.CertFile),
				path.Join(cfg.TLS.CertsPath, cfg.TLS.CertKey))
		}
	}()

	// =========================================================================
	// Shutdown

	// Blocking main and waiting for shutdown.
	select {
	case err := <-serverErrors:
		return errors.Wrap(err, "server error")

	case sig := <-shutdown:
		logger.Info().Msgf("%s: %v: Start shutdown", logPrefix, sig)

		// Asking listener to shutdown and shed load.
		if err := api.Shutdown(); err != nil {
			return errors.Wrap(err, "could not stop server gracefully")
		}
		logger.Info().Msgf("%s: %v: Completed shutdown", logPrefix, sig)

		// Close proxy pool
		pool.Close()
	}

	return nil

}
