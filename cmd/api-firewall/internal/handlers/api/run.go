package api

import (
	"net/url"
	"os"
	"os/signal"
	"path"
	"sync"
	"syscall"

	"github.com/ardanlabs/conf"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"

	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/allowiplist"
	"github.com/wallarm/api-firewall/internal/platform/metrics"
	"github.com/wallarm/api-firewall/internal/platform/storage"
	"github.com/wallarm/api-firewall/internal/version"
)

const (
	livenessEndpoint  = "/v1/liveness"
	readinessEndpoint = "/v1/readiness"
)

func Run(logger zerolog.Logger) error {

	// =========================================================================
	// Configuration

	var cfg config.APIMode
	cfg.Version.SVN = version.Version
	cfg.Version.Desc = version.ProjectName

	if err := conf.Parse(os.Args[1:], version.Namespace, &cfg); err != nil {
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

	// make a channel to listen for an interrupt or terminate signal from the OS
	// use a buffered channel because the signal package requires it
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// DB Usage Lock
	var dbLock sync.RWMutex

	// make a channel to listen for errors coming from the listener. Use a
	// buffered channel so the goroutine can exit if we don't collect this error.
	serverErrors := make(chan error, 1)

	// load spec from the database
	specStorage, err := storage.NewOpenAPIDB(cfg.PathToSpecDB, cfg.DBVersion)
	if err != nil {
		logger.Error().Msgf("%s: trying to load API Spec value from SQLLite Database : %v\n", logPrefix, err.Error())
	}

	if specStorage != nil {
		logger.Debug().Msgf("OpenAPI specifications with the following IDs were found in the DB: %v", specStorage.SchemaIDs())
	}

	// =========================================================================
	// Init ModSecurity Core

	waf, err := config.LoadModSecurityConfiguration(&cfg.ModSecurity, logger)
	if err != nil {
		logger.Fatal().Err(err)
		return err
	}

	if waf != nil {
		logger.Info().Msgf("%s: The ModSecurity configuration has been loaded successfully", logPrefix)
	}

	// Init Allow IP List

	logger.Info().Msgf("%s: Initializing IP Whitelist Cache", logPrefix)

	allowedIPCache, err := allowiplist.New(&cfg.AllowIP, logger)
	if err != nil {
		return errors.Wrap(err, "The allow IP list init error")
	}

	switch allowedIPCache {
	case nil:
		logger.Info().Msgf("%s: The allow IP list is not configured", logPrefix)
	default:
		logger.Info().Msgf("%s: Loaded %d Whitelisted IP's to the cache", logPrefix, allowedIPCache.ElementsNum)
	}

	// =========================================================================
	// Init ZeroLogger

	zeroLogger := &config.ZerologAdapter{Logger: logger}

	// =========================================================================
	// Init Metrics

	// make a channel to listen for errors coming from the metrics listener. Use a
	// buffered channel so the goroutine can exit if we don't collect this error.
	metricsErrors := make(chan error, 1)

	options := metrics.Options{
		EndpointName: cfg.Metrics.EndpointName,
		Host:         cfg.Metrics.Host,
		ReadTimeout:  cfg.Metrics.ReadTimeout,
		WriteTimeout: cfg.Metrics.WriteTimeout,
	}

	metricsController := metrics.NewPrometheusMetrics(cfg.Metrics.Enabled)

	if cfg.Metrics.Enabled {
		go func() {
			metricsErrors <- metricsController.StartService(&logger, &options)
		}()
	}

	// =========================================================================
	// Init Handlers

	requestHandlers := Handlers(&dbLock, &cfg, shutdown, logger, metricsController, specStorage, allowedIPCache, waf)

	// =========================================================================
	// Start Health API Service

	healthData := Health{
		OpenAPIDB: specStorage,
	}

	// health service handler
	healthHandler := func(ctx *fasthttp.RequestCtx) {
		switch string(ctx.Path()) {
		case livenessEndpoint:
			if err := healthData.Liveness(ctx); err != nil {
				logger.Error().Msgf("%s: liveness: %s", logPrefix, err.Error())
			}
		case readinessEndpoint:
			if err := healthData.Readiness(ctx); err != nil {
				logger.Error().Msgf("%s: readiness: %s", logPrefix, err.Error())
			}
		default:
			ctx.Error("Unsupported path", fasthttp.StatusNotFound)
		}
	}

	healthAPI := fasthttp.Server{
		Handler:               healthHandler,
		ReadTimeout:           cfg.ReadTimeout,
		WriteTimeout:          cfg.WriteTimeout,
		NoDefaultServerHeader: true,
		Logger:                zeroLogger,
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
				Bytes("host", ctx.Request.Header.Host()).
				Bytes("path", ctx.Path()).
				Bytes("method", ctx.Request.Header.Method()).
				Msg("request processing error")

			ctx.Error("", fasthttp.StatusInternalServerError)
		},
		Logger:                zeroLogger,
		NoDefaultServerHeader: true,
	}

	// =========================================================================
	// Init Regular Update Controller

	updSpecErrors := make(chan error, 1)

	updOpenAPISpec := NewHandlerUpdater(&dbLock, logger, metricsController, specStorage, &cfg, &api, shutdown, &healthData, allowedIPCache, waf)

	// disable updater if SpecificationUpdatePeriod == 0
	if cfg.SpecificationUpdatePeriod.Seconds() > 0 {
		go func() {
			logger.Info().Msgf("%s: starting specification regular update process every %.0f seconds", logPrefix, cfg.SpecificationUpdatePeriod.Seconds())
			updSpecErrors <- updOpenAPISpec.Start()
		}()
	}

	// start the service listening for requests.
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

	// blocking main and waiting for shutdown.
	select {
	case err := <-serverErrors:
		return errors.Wrap(err, "server error")

	case err := <-updSpecErrors:
		return errors.Wrap(err, "regular updater error")

	case err := <-metricsErrors:
		return errors.Wrap(err, "metrics error")

	case sig := <-shutdown:
		logger.Info().Msgf("%s: %v: Start shutdown", logPrefix, sig)

		if cfg.SpecificationUpdatePeriod.Seconds() > 0 {
			if err := updOpenAPISpec.Shutdown(); err != nil {
				return errors.Wrap(err, "could not stop configuration updater gracefully")
			}
		}

		// asking listener to shutdown and shed load.
		if err := api.Shutdown(); err != nil {
			return errors.Wrap(err, "could not stop server gracefully")
		}
		logger.Info().Msgf("%s: %v: Completed shutdown", logPrefix, sig)
	}

	return nil

}
