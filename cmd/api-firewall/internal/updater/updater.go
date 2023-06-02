package updater

import (
	"fmt"
	"os"
	"reflect"
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
	Update() error
}

type Specification struct {
	logger         *logrus.Logger
	sqlLiteStorage database.DBOpenAPILoader
	stop           chan struct{}
	updateTime     time.Duration
	cfg            *config.APIFWConfigurationAPIMode
	api            *fasthttp.Server
	shutdown       chan os.Signal
	health         *handlersAPI.Health
	lock           *sync.RWMutex
}

// NewController function defines configuration updater controller
func NewController(lock *sync.RWMutex, logger *logrus.Logger, sqlLiteStorage database.DBOpenAPILoader, cfg *config.APIFWConfigurationAPIMode, api *fasthttp.Server, shutdown chan os.Signal, health *handlersAPI.Health) Updater {
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

func getSchemaVersions(dbSpecs database.DBOpenAPILoader) map[int]string {
	result := make(map[int]string)
	schemaIDs := dbSpecs.SchemaIDs()
	for _, schemaID := range schemaIDs {
		result[schemaID] = dbSpecs.SpecificationVersion(schemaID)
	}
	return result
}

// Start function starts update process every ConfigurationUpdatePeriod
func (s *Specification) Start() error {

	go func() {
		updateTicker := time.NewTicker(s.updateTime)
		for {
			select {
			case <-updateTicker.C:
				beforeUpdateSpecs := getSchemaVersions(s.sqlLiteStorage)
				if err := s.Update(); err != nil {
					s.logger.WithFields(logrus.Fields{"error": err}).Error("updating OpenAPI specification")
					continue
				}
				afterUpdateSpecs := getSchemaVersions(s.sqlLiteStorage)
				if !reflect.DeepEqual(beforeUpdateSpecs, afterUpdateSpecs) {
					s.logger.Debugf("OpenAPI specifications has been updated. Loaded OpenAPI specification versions: %v", afterUpdateSpecs)
					s.lock.Lock()
					s.api.Handler = handlersAPI.Handlers(s.lock, s.cfg, s.shutdown, s.logger, s.sqlLiteStorage)
					s.health.OpenAPIDB = s.sqlLiteStorage
					s.lock.Unlock()
					continue
				}
				s.logger.Debugf("regular update checker: new OpenAPI specifications not found")
			}
		}
	}()

	<-s.stop
	return nil
}

// Shutdown function stops update process
func (s *Specification) Shutdown() error {
	defer s.logger.Infof("specification updater: stopped")
	s.stop <- struct{}{}
	return nil
}

// Update function performs a specification update
func (s *Specification) Update() error {

	// Update specification
	if err := s.sqlLiteStorage.Load(s.cfg.PathToSpecDB); err != nil {
		return fmt.Errorf("error while spicification update: %w", err)
	}

	return nil
}
