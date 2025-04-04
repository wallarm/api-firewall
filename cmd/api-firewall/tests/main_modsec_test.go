package tests

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"os/signal"
	"path"
	"sync"
	"syscall"
	"testing"

	"github.com/corazawaf/coraza/v3"
	"github.com/corazawaf/coraza/v3/types"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"

	proxy2 "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/proxy"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/loader"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/storage"
)

const openAPISpecModSecTest = `
openapi: 3.0.1
info:
  title: Service
  version: 1.0.0
servers:
  - url: /
paths:
  /cookie_params:
    get:
      tags:
        - Cookie parameters
      summary: The endpoint with cookie parameters only
      parameters:
        - name: cookie_mandatory
          in: cookie
          description: mandatory cookie parameter
          required: true
          schema:
            type: string
        - name: cookie_optional
          in: cookie
          description: optional cookie parameter
          required: false
          schema:
            type: integer
            enum: [0, 10, 100]
      responses:
        '200':
          description: Set cookies.
          content: {}
  '/token/{token}':
    get:
      parameters:
        - name: token
          in: path
          required: true
          schema:
            maxLength: 36
            type: string 
      responses:
        '200':
          description: Static page
          content: {}
        '403':
          description: operation forbidden
          content: {}
  /get/test:
    get:
      summary: Get Test Info
      responses:
        200:
          description: Ok
          content: { }
    post:
      summary: Get Test Info
      responses:
        200:
          description: Ok
          content: { }
components:
  securitySchemes:
    petstore_auth:
      type: oauth2
      flows:
        implicit:
          authorizationUrl: /login
          scopes:
            read: read
            write: write
`

type ModSecIntegrationTests struct {
	serverUrl   *url.URL
	shutdown    chan os.Signal
	proxy       *proxy.MockPool
	client      *proxy.MockHTTPClient
	swagRouter  *loader.Router
	wafRules    string
	wafConfFile string
	dbSpec      *storage.MockDBOpenAPILoader
	lock        *sync.RWMutex
}

func TestModSec(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	var lock sync.RWMutex
	dbSpec := storage.NewMockDBOpenAPILoader(mockCtrl)

	serverUrl, err := url.ParseRequestURI("http://127.0.0.1:80")
	if err != nil {
		t.Fatalf("parsing API Host URL: %s", err.Error())
	}

	pool := proxy.NewMockPool(mockCtrl)
	client := proxy.NewMockHTTPClient(mockCtrl)

	swagger, err := openapi3.NewLoader().LoadFromData([]byte(openAPISpecModSecTest))
	if err != nil {
		t.Fatalf("loading OpenAPI specification file: %s", err.Error())
	}

	dbSpec.EXPECT().SchemaIDs().Return([]int{}).AnyTimes()
	dbSpec.EXPECT().Specification(gomock.Any()).Return(swagger).AnyTimes()
	dbSpec.EXPECT().SpecificationVersion(gomock.Any()).Return("").AnyTimes()
	dbSpec.EXPECT().IsLoaded(gomock.Any()).Return(true).AnyTimes()
	dbSpec.EXPECT().IsReady().Return(true).AnyTimes()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	apifwTests := ModSecIntegrationTests{
		serverUrl:   serverUrl,
		shutdown:    shutdown,
		proxy:       pool,
		client:      client,
		wafConfFile: "../../../resources/test/modsec/coraza.conf",
		wafRules:    path.Join("../../../resources/test/modsec/rules_test", "*.conf"),
		lock:        &lock,
		dbSpec:      dbSpec,
	}

	// basic test
	t.Run("basicMaliciousRequestBlockMode", apifwTests.basicMaliciousRequestBlockMode)
	t.Run("basicMaliciousRequestLogOnlyMode", apifwTests.basicMaliciousRequestLogOnlyMode)
	t.Run("basicMaliciousRequestDisableMode", apifwTests.basicMaliciousRequestDisableMode)

	t.Run("basicMaliciousResponseBlockMode", apifwTests.basicMaliciousResponseBlockMode)
	t.Run("basicMaliciousResponseLogOnlyMode", apifwTests.basicMaliciousResponseLogOnlyMode)
	t.Run("basicMaliciousResponseDisableMode", apifwTests.basicMaliciousResponseDisableMode)

	// handling parts of the request
	t.Run("basicMaliciousRequestBody", apifwTests.basicMaliciousRequestBody)
	t.Run("basicMaliciousRequestHeader", apifwTests.basicMaliciousRequestHeader)
	t.Run("basicMaliciousRequestCookie", apifwTests.basicMaliciousRequestCookie)
	t.Run("basicMaliciousRequestPath", apifwTests.basicMaliciousRequestPath)

	// redirect
	t.Run("basicResponseActionRedirect", apifwTests.basicResponseActionRedirect)
}

func (s *ModSecIntegrationTests) basicMaliciousRequestBlockMode(t *testing.T) {

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	logger = logger.Level(zerolog.ErrorLevel)

	logErr := func(error types.MatchedRule) {
		logger.Error().
			Strs("tags", error.Rule().Tags()).
			Str("version", error.Rule().Version()).
			Str("severity", error.Rule().Severity().String()).
			Int("rule_id", error.Rule().ID()).
			Str("file", error.Rule().File()).
			Int("line", error.Rule().Line()).
			Int("maturity", error.Rule().Maturity()).
			Int("accuracy", error.Rule().Accuracy()).
			Str("uri", error.URI()).
			Msg(error.Message())
	}

	waf, err := coraza.NewWAF(
		coraza.NewWAFConfig().
			WithErrorCallback(logErr).
			WithDirectivesFromFile(s.wafConfFile).
			WithDirectivesFromFile(s.wafRules),
	)
	if err != nil {
		t.Fatal(err)
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, logger, s.proxy, s.dbSpec, nil, nil, waf)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/get/test")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// Repeat request with SQLI
	req.SetRequestURI("/get/test?id='or'1'='1")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ModSecIntegrationTests) basicMaliciousRequestLogOnlyMode(t *testing.T) {

	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()
	logger = logger.Level(zerolog.ErrorLevel)

	logErr := func(error types.MatchedRule) {
		logger.Error().
			Strs("tags", error.Rule().Tags()).
			Str("version", error.Rule().Version()).
			Str("severity", error.Rule().Severity().String()).
			Int("rule_id", error.Rule().ID()).
			Str("file", error.Rule().File()).
			Int("line", error.Rule().Line()).
			Int("maturity", error.Rule().Maturity()).
			Int("accuracy", error.Rule().Accuracy()).
			Str("uri", error.URI()).
			Msg(error.Message())
	}

	waf, err := coraza.NewWAF(
		coraza.NewWAFConfig().
			WithErrorCallback(logErr).
			WithDirectivesFromFile(s.wafConfFile).
			WithDirectivesFromFile(s.wafRules),
	)
	if err != nil {
		t.Fatal(err)
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "LOG_ONLY",
		ResponseValidation:        "LOG_ONLY",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, logger, s.proxy, s.dbSpec, nil, nil, waf)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/get/test")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// Repeat request with SQLI
	req.SetRequestURI("/get/test?id='or'1'='1")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// check logs
	var expectedRuleId float64 = 942100

	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, err, "invalid JSON")

	triggeredRuleID, exists := logEntry["rule_id"].(float64)
	assert.True(t, exists, "field 'rule_id' doesn't exist")

	if triggeredRuleID != expectedRuleId {
		t.Errorf("Got rule_id: %f; Expected rule ID: %f", triggeredRuleID, expectedRuleId)
	}

}

func (s *ModSecIntegrationTests) basicMaliciousRequestDisableMode(t *testing.T) {

	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()
	logger = logger.Level(zerolog.ErrorLevel)

	logErr := func(error types.MatchedRule) {
		logger.Error().
			Strs("tags", error.Rule().Tags()).
			Str("version", error.Rule().Version()).
			Str("severity", error.Rule().Severity().String()).
			Int("rule_id", error.Rule().ID()).
			Str("file", error.Rule().File()).
			Int("line", error.Rule().Line()).
			Int("maturity", error.Rule().Maturity()).
			Int("accuracy", error.Rule().Accuracy()).
			Str("uri", error.URI()).
			Msg(error.Message())
	}

	waf, err := coraza.NewWAF(
		coraza.NewWAFConfig().
			WithErrorCallback(logErr).
			WithDirectivesFromFile(s.wafConfFile).
			WithDirectivesFromFile(s.wafRules),
	)
	if err != nil {
		t.Fatal(err)
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "DISABLE",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, logger, s.proxy, s.dbSpec, nil, nil, waf)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/get/test")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// Repeat request with SQLI
	req.SetRequestURI("/get/test?id='or'1'='1")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	if buf.Len() > 0 {
		t.Errorf("Expected number of bytes (error logs) is 0. Got %d bytes (error logs)", buf.Len())
	}
}

func (s *ModSecIntegrationTests) basicMaliciousResponseBlockMode(t *testing.T) {

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	logger = logger.Level(zerolog.ErrorLevel)

	logErr := func(error types.MatchedRule) {
		logger.Error().
			Strs("tags", error.Rule().Tags()).
			Str("version", error.Rule().Version()).
			Str("severity", error.Rule().Severity().String()).
			Int("rule_id", error.Rule().ID()).
			Str("file", error.Rule().File()).
			Int("line", error.Rule().Line()).
			Int("maturity", error.Rule().Maturity()).
			Int("accuracy", error.Rule().Accuracy()).
			Str("uri", error.URI()).
			Msg(error.Message())
	}

	waf, err := coraza.NewWAF(
		coraza.NewWAFConfig().
			WithErrorCallback(logErr).
			WithDirectivesFromFile(s.wafConfFile).
			WithDirectivesFromFile(s.wafRules),
	)
	if err != nil {
		t.Fatal(err)
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, logger, s.proxy, s.dbSpec, nil, nil, waf)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/get/test")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// Repeat request with malicious response
	req.SetRequestURI("/get/test")
	resp = fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("text/html")
	resp.SetBody([]byte("<title>r57 shell</title>"))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ModSecIntegrationTests) basicMaliciousResponseLogOnlyMode(t *testing.T) {

	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()
	logger = logger.Level(zerolog.ErrorLevel)

	logErr := func(error types.MatchedRule) {
		logger.Error().
			Strs("tags", error.Rule().Tags()).
			Str("version", error.Rule().Version()).
			Str("severity", error.Rule().Severity().String()).
			Int("rule_id", error.Rule().ID()).
			Str("file", error.Rule().File()).
			Int("line", error.Rule().Line()).
			Int("maturity", error.Rule().Maturity()).
			Int("accuracy", error.Rule().Accuracy()).
			Str("uri", error.URI()).
			Msg(error.Message())
	}

	waf, err := coraza.NewWAF(
		coraza.NewWAFConfig().
			WithErrorCallback(logErr).
			WithDirectivesFromFile(s.wafConfFile).
			WithDirectivesFromFile(s.wafRules),
	)
	if err != nil {
		t.Fatal(err)
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "LOG_ONLY",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, logger, s.proxy, s.dbSpec, nil, nil, waf)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/get/test")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// Repeat request with malicious response
	req.SetRequestURI("/get/test?id=111")
	resp = fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("text/html")
	resp.SetBody([]byte("<title>r57 shell</title>"))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// check logs
	var expectedRuleId float64 = 955110

	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, err, "invalid JSON")

	triggeredRuleID, exists := logEntry["rule_id"].(float64)
	assert.True(t, exists, "field 'rule_id' doesn't exist")

	if triggeredRuleID != expectedRuleId {
		t.Errorf("Got rule_id: %f; Expected rule ID: %f", triggeredRuleID, expectedRuleId)
	}

}

func (s *ModSecIntegrationTests) basicMaliciousResponseDisableMode(t *testing.T) {

	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()
	logger = logger.Level(zerolog.ErrorLevel)

	logErr := func(error types.MatchedRule) {
		logger.Error().
			Strs("tags", error.Rule().Tags()).
			Str("version", error.Rule().Version()).
			Str("severity", error.Rule().Severity().String()).
			Int("rule_id", error.Rule().ID()).
			Str("file", error.Rule().File()).
			Int("line", error.Rule().Line()).
			Int("maturity", error.Rule().Maturity()).
			Int("accuracy", error.Rule().Accuracy()).
			Str("uri", error.URI()).
			Msg(error.Message())
	}

	waf, err := coraza.NewWAF(
		coraza.NewWAFConfig().
			WithErrorCallback(logErr).
			WithDirectivesFromFile(s.wafConfFile).
			WithDirectivesFromFile(s.wafRules),
	)
	if err != nil {
		t.Fatal(err)
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "DISABLE",
		ResponseValidation:        "DISABLE",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, logger, s.proxy, s.dbSpec, nil, nil, waf)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/get/test")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// Repeat request with malicious response
	req.SetRequestURI("/get/test?id=111")
	resp = fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("text/html")
	resp.SetBody([]byte("<title>r57 shell</title>"))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	if buf.Len() > 0 {
		t.Errorf("Expected number of bytes (error logs) is 0. Got %d bytes (error logs)", buf.Len())
	}
}

func (s *ModSecIntegrationTests) basicMaliciousRequestBody(t *testing.T) {

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	logger = logger.Level(zerolog.ErrorLevel)

	logErr := func(error types.MatchedRule) {
		logger.Error().
			Strs("tags", error.Rule().Tags()).
			Str("version", error.Rule().Version()).
			Str("severity", error.Rule().Severity().String()).
			Int("rule_id", error.Rule().ID()).
			Str("file", error.Rule().File()).
			Int("line", error.Rule().Line()).
			Int("maturity", error.Rule().Maturity()).
			Int("accuracy", error.Rule().Accuracy()).
			Str("uri", error.URI()).
			Msg(error.Message())
	}

	waf, err := coraza.NewWAF(
		coraza.NewWAFConfig().
			WithErrorCallback(logErr).
			WithDirectivesFromFile(s.wafConfFile).
			WithDirectivesFromFile(s.wafRules),
	)
	if err != nil {
		t.Fatal(err)
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, logger, s.proxy, s.dbSpec, nil, nil, waf)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/get/test")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// Repeat request with SQLI
	req.Header.SetMethod("POST")
	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.SetBody([]byte("'+or+'1'='1"))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ModSecIntegrationTests) basicMaliciousRequestHeader(t *testing.T) {

	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()
	logger = logger.Level(zerolog.ErrorLevel)

	logErr := func(error types.MatchedRule) {
		logger.Error().
			Strs("tags", error.Rule().Tags()).
			Str("version", error.Rule().Version()).
			Str("severity", error.Rule().Severity().String()).
			Int("rule_id", error.Rule().ID()).
			Str("file", error.Rule().File()).
			Int("line", error.Rule().Line()).
			Int("maturity", error.Rule().Maturity()).
			Int("accuracy", error.Rule().Accuracy()).
			Str("uri", error.URI()).
			Msg(error.Message())
	}

	waf, err := coraza.NewWAF(
		coraza.NewWAFConfig().
			WithErrorCallback(logErr).
			WithDirectivesFromFile(s.wafConfFile).
			WithDirectivesFromFile(s.wafRules),
	)
	if err != nil {
		t.Fatal(err)
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, logger, s.proxy, s.dbSpec, nil, nil, waf)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/get/test")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// Repeat request with SQLi
	req.Header.SetMethod("GET")
	req.Header.Add("test", "'+or+'1'='1")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	// check logs
	var expectedRuleId float64 = 942100

	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, err, "invalid JSON")

	triggeredRuleID, exists := logEntry["rule_id"].(float64)
	assert.True(t, exists, "field 'rule_id' doesn't exist")

	if triggeredRuleID != expectedRuleId {
		t.Errorf("Got rule_id: %f; Expected rule ID: %f", triggeredRuleID, expectedRuleId)
	}

}

func (s *ModSecIntegrationTests) basicMaliciousRequestCookie(t *testing.T) {

	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()
	logger = logger.Level(zerolog.ErrorLevel)

	logErr := func(error types.MatchedRule) {
		logger.Error().
			Strs("tags", error.Rule().Tags()).
			Str("version", error.Rule().Version()).
			Str("severity", error.Rule().Severity().String()).
			Int("rule_id", error.Rule().ID()).
			Str("file", error.Rule().File()).
			Int("line", error.Rule().Line()).
			Int("maturity", error.Rule().Maturity()).
			Int("accuracy", error.Rule().Accuracy()).
			Str("uri", error.URI()).
			Msg(error.Message())
	}

	waf, err := coraza.NewWAF(
		coraza.NewWAFConfig().
			WithErrorCallback(logErr).
			WithDirectivesFromFile(s.wafConfFile).
			WithDirectivesFromFile(s.wafRules),
	)
	if err != nil {
		t.Fatal(err)
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, logger, s.proxy, s.dbSpec, nil, nil, waf)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/get/test")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// Repeat request with SQLi
	req.Header.SetMethod("GET")
	req.Header.SetCookie("test", "'+or+'1'='1")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	// check logs
	var expectedRuleId float64 = 942100

	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, err, "invalid JSON")

	triggeredRuleID, exists := logEntry["rule_id"].(float64)
	assert.True(t, exists, "field 'rule_id' doesn't exist")

	if triggeredRuleID != expectedRuleId {
		t.Errorf("Got rule_id: %f; Expected rule ID: %f", triggeredRuleID, expectedRuleId)
	}

}

func (s *ModSecIntegrationTests) basicMaliciousRequestPath(t *testing.T) {

	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()
	logger = logger.Level(zerolog.ErrorLevel)

	logErr := func(error types.MatchedRule) {
		logger.Error().
			Strs("tags", error.Rule().Tags()).
			Str("version", error.Rule().Version()).
			Str("severity", error.Rule().Severity().String()).
			Int("rule_id", error.Rule().ID()).
			Str("file", error.Rule().File()).
			Int("line", error.Rule().Line()).
			Int("maturity", error.Rule().Maturity()).
			Int("accuracy", error.Rule().Accuracy()).
			Str("uri", error.URI()).
			Msg(error.Message())
	}

	waf, err := coraza.NewWAF(
		coraza.NewWAFConfig().
			WithErrorCallback(logErr).
			WithDirectivesFromFile(s.wafConfFile).
			WithDirectivesFromFile(s.wafRules),
	)
	if err != nil {
		t.Fatal(err)
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, logger, s.proxy, s.dbSpec, nil, nil, waf)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/token/test")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// Repeat request with SQLi
	req.SetRequestURI("/token/'+or+'1'='1")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	// check logs
	var expectedRuleId float64 = 942100

	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, err, "invalid JSON")

	triggeredRuleID, exists := logEntry["rule_id"].(float64)
	assert.True(t, exists, "field 'rule_id' doesn't exist")

	if triggeredRuleID != expectedRuleId {
		t.Errorf("Got rule_id: %f; Expected rule ID: %f", triggeredRuleID, expectedRuleId)
	}

}

func (s *ModSecIntegrationTests) basicResponseActionRedirect(t *testing.T) {

	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()
	logger = logger.Level(zerolog.ErrorLevel)

	logErr := func(error types.MatchedRule) {
		logger.Error().
			Strs("tags", error.Rule().Tags()).
			Str("version", error.Rule().Version()).
			Str("severity", error.Rule().Severity().String()).
			Int("rule_id", error.Rule().ID()).
			Str("file", error.Rule().File()).
			Int("line", error.Rule().Line()).
			Int("maturity", error.Rule().Maturity()).
			Int("accuracy", error.Rule().Accuracy()).
			Str("uri", error.URI()).
			Msg(error.Message())
	}

	waf, err := coraza.NewWAF(
		coraza.NewWAFConfig().
			WithErrorCallback(logErr).
			WithDirectivesFromFile(s.wafConfFile).
			WithDirectivesFromFile(s.wafRules),
	)
	if err != nil {
		t.Fatal(err)
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, logger, s.proxy, s.dbSpec, nil, nil, waf)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/token/test")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// Repeat request with SQLi
	req.SetRequestURI("/token/test")
	req.Header.SetUserAgent("Test")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 302 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	expecrtedLocation := "http://www.example.com/failed.html"
	currentLocation := string(reqCtx.Response.Header.Peek("Location"))

	if currentLocation != expecrtedLocation {
		t.Errorf("Incorrect redirect URL. Expected: %s and got %s",
			expecrtedLocation, currentLocation)
	}

	// check logs
	var expectedRuleId float64 = 130

	var logEntry map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &logEntry)
	assert.NoError(t, err, "invalid JSON")

	triggeredRuleID, exists := logEntry["rule_id"].(float64)
	assert.True(t, exists, "field 'rule_id' doesn't exist")

	if triggeredRuleID != expectedRuleId {
		t.Errorf("Got rule_id: %f; Expected rule ID: %f", triggeredRuleID, expectedRuleId)
	}

}
