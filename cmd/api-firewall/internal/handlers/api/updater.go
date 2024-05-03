package api

import (
	"os"
	"runtime/debug"
	"slices"
	"sync"
	"time"

	"github.com/corazawaf/coraza/v3"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/allowiplist"
	"github.com/wallarm/api-firewall/internal/platform/database"
	"github.com/wallarm/api-firewall/internal/platform/database/updater"
	"github.com/wallarm/api-firewall/internal/platform/router"
)

const (
	logPrefix = "Regular OpenAPI specification updater"
)

type Specification struct {
	logger         *logrus.Logger
	waf            coraza.WAF
	sqlLiteStorage database.DBOpenAPILoader
	stop           chan struct{}
	updateTime     time.Duration
	cfg            *config.APIMode
	api            *fasthttp.Server
	shutdown       chan os.Signal
	health         *Health
	lock           *sync.RWMutex
	allowedIPCache *allowiplist.AllowedIPsType
}

// NewHandlerUpdater function defines configuration updater controller
func NewHandlerUpdater(lock *sync.RWMutex, logger *logrus.Logger, sqlLiteStorage database.DBOpenAPILoader, cfg *config.APIMode, api *fasthttp.Server, shutdown chan os.Signal, health *Health, allowedIPCache *allowiplist.AllowedIPsType, waf coraza.WAF) updater.Updater {
	return &Specification{
		logger:         logger,
		waf:            waf,
		sqlLiteStorage: sqlLiteStorage,
		stop:           make(chan struct{}),
		updateTime:     cfg.SpecificationUpdatePeriod,
		cfg:            cfg,
		api:            api,
		shutdown:       shutdown,
		health:         health,
		lock:           lock,
		allowedIPCache: allowedIPCache,
	}
}

// Run function performs update of the specification
func (s *Specification) Run() {

	// handle panic
	defer func() {
		if r := recover(); r != nil {
			s.logger.Errorf("panic: %v", r)

			// Log the Go stack trace for this panic'd goroutine.
			s.logger.Debugf("%s", debug.Stack())
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
				s.logger.WithFields(logrus.Fields{"error": err}).Errorf("%s: loading specifications", logPrefix)
				continue
			}

			// do not downgrade the db version
			if s.sqlLiteStorage.Version() > newSpecDB.Version() {
				s.logger.Errorf("%s: version of the new DB structure is lower then current one (V2)", logPrefix)
				continue
			}

			if s.sqlLiteStorage.ShouldUpdate(newSpecDB) {
				s.logger.Debugf("%s: OpenAPI specifications with the following IDs were updated: %v", logPrefix, newSpecDB.SchemaIDs())

				// find new IDs and log them
				newScemaIDs := newSpecDB.SchemaIDs()
				oldSchemaIDs := s.sqlLiteStorage.SchemaIDs()
				for _, ns := range newScemaIDs {
					if !slices.Contains(oldSchemaIDs, ns) {
						s.logger.Infof("%s: fetched new OpenAPI specification from the database with id: %d", logPrefix, ns)
					}
				}

				s.lock.Lock()
				s.sqlLiteStorage = newSpecDB
				s.api.Handler = Handlers(s.lock, s.cfg, s.shutdown, s.logger, s.sqlLiteStorage, s.allowedIPCache, s.waf)
				s.health.OpenAPIDB = s.sqlLiteStorage
				if err := s.sqlLiteStorage.AfterLoad(s.cfg.PathToSpecDB); err != nil {
					s.logger.WithFields(logrus.Fields{"error": err}).Errorf("%s: error in after specification loading function", logPrefix)
				}
				s.lock.Unlock()

				continue
			}

			s.logger.Debugf("%s: new OpenAPI specifications not found", logPrefix)
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
	defer s.logger.Infof("%s: stopped", logPrefix)

	// close worker and finish Start function
	for i := 0; i < 2; i++ {
		s.stop <- struct{}{}
	}

	return nil
}

// Load function reads DB file and returns it
func (s *Specification) Load() (database.DBOpenAPILoader, error) {

	// Load specification
	return database.NewOpenAPIDB(s.cfg.PathToSpecDB, s.cfg.DBVersion)
}

// Find function searches for the handler by path and method
func (s *Specification) Find(rctx *router.Context, schemaID int, method, path string) (router.Handler, error) {
	return nil, nil
}
