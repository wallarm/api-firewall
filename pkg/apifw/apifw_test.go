package apifw

import (
	"bufio"
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	strconv2 "github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/database"
)

const (
	defaultIntSchemaID = 1
	wrongIntSchemaID   = 0
	testEmptyFile      = "./wallarm_apifw_empty.db"

	testYamlSpecification = `openapi: 3.0.1
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
)

func insertSpecV1(dbFilePath, newSpec string) (*database.SpecificationEntryV1, error) {

	db, err := sql.Open("sqlite3", dbFilePath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	q := fmt.Sprintf("INSERT INTO openapi_schemas(schema_version,schema_format,schema_content) VALUES ('1', 'yaml', '%s')", newSpec)
	_, err = db.Exec(q)
	if err != nil {
		return nil, err
	}

	// entry of the V1
	entry := database.SpecificationEntryV1{}

	rows, err := db.Query("SELECT * FROM openapi_schemas ORDER BY schema_id DESC LIMIT 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		err = rows.Scan(&entry.SchemaID, &entry.SchemaVersion, &entry.SchemaFormat, &entry.SchemaContent)
		if err != nil {
			return nil, err
		}
	}

	return &entry, nil
}

func validate200req(t *testing.T, apifw APIFirewall, schemaID int) {
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/?str=test")
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.SetHost("localhost")

	res, err := apifw.ValidateRequest(schemaID, ctx.Request.Header.RequestURI(), ctx.Request.Header.Method(), ctx.Request.Body(), http.Header{})
	if err != nil {
		t.Error(err)
	}

	if len(res.Summary) != 1 {
		t.Fatalf("expected response with 1 summary for 1 request. Got %d entries in the summary", len(res.Summary))
	}

	if *res.Summary[0].SchemaID != schemaID {
		t.Errorf("expected schema ID value %d. Got schema ID %d", schemaID, *res.Summary[0].SchemaID)
	}

	if *res.Summary[0].StatusCode != 200 {
		t.Errorf("expected status code 200. Got status code %d", *res.Summary[0].StatusCode)
	}

	if len(res.Errors) != 0 {
		t.Errorf("expected 0 error in the list. Got %d errors in the list", len(res.Errors))
	}
}

func validate403WrongMethodReq(t *testing.T, apifw APIFirewall, schemaID int) {
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/?str=test")
	ctx.Request.Header.SetMethod("PUT")
	ctx.Request.Header.SetHost("localhost")

	res, err := apifw.ValidateRequest(schemaID, ctx.Request.Header.RequestURI(), ctx.Request.Header.Method(), ctx.Request.Body(), http.Header{})
	if err != nil {
		t.Error(err)
	}

	if len(res.Summary) != 1 {
		t.Fatalf("expected response with 1 summary for 1 request. Got %d entries in the summary", len(res.Summary))
	}

	if *res.Summary[0].SchemaID != schemaID {
		t.Errorf("expected schema ID value %d. Got schema ID %d", schemaID, *res.Summary[0].SchemaID)
	}

	if *res.Summary[0].StatusCode != 403 {
		t.Errorf("expected status code 403. Got status code %d", *res.Summary[0].StatusCode)
	}

	if len(res.Errors) != 1 {
		t.Errorf("expected 1 error in the list. Got %d errors in the list", len(res.Errors))
	}
}

func validate403UnknownParamReq(t *testing.T, apifw APIFirewall, schemaID int) {
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/?str=test&id=123")
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.SetHost("localhost")

	res, err := apifw.ValidateRequest(schemaID, ctx.Request.Header.RequestURI(), ctx.Request.Header.Method(), ctx.Request.Body(), http.Header{})
	if err != nil {
		t.Error(err)
	}

	if len(res.Summary) != 1 {
		t.Fatalf("expected response with 1 summary for 1 request. Got %d entries in the summary", len(res.Summary))
	}

	if *res.Summary[0].SchemaID != schemaID {
		t.Errorf("expected schema ID value %d. Got schema ID %d", schemaID, *res.Summary[0].SchemaID)
	}

	if *res.Summary[0].StatusCode != 403 {
		t.Errorf("expected status code 403. Got status code %d", *res.Summary[0].StatusCode)
	}

	if len(res.Errors) != 1 {
		t.Errorf("expected 1 error in the list. Got %d errors in the list", len(res.Errors))
	}
}

func validate403RequiredParamMissedReq(t *testing.T, apifw APIFirewall, schemaID int) {
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.SetHost("localhost")

	res, err := apifw.ValidateRequest(schemaID, ctx.Request.Header.RequestURI(), ctx.Request.Header.Method(), ctx.Request.Body(), http.Header{})
	if err != nil {
		t.Error(err)
	}

	if len(res.Summary) != 1 {
		t.Fatalf("expected response with 1 summary for 1 request. Got %d entries in the summary", len(res.Summary))
	}

	if *res.Summary[0].SchemaID != schemaID {
		t.Errorf("expected schema ID value %d. Got schema ID %d", schemaID, *res.Summary[0].SchemaID)
	}

	if *res.Summary[0].StatusCode != 403 {
		t.Errorf("expected status code 403. Got status code %d", *res.Summary[0].StatusCode)
	}

	if len(res.Errors) != 1 {
		t.Errorf("expected 1 error in the list. Got %d errors in the list", len(res.Errors))
	}
}

func validate500UnknownCTReq(t *testing.T, apifw APIFirewall, schemaID int) {
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.Header.SetContentType("application/unknownCT")
	ctx.Request.SetBody([]byte("test"))
	ctx.Request.Header.SetHost("localhost")

	headers := http.Header{}
	ctx.Request.Header.VisitAll(func(k, v []byte) {
		sk := strconv2.B2S(k)
		sv := strconv2.B2S(v)

		headers.Set(sk, sv)
	})

	res, err := apifw.ValidateRequest(schemaID, ctx.Request.Header.RequestURI(), ctx.Request.Header.Method(), ctx.Request.Body(), headers)
	if !errors.Is(err, ErrRequestParsing) {
		t.Error(err)
	}

	if len(res.Summary) != 1 {
		t.Fatalf("expected response with 1 summary for 1 request. Got %d entries in the summary", len(res.Summary))
	}

	if *res.Summary[0].SchemaID != schemaID {
		t.Errorf("expected schema ID value %d. Got schema ID %d", schemaID, *res.Summary[0].SchemaID)
	}

	if *res.Summary[0].StatusCode != 500 {
		t.Errorf("expected status code 500. Got status code %d", *res.Summary[0].StatusCode)
	}

	if len(res.Errors) != 0 {
		t.Errorf("expected 0 error in the list. Got %d errors in the list", len(res.Errors))
	}
}

func validate200OptionsReq(t *testing.T, apifw APIFirewall, schemaID int) {
	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod("OPTIONS")
	ctx.Request.Header.SetHost("localhost")

	res, err := apifw.ValidateRequest(schemaID, ctx.Request.Header.RequestURI(), ctx.Request.Header.Method(), ctx.Request.Body(), http.Header{})
	if err != nil {
		t.Error(err)
	}

	if len(res.Summary) != 1 {
		t.Fatalf("expected response with 1 summary for 1 request. Got %d entries in the summary", len(res.Summary))
	}

	if *res.Summary[0].SchemaID != schemaID {
		t.Errorf("expected schema ID value %d. Got schema ID %d", schemaID, *res.Summary[0].SchemaID)
	}

	if *res.Summary[0].StatusCode != 200 {
		t.Errorf("expected status code 200. Got status code %d", *res.Summary[0].StatusCode)
	}

	if len(res.Errors) != 0 {
		t.Errorf("expected 0 error in the list. Got %d errors in the list", len(res.Errors))
	}
}

func TestAPIFWBasic(t *testing.T) {

	apifw, err := NewAPIFirewall(
		WithPathToDB("./wallarm_apifw_test.db"),
	)
	if err != nil {
		t.Fatal(err)
	}

	validate200req(t, apifw, defaultIntSchemaID)

	validate403WrongMethodReq(t, apifw, defaultIntSchemaID)

	validate403UnknownParamReq(t, apifw, defaultIntSchemaID)

	validate403RequiredParamMissedReq(t, apifw, defaultIntSchemaID)

	validate200OptionsReq(t, apifw, defaultIntSchemaID)

	validate500UnknownCTReq(t, apifw, defaultIntSchemaID)

}

func TestAPIFWBasicUpdate(t *testing.T) {

	source, err := os.Open(testEmptyFile) //open the source file
	if err != nil {
		panic(err)
	}
	defer source.Close()

	tmpFileName := "./wallarm_apifw_tmp.db"

	destination, err := os.Create(tmpFileName)
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()

	// delete file
	defer func() {
		if err := os.Remove(tmpFileName); err != nil {
			log.Fatal(err)
		}
	}()

	_, err = io.Copy(destination, source)
	if err != nil {
		t.Fatal(err)
	}

	entry, err := insertSpecV1(tmpFileName, testYamlSpecification)
	if err != nil {
		t.Fatal(err)
	}

	apifw, err := NewAPIFirewall(
		WithPathToDB(tmpFileName),
		DisableUnknownParameters(),
		DisablePassOptionsRequests(),
	)
	if err != nil {
		t.Fatal(err)
	}

	validate200req(t, apifw, entry.SchemaID)

	// update DB without spec update
	currentSIDs, isUpdated, err := apifw.UpdateSpecsStorage()
	if err != nil {
		t.Error(err)
	}

	if len(currentSIDs) == 1 {
		if currentSIDs[0] != entry.SchemaID {
			t.Errorf("expected schema ID is %d. Got schema ID %d", entry.SchemaID, currentSIDs[0])
		}
	} else {
		t.Errorf("expected len of the schema IDs is 1. Got len of the schema IDs %d", len(currentSIDs))
	}

	if isUpdated {
		t.Errorf("expected isUpdated == false. Got isUpdated %t", isUpdated)
	}

	// insert
	entry2, err := insertSpecV1(tmpFileName, testYamlSpecification)
	if err != nil {
		t.Fatal(err)
	}

	// update DB without spec update
	currentSIDs, isUpdated, err = apifw.UpdateSpecsStorage()
	if err != nil {
		t.Error(err)
	}

	if len(currentSIDs) == 2 {
		if currentSIDs[0] != entry.SchemaID {
			t.Errorf("expected schema ID is %d. Got schema ID %d", entry.SchemaID, currentSIDs[0])
		}
		if currentSIDs[1] != entry2.SchemaID {
			t.Errorf("expected schema ID is %d. Got schema ID %d", entry2.SchemaID, currentSIDs[1])
		}
	} else {
		t.Errorf("expected len of the schema IDs is 2. Got len of the schema IDs %d", len(currentSIDs))
	}

	if !isUpdated {
		t.Errorf("expected isUpdated == true. Got isUpdated %t", isUpdated)
	}

	validate200req(t, apifw, entry2.SchemaID)

}

func TestAPIFWBasicErrors(t *testing.T) {

	// check ErrSpecLoading
	_, err := NewAPIFirewall(
		WithPathToDB("./wallarm_apifw_invalid_db_schema.db"),
		DisableUnknownParameters(),
		DisablePassOptionsRequests(),
	)
	if !errors.Is(err, ErrSpecLoading) {
		t.Errorf("expected ErrSpecLoading but got %v", err)
	}

	// check ErrSpecParsing
	_, err = NewAPIFirewall(
		WithPathToDB("./wallarm_apifw_invalid_spec.db"),
		DisableUnknownParameters(),
		DisablePassOptionsRequests(),
	)
	if !errors.Is(err, ErrSpecParsing) {
		t.Errorf("expected ErrSpecParsing but got %v", err)
	}

	// check ErrRequestParsing
	apifw, err := NewAPIFirewall(
		WithPathToDB("./wallarm_apifw_test.db"),
		DisableUnknownParameters(),
		DisablePassOptionsRequests(),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.SetHost("localhost")

	w := new(bytes.Buffer)
	bw := bufio.NewWriter(w)
	bw.Write([]byte("GET \\ HTTP/1.1.1"))
	r := bufio.NewReader(w)

	_, err = apifw.ValidateRequestFromReader(defaultIntSchemaID, r)
	if !errors.Is(err, ErrRequestParsing) {
		t.Errorf("expected ErrRequestParsing but got %v", err)
	}

	if err := ctx.Request.Write(bw); err != nil {
		t.Fatal(err)
	}
	bw.Flush()

	// check ErrSchemaNotFound
	_, err = apifw.ValidateRequestFromReader(wrongIntSchemaID, r)
	if !errors.Is(err, ErrSchemaNotFound) {
		t.Errorf("expected ErrSchemaNotFound but got %v", err)
	}

	// check ErrSpecValidation
	_, err = NewAPIFirewall(
		WithPathToDB("./wallarm_apifw_spec_validation_failed.db"),
		DisableUnknownParameters(),
		DisablePassOptionsRequests(),
	)
	if !errors.Is(err, ErrSpecValidation) {
		t.Errorf("expected ErrSpecValidation but got %v", err)
	}
}

func TestSafeCounterThreadSafety(t *testing.T) {
	var wg sync.WaitGroup
	count := 10000
	numGoroutines := 10

	apifw, err := NewAPIFirewall(
		WithPathToDB("./wallarm_apifw_test.db"),
		DisablePassOptionsRequests(),
	)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			r := rand.Intn(500)
			time.Sleep(time.Duration(r) * time.Millisecond)
			if _, _, err := apifw.UpdateSpecsStorage(); err != nil {
				t.Error(err)
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < count; j++ {
				validate200req(t, apifw, defaultIntSchemaID)
				validate403RequiredParamMissedReq(t, apifw, defaultIntSchemaID)
				validate403UnknownParamReq(t, apifw, defaultIntSchemaID)
			}
		}()
	}

	wg.Wait()
}
