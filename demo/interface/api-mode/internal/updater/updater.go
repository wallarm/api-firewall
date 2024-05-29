package updater

import (
	"time"

	"github.com/sirupsen/logrus"
	"github.com/wallarm/api-firewall/pkg/APIMode"
)

const (
	logPrefix = "Regular OpenAPI specification updater"
)

type DatabaseUpdater interface {
	Start() error
	Shutdown() error
}

type Specification struct {
	logger     *logrus.Logger
	stop       chan struct{}
	updateTime time.Duration
	apifw      APIMode.APIFirewall
}

// NewUpdater function defines configuration updater controller
func NewUpdater(logger *logrus.Logger, apifw APIMode.APIFirewall, updateTime time.Duration) DatabaseUpdater {
	return &Specification{
		logger:     logger,
		stop:       make(chan struct{}),
		updateTime: updateTime,
		apifw:      apifw,
	}
}

// Run function performs update of the specification
func (s *Specification) Run() {

	updateTicker := time.NewTicker(s.updateTime)
	for {
		select {
		case <-updateTicker.C:

			currentSIDs, isUpdated, err := s.apifw.UpdateSpecsStorage()
			if err != nil {
				s.logger.Errorf("%s: error while OpenAPI specifications update: %v", logPrefix, err)
			}

			if isUpdated {
				s.logger.Debugf("%s: OpenAPI specifications were updated: current schema IDs list %v", logPrefix, currentSIDs)
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
	defer s.logger.Infof("%s: stopped", logPrefix)

	// close worker and finish Start function
	for i := 0; i < 2; i++ {
		s.stop <- struct{}{}
	}

	return nil
}
