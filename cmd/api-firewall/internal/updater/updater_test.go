package updater

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	handlersAPI "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/api"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/database"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

const (
	DefaultSchemaID = 1
)

var cfg = config.APIMode{
	APIFWMode:                  config.APIFWMode{Mode: web.APIMode},
	SpecificationUpdatePeriod:  2 * time.Second,
	PathToSpecDB:               "./wallarm_api_after_update.db",
	UnknownParametersDetection: true,
	PassOptionsRequests:        false,
}

func TestUpdaterBasic(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// load spec from the database
	specStorage, err := database.NewOpenAPIDB(logger, "./wallarm_api_before_update.db")
	if err != nil {
		t.Fatal(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{}
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage)
	health := handlersAPI.Health{}

	// invalid route in the old spec
	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/new")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	lock.RLock()
	api.Handler(&reqCtx)
	lock.RUnlock()

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	// valid route in the old spec
	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	lock.RLock()
	api.Handler(&reqCtx)
	lock.RUnlock()

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// start updater
	updSpecErrors := make(chan error, 1)
	updater := NewController(&lock, logger, specStorage, &cfg, &api, shutdown, &health)
	go func() {
		t.Logf("starting specification regular update process every %.0f seconds", cfg.SpecificationUpdatePeriod.Seconds())
		updSpecErrors <- updater.Start()
	}()

	time.Sleep(3 * time.Second)

	if err := updater.Shutdown(); err != nil {
		t.Fatal(err)
	}

	// valid route in the new spec
	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/new")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	lock.RLock()
	api.Handler(&reqCtx)
	lock.RUnlock()

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// invalid route in the new spec
	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	lock.RLock()
	api.Handler(&reqCtx)
	lock.RUnlock()

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}
