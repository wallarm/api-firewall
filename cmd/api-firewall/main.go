package main

import (
	"expvar" // Register the expvar handlers
	"fmt"
	"mime"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/ardanlabs/conf"
	"github.com/go-playground/validator"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/denylist"
	"github.com/wallarm/api-firewall/internal/platform/openapi3"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/router"
)

var build = "develop"

const (
	namespace = "apifw"
	logPrefix = "main"
)

func main() {
	logger := logrus.New()

	logger.SetLevel(logrus.DebugLevel)

	logger.SetFormatter(&logrus.TextFormatter{
		DisableQuote:  true,
		FullTimestamp: true,
	})

	if err := run(logger); err != nil {
		logger.Infof("%s: error: %s", logPrefix, err)
		os.Exit(1)
	}
}

func run(logger *logrus.Logger) error {

	// =========================================================================
	// Configuration

	var cfg config.APIFWConfiguration
	cfg.Version.SVN = build
	cfg.Version.Desc = "Wallarm API-Firewall"

	if err := conf.Parse(os.Args[1:], namespace, &cfg); err != nil {
		switch err {
		case conf.ErrHelpWanted:
			usage, err := conf.Usage(namespace, &cfg)
			if err != nil {
				return errors.Wrap(err, "generating config usage")
			}
			fmt.Println(usage)
			return nil
		case conf.ErrVersionWanted:
			version, err := conf.VersionString(namespace, &cfg)
			if err != nil {
				return errors.Wrap(err, "generating config version")
			}
			fmt.Println(version)
			return nil
		}
		return errors.Wrap(err, "parsing config")
	}

	// validate
	validate := validator.New()

	if err := validate.RegisterValidation("HttpStatusCodes", config.ValidateStatusList); err != nil {
		return errors.Errorf("configuration validation error: %s", err.Error())
	}

	if err := validate.Struct(cfg); err != nil {

		for _, err := range err.(validator.ValidationErrors) {
			switch err.Tag() {
			case "gt":
				return errors.Errorf("configuration validation error: parameter %s should be > %s. Actual value: %d", err.Field(), err.Param(), err.Value())
			case "url":
				return errors.Errorf("configuration validation error: parameter %s should be a string in URL format. Example: http://localhost:8080/; actual value: %s", err.Field(), err.Value())
			case "oneof":
				return errors.Errorf("configuration validation error: parameter %s should have one of the following value: %s; actual value: %s", err.Field(), err.Param(), err.Value())
			}
		}
		return errors.Wrap(err, "configuration validation error")
	}

	// oauth introspection endpoint: validate format of configured content-type
	if cfg.Server.Oauth.Introspection.ContentType != "" {
		_, _, err := mime.ParseMediaType(cfg.Server.Oauth.Introspection.ContentType)
		if err != nil {
			return errors.Wrap(err, "configuration validation error")
		}
	}

	// =========================================================================
	// Init Logger

	if strings.ToLower(cfg.LogFormat) == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	}

	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	case "warning":
		logger.SetLevel(logrus.WarnLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	default:
		return errors.New("invalid log level")
	}

	// =========================================================================
	// App Starting

	// Print the build version for our logs. Also expose it under /debug/vars.
	expvar.NewString("build").Set(build)

	logger.Infof("%s : Started : Application initializing : version %q", logPrefix, build)
	defer logger.Infof("%s: Completed", logPrefix)

	out, err := conf.String(&cfg)
	if err != nil {
		return errors.Wrap(err, "generating config for output")
	}
	logger.Infof("%s: Configuration Loaded :\n%v\n", logPrefix, out)

	// =========================================================================
	// Init Swagger

	var swagger *openapi3.Swagger

	apiSpecUrl, err := url.ParseRequestURI(cfg.APISpecs)
	if err != nil {
		logger.Debugf("%s: Trying to parse API Spec value as URL : %v\n", logPrefix, err.Error())
	}

	switch apiSpecUrl {
	case nil:
		swagger, err = openapi3.NewSwaggerLoader().LoadSwaggerFromFile(cfg.APISpecs)
		if err != nil {
			return errors.Wrap(err, "loading swagwaf file")
		}
	default:
		swagger, err = openapi3.NewSwaggerLoader().LoadSwaggerFromURI(apiSpecUrl)
		if err != nil {
			return errors.Wrap(err, "loading swagwaf url")
		}
	}

	swagRouter, err := router.NewRouter(swagger)
	if err != nil {
		return errors.Wrap(err, "parsing swagwaf file")
	}

	// =========================================================================
	// Init Proxy Client

	serverUrl, err := url.ParseRequestURI(cfg.Server.URL)
	if err != nil {
		return errors.Wrap(err, "parsing proxy URL")
	}
	host := serverUrl.Host
	if serverUrl.Port() == "" {
		switch serverUrl.Scheme {
		case "https":
			host += ":443"
		case "http":
			host += ":80"
		}
	}

	initialCap := 100

	if cfg.Server.ClientPoolCapacity < 100 {
		initialCap = 1
	}

	pool, err := proxy.NewChanPool(initialCap, cfg.Server.ClientPoolCapacity, host, &cfg.Server)
	if err != nil {
		return errors.Wrap(err, "proxy pool init")
	}

	// =========================================================================
	// Init Cache

	logger.Infof("%s: Initializing Cache", logPrefix)

	deniedTokens, err := denylist.New(&cfg, logger)
	if err != nil {
		return errors.Wrap(err, "denylist init error")
	}

	logger.Infof("%s: Loaded %d tokens to the cache", logPrefix, deniedTokens.ElementsNum)

	// =========================================================================
	// Start API Service

	logger.Infof("%s: Initializing API support", logPrefix)

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

	// Make a channel to listen for an interrupt or terminate signal from the OS.
	// Use a buffered channel because the signal package requires it.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{
		Handler:               handlers.OpenapiProxy(&cfg, serverUrl, shutdown, logger, pool, swagRouter, deniedTokens),
		ReadTimeout:           cfg.ReadTimeout,
		WriteTimeout:          cfg.WriteTimeout,
		Logger:                logger,
		NoDefaultServerHeader: true,
	}

	// Make a channel to listen for errors coming from the listener. Use a
	// buffered channel so the goroutine can exit if we don't collect this error.
	serverErrors := make(chan error, 1)

	// Start the service listening for requests.
	go func() {
		logger.Infof("%s: API listening on %s", logPrefix, cfg.APIHost)
		switch isTLS {
		case false:
			serverErrors <- api.ListenAndServe(apiHost.Host)
		case true:
			serverErrors <- api.ListenAndServeTLS(apiHost.Host, path.Join(cfg.TLS.CertsPath, cfg.TLS.CertFile),
				path.Join(cfg.TLS.CertsPath, cfg.TLS.CertKey))
		}
	}()

	// =========================================================================
	// Start Health API Service

	healthData := handlers.Health{
		Build:  build,
		Logger: logger,
		Pool:   pool,
	}

	// health service handler
	healthHandler := func(ctx *fasthttp.RequestCtx) {
		switch string(ctx.Path()) {
		case "/v1/liveness":
			if err := healthData.Liveness(ctx); err != nil {
				healthData.Logger.Errorf("%s: liveness: %s", logPrefix, err.Error())
			}
		case "/v1/readiness":
			if err := healthData.Readiness(ctx); err != nil {
				healthData.Logger.Errorf("%s: readiness: %s", logPrefix, err.Error())
			}
		default:
			ctx.Error("Unsupported path", fasthttp.StatusNotFound)
		}
	}

	healthApi := fasthttp.Server{
		Handler:               healthHandler,
		ReadTimeout:           cfg.ReadTimeout,
		WriteTimeout:          cfg.WriteTimeout,
		Logger:                logger,
		NoDefaultServerHeader: true,
	}

	// Start the service listening for requests.
	go func() {
		logger.Infof("%s: Health API listening on %s", logPrefix, cfg.HealthAPIHost)
		serverErrors <- healthApi.ListenAndServe(cfg.HealthAPIHost)
	}()

	// =========================================================================
	// Shutdown

	// Blocking main and waiting for shutdown.
	select {
	case err := <-serverErrors:
		return errors.Wrap(err, "server error")

	case sig := <-shutdown:
		logger.Infof("%s: %v: Start shutdown", logPrefix, sig)

		// Asking listener to shutdown and shed load.
		if err := api.Shutdown(); err != nil {
			return errors.Wrap(err, "could not stop server gracefully")
		}
		logger.Infof("%s: %v: Completed shutdown", logPrefix, sig)

		// Close proxy pool
		pool.Close()
	}

	return nil
}
