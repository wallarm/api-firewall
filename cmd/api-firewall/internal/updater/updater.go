package updater

import (
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	handlersAPI "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/api"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/database"
)

type Updater interface {
	Start() error
	Shutdown() error
	Load() (database.DBOpenAPILoader, error)
}

type Specification struct {
	logger         *logrus.Logger
	sqlLiteStorage database.DBOpenAPILoader
	stop           chan struct{}
	updateTime     time.Duration
	cfg            *config.APIMode
	api            *fasthttp.Server
	shutdown       chan os.Signal
	health         *handlersAPI.Health
	lock           *sync.RWMutex
}

// NewController function defines configuration updater controller
func NewController(lock *sync.RWMutex, logger *logrus.Logger, sqlLiteStorage database.DBOpenAPILoader, cfg *config.APIMode, api *fasthttp.Server, shutdown chan os.Signal, health *handlersAPI.Health) Updater {
	return &Specification{
		logger:         logger,
		sqlLiteStorage: sqlLiteStorage,
		stop:           make(chan struct{}),
		updateTime:     cfg.SpecificationUpdatePeriod,
		cfg:            cfg,
		api:            api,
		shutdown:       shutdown,
		health:         health,
		lock:           lock,
	}
}

// Run function performs update of the specification
func (s *Specification) Run() {
	updateTicker := time.NewTicker(s.updateTime)
	for {
		select {
		case <-updateTicker.C:

			// load new schemes
			newSpecDB, err := s.Load()
			if err != nil {
				s.logger.WithFields(logrus.Fields{"error": err}).Error("updating OpenAPI specification")
				continue
			}

			// do not downgrade the db version
			if s.sqlLiteStorage.Version() > newSpecDB.Version() {
				s.logger.Error("regular update checker: version of the new OpenAPI specification is lower then current")
				continue
			}

			if s.sqlLiteStorage.ShouldUpdate(newSpecDB) {
				s.logger.Debugf("OpenAPI specifications has been updated. The schemas with the following IDs were updated: %v", newSpecDB.SchemaIDs())

				s.lock.Lock()
				s.sqlLiteStorage = newSpecDB
				s.api.Handler = handlersAPI.Handlers(s.lock, s.cfg, s.shutdown, s.logger, s.sqlLiteStorage)
				s.health.OpenAPIDB = s.sqlLiteStorage
				if err := s.sqlLiteStorage.AfterLoad(s.cfg.PathToSpecDB); err != nil {
					s.logger.WithFields(logrus.Fields{"error": err}).Error("regular update checker: error in after specification loading function")
				}
				s.lock.Unlock()

				continue
			}

			s.logger.Debugf("regular update checker: new OpenAPI specifications not found")
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
	defer s.logger.Infof("specification updater: stopped")

	// close worker and finish Start function
	for i := 0; i < 2; i++ {
		s.stop <- struct{}{}
	}

	return nil
}

// Load function reads DB file and returns it
func (s *Specification) Load() (database.DBOpenAPILoader, error) {

	// Load specification
	return database.NewOpenAPIDB(s.logger, s.cfg.PathToSpecDB, s.cfg.DBVersion)
}
