package updater

import (
	"encoding/json"
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
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil)
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

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}
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

	apifwResponse = web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

	// start updater
	updSpecErrors := make(chan error, 1)
	updater := NewController(&lock, logger, specStorage, &cfg, &api, shutdown, &health, nil)
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

	apifwResponse = web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
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

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse = web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}
	}

}

func TestUpdaterFromEmptyDB(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// load spec from the database
	specStorage, err := database.NewOpenAPIDB(logger, "./wallarm_api_empty.db")
	if err != nil {
		t.Fatal(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{}
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil)
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

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusInternalServerError {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusInternalServerError, *apifwResponse.Summary[0].StatusCode)
		}
	}

	// start updater
	updSpecErrors := make(chan error, 1)
	updater := NewController(&lock, logger, specStorage, &cfg, &api, shutdown, &health, nil)
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

	apifwResponse = web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
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

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse = web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}
	}

}

func TestUpdaterToEmptyDB(t *testing.T) {

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
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil)
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

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}
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

	apifwResponse = web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

	var cfgEmpty = config.APIMode{
		APIFWMode:                  config.APIFWMode{Mode: web.APIMode},
		SpecificationUpdatePeriod:  2 * time.Second,
		PathToSpecDB:               "./wallarm_api_empty.db",
		UnknownParametersDetection: true,
		PassOptionsRequests:        false,
	}

	// start updater
	updSpecErrors := make(chan error, 1)
	updater := NewController(&lock, logger, specStorage, &cfgEmpty, &api, shutdown, &health, nil)
	go func() {
		t.Logf("starting specification regular update process every %.0f seconds", cfg.SpecificationUpdatePeriod.Seconds())
		updSpecErrors <- updater.Start()
	}()

	time.Sleep(3 * time.Second)

	if err := updater.Shutdown(); err != nil {
		t.Fatal(err)
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

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse = web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusInternalServerError {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusInternalServerError, *apifwResponse.Summary[0].StatusCode)
		}
	}

}

func TestUpdaterInvalidDBSchema(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// load spec from the database
	specStorage, err := database.NewOpenAPIDB(logger, "./wallarm_api_invalid_schema.db")
	if err != nil {
		t.Log(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{}
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil)

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

	if reqCtx.Response.StatusCode() != 500 {
		t.Errorf("Incorrect response status code. Expected: 500 and got %d",
			reqCtx.Response.StatusCode())
	}

	if len(reqCtx.Response.Body()) > 0 {
		t.Errorf("Incorrect response body size. Expected: 0 and got %d",
			len(reqCtx.Response.Body()))
		t.Logf("Response body: %s", string(reqCtx.Response.Body()))
	}
}

func TestUpdaterInvalidDBFile(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// load spec from the database
	specStorage, err := database.NewOpenAPIDB(logger, "./wallarm_api_invalid_file.db")
	if err != nil {
		t.Log(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{}
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil)

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

	if reqCtx.Response.StatusCode() != 500 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	if len(reqCtx.Response.Body()) > 0 {
		t.Errorf("Incorrect response body size. Expected: 0 and got %d",
			len(reqCtx.Response.Body()))
		t.Logf("Response body: %s", string(reqCtx.Response.Body()))
	}
}

func TestUpdaterToInvalidDB(t *testing.T) {

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
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil)
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

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}
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

	apifwResponse = web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

	var cfgEmpty = config.APIMode{
		APIFWMode:                  config.APIFWMode{Mode: web.APIMode},
		SpecificationUpdatePeriod:  2 * time.Second,
		PathToSpecDB:               "./wallarm_api_invalid_schema.db",
		UnknownParametersDetection: true,
		PassOptionsRequests:        false,
	}

	// start updater
	updSpecErrors := make(chan error, 1)
	updater := NewController(&lock, logger, specStorage, &cfgEmpty, &api, shutdown, &health, nil)
	go func() {
		t.Logf("starting specification regular update process every %.0f seconds", cfg.SpecificationUpdatePeriod.Seconds())
		updSpecErrors <- updater.Start()
	}()

	time.Sleep(3 * time.Second)

	if err := updater.Shutdown(); err != nil {
		t.Fatal(err)
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

	apifwResponse = web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

}

func TestUpdaterFromInvalidDB(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// load spec from the database
	specStorage, err := database.NewOpenAPIDB(logger, "./wallarm_api_invalid.db")
	if err != nil {
		t.Log(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{}
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil)
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

	if reqCtx.Response.StatusCode() != 500 {
		t.Errorf("Incorrect response status code. Expected: 500 and got %d",
			reqCtx.Response.StatusCode())
	}

	if len(reqCtx.Response.Body()) > 0 {
		t.Errorf("Incorrect response body size. Expected: 0 and got %d",
			len(reqCtx.Response.Body()))
		t.Logf("Response body: %s", string(reqCtx.Response.Body()))
	}

	// start updater
	updSpecErrors := make(chan error, 1)
	updater := NewController(&lock, logger, specStorage, &cfg, &api, shutdown, &health, nil)
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

	apifwResponse := web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
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

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse = web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}
	}

}

func TestUpdaterToNotExistDB(t *testing.T) {

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
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil)
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

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}
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

	apifwResponse = web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

	var cfgEmpty = config.APIMode{
		APIFWMode:                  config.APIFWMode{Mode: web.APIMode},
		SpecificationUpdatePeriod:  2 * time.Second,
		PathToSpecDB:               "./wallarm_api_not_exist.db",
		UnknownParametersDetection: true,
		PassOptionsRequests:        false,
	}

	// start updater
	updSpecErrors := make(chan error, 1)
	updater := NewController(&lock, logger, specStorage, &cfgEmpty, &api, shutdown, &health, nil)
	go func() {
		t.Logf("starting specification regular update process every %.0f seconds", cfg.SpecificationUpdatePeriod.Seconds())
		updSpecErrors <- updater.Start()
	}()

	time.Sleep(3 * time.Second)

	if err := updater.Shutdown(); err != nil {
		t.Fatal(err)
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

	apifwResponse = web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

}

func TestUpdaterFromNotExistDB(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// load spec from the database
	specStorage, err := database.NewOpenAPIDB(logger, "./wallarm_api_not_exist.db")
	if err != nil {
		t.Log(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{}
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil)
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

	if reqCtx.Response.StatusCode() != 500 {
		t.Errorf("Incorrect response status code. Expected: 500 and got %d",
			reqCtx.Response.StatusCode())
	}

	if len(reqCtx.Response.Body()) > 0 {
		t.Errorf("Incorrect response body size. Expected: 0 and got %d",
			len(reqCtx.Response.Body()))
		t.Logf("Response body: %s", string(reqCtx.Response.Body()))
	}

	// start updater
	updSpecErrors := make(chan error, 1)
	updater := NewController(&lock, logger, specStorage, &cfg, &api, shutdown, &health, nil)
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

	apifwResponse := web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
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

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse = web.APIModeResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}
	}

}
