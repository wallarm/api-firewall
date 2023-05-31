package updater

import (
	"fmt"
	handlersAPI "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/api"
	"net/url"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/database"
	"github.com/wallarm/api-firewall/internal/platform/router"
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
	swagRouter     *router.Router
	cfg            *config.APIFWConfiguration
	api            *fasthttp.Server
	serverURL      *url.URL
	shutdown       chan os.Signal
}

// NewController function defines configuration updater controller
func NewController(logger *logrus.Logger, sqlLiteStorage database.DBOpenAPILoader, cfg *config.APIFWConfiguration, api *fasthttp.Server, serverURL *url.URL, shutdown chan os.Signal, swagRouter *router.Router) Updater {
	return &Specification{
		logger:         logger,
		sqlLiteStorage: sqlLiteStorage,
		stop:           make(chan struct{}),
		updateTime:     cfg.SpecificationUpdatePeriod,
		cfg:            cfg,
		swagRouter:     swagRouter,
		api:            api,
		serverURL:      serverURL,
		shutdown:       shutdown,
	}
}

// Start function starts update process every ConfigurationUpdatePeriod
func (s *Specification) Start() error {

	go func() {
		updateTicker := time.NewTicker(s.updateTime)
		for {
			select {
			case <-updateTicker.C:
				currentVersion := s.sqlLiteStorage.SpecificationVersion()
				if err := s.Update(); err != nil {
					s.logger.WithFields(logrus.Fields{"error": err}).Error("updating OpenAPI specification")
				}
				s.logger.Debugf("OpenAPI specification has been updated. Loaded OpenAPI specification version: %s", s.sqlLiteStorage.SpecificationVersion())
				if s.sqlLiteStorage.SpecificationVersion() != currentVersion {

					// get new router
					newSwagRouter, err := router.NewRouterDBLoader(s.sqlLiteStorage)
					if err != nil {
						s.logger.WithFields(logrus.Fields{"error": err}).Error("new router creation failed")
					}

					s.api.Handler = handlersAPI.APIModeHandlers(s.cfg, s.serverURL, s.shutdown, s.logger, newSwagRouter)
				}
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
	if err := s.sqlLiteStorage.Load(""); err != nil {
		return fmt.Errorf("error while spicification update: %w", err)
	}

	return nil
}
