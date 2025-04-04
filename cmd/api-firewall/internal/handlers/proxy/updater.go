package proxy

import (
	"net/url"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/corazawaf/coraza/v3"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"

	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/allowiplist"
	"github.com/wallarm/api-firewall/internal/platform/denylist"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/storage"
	"github.com/wallarm/api-firewall/internal/platform/storage/updater"
)

const (
	logPrefix = "Regular OpenAPI specification updater"
)

type Specification struct {
	logger         zerolog.Logger
	waf            coraza.WAF
	oasStorage     storage.DBOpenAPILoader
	stop           chan struct{}
	updateTime     time.Duration
	cfg            *config.ProxyMode
	api            *fasthttp.Server
	shutdown       chan os.Signal
	lock           *sync.RWMutex
	pool           proxy.Pool
	serverURL      *url.URL
	deniedTokens   *denylist.DeniedTokens
	allowedIPCache *allowiplist.AllowedIPsType
}

// NewHandlerUpdater function defines configuration updater controller
func NewHandlerUpdater(lock *sync.RWMutex, logger zerolog.Logger, oasStorage storage.DBOpenAPILoader, cfg *config.ProxyMode, serverURL *url.URL, api *fasthttp.Server, shutdown chan os.Signal, pool proxy.Pool, deniedTokens *denylist.DeniedTokens, allowedIPCache *allowiplist.AllowedIPsType, waf coraza.WAF) updater.Updater {
	return &Specification{
		logger:         logger,
		waf:            waf,
		oasStorage:     oasStorage,
		stop:           make(chan struct{}),
		updateTime:     cfg.SpecificationUpdatePeriod,
		cfg:            cfg,
		api:            api,
		shutdown:       shutdown,
		lock:           lock,
		pool:           pool,
		serverURL:      serverURL,
		deniedTokens:   deniedTokens,
		allowedIPCache: allowedIPCache,
	}
}

// Run function performs update of the specification
func (s *Specification) Run() {

	// handle panic
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error().Msgf("panic: %v", r)

			// Log the Go stack trace for this panic'd goroutine.
			s.logger.Debug().Msgf("%s", debug.Stack())
			return
		}
	}()

	updateTicker := time.NewTicker(s.updateTime)
	for {
		select {
		case <-updateTicker.C:

			// load new schemes
			newSpecDB, err := s.Load()
			if err != nil {
				s.logger.Error().Err(err).Msgf("%s: loading specifications", logPrefix)
				continue
			}

			if s.oasStorage.ShouldUpdate(newSpecDB) {

				s.lock.Lock()
				s.oasStorage = newSpecDB
				s.api.Handler = Handlers(s.lock, s.cfg, s.serverURL, s.shutdown, s.logger, s.pool, s.oasStorage, s.deniedTokens, s.allowedIPCache, s.waf)
				if err := s.oasStorage.AfterLoad(s.cfg.APISpecs); err != nil {
					s.logger.Error().Err(err).Msgf("%s: error in after specification loading function", logPrefix)
				}
				s.lock.Unlock()

				s.logger.Debug().Msgf("%s: OpenAPI specification has been updated", logPrefix)

				continue
			}
		case <-s.stop:
			updateTicker.Stop()
			return
		}
	}
}

// Start function starts update process every ConfigurationUpdatePeriod
func (s *Specification) Start() error {
	go s.Run()

	<-s.stop
	return nil
}

// Shutdown function stops update process
func (s *Specification) Shutdown() error {
	defer s.logger.Info().Msgf("%s: stopped", logPrefix)

	// close worker and finish Start function
	for i := 0; i < 2; i++ {
		s.stop <- struct{}{}
	}

	return nil
}

// Load function reads DB file and returns it
func (s *Specification) Load() (storage.DBOpenAPILoader, error) {
	return storage.NewOpenAPIFromFileOrURL(s.cfg.APISpecs, &s.cfg.APISpecsCustomHeader)
}

// Find function searches for the handler by path and method
func (s *Specification) Find(rctx *router.Context, schemaID int, method, path string) (router.Handler, error) {
	return nil, nil
}
