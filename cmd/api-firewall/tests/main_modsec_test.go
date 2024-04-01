package tests

import (
	"net/url"
	"os"
	"os/signal"
	"path"
	"syscall"
	"testing"

	"github.com/corazawaf/coraza/v3"
	"github.com/corazawaf/coraza/v3/types"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/valyala/fasthttp"
	proxy2 "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/proxy"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/router"
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
	serverUrl  *url.URL
	shutdown   chan os.Signal
	logger     *logrus.Logger
	proxy      *proxy.MockPool
	client     *proxy.MockHTTPClient
	swagRouter *router.Router
	waf        coraza.WAF
	loggerHook *test.Hook
}

func TestModSec(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	testLogger, hook := test.NewNullLogger()
	testLogger.SetLevel(logrus.ErrorLevel)

	serverUrl, err := url.ParseRequestURI("http://127.0.0.1:80")
	if err != nil {
		t.Fatalf("parsing API Host URL: %s", err.Error())
	}

	pool := proxy.NewMockPool(mockCtrl)
	client := proxy.NewMockHTTPClient(mockCtrl)

	swagger, err := openapi3.NewLoader().LoadFromData([]byte(openAPISpecModSecTest))
	if err != nil {
		t.Fatalf("loading swagwaf file: %s", err.Error())
	}

	swagRouter, err := router.NewRouter(swagger)
	if err != nil {
		t.Fatalf("parsing swagwaf file: %s", err.Error())
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	var waf coraza.WAF

	msRulesDir := "../../../resources/test/modsec/rules_test"
	msConfFile := "../../../resources/test/modsec/coraza.conf"

	logErr := func(error types.MatchedRule) {
		testLogger.WithFields(logrus.Fields{
			"tags":     error.Rule().Tags(),
			"version":  error.Rule().Version(),
			"severity": error.Rule().Severity(),
			"rule_id":  error.Rule().ID(),
			"file":     error.Rule().File(),
			"line":     error.Rule().Line(),
			"maturity": error.Rule().Maturity(),
			"accuracy": error.Rule().Accuracy(),
			"uri":      error.URI(),
		}).Error(error.Message())
	}

	rules := path.Join(msRulesDir, "*.conf")
	waf, err = coraza.NewWAF(
		coraza.NewWAFConfig().
			WithErrorCallback(logErr).
			WithDirectivesFromFile(msConfFile).
			WithDirectivesFromFile(rules),
	)
	if err != nil {
		t.Fatal(err)
	}

	apifwTests := ModSecIntegrationTests{
		serverUrl:  serverUrl,
		shutdown:   shutdown,
		logger:     testLogger,
		proxy:      pool,
		client:     client,
		swagRouter: swagRouter,
		waf:        waf,
		loggerHook: hook,
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

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, nil, s.waf)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

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

	var cfg = config.ProxyMode{
		RequestValidation:         "LOG_ONLY",
		ResponseValidation:        "LOG_ONLY",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, nil, s.waf)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	ruleId := 942100
	triggeredRuleID := s.loggerHook.AllEntries()[0].Data["rule_id"]

	if triggeredRuleID != ruleId {
		t.Errorf("Got  message: %s; Expected triggered rule ID: %d", triggeredRuleID, ruleId)
	}

	s.loggerHook.Reset()

}

func (s *ModSecIntegrationTests) basicMaliciousRequestDisableMode(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "DISABLE",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, nil, s.waf)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	if len(s.loggerHook.AllEntries()) > 0 {
		t.Errorf("Expected number of errors is 0. Got %d errors", len(s.loggerHook.AllEntries()))
	}
}

func (s *ModSecIntegrationTests) basicMaliciousResponseBlockMode(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, nil, s.waf)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ModSecIntegrationTests) basicMaliciousResponseLogOnlyMode(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "LOG_ONLY",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, nil, s.waf)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	ruleId := 955110
	triggeredRuleID := s.loggerHook.AllEntries()[0].Data["rule_id"]

	if triggeredRuleID != ruleId {
		t.Errorf("Got  message: %d; Expected triggered rule ID: %d", triggeredRuleID, ruleId)
	}

	s.loggerHook.Reset()

}

func (s *ModSecIntegrationTests) basicMaliciousResponseDisableMode(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "DISABLE",
		ResponseValidation:        "DISABLE",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, nil, s.waf)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	if len(s.loggerHook.AllEntries()) > 0 {
		t.Errorf("Expected number of errors is 0. Got %d errors", len(s.loggerHook.AllEntries()))
	}

	s.loggerHook.Reset()

}

func (s *ModSecIntegrationTests) basicMaliciousRequestBody(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, nil, s.waf)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

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

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, nil, s.waf)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

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

	ruleId := 942100
	triggeredRuleID := s.loggerHook.AllEntries()[0].Data["rule_id"]

	if triggeredRuleID != ruleId {
		t.Errorf("Got  message: %d; Expected triggered rule ID: %d", triggeredRuleID, ruleId)
	}

	s.loggerHook.Reset()

}

func (s *ModSecIntegrationTests) basicMaliciousRequestCookie(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, nil, s.waf)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

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

	ruleId := 942100
	triggeredRuleID := s.loggerHook.AllEntries()[0].Data["rule_id"]

	if triggeredRuleID != ruleId {
		t.Errorf("Got  message: %d; Expected triggered rule ID: %d", triggeredRuleID, ruleId)
	}

	s.loggerHook.Reset()

}

func (s *ModSecIntegrationTests) basicMaliciousRequestPath(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, nil, s.waf)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

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

	ruleId := 942100
	triggeredRuleID := s.loggerHook.AllEntries()[0].Data["rule_id"]

	if triggeredRuleID != ruleId {
		t.Errorf("Got  message: %d; Expected triggered rule ID: %d", triggeredRuleID, ruleId)
	}

	s.loggerHook.Reset()

}

func (s *ModSecIntegrationTests) basicResponseActionRedirect(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, nil, s.waf)

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

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

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

	ruleId := 130
	triggeredRuleID := s.loggerHook.AllEntries()[0].Data["rule_id"]

	if triggeredRuleID != ruleId {
		t.Errorf("Got  message: %d; Expected triggered rule ID: %d", triggeredRuleID, ruleId)
	}

	s.loggerHook.Reset()

}
