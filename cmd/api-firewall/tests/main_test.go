package tests

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/denylist"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/shadowAPI"
)

const openAPISpecTest = `
openapi: 3.0.1
info:
  title: Service
  version: 1.0.0
servers:
  - url: /
paths:
  /users/{id}/{test}:
    parameters:
      - in: path
        name: id
        schema:
          type: integer
        required: true
        description: The user ID.
    # GET /users/id1,id2,id3 - uses one or more user IDs.
    # Overrides the path-level {id} parameter.
    get:
      summary: Gets one or more users by ID.
      parameters:
        - in: path
          name: test
          required: true
          description: A comma-separated list of user IDs.
          schema:
            type: array
            items:
              type: integer
            minItems: 1
          explode: false
          style: simple
      responses:
        '200':
          description: OK
  /test/signup:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required:
                - email
                - firstname
                - lastname
              properties:
                email:
                  type: string
                  format: email
                  example: example@mail.com
                firstname:
                  type: string
                  example: test
                lastname:
                  type: string
                  example: test
      responses:
        '200':
          description: successful operation
          content:
            application/json:
              schema:
                type: object
                required:
                  - status
                properties:
                  status:
                    type: string
                    example: "success"
                  error:
                    type: string
        '403':
          description: operation forbidden
          content: {}
  '/test/{token}':
    get:
      parameters:
        - name: token
          in: path
          required: true
          schema:
            maxLength: 36
            minLength: 36
            type: string
        - name: id
          in: query
          required: true
          schema:
            pattern: '^\w{1,10}$'
            type: string
      responses:
        '200':
          description: Static page
          content: {}
        '403':
          description: operation forbidden
          content: {}
  /user:
    get:
      summary: Get User Info
      responses:
        200:
          description: Ok
          content: { }
        401:
          description: Unauthorized
          content: {}
      security:
        - petstore_auth:
          - read
  /user/1:
    get:
      summary: Get User Info with ID 1
      responses:
        200:
          description: Ok
          content: { }
        401:
          description: Unauthorized
          content: {}
      security:
        - petstore_auth:
          - read
          - write
  /test/headers/request:
    get:
      summary: Get Request to test Request Headers validation
      parameters:
        - in: header
          name: X-Request-Test
          schema:
            type: string
            format: uuid
          required: true
      responses:
        200:
          description: Ok
          content: { }
  /test/headers/response:
    get:
      summary: Get Request to test Response Headers validation 
      responses:
        200:
          description: Ok
          headers:
            X-Response-Test:
              schema:
                type: string
                format: uuid
              required: true
        401:
          description: Unauthorized
          content: {}
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

const (
	testOauthBearerToken = "testtesttest"
	testOauthJWTTokenRS  = "eyJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJqd3QudGVzdC5naXRodWIuaW8iLCJzdWIiOiJldmFuZGVyIiwiYXVkIjoibmFpbWlzaCIsImlhdCI6MTYzODUwNjIxNywiZXhwIjozNTMxOTM3ODc1LCJzY29wZSI6InJlYWQgd3JpdGUifQ.MPC35ZX52qWE4AktY1Bs-HVEWUUYrByfRVUSL9GbzZhZfXlfcNkF-qNRK_EDG2eviE4UHb6CFVZeYTsO5MyKg0H3shp79LeZTA2XzCuCZvzAqA7EQrpUKiKof-9af5g3jIRU4YFxvtpp8XxXGHaMvbIy4gqQJ7WEsOksYOytEsbLtsCs880zxCJb1iM4Bu9Q_Nl-wW1NeYSZyHYZP7es7gVvb9Bbm6qYW4qcVbt20pW4dguBGEvUvLM6axqeTZe7JgtqU__uUwkcIS6bu711Y7Zi-TpeZAMp506Wx8qZrhi7Ea0QFZUMjoF0O7jgRtps_BlbqBXNoleMO-kKnSkd6A"
	testOauthJWTTokenHS  = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2Mzg1MDU4OTYsImV4cCI6MTc3MDA0MTg5NiwiYXVkIjoid3d3LmV4YW1wbGUuY29tIiwic3ViIjoianJvY2tldEBleGFtcGxlLmNvbSIsIkdpdmVuTmFtZSI6IkpvaG5ueSIsIlN1cm5hbWUiOiJSb2NrZXQiLCJFbWFpbCI6Impyb2NrZXRAZXhhbXBsZS5jb20iLCJSb2xlIjoiTWFuYWdlciIsInNjb3BlIjoicmVhZCB3cml0ZSJ9.GgtDHEjw_zCbzcYR0rxrC-A2QKDeSpif7QBhCUlmqdk"
	testOauthJWTKeyHS    = "qwertyuiopasdfghjklzxcvbnm123456"
	testContentType      = "test"

	testDeniedCookieName = "testCookieName"
	testDeniedToken      = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzb21lIjoicGF5bG9hZDk5OTk5ODUifQ.S9P-DEiWg7dlI81rLjnJWCA6h9Q4ewTizxrsxOPGmNA"

	testShadowAPIendpoint = "/shadowAPItest"

	testRequestHeader  = "X-Request-Test"
	testResponseHeader = "X-Response-Test"
)

type ServiceTests struct {
	serverUrl  *url.URL
	shutdown   chan os.Signal
	logger     *logrus.Logger
	proxy      *proxy.MockPool
	client     *proxy.MockHTTPClient
	swagRouter *router.Router
	shadowAPI  *shadowAPI.MockChecker
}

func compressFlate(data []byte) ([]byte, error) {
	var b bytes.Buffer
	w, err := flate.NewWriter(&b, 9)
	if err != nil {
		return nil, err
	}
	if _, err = w.Write(data); err != nil {
		return nil, err
	}
	if err = w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func compressBrotli(data []byte) ([]byte, error) {
	var b bytes.Buffer
	w := brotli.NewWriterLevel(&b, brotli.BestCompression)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func compressGzip(data []byte) ([]byte, error) {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// POST /test/signup <- 200
// POST /test/shadow <- 200
func TestBasic(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	serverUrl, err := url.ParseRequestURI("http://127.0.0.1:80")
	if err != nil {
		t.Fatalf("parsing API Host URL: %s", err.Error())
	}

	pool := proxy.NewMockPool(mockCtrl)
	client := proxy.NewMockHTTPClient(mockCtrl)
	checker := shadowAPI.NewMockChecker(mockCtrl)

	swagger, err := openapi3.NewLoader().LoadFromData([]byte(openAPISpecTest))
	if err != nil {
		t.Fatalf("loading swagwaf file: %s", err.Error())
	}

	swagRouter, err := router.NewRouter(swagger)
	if err != nil {
		t.Fatalf("parsing swagwaf file: %s", err.Error())
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	apifwTests := ServiceTests{
		serverUrl:  serverUrl,
		shutdown:   shutdown,
		logger:     logger,
		proxy:      pool,
		client:     client,
		swagRouter: swagRouter,
		shadowAPI:  checker,
	}

	// basic test
	t.Run("basicBlockBlockMode", apifwTests.testBlockMode)
	t.Run("basicLogOnlyLogOnlyMode", apifwTests.testLogOnlyMode)
	t.Run("basicDisableDisableMode", apifwTests.testDisableMode)

	t.Run("basicBlockLogOnlyMode", apifwTests.testBlockLogOnlyMode)
	t.Run("basicLogOnlyBlockMode", apifwTests.testLogOnlyBlockMode)

	t.Run("commonParamters", apifwTests.testCommonParameters)

	t.Run("basicDenylist", apifwTests.testDenylist)
	t.Run("basicShadowAPI", apifwTests.testShadowAPI)

	t.Run("oauthIntrospectionReadSuccess", apifwTests.testOauthIntrospectionReadSuccess)
	t.Run("oauthIntrospectionReadUnsuccessful", apifwTests.testOauthIntrospectionReadUnsuccessful)
	t.Run("oauthIntrospectionInvalidResponse", apifwTests.testOauthIntrospectionInvalidResponse)
	t.Run("oauthIntrospectionReadWriteSuccess", apifwTests.testOauthIntrospectionReadWriteSuccess)
	t.Run("oauthIntrospectionContentTypeRequest", apifwTests.testOauthIntrospectionContentTypeRequest)

	t.Run("oauthJWTRS256", apifwTests.testOauthJWTRS256)
	t.Run("oauthJWTHS256", apifwTests.testOauthJWTHS256)

	t.Run("requestHeaders", apifwTests.testRequestHeaders)
	t.Run("responseHeaders", apifwTests.testResponseHeaders)

	t.Run("responseBodyCompressionGzip", apifwTests.testResponseBodyCompressionGzip)
	t.Run("responseBodyCompressionBr", apifwTests.testResponseBodyCompressionBr)
	t.Run("responseBodyCompressionDeflate", apifwTests.testResponseBodyCompressionDeflate)

}

func (s *ServiceTests) testBlockMode(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	p, err := json.Marshal(map[string]interface{}{
		"firstname": "test",
		"lastname":  "test",
		"job":       "test",
		"email":     "test@wallarm.com",
		"url":       "http://wallarm.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

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

	// Repeat request with invalid email
	reqInvalidEmail, err := json.Marshal(map[string]interface{}{
		"firstname": "test",
		"lastname":  "test",
		"job":       "test",
		"email":     "wallarm.com",
		"url":       "http://wallarm.com",
	})

	req.SetBodyStream(bytes.NewReader(reqInvalidEmail), -1)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.proxy.EXPECT().Put(s.client)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testDenylist(t *testing.T) {

	tokensCfg := config.Token{
		CookieName: testDeniedCookieName,
		HeaderName: "",
		File:       "../../../resources/test/tokens/test.db",
	}

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Denylist: struct {
			Tokens config.Token
		}{Tokens: tokensCfg},
	}

	deniedTokens, err := denylist.New(&cfg, s.logger)
	if err != nil {
		t.Fatal(err)
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, deniedTokens, s.shadowAPI)

	p, err := json.Marshal(map[string]interface{}{
		"firstname": "test",
		"lastname":  "test",
		"job":       "test",
		"email":     "test@wallarm.com",
		"url":       "http://wallarm.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

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

	// add denied token to the Cookie header of the successful HTTP request (200)
	req.Header.SetCookie(testDeniedCookieName, testDeniedToken)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testShadowAPI(t *testing.T) {

	tokensCfg := config.Token{
		CookieName: testDeniedCookieName,
		HeaderName: "",
		File:       "",
	}

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "LOG_ONLY",
		ResponseValidation:        "LOG_ONLY",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Denylist: struct {
			Tokens config.Token
		}{Tokens: tokensCfg},
	}

	deniedTokens, err := denylist.New(&cfg, s.logger)
	if err != nil {
		t.Fatal(err)
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, deniedTokens, s.shadowAPI)

	p, err := json.Marshal(map[string]interface{}{
		"firstname": "test",
		"lastname":  "test",
		"job":       "test",
		"email":     "test@wallarm.com",
		"url":       "http://wallarm.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI(testShadowAPIendpoint)
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.shadowAPI.EXPECT().Check(gomock.Any()).Times(1)
	s.proxy.EXPECT().Put(s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testLogOnlyMode(t *testing.T) {
	var cfg = config.APIFWConfiguration{
		RequestValidation:         "LOG_ONLY",
		ResponseValidation:        "LOG_ONLY",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	p, err := json.Marshal(map[string]interface{}{
		"firstname": "test",
		"lastname":  "test",
		"job":       "test",
		"email":     "wallarm.com",
		"url":       "http://wallarm.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.shadowAPI.EXPECT().Check(gomock.Any()).Times(0)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testDisableMode(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "DISABLE",
		ResponseValidation:        "DISABLE",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	p, err := json.Marshal(map[string]interface{}{
		"email": "wallarm.com",
		"url":   "http://wallarm.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

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

}

func (s *ServiceTests) testBlockLogOnlyMode(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "LOG_ONLY",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	p, err := json.Marshal(map[string]interface{}{
		"firstname": "test",
		"lastname":  "test",
		"job":       "test",
		"email":     "test@wallarm.com",
		"url":       "http://wallarm.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	// 503 status code not defined in the OpenAPI spec
	resp.SetStatusCode(fasthttp.StatusServiceUnavailable)

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != fasthttp.StatusServiceUnavailable {
		t.Errorf("Incorrect response status code. Expected: 503 and got %d",
			reqCtx.Response.StatusCode())
	}
}

func (s *ServiceTests) testLogOnlyBlockMode(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "LOG_ONLY",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	p, err := json.Marshal(map[string]interface{}{
		"firstname": "test",
		"lastname":  "test",
		"job":       "test",
		"email":     "test@wallarm.com",
		"url":       "http://wallarm.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	// 503 status code not defined in the OpenAPI spec
	resp.SetStatusCode(fasthttp.StatusServiceUnavailable)

	reqCtx := fasthttp.RequestCtx{
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

func (s *ServiceTests) testCommonParameters(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/users/1/1")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

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

}

func introspectionEndpointWithoutRead(ctx *fasthttp.RequestCtx) {
	authHeader := string(ctx.Request.Header.Peek("Authorization"))
	contentType := string(ctx.Request.Header.ContentType())
	if authHeader == "Bearer "+testOauthBearerToken && contentType == "" {
		ctx.SetBodyString("{\n\t\t\"active\": true,\n\t\t\"client_id\": \"l238j323ds-23ij4\",\n\t\t\"username\": \"jdoe\",\n\t\t\"scope\": \"dolphin\",\n\t\t\"sub\": \"Z5O3upPC88QrAjx00dis\",\n\t\t\"aud\": \"https://protected.example.net/resource\",\n\t\t\"iss\": \"https://server.example.com/\",\n\t\t\"exp\": 1419356238,\n\t\t\"iat\": 1419350238,\n\t\t\"extension_field\": \"twenty-seven\"\n\t}")
		ctx.SetStatusCode(fasthttp.StatusOK)
	} else {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
	}
}

func introspectionEndpointInvalid(ctx *fasthttp.RequestCtx) {
	ctx.SetBodyString("{\n\t\t\"active\": false\n\t}")
	ctx.SetStatusCode(fasthttp.StatusOK)
}

func introspectionEndpointWithRead(ctx *fasthttp.RequestCtx) {
	failRequest := string(ctx.Request.Header.Peek("X-Fail-Request"))
	if failRequest == "1" {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}
	authHeader := string(ctx.Request.Header.Peek("Authorization"))
	if authHeader == "Bearer "+testOauthBearerToken {
		ctx.SetBodyString("{\n\t\t\"active\": true,\n\t\t\"client_id\": \"l238j323ds-23ij4\",\n\t\t\"username\": \"jdoe\",\n\t\t\"scope\": \"read dolphin\",\n\t\t\"sub\": \"Z5O3upPC88QrAjx00dis\",\n\t\t\"aud\": \"https://protected.example.net/resource\",\n\t\t\"iss\": \"https://server.example.com/\",\n\t\t\"exp\": 1419356238,\n\t\t\"iat\": 1419350238,\n\t\t\"extension_field\": \"twenty-seven\"\n\t}")
		ctx.SetStatusCode(fasthttp.StatusOK)
	} else {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
	}
}

func introspectionEndpointWithReadWrite(ctx *fasthttp.RequestCtx) {
	authHeader := string(ctx.Request.Header.Peek("Authorization"))
	contentType := string(ctx.Request.Header.ContentType())
	if authHeader == "Bearer "+testOauthBearerToken && contentType == "" {
		ctx.SetBodyString("{\n\t\t\"active\": true,\n\t\t\"client_id\": \"l238j323ds-23ij4\",\n\t\t\"username\": \"jdoe\",\n\t\t\"scope\": \"read write\",\n\t\t\"sub\": \"Z5O3upPC88QrAjx00dis\",\n\t\t\"aud\": \"https://protected.example.net/resource\",\n\t\t\"iss\": \"https://server.example.com/\",\n\t\t\"exp\": 1419356238,\n\t\t\"iat\": 1419350238,\n\t\t\"extension_field\": \"twenty-seven\"\n\t}")
		ctx.SetStatusCode(fasthttp.StatusOK)
	} else {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
	}
}

func testContentTypeHandler(ctx *fasthttp.RequestCtx) {
	authHeader := string(ctx.Request.Header.Peek("Authorization"))
	contentType := string(ctx.Request.Header.ContentType())
	if contentType == testContentType && authHeader == "Bearer "+testOauthBearerToken {
		ctx.SetBodyString("{\n\t\t\"active\": true,\n\t\t\"client_id\": \"l238j323ds-23ij4\",\n\t\t\"username\": \"jdoe\",\n\t\t\"scope\": \"read write\",\n\t\t\"sub\": \"Z5O3upPC88QrAjx00dis\",\n\t\t\"aud\": \"https://protected.example.net/resource\",\n\t\t\"iss\": \"https://server.example.com/\",\n\t\t\"exp\": 1419356238,\n\t\t\"iat\": 1419350238,\n\t\t\"extension_field\": \"twenty-seven\"\n\t}")
		ctx.SetStatusCode(fasthttp.StatusOK)
	} else {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
	}
}

func startServerOnPort(t *testing.T, port int, h fasthttp.RequestHandler) io.Closer {
	ln, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		t.Fatalf("cannot start tcp server on port %d: %s", port, err)
	}
	go fasthttp.Serve(ln, h)
	return ln
}

func (s *ServiceTests) testOauthIntrospectionReadSuccess(t *testing.T) {

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/user")
	req.Header.SetMethod("GET")
	req.Header.Set("Authorization", "Bearer "+testOauthBearerToken)

	port := 28282
	defer startServerOnPort(t, port, introspectionEndpointWithRead).Close()

	oauthConf := config.Oauth{
		ValidationType: "INTROSPECTION",
		JWT:            config.JWT{},
		Introspection: config.Introspection{
			ClientAuthBearerToken: "",
			Endpoint:              "http://localhost:28282",
			EndpointParams:        "",
			TokenParamName:        "",
			EndpointMethod:        "GET",
			RefreshInterval:       time.Second * 100,
		},
	}

	serverConf := config.Server{
		URL:                "",
		ClientPoolCapacity: 1000,
		InsecureConnection: false,
		RootCA:             "",
		MaxConnsPerHost:    512,
		ReadTimeout:        time.Second * 5,
		WriteTimeout:       time.Second * 5,
		DialTimeout:        time.Second * 5,
		Oauth:              oauthConf,
	}

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Server: serverConf,
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

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

	// using introspection cache to get response info
	req.Header.Set("X-Fail-Request", "1")

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
}

func (s *ServiceTests) testOauthIntrospectionReadUnsuccessful(t *testing.T) {

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/user")
	req.Header.SetMethod("GET")
	req.Header.Set("Authorization", "Bearer "+testOauthBearerToken)

	port := 28283
	defer startServerOnPort(t, port, introspectionEndpointWithoutRead).Close()

	oauthConf := config.Oauth{
		ValidationType: "INTROSPECTION",
		JWT:            config.JWT{},
		Introspection: config.Introspection{
			ClientAuthBearerToken: "",
			Endpoint:              "http://localhost:28283",
			EndpointParams:        "",
			TokenParamName:        "",
			EndpointMethod:        "GET",
			RefreshInterval:       time.Second * 100,
		},
	}

	serverConf := config.Server{
		URL:                "",
		ClientPoolCapacity: 1000,
		InsecureConnection: false,
		RootCA:             "",
		MaxConnsPerHost:    512,
		ReadTimeout:        time.Second * 5,
		WriteTimeout:       time.Second * 5,
		DialTimeout:        time.Second * 5,
		Oauth:              oauthConf,
	}

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Server: serverConf,
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.proxy.EXPECT().Put(s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testOauthIntrospectionInvalidResponse(t *testing.T) {

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/user")
	req.Header.SetMethod("GET")
	req.Header.Set("Authorization", "Bearer "+testOauthBearerToken)

	port := 28283
	defer startServerOnPort(t, port, introspectionEndpointInvalid).Close()

	oauthConf := config.Oauth{
		ValidationType: "INTROSPECTION",
		JWT:            config.JWT{},
		Introspection: config.Introspection{
			ClientAuthBearerToken: "",
			Endpoint:              "http://localhost:28283",
			EndpointParams:        "",
			TokenParamName:        "",
			EndpointMethod:        "GET",
			RefreshInterval:       time.Second * 100,
		},
	}

	serverConf := config.Server{
		URL:                "",
		ClientPoolCapacity: 1000,
		InsecureConnection: false,
		RootCA:             "",
		MaxConnsPerHost:    512,
		ReadTimeout:        time.Second * 5,
		WriteTimeout:       time.Second * 5,
		DialTimeout:        time.Second * 5,
		Oauth:              oauthConf,
	}

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Server: serverConf,
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, nil)
	//s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testOauthIntrospectionReadWriteSuccess(t *testing.T) {

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/user/1")
	req.Header.SetMethod("GET")
	req.Header.Set("Authorization", "Bearer "+testOauthBearerToken)

	port := 28284
	defer startServerOnPort(t, port, introspectionEndpointWithReadWrite).Close()

	oauthConf := config.Oauth{
		ValidationType: "INTROSPECTION",
		JWT:            config.JWT{},
		Introspection: config.Introspection{
			ClientAuthBearerToken: "",
			Endpoint:              "http://localhost:28284",
			EndpointParams:        "",
			TokenParamName:        "",
			EndpointMethod:        "GET",
			RefreshInterval:       time.Second * 100,
		},
	}

	serverConf := config.Server{
		URL:                "",
		ClientPoolCapacity: 1000,
		InsecureConnection: false,
		RootCA:             "",
		MaxConnsPerHost:    512,
		ReadTimeout:        time.Second * 5,
		WriteTimeout:       time.Second * 5,
		DialTimeout:        time.Second * 5,
		Oauth:              oauthConf,
	}

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Server: serverConf,
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

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

}

func (s *ServiceTests) testOauthIntrospectionContentTypeRequest(t *testing.T) {

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/user/1")
	req.Header.SetMethod("GET")
	req.Header.Set("Authorization", "Bearer "+testOauthBearerToken)

	port := 28285
	defer startServerOnPort(t, port, testContentTypeHandler).Close()

	oauthConf := config.Oauth{
		ValidationType: "INTROSPECTION",
		JWT:            config.JWT{},
		Introspection: config.Introspection{
			ContentType:           "test",
			ClientAuthBearerToken: "",
			Endpoint:              "http://localhost:28285",
			EndpointParams:        "",
			TokenParamName:        "",
			EndpointMethod:        "GET",
			RefreshInterval:       time.Second * 100,
		},
	}

	serverConf := config.Server{
		URL:                "",
		ClientPoolCapacity: 1000,
		InsecureConnection: false,
		RootCA:             "",
		MaxConnsPerHost:    512,
		ReadTimeout:        time.Second * 5,
		WriteTimeout:       time.Second * 5,
		DialTimeout:        time.Second * 5,
		Oauth:              oauthConf,
	}

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Server: serverConf,
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

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

}

func (s *ServiceTests) testOauthJWTRS256(t *testing.T) {

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/user")
	req.Header.SetMethod("GET")
	req.Header.Set("Authorization", "Bearer "+testOauthJWTTokenRS)

	oauthConf := config.Oauth{
		ValidationType: "JWT",
		JWT: config.JWT{
			SignatureAlgorithm: "RS256",
			PubCertFile:        "../../../resources/test/jwt/pub.pem",
		},
		Introspection: config.Introspection{},
	}

	serverConf := config.Server{
		URL:                "",
		ClientPoolCapacity: 1000,
		InsecureConnection: false,
		RootCA:             "",
		MaxConnsPerHost:    512,
		ReadTimeout:        time.Second * 5,
		WriteTimeout:       time.Second * 5,
		DialTimeout:        time.Second * 5,
		Oauth:              oauthConf,
	}

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Server: serverConf,
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

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

	// Send request with incorrect JWT token
	req.Header.Set("Authorization", "Bearer 123")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.proxy.EXPECT().Put(s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testOauthJWTHS256(t *testing.T) {

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/user")
	req.Header.SetMethod("GET")
	req.Header.Set("Authorization", "Bearer "+testOauthJWTTokenHS)

	oauthConf := config.Oauth{
		ValidationType: "JWT",
		JWT: config.JWT{
			SignatureAlgorithm: "HS256",
			SecretKey:          testOauthJWTKeyHS,
		},
		Introspection: config.Introspection{},
	}

	serverConf := config.Server{
		URL:                "",
		ClientPoolCapacity: 1000,
		InsecureConnection: false,
		RootCA:             "",
		MaxConnsPerHost:    512,
		ReadTimeout:        time.Second * 5,
		WriteTimeout:       time.Second * 5,
		DialTimeout:        time.Second * 5,
		Oauth:              oauthConf,
	}

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Server: serverConf,
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

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

	// Send request with incorrect JWT token
	req.Header.Set("Authorization", "Bearer invalid")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.proxy.EXPECT().Put(s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testRequestHeaders(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	xReqTestValue := uuid.New()

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/headers/request")
	req.Header.SetMethod("GET")
	req.Header.Set(testRequestHeader, xReqTestValue.String())

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

	// Repeat request without a required header
	req.Header.Del(testRequestHeader)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.proxy.EXPECT().Put(s.client)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testResponseHeaders(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	xRespTestValue := uuid.New()

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/headers/response")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.Set(testResponseHeader, xRespTestValue.String())
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

	// Repeat request without a required header
	resp.Header.Del(testResponseHeader)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testResponseBodyCompressionGzip(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	p, err := json.Marshal(map[string]interface{}{
		"firstname": "test",
		"lastname":  "test",
		"job":       "test",
		"email":     "test@wallarm.com",
		"url":       "http://wallarm.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.Header.Set("Content-Encoding", "gzip")

	// compress using gzip
	body, err := compressGzip([]byte("{\"status\":\"success\"}"))
	if err != nil {
		t.Fatal(err)
	}
	resp.SetBody(body)

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

	// Repeat request with wrong JSON in response

	// compress using gzip
	body, err = compressGzip([]byte("{\"status\": 123}"))
	if err != nil {
		t.Fatal(err)
	}
	resp.SetBody(body)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testResponseBodyCompressionBr(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	p, err := json.Marshal(map[string]interface{}{
		"firstname": "test",
		"lastname":  "test",
		"job":       "test",
		"email":     "test@wallarm.com",
		"url":       "http://wallarm.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.Header.Set("Content-Encoding", "br")

	// compress using brotli
	body, err := compressBrotli([]byte("{\"status\":\"success\"}"))
	if err != nil {
		t.Fatal(err)
	}
	resp.SetBody(body)

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

	// Repeat request with wrong JSON in response

	// compress using brotli
	body, err = compressBrotli([]byte("{\"status\": 123}"))
	if err != nil {
		t.Fatal(err)
	}
	resp.SetBody(body)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testResponseBodyCompressionDeflate(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := handlers.OpenapiProxy(&cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil, s.shadowAPI)

	p, err := json.Marshal(map[string]interface{}{
		"firstname": "test",
		"lastname":  "test",
		"job":       "test",
		"email":     "test@wallarm.com",
		"url":       "http://wallarm.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.Header.Set("Content-Encoding", "deflate")

	// compress using flate
	body, err := compressFlate([]byte("{\"status\":\"success\"}"))
	if err != nil {
		t.Fatal(err)
	}
	resp.SetBody(body)

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

	// Repeat request with wrong JSON in response

	// compress using flate
	body, err = compressFlate([]byte("{\"status\": 123}"))
	if err != nil {
		t.Fatal(err)
	}
	resp.SetBody(body)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(s.client)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}
