package main

import (
	"context"
	"fmt"
	"github.com/ardanlabs/conf"
	"github.com/go-playground/validator"
	"github.com/pkg/errors"
	"github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wundergraph/graphql-go-tools/pkg/graphql"
	"mime"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"

	handlersAPI "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/api"
	handlersGQL "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/graphql"
	handlersProxy "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/proxy"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/allowiplist"
	"github.com/wallarm/api-firewall/internal/platform/denylist"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/storage"
	"github.com/wallarm/api-firewall/internal/platform/web"
	"github.com/wallarm/api-firewall/internal/version"
)

const (
	namespace   = "apifw"
	logPrefix   = "main"
	projectName = "Wallarm API-Firewall"
)

const (
	initialPoolCapacity = 100
	livenessEndpoint    = "/v1/liveness"
	readinessEndpoint   = "/v1/readiness"
)

func main() {
	logger := logrus.New()

	logger.SetLevel(logrus.DebugLevel)

	cFormatter := &config.CustomFormatter{}
	cFormatter.DisableQuote = true
	cFormatter.FullTimestamp = true
	cFormatter.DisableLevelTruncation = true

	logger.SetFormatter(cFormatter)

	// if MODE var has invalid value then proxy mode will be used
	var currentMode config.APIFWMode
	if err := conf.Parse(os.Args[1:], namespace, &currentMode); err != nil {
		if err := runProxyMode(logger); err != nil {
			logger.Infof("%s: error: %s", logPrefix, err)
			os.Exit(1)
		}
		return
	}

	// if MODE var has valid or default value then an appropriate mode will be used
	switch strings.ToLower(currentMode.Mode) {
	case web.APIMode:
		if err := runAPIMode(logger); err != nil {
			logger.Infof("%s: error: %s", logPrefix, err)
			os.Exit(1)
		}
	case web.GraphQLMode:
		if err := runGraphQLMode(logger); err != nil {
			logger.Infof("%s: error: %s", logPrefix, err)
			os.Exit(1)
		}
	default:
		if err := runProxyMode(logger); err != nil {
			logger.Infof("%s: error: %s", logPrefix, err)
			os.Exit(1)
		}
	}

}

func runAPIMode(logger *logrus.Logger) error {

	// =========================================================================
	// Configuration

	var cfg config.APIMode
	cfg.Version.SVN = version.Version
	cfg.Version.Desc = projectName

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

	// =========================================================================
	// Init Logger

	if strings.EqualFold(cfg.LogFormat, "json") {
		logger.SetFormatter(&logrus.JSONFormatter{})
	}

	switch strings.ToLower(cfg.LogLevel) {
	case "trace":
		logger.SetLevel(logrus.TraceLevel)
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

	logger.Infof("%s : Started : Application initializing : version %q", logPrefix, version.Version)
	defer logger.Infof("%s: Completed", logPrefix)

	out, err := conf.String(&cfg)
	if err != nil {
		return errors.Wrap(err, "generating config for output")
	}
	logger.Infof("%s: Configuration Loaded :\n%v\n", logPrefix, out)

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
		logger.Errorf("%s: trying to load API Spec value from SQLLite Database : %v\n", logPrefix, err.Error())
	}

	if specStorage != nil {
		logger.Debugf("OpenAPI specifications with the following IDs were found in the DB: %v", specStorage.SchemaIDs())
	}

	// =========================================================================
	// Init ModSecurity Core

	waf, err := config.LoadModSecurityConfiguration(logger, &cfg.ModSecurity)
	if err != nil {
		logger.Fatal(err)
		return err
	}

	if waf != nil {
		logger.Infof("%s: The ModSecurity configuration has been loaded successfully", logPrefix)
	}

	// Init Allow IP List

	logger.Infof("%s: Initializing IP Whitelist Cache", logPrefix)

	allowedIPCache, err := allowiplist.New(&cfg.AllowIP, logger)
	if err != nil {
		return errors.Wrap(err, "The allow IP list init error")
	}

	switch allowedIPCache {
	case nil:
		logger.Infof("%s: The allow IP list is not configured", logPrefix)
	default:
		logger.Infof("%s: Loaded %d Whitelisted IP's to the cache", logPrefix, allowedIPCache.ElementsNum)
	}

	// =========================================================================
	// Init Handlers
	requestHandlers := handlersAPI.Handlers(&dbLock, &cfg, shutdown, logger, specStorage, allowedIPCache, waf)

	// =========================================================================
	// Start Health API Service

	healthData := handlersAPI.Health{
		Logger:    logger,
		OpenAPIDB: specStorage,
	}

	// health service handler
	healthHandler := func(ctx *fasthttp.RequestCtx) {
		switch string(ctx.Path()) {
		case livenessEndpoint:
			if err := healthData.Liveness(ctx); err != nil {
				healthData.Logger.Errorf("%s: liveness: %s", logPrefix, err.Error())
			}
		case readinessEndpoint:
			if err := healthData.Readiness(ctx); err != nil {
				healthData.Logger.Errorf("%s: readiness: %s", logPrefix, err.Error())
			}
		default:
			ctx.Error("Unsupported path", fasthttp.StatusNotFound)
		}
	}

	healthAPI := fasthttp.Server{
		Handler:               healthHandler,
		ReadTimeout:           cfg.ReadTimeout,
		WriteTimeout:          cfg.WriteTimeout,
		Logger:                logger,
		NoDefaultServerHeader: true,
	}

	// Start the service listening for requests.
	go func() {
		logger.Infof("%s: Health API listening on %s", logPrefix, cfg.HealthAPIHost)
		serverErrors <- healthAPI.ListenAndServe(cfg.HealthAPIHost)
	}()

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
			logger.WithFields(logrus.Fields{
				"error":  err,
				"host":   strconv.B2S(ctx.Request.Header.Host()),
				"path":   strconv.B2S(ctx.Path()),
				"method": strconv.B2S(ctx.Request.Header.Method()),
			}).Error("request processing error")

			ctx.Error("", fasthttp.StatusInternalServerError)
		},
		Logger:                logger,
		NoDefaultServerHeader: true,
	}

	// =========================================================================
	// Init Regular Update Controller

	updSpecErrors := make(chan error, 1)

	updOpenAPISpec := handlersAPI.NewHandlerUpdater(&dbLock, logger, specStorage, &cfg, &api, shutdown, &healthData, allowedIPCache, waf)

	// disable updater if SpecificationUpdatePeriod == 0
	if cfg.SpecificationUpdatePeriod.Seconds() > 0 {
		go func() {
			logger.Infof("%s: starting specification regular update process every %.0f seconds", logPrefix, cfg.SpecificationUpdatePeriod.Seconds())
			updSpecErrors <- updOpenAPISpec.Start()
		}()
	}

	// start the service listening for requests.
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
	// Shutdown

	// blocking main and waiting for shutdown.
	select {
	case err := <-serverErrors:
		return errors.Wrap(err, "server error")

	case err := <-updSpecErrors:
		return errors.Wrap(err, "regular updater error")

	case sig := <-shutdown:
		logger.Infof("%s: %v: Start shutdown", logPrefix, sig)

		if cfg.SpecificationUpdatePeriod.Seconds() > 0 {
			if err := updOpenAPISpec.Shutdown(); err != nil {
				return errors.Wrap(err, "could not stop configuration updater gracefully")
			}
		}

		// asking listener to shutdown and shed load.
		if err := api.Shutdown(); err != nil {
			return errors.Wrap(err, "could not stop server gracefully")
		}
		logger.Infof("%s: %v: Completed shutdown", logPrefix, sig)
	}

	return nil

}

func runGraphQLMode(logger *logrus.Logger) error {

	// =========================================================================
	// Configuration

	var cfg config.GraphQLMode
	cfg.Version.SVN = version.Version
	cfg.Version.Desc = projectName

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

	// =========================================================================
	// Init Logger

	if strings.EqualFold(cfg.LogFormat, "json") {
		logger.SetFormatter(&logrus.JSONFormatter{})
	}

	switch strings.ToLower(cfg.LogLevel) {
	case "trace":
		logger.SetLevel(logrus.TraceLevel)
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

	logger.Infof("%s : Started : Application initializing : version %q", logPrefix, version.Version)
	defer logger.Infof("%s: Completed", logPrefix)

	out, err := conf.String(&cfg)
	if err != nil {
		return errors.Wrap(err, "generating config for output")
	}
	logger.Infof("%s: Configuration Loaded :\n%v\n", logPrefix, out)

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
		logger.Fatalf("Loading GraphQL Schema error: %v", err)
		return err
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromReader(f)
	if err != nil {
		logger.Fatalf("Loading GraphQL Schema error: %v", err)
		return err
	}

	validationRes, err := schema.Validate()
	if err != nil {
		logger.Fatalf("GraphQL Schema validator error: %v", err)
		return err
	}

	if !validationRes.Valid {
		logger.Fatalf("GraphQL Schema validator error: %v", validationRes.Errors)
		return validationRes.Errors
	}

	if err := f.Close(); err != nil {
		logger.Fatalf("Loading GraphQL Schema error: %v", err)
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

	logger.Infof("%s: Initializing DenyList Cache", logPrefix)

	deniedTokens, err := denylist.New(&cfg.Denylist, logger)
	if err != nil {
		return errors.Wrap(err, "denylist init error")
	}

	switch deniedTokens {
	case nil:
		logger.Infof("%s: Denylist not configured", logPrefix)
	default:
		logger.Infof("%s: Loaded %d tokens to the cache", logPrefix, deniedTokens.ElementsNum)
	}

	// =========================================================================
	// Init Allow IP List

	logger.Infof("%s: Initializing IP Whitelist Cache", logPrefix)

	allowedIPCache, err := allowiplist.New(&cfg.AllowIP, logger)
	if err != nil {
		return errors.Wrap(err, "The allow IP list init error")
	}

	switch allowedIPCache {
	case nil:
		logger.Infof("%s: The allow ip list is not configured", logPrefix)
	default:
		logger.Infof("%s: Loaded %d Whitelisted IP's to the cache", logPrefix, allowedIPCache.ElementsNum)
	}

	// =========================================================================
	// Init Handlers

	requestHandlers := handlersGQL.Handlers(&cfg, schema, serverURL, shutdown, logger, pool, wsPool, deniedTokens, allowedIPCache)

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
				healthData.Logger.Errorf("%s: liveness: %s", logPrefix, err.Error())
			}
		case readinessEndpoint:
			if err := healthData.Readiness(ctx); err != nil {
				healthData.Logger.Errorf("%s: readiness: %s", logPrefix, err.Error())
			}
		default:
			ctx.Error("Unsupported path", fasthttp.StatusNotFound)
		}
	}

	healthAPI := fasthttp.Server{
		Handler:               healthHandler,
		ReadTimeout:           cfg.ReadTimeout,
		WriteTimeout:          cfg.WriteTimeout,
		Logger:                logger,
		NoDefaultServerHeader: true,
	}

	// Start the service listening for requests.
	go func() {
		logger.Infof("%s: Health API listening on %s", logPrefix, cfg.HealthAPIHost)
		serverErrors <- healthAPI.ListenAndServe(cfg.HealthAPIHost)
	}()

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
			logger.WithFields(logrus.Fields{
				"error": err,
			}).Error("request processing error")

			ctx.Error("", fasthttp.StatusForbidden)
		},
		Logger:                logger,
		NoDefaultServerHeader: true,
	}

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

func runProxyMode(logger *logrus.Logger) error {

	// =========================================================================
	// Configuration

	var cfg config.ProxyMode
	cfg.Version.SVN = version.Version
	cfg.Version.Desc = projectName

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

	// validate env parameter values
	validate := validator.New()

	if err := validate.RegisterValidation("HttpStatusCodes", config.ValidateStatusList); err != nil {
		return errors.Errorf("Configuration validator error: %s", err.Error())
	}

	if err := validate.Struct(cfg); err != nil {

		for _, err := range err.(validator.ValidationErrors) {
			switch err.Tag() {
			case "gt":
				return errors.Errorf("configuration validator error: parameter %s should be > %s. Actual value: %d", err.Field(), err.Param(), err.Value())
			case "url":
				return errors.Errorf("configuration validator error: parameter %s should be a string in URL format. Example: http://localhost:8080/; actual value: %s", err.Field(), err.Value())
			case "oneof":
				return errors.Errorf("configuration validator error: parameter %s should have one of the following value: %s; actual value: %s", err.Field(), err.Param(), err.Value())
			}
		}
		return errors.Wrap(err, "configuration validator error")
	}

	// oauth introspection endpoint: validate format of configured content-type
	if cfg.Server.Oauth.Introspection.ContentType != "" {
		_, _, err := mime.ParseMediaType(cfg.Server.Oauth.Introspection.ContentType)
		if err != nil {
			return errors.Wrap(err, "configuration validator error")
		}
	}

	// =========================================================================
	// Init Logger

	if strings.EqualFold(cfg.LogFormat, "json") {
		logger.SetFormatter(&logrus.JSONFormatter{})
	}

	switch strings.ToLower(cfg.LogLevel) {
	case "trace":
		logger.SetLevel(logrus.TraceLevel)
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

	logger.Infof("%s : Started : Application initializing : version %q", logPrefix, version.Version)
	defer logger.Infof("%s: Completed", logPrefix)

	out, err := conf.String(&cfg)
	if err != nil {
		return errors.Wrap(err, "generating config for output")
	}
	logger.Infof("%s: Configuration Loaded :\n%v\n", logPrefix, out)

	var requestHandlers fasthttp.RequestHandler

	// Make a channel to listen for an interrupt or terminate signal from the OS.
	// Use a buffered channel because the signal package requires it.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Make a channel to listen for errors coming from the listener. Use a
	// buffered channel so the goroutine can exit if we don't collect this error.
	serverErrors := make(chan error, 1)

	// OAS Usage Lock
	var lock sync.RWMutex

	// =========================================================================
	// Init Swagger

	specStorage, err := storage.NewOpenAPIFromFileOrURL(cfg.APISpecs, &cfg.APISpecsCustomHeader)
	if err != nil {
		return errors.Wrap(err, "loading OpenAPI specification from File or URL")
	}

	// =========================================================================
	// Init Proxy Client

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

	// default DNS resolver
	resolver := &net.Resolver{
		PreferGo:     true,
		StrictErrors: false,
	}

	// configuration of the custom DNS server
	if cfg.DNS.Nameserver.Host != "" {
		var builder strings.Builder
		builder.WriteString(cfg.DNS.Nameserver.Host)
		builder.WriteString(":")
		builder.WriteString(cfg.DNS.Nameserver.Port)

		resolver.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: cfg.DNS.LookupTimeout,
			}
			return d.DialContext(ctx, cfg.DNS.Nameserver.Proto, builder.String())
		}
	}

	// init DNS resolver
	dnsCacheOptions := proxy.DNSCacheOptions{
		UseCache:      cfg.DNS.Cache,
		Logger:        logger,
		FetchTimeout:  cfg.DNS.FetchTimeout,
		LookupTimeout: cfg.DNS.LookupTimeout,
	}

	dnsResolver, err := proxy.NewDNSResolver(resolver, &dnsCacheOptions)
	if err != nil {
		return errors.Wrap(err, "DNS cache resolver init")
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
		DNSConfig:           cfg.DNS,
		Logger:              logger,
		DNSResolver:         dnsResolver,
	}
	pool, err := proxy.NewChanPool(host, &options)
	if err != nil {
		return errors.Wrap(err, "proxy pool init")
	}

	// =========================================================================
	// Init Deny List Cache

	logger.Infof("%s: Initializing Token Cache", logPrefix)

	deniedTokens, err := denylist.New(&cfg.Denylist, logger)
	if err != nil {
		return errors.Wrap(err, "denylist init error")
	}

	switch deniedTokens {
	case nil:
		logger.Infof("%s: Denylist not configured", logPrefix)
	default:
		logger.Infof("%s: Loaded %d tokens to the cache", logPrefix, deniedTokens.ElementsNum)
	}

	// =========================================================================
	// Init IP Allow List Cache

	logger.Infof("%s: Initializing IP Whitelist Cache", logPrefix)

	allowedIPCache, err := allowiplist.New(&cfg.AllowIP, logger)
	if err != nil {
		return errors.Wrap(err, "The allow IP list init error")
	}

	switch allowedIPCache {
	case nil:
		logger.Infof("%s: The allow ip list is not configured", logPrefix)
	default:
		logger.Infof("%s: Loaded %d Whitelisted IP's to the cache", logPrefix, allowedIPCache.ElementsNum)
	}

	// =========================================================================
	// Init ModSecurity Core

	waf, err := config.LoadModSecurityConfiguration(logger, &cfg.ModSecurity)
	if err != nil {
		logger.Fatal(err)
		return err
	}

	if waf != nil {
		logger.Infof("%s: The ModSecurity configuration has been loaded successfully", logPrefix)
	}

	// =========================================================================
	// Init Handlers

	requestHandlers = handlersProxy.Handlers(&lock, &cfg, serverURL, shutdown, logger, pool, specStorage, deniedTokens, allowedIPCache, waf)

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
				healthData.Logger.Errorf("%s: liveness: %s", logPrefix, err.Error())
			}
		case readinessEndpoint:
			if err := healthData.Readiness(ctx); err != nil {
				healthData.Logger.Errorf("%s: readiness: %s", logPrefix, err.Error())
			}
		default:
			ctx.Error("Unsupported path", fasthttp.StatusNotFound)
		}
	}

	healthAPI := fasthttp.Server{
		Handler:               healthHandler,
		ReadTimeout:           cfg.ReadTimeout,
		WriteTimeout:          cfg.WriteTimeout,
		Logger:                logger,
		NoDefaultServerHeader: true,
	}

	// Start the service listening for requests.
	go func() {
		logger.Infof("%s: Health API listening on %s", logPrefix, cfg.HealthAPIHost)
		serverErrors <- healthAPI.ListenAndServe(cfg.HealthAPIHost)
	}()

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
			logger.WithFields(logrus.Fields{
				"error": err,
			}).Error("request processing error")

			ctx.Error("", cfg.CustomBlockStatusCode)
		},
		Logger:                logger,
		NoDefaultServerHeader: true,
	}

	// =========================================================================
	// Init Regular Update Controller

	updSpecErrors := make(chan error, 1)

	updOpenAPISpec := handlersProxy.NewHandlerUpdater(&lock, logger, specStorage, &cfg, serverURL, &api, shutdown, pool, deniedTokens, allowedIPCache, waf)

	// disable updater if SpecificationUpdatePeriod == 0
	if cfg.SpecificationUpdatePeriod.Seconds() > 0 {
		go func() {
			logger.Infof("%s: starting specification regular update process every %.0f seconds", logPrefix, cfg.SpecificationUpdatePeriod.Seconds())
			updSpecErrors <- updOpenAPISpec.Start()
		}()
	}

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
