package tests

import (
	"database/sql"
	"encoding/json"
	"errors"
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
	"github.com/wallarm/api-firewall/internal/platform/storage"
	"github.com/wallarm/api-firewall/internal/platform/web"
	"github.com/wallarm/api-firewall/pkg/APIMode/validator"
)

const (
	dbVersionV2 = 2
)

const testYamlSpecification = `openapi: 3.0.1
info:
  title: Service
  version: 1.1.1
servers:
  - url: /
paths:
  /:
    get:
      tags:
        - Redirects
      summary: Absolutely 302 Redirects n times.
      responses:
        ''200'':
          description: A redirection.
          content: {}
`

var currentDBPath = "./wallarm_api2_update.db"

var cfgV2 = config.APIMode{
	APIFWMode:                  config.APIFWMode{Mode: web.APIMode},
	SpecificationUpdatePeriod:  2 * time.Second,
	PathToSpecDB:               currentDBPath,
	UnknownParametersDetection: true,
	PassOptionsRequests:        false,
}

type EntryV2 struct {
	SchemaID      int    `db:"schema_id"`
	SchemaVersion string `db:"schema_version"`
	SchemaFormat  string `db:"schema_format"`
	SchemaContent string `db:"schema_content"`
	Status        string `db:"status"`
}

func insertSpecV2(dbFilePath, newSpec string) (*EntryV2, error) {

	db, err := sql.Open("sqlite3", dbFilePath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	q := fmt.Sprintf("INSERT INTO openapi_schemas(schema_version,schema_format,schema_content,status) VALUES ('1', 'yaml', '%s', 'new')", newSpec)
	_, err = db.Exec(q)
	if err != nil {
		return nil, err
	}

	// entry of the V2
	entry := EntryV2{}

	rows, err := db.Query("SELECT * FROM openapi_schemas ORDER BY schema_id DESC LIMIT 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&entry.SchemaID, &entry.SchemaVersion, &entry.SchemaFormat, &entry.SchemaContent, &entry.Status)
		if err != nil {
			return nil, err
		}
	}

	return &entry, nil
}

// check that row is applied and delete this row
func cleanSpecV2(dbFilePath string, schemaID int) error {

	db, err := sql.Open("sqlite3", dbFilePath)
	if err != nil {
		return err
	}
	defer db.Close()

	q := fmt.Sprintf("SELECT * FROM openapi_schemas WHERE schema_id = %d", schemaID)
	rows, err := db.Query(q)
	if err != nil {
		return err
	}
	defer rows.Close()

	entry := EntryV2{}

	for rows.Next() {
		err = rows.Scan(&entry.SchemaID, &entry.SchemaVersion, &entry.SchemaFormat, &entry.SchemaContent, &entry.Status)
		if err != nil {
			return err
		}
	}

	q = fmt.Sprintf("DELETE FROM openapi_schemas WHERE schema_id = %d", entry.SchemaID)
	_, err = db.Exec(q)
	if err != nil {
		return err
	}

	if entry.Status != "applied" {
		return errors.New("the status have not changed for the updated record")
	}

	return nil
}

func TestLoadBasicV2(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// create DB entry with spec and status = 'new'
	entry, err := insertSpecV2(currentDBPath, testYamlSpecification)
	if err != nil {
		t.Fatal(err)
	}

	//check and clean
	defer func() {
		if err := cleanSpecV2(currentDBPath, entry.SchemaID); err != nil {
			t.Fatal(err)
		}
	}()

	// load spec from the database
	specStorage, err := storage.NewOpenAPIDB(currentDBPath, 0)
	if err != nil {
		t.Fatal(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{}
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil, nil)

	// invalid route in the old spec
	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/new")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", entry.SchemaID))

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

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != entry.SchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				entry.SchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}
	}

	// valid route in the same spec
	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", entry.SchemaID))

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

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != entry.SchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				entry.SchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}
}

func TestUpdaterBasicV2(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// load spec from the database
	specStorage, err := storage.NewOpenAPIDB(currentDBPath, 0)
	if err != nil {
		t.Fatal(err)
	}

	// create DB entry with spec and status = 'new'
	entry, err := insertSpecV2(currentDBPath, testYamlSpecification)
	if err != nil {
		t.Fatal(err)
	}

	//check and clean
	defer func() {
		if err := cleanSpecV2(currentDBPath, entry.SchemaID); err != nil {
			t.Fatal(err)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{}
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil, nil)
	health := handlersAPI.Health{}

	// invalid route in the old spec
	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", entry.SchemaID))

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

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != entry.SchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				entry.SchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusInternalServerError {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusInternalServerError, *apifwResponse.Summary[0].StatusCode)
		}
	}

	// valid route in the old spec
	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

	// start updater
	updSpecErrors := make(chan error, 1)
	updater := handlersAPI.NewHandlerUpdater(&lock, logger, specStorage, &cfgV2, &api, shutdown, &health, nil, nil)
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
	req.SetRequestURI("/")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", entry.SchemaID))

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

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != entry.SchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				entry.SchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

}

func TestUpdaterFromEmptyDBV2(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// load spec from the database
	specStorage, err := storage.NewOpenAPIDB("./wallarm_api2_empty.db", 0)
	if err != nil {
		t.Fatal(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{}
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil, nil)
	health := handlersAPI.Health{}

	// invalid route in the old spec
	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusInternalServerError {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusInternalServerError, *apifwResponse.Summary[0].StatusCode)
		}
	}

	// start updater
	updSpecErrors := make(chan error, 1)
	updater := handlersAPI.NewHandlerUpdater(&lock, logger, specStorage, &cfgV2, &api, shutdown, &health, nil, nil)
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
	req.SetRequestURI("/")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

	// invalid route in the new spec
	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/new")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}
	}

}

func TestUpdaterToEmptyDBV2(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// load spec from the database
	specStorage, err := storage.NewOpenAPIDB("./wallarm_api2_update.db", dbVersionV2)
	if err != nil {
		t.Fatal(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{}
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil, nil)
	health := handlersAPI.Health{}

	// invalid route in the old spec
	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/new")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
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
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

	var cfgV2Empty = config.APIMode{
		APIFWMode:                  config.APIFWMode{Mode: web.APIMode},
		SpecificationUpdatePeriod:  2 * time.Second,
		PathToSpecDB:               "./wallarm_api2_empty.db",
		UnknownParametersDetection: true,
		PassOptionsRequests:        false,
	}

	// start updater
	updSpecErrors := make(chan error, 1)
	updater := handlersAPI.NewHandlerUpdater(&lock, logger, specStorage, &cfgV2Empty, &api, shutdown, &health, nil, nil)
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
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusInternalServerError {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusInternalServerError, *apifwResponse.Summary[0].StatusCode)
		}
	}

}

func TestUpdaterInvalidDBSchemaV2(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// load spec from the database
	specStorage, err := storage.NewOpenAPIDB("./wallarm_api_invalid_schema.db", dbVersionV2)
	if err != nil {
		t.Log(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{}
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

func TestUpdaterToInvalidDBV2(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// load spec from the database
	specStorage, err := storage.NewOpenAPIDB("./wallarm_api2_update.db", dbVersionV2)
	if err != nil {
		t.Fatal(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{}
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil, nil)
	health := handlersAPI.Health{}

	// invalid route in the old spec
	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/new")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
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
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

	var cfgV2Invalid = config.APIMode{
		APIFWMode:                  config.APIFWMode{Mode: web.APIMode},
		SpecificationUpdatePeriod:  2 * time.Second,
		PathToSpecDB:               "./wallarm_api_invalid_schema.db",
		UnknownParametersDetection: true,
		PassOptionsRequests:        false,
	}

	// start updater
	updSpecErrors := make(chan error, 1)
	updater := handlersAPI.NewHandlerUpdater(&lock, logger, specStorage, &cfgV2Invalid, &api, shutdown, &health, nil, nil)
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
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

}

func TestUpdaterFromInvalidDBV2(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// load spec from the database
	specStorage, err := storage.NewOpenAPIDB("./wallarm_api_invalid.db", dbVersionV2)
	if err != nil {
		t.Log(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{}
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil, nil)
	health := handlersAPI.Health{}

	// invalid route in the old spec
	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/new")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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
	updater := handlersAPI.NewHandlerUpdater(&lock, logger, specStorage, &cfgV2, &api, shutdown, &health, nil, nil)
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
	req.SetRequestURI("/")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

	// invalid route in the new spec
	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/new")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}
	}

}

func TestUpdaterFromV1DBToV2(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// load spec from the database
	specStorage, err := storage.NewOpenAPIDB("./wallarm_api_before_update.db", dbVersion)
	if err != nil {
		t.Fatal(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{}
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil, nil)
	health := handlersAPI.Health{}

	// invalid route in the old spec
	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/new")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
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
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

	// prepare spec with status = 'new'
	// create DB entry with spec and status = 'new'
	entry, err := insertSpecV2(currentDBPath, testYamlSpecification)
	if err != nil {
		t.Fatal(err)
	}

	//check and clean
	defer func() {
		if err := cleanSpecV2(currentDBPath, entry.SchemaID); err != nil {
			t.Fatal(err)
		}
	}()

	// start updater
	updSpecErrors := make(chan error, 1)
	updater := handlersAPI.NewHandlerUpdater(&lock, logger, specStorage, &cfgV2, &api, shutdown, &health, nil, nil)
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
	req.SetRequestURI("/")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", entry.SchemaID))

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

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != entry.SchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				entry.SchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

	// invalid route in the new spec
	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/new")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", entry.SchemaID))

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

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != entry.SchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				entry.SchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}
	}

}

func TestUpdaterFromV2DBToV1(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// load spec from the database
	specStorage, err := storage.NewOpenAPIDB("./wallarm_api2_update.db", dbVersionV2)
	if err != nil {
		t.Log(err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	api := fasthttp.Server{}
	api.Handler = handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage, nil, nil)
	health := handlersAPI.Health{}

	// invalid route in the old spec
	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/new")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}
	}

	// start updater
	updSpecErrors := make(chan error, 1)
	updater := handlersAPI.NewHandlerUpdater(&lock, logger, specStorage, &cfg, &api, shutdown, &health, nil, nil)
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
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

	// invalid route in the old spec
	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/new")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultUpdaterSchemaID))

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

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != DefaultUpdaterSchemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				DefaultUpdaterSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}
	}

}
