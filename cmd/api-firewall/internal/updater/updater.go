package updater

import (
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	handlersAPI "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/api"
	"github.com/wallarm/api-firewall/internal/config"
	coraza "github.com/wallarm/api-firewall/internal/modsec"
	"github.com/wallarm/api-firewall/internal/platform/database"
)

type Updater interface {
	Start() error
	Shutdown() error
	Load() (database.DBOpenAPILoader, error)
}

type Specification struct {
	logger         *logrus.Logger
	waf            coraza.WAF
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
func NewController(lock *sync.RWMutex, logger *logrus.Logger, sqlLiteStorage database.DBOpenAPILoader, cfg *config.APIMode, api *fasthttp.Server, shutdown chan os.Signal, health *handlersAPI.Health, waf coraza.WAF) Updater {
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
	}
}

func getSchemaVersions(dbSpecs database.DBOpenAPILoader) map[int]string {
	result := make(map[int]string)
	schemaIDs := dbSpecs.SchemaIDs()
	for _, schemaID := range schemaIDs {
		result[schemaID] = dbSpecs.SpecificationVersion(schemaID)
	}
	return result
}

// Run function performs update of the specification
func (s *Specification) Run() {
	updateTicker := time.NewTicker(s.updateTime)
	for {
		select {
		case <-updateTicker.C:
			beforeUpdateSpecs := getSchemaVersions(s.sqlLiteStorage)
			newSpecDB, err := s.Load()
			if err != nil {
				s.logger.WithFields(logrus.Fields{"error": err}).Error("updating OpenAPI specification")
				continue
			}
			afterUpdateSpecs := getSchemaVersions(newSpecDB)
			if !reflect.DeepEqual(beforeUpdateSpecs, afterUpdateSpecs) {
				s.logger.Debugf("OpenAPI specifications has been updated. Loaded OpenAPI specification versions: %v", afterUpdateSpecs)
				s.lock.Lock()
				s.sqlLiteStorage = newSpecDB
				s.api.Handler = handlersAPI.Handlers(s.lock, s.cfg, s.shutdown, s.logger, s.sqlLiteStorage, s.waf)
				s.health.OpenAPIDB = s.sqlLiteStorage
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
	return database.NewOpenAPIDB(s.logger, s.cfg.PathToSpecDB)
}
