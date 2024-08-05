package tests

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/savsgio/gotils/strconv"
	"io"
	"net"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"

	proxy2 "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/proxy"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/allowiplist"
	"github.com/wallarm/api-firewall/internal/platform/denylist"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/storage"
)

const (
	resolvedIP      = "127.0.0.1"
	openAPISpecTest = `
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
  /cookie_params_min_max:
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
        - name: cookie_optional_min_max
          in: cookie
          description: optional cookie parameter
          required: false
          schema:
            type: integer
            minimum: 1000
            maximum: 2000
      responses:
        '200':
          description: Set cookies.
          content: {}
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
          application/unsupported-type:
            schema:
              {}
          application/x-www-form-urlencoded:
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
                  pattern: '^[0-9a-zA-Z]+@[0-9a-zA-Z\.]+$'
                  example: example@mail.com
                firstname:
                  type: string
                  example: test
                lastname:
                  type: string
                  example: test
                url:
                  type: string
                  example: test
                job:
                  type: string
                  example: test
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
                  pattern: '^[0-9a-zA-Z]+@[0-9a-zA-Z\.]+$'
                  example: example@mail.com
                firstname:
                  type: string
                  example: test
                lastname:
                  type: string
                  example: test
                url:
                  type: string
                  example: test
                job:
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
  /get/test:
    get:
      summary: Get Test Info
      responses:
        200:
          description: Ok
          content: { }
  /path/{test}:
    get:
      parameters:
        - name: test
          in: path
          required: true
          schema:
            type: string
            enum:
              - testValue1
              - testValue1
      summary: Get Test Info
      responses:
        200:
          description: Ok
          content: { }
  /path/{test}.php:
    get:
      parameters:
        - name: test
          in: path
          required: true
          schema:
            type: string
            enum:
              - value1
              - value2
      summary: Get Test Info
      responses:
        200:
          description: Ok
          content: { }
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
)

var testSupportedEncodingSchemas = []string{"gzip", "deflate", "br"}

const (
	testOauthBearerToken = "testtesttest"
	testOauthJWTTokenRS  = "eyJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJqd3QudGVzdC5naXRodWIuaW8iLCJzdWIiOiJldmFuZGVyIiwiYXVkIjoibmFpbWlzaCIsImlhdCI6MTYzODUwNjIxNywiZXhwIjozNTMxOTM3ODc1LCJzY29wZSI6InJlYWQgd3JpdGUifQ.MPC35ZX52qWE4AktY1Bs-HVEWUUYrByfRVUSL9GbzZhZfXlfcNkF-qNRK_EDG2eviE4UHb6CFVZeYTsO5MyKg0H3shp79LeZTA2XzCuCZvzAqA7EQrpUKiKof-9af5g3jIRU4YFxvtpp8XxXGHaMvbIy4gqQJ7WEsOksYOytEsbLtsCs880zxCJb1iM4Bu9Q_Nl-wW1NeYSZyHYZP7es7gVvb9Bbm6qYW4qcVbt20pW4dguBGEvUvLM6axqeTZe7JgtqU__uUwkcIS6bu711Y7Zi-TpeZAMp506Wx8qZrhi7Ea0QFZUMjoF0O7jgRtps_BlbqBXNoleMO-kKnSkd6A"
	testOauthJWTTokenHS  = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2Mzg1MDU4OTYsImV4cCI6MTc3MDA0MTg5NiwiYXVkIjoid3d3LmV4YW1wbGUuY29tIiwic3ViIjoianJvY2tldEBleGFtcGxlLmNvbSIsIkdpdmVuTmFtZSI6IkpvaG5ueSIsIlN1cm5hbWUiOiJSb2NrZXQiLCJFbWFpbCI6Impyb2NrZXRAZXhhbXBsZS5jb20iLCJSb2xlIjoiTWFuYWdlciIsInNjb3BlIjoicmVhZCB3cml0ZSJ9.GgtDHEjw_zCbzcYR0rxrC-A2QKDeSpif7QBhCUlmqdk"
	testOauthJWTKeyHS    = "qwertyuiopasdfghjklzxcvbnm123456"
	testContentType      = "test"

	testDeniedCookieName = "testCookieName"
	testDeniedToken      = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzb21lIjoicGF5bG9hZDk5OTk5ODUifQ.S9P-DEiWg7dlI81rLjnJWCA6h9Q4ewTizxrsxOPGmNA"

	testAllowIPHeaderName        = "X-Real-IP"
	testAllowIPXForwardedForName = "X-Forwarded-For"

	testShadowAPIendpoint = "/shadowAPItest"

	testRequestHeader  = "X-Request-Test"
	testResponseHeader = "X-Response-Test"
)

var (
	listOfAllowedIPs    = []string{"127.0.0.1", "127.0.0.3", "10.1.2.128", "10.1.2.254", "2001:0db8:11a3:09d7:1f34:8a2e:07a0:765d", "2001:0db8:11a3:09d7:1f34:8a2e:07a0:7655"}
	listOfNotAllowedIPs = []string{"1.1.1.1", "10.1.2.0", "10.1.3.1", "10.1.2.255"}
)

type ServiceTests struct {
	serverUrl *url.URL
	shutdown  chan os.Signal
	logger    *logrus.Logger
	proxy     *proxy.MockPool
	client    *proxy.MockHTTPClient
	lock      *sync.RWMutex
	dbSpec    *storage.MockDBOpenAPILoader
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

func compressData(data []byte, encodingSchema string) ([]byte, error) {
	switch encodingSchema {
	case "br":
		return compressBrotli(data)
	case "deflate":
		return compressFlate(data)
	case "gzip":
		return compressGzip(data)
	}

	return nil, errors.New("encoding schema not supported")
}

// POST /test/signup <- 200
// POST /test/shadow <- 200
func TestBasic(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex
	dbSpec := storage.NewMockDBOpenAPILoader(mockCtrl)

	serverUrl, err := url.ParseRequestURI("http://127.0.0.1:80")
	if err != nil {
		t.Fatalf("parsing API Host URL: %s", err.Error())
	}

	pool := proxy.NewMockPool(mockCtrl)
	client := proxy.NewMockHTTPClient(mockCtrl)

	swagger, err := openapi3.NewLoader().LoadFromData([]byte(openAPISpecTest))
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

	apifwTests := ServiceTests{
		serverUrl: serverUrl,
		shutdown:  shutdown,
		logger:    logger,
		proxy:     pool,
		client:    client,
		lock:      &lock,
		dbSpec:    dbSpec,
	}

	// basic test
	t.Run("basicCustomBlockStatusCode", apifwTests.testCustomBlockStatusCode)
	t.Run("basicPathNotExists", apifwTests.testPathNotExists)
	t.Run("basicBlockBlockMode", apifwTests.testBlockMode)
	t.Run("basicLogOnlyLogOnlyMode", apifwTests.testLogOnlyMode)
	t.Run("basicDisableDisableMode", apifwTests.testDisableMode)

	t.Run("basicBlockLogOnlyMode", apifwTests.testBlockLogOnlyMode)
	t.Run("basicLogOnlyBlockMode", apifwTests.testLogOnlyBlockMode)

	t.Run("commonParameters", apifwTests.testCommonParameters)

	t.Run("basicDenylist", apifwTests.testDenylist)
	t.Run("basicAllowlist", apifwTests.testAllowlist)
	t.Run("basicAllowlistXForwardedFor", apifwTests.testAllowlistXForwardedFor)
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

	t.Run("reqBodyCompression", apifwTests.testRequestBodyCompression)
	t.Run("respBodyCompression", apifwTests.testResponseBodyCompression)

	t.Run("requestOptionalCookies", apifwTests.requestOptionalCookies)
	t.Run("requestOptionalMinMaxCookies", apifwTests.requestOptionalMinMaxCookies)

	// unknown parameters in requests
	t.Run("unknownParamQuery", apifwTests.unknownParamQuery)
	t.Run("unknownParamPostBody", apifwTests.unknownParamPostBody)
	t.Run("unknownParamJSONParam", apifwTests.unknownParamJSONParam)
	t.Run("unknownParamInvalidMimeType", apifwTests.unknownParamUnsupportedMimeType)

	t.Run("testConflictPaths", apifwTests.testConflictPaths)

	t.Run("testCustomHostHeader", apifwTests.testCustomHostHeader)
	t.Run("testCustomHeaderOASviaURL", apifwTests.testCustomHeaderOASviaURL)
}

func (s *ServiceTests) testCustomBlockStatusCode(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

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
	req.SetRequestURI("/test/signupNotExist")
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

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != cfg.CustomBlockStatusCode {
		t.Errorf("Incorrect response status code. Expected: %d and got %d",
			cfg.CustomBlockStatusCode, reqCtx.Response.StatusCode())
	}

	// Repeat request with new Custom block status code
	cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     401,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler = proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != cfg.CustomBlockStatusCode {
		t.Errorf("Incorrect response status code. Expected: %d and got %d",
			cfg.CustomBlockStatusCode, reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testPathNotExists(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

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
	req.SetRequestURI("/test/signupNotExist")
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

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != cfg.CustomBlockStatusCode {
		t.Errorf("Incorrect response status code. Expected: %d and got %d",
			cfg.CustomBlockStatusCode, reqCtx.Response.StatusCode())
	}

	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("TRACE")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != cfg.CustomBlockStatusCode {
		t.Errorf("Incorrect response status code. Expected: %d and got %d",
			cfg.CustomBlockStatusCode, reqCtx.Response.StatusCode())
	}

	// Repeat request with new Custom block status code
	cfg = config.ProxyMode{
		RequestValidation:         "LOG_ONLY",
		ResponseValidation:        "LOG_ONLY",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler = proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// Repeat request with new Custom block status code
	cfg = config.ProxyMode{
		RequestValidation:         "DISABLE",
		ResponseValidation:        "DISABLE",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler = proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testBlockMode(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

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

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

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

	var cfg = config.ProxyMode{
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

	deniedTokens, err := denylist.New(&cfg.Denylist, s.logger)
	if err != nil {
		t.Fatal(err)
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, deniedTokens, nil, nil)

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

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

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

func (s *ServiceTests) testAllowlist(t *testing.T) {

	allowedListCfg := config.AllowIP{
		HeaderName: testAllowIPHeaderName,
		File:       "../../../resources/test/allowed.iplist.db",
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		AllowIP: allowedListCfg,
	}

	allowedIPs, err := allowiplist.New(&allowedListCfg, s.logger)
	if err != nil {
		t.Fatal(err)
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, allowedIPs, nil)

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

	// no header
	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	// check not allowed IPs
	for _, ip := range listOfNotAllowedIPs {
		req.Header.Set(testAllowIPHeaderName, ip)

		reqCtx = fasthttp.RequestCtx{
			Request: *req,
		}

		handler(&reqCtx)

		if reqCtx.Response.StatusCode() != 403 {
			t.Errorf("Incorrect response status code. Expected: 403 and got %d",
				reqCtx.Response.StatusCode())
		}
	}

	// check allowed IPs
	for _, ip := range listOfAllowedIPs {
		req.Header.Set(testAllowIPHeaderName, ip)

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
	}

}

func (s *ServiceTests) testAllowlistXForwardedFor(t *testing.T) {

	allowedListCfg := config.AllowIP{
		HeaderName: testAllowIPXForwardedForName,
		File:       "../../../resources/test/allowed.iplist.db",
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		AllowIP: allowedListCfg,
	}

	allowedIPs, err := allowiplist.New(&allowedListCfg, s.logger)
	if err != nil {
		t.Fatal(err)
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, allowedIPs, nil)

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

	// check not allowed IPs
	req.Header.Set(testAllowIPXForwardedForName, fmt.Sprintf("%s, %s", listOfNotAllowedIPs[0], listOfAllowedIPs[0]))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	// check allowed IPs
	req.Header.Set(testAllowIPXForwardedForName, fmt.Sprintf("%s, %s", listOfAllowedIPs[0], listOfNotAllowedIPs[0]))

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

	// check allowed IPs
	req.Header.Set(testAllowIPXForwardedForName, fmt.Sprintf("%s", listOfAllowedIPs[0]))

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

	// check not allowed IPs
	req.Header.Set(testAllowIPXForwardedForName, fmt.Sprintf("%s", listOfNotAllowedIPs[0]))

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

	var cfg = config.ProxyMode{
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

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

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

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testLogOnlyMode(t *testing.T) {
	var cfg = config.ProxyMode{
		RequestValidation:         "LOG_ONLY",
		ResponseValidation:        "LOG_ONLY",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

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

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testDisableMode(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "DISABLE",
		ResponseValidation:        "DISABLE",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

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

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testBlockLogOnlyMode(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "LOG_ONLY",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

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

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != fasthttp.StatusServiceUnavailable {
		t.Errorf("Incorrect response status code. Expected: 503 and got %d",
			reqCtx.Response.StatusCode())
	}
}

func (s *ServiceTests) testLogOnlyBlockMode(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "LOG_ONLY",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

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

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testCommonParameters(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/users/1/1")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

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
		Backend: config.Backend{
			URL:                "",
			ClientPoolCapacity: 1000,
			InsecureConnection: false,
			RootCA:             "",
			MaxConnsPerHost:    512,
			ReadTimeout:        time.Second * 5,
			WriteTimeout:       time.Second * 5,
			DialTimeout:        time.Second * 5,
		},
		Oauth: oauthConf,
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Server: serverConf,
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

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

	// using introspection cache to get response info
	req.Header.Set("X-Fail-Request", "1")

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
		Backend: config.Backend{
			URL:                "",
			ClientPoolCapacity: 1000,
			InsecureConnection: false,
			RootCA:             "",
			MaxConnsPerHost:    512,
			ReadTimeout:        time.Second * 5,
			WriteTimeout:       time.Second * 5,
			DialTimeout:        time.Second * 5,
		},
		Oauth: oauthConf,
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Server: serverConf,
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

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
		Backend: config.Backend{
			URL:                "",
			ClientPoolCapacity: 1000,
			InsecureConnection: false,
			RootCA:             "",
			MaxConnsPerHost:    512,
			ReadTimeout:        time.Second * 5,
			WriteTimeout:       time.Second * 5,
			DialTimeout:        time.Second * 5,
		},
		Oauth: oauthConf,
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Server: serverConf,
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

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
		Backend: config.Backend{
			URL:                "",
			ClientPoolCapacity: 1000,
			InsecureConnection: false,
			RootCA:             "",
			MaxConnsPerHost:    512,
			ReadTimeout:        time.Second * 5,
			WriteTimeout:       time.Second * 5,
			DialTimeout:        time.Second * 5,
		},
		Oauth: oauthConf,
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Server: serverConf,
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

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
		Backend: config.Backend{
			URL:                "",
			ClientPoolCapacity: 1000,
			InsecureConnection: false,
			RootCA:             "",
			MaxConnsPerHost:    512,
			ReadTimeout:        time.Second * 5,
			WriteTimeout:       time.Second * 5,
			DialTimeout:        time.Second * 5,
		},
		Oauth: oauthConf,
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Server: serverConf,
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

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
		Backend: config.Backend{
			URL:                "",
			ClientPoolCapacity: 1000,
			InsecureConnection: false,
			RootCA:             "",
			MaxConnsPerHost:    512,
			ReadTimeout:        time.Second * 5,
			WriteTimeout:       time.Second * 5,
			DialTimeout:        time.Second * 5,
		},
		Oauth: oauthConf,
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Server: serverConf,
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

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

	// Send request with incorrect JWT token
	req.Header.Set("Authorization", "Bearer 123")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

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
		Backend: config.Backend{
			URL:                "",
			ClientPoolCapacity: 1000,
			InsecureConnection: false,
			RootCA:             "",
			MaxConnsPerHost:    512,
			ReadTimeout:        time.Second * 5,
			WriteTimeout:       time.Second * 5,
			DialTimeout:        time.Second * 5,
		},
		Oauth: oauthConf,
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Server: serverConf,
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

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

	// Send request with incorrect JWT token
	req.Header.Set("Authorization", "Bearer invalid")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testRequestHeaders(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

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

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

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

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testResponseHeaders(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

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

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

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

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testRequestBodyCompression(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	var p []byte
	var err error

	for _, encSchema := range testSupportedEncodingSchemas {

		p, err = json.Marshal(map[string]interface{}{
			"firstname": "test",
			"lastname":  "test",
			"job":       "test",
			"email":     "test@wallarm.com",
			"url":       "http://wallarm.com",
		})
		if err != nil {
			t.Fatal(err)
		}

		// compress request body using gzip
		reqBodyRaw, err := io.ReadAll(bytes.NewReader(p))
		if err != nil {
			t.Fatal(err)
		}

		reqBody, err := compressData(reqBodyRaw, encSchema)
		if err != nil {
			t.Fatal(err)
		}

		req.SetBody(reqBody)
		req.Header.SetContentEncoding(encSchema)
		req.Header.SetContentType("application/json")

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

		// Repeat request with wrong JSON in request

		p, err = json.Marshal(map[string]interface{}{
			"firstname": "test",
			"lastname":  "test",
			"job":       "test",
			"email":     "wrong_email_test",
			"url":       "http://wallarm.com",
		})

		// compress request body using gzip
		reqBodyRaw, err = io.ReadAll(bytes.NewReader(p))
		if err != nil {
			t.Fatal(err)
		}

		reqBody, err = compressData(reqBodyRaw, encSchema)
		if err != nil {
			t.Fatal(err)
		}

		req.SetBody(reqBody)

		reqCtx = fasthttp.RequestCtx{
			Request: *req,
		}

		handler(&reqCtx)

		if reqCtx.Response.StatusCode() != 403 {
			t.Errorf("Incorrect response status code. Expected: 403 and got %d",
				reqCtx.Response.StatusCode())
		}

	}

}

func (s *ServiceTests) testResponseBodyCompression(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

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

	for _, encSchema := range testSupportedEncodingSchemas {

		// compress response body using gzip
		body, err := compressData([]byte("{\"status\":\"success\"}"), encSchema)
		if err != nil {
			t.Fatal(err)
		}

		resp.SetBody(body)
		resp.Header.SetContentEncoding(encSchema)

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

		// Repeat request with wrong JSON in response

		// compress using gzip
		body, err = compressData([]byte("{\"status\": 123}"), encSchema)
		if err != nil {
			t.Fatal(err)
		}
		resp.SetBody(body)

		reqCtx = fasthttp.RequestCtx{
			Request: *req,
		}

		s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
		s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
		s.proxy.EXPECT().Put(resolvedIP, s.client)

		handler(&reqCtx)

		if reqCtx.Response.StatusCode() != 403 {
			t.Errorf("Incorrect response status code. Expected: 403 and got %d",
				reqCtx.Response.StatusCode())
		}
	}

}

func (s *ServiceTests) requestOptionalCookies(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/cookie_params")
	req.Header.SetMethod("GET")
	req.Header.SetCookie("cookie_mandatory", "test")
	req.Header.SetCookie("cookie_optional", "10")

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

	// Repeat request without an optional cookie
	req.Header.DelCookie("cookie_optional")

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

	// Repeat request with an optional cookie but optional cookie has invalid value
	req.Header.SetCookie("cookie_optional", "wrongValue")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	// Repeat request without an optional cookie
	req.Header.DelCookie("cookie_mandatory")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) requestOptionalMinMaxCookies(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/cookie_params_min_max")
	req.Header.SetMethod("GET")
	req.Header.SetCookie("cookie_mandatory", "test")
	req.Header.SetCookie("cookie_optional_min_max", "1001")

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

	// Repeat request without an optional cookie
	req.Header.DelCookie("cookie_optional_min_max")

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

	// Repeat request with an optional cookie but optional cookie has invalid value
	req.Header.SetCookie("cookie_optional_min_max", "999")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	// Repeat request with an optional cookie but optional cookie has invalid value
	req.Header.SetCookie("cookie_optional_min_max", "wrongValue")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	// Repeat request without an optional cookie
	req.Header.DelCookie("cookie_mandatory")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) unknownParamQuery(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList:                []int{404, 401},
			UnknownParametersDetection: true,
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/get/test")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)

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

	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/get/test?test=123")
	req.Header.SetMethod("GET")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) unknownParamPostBody(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList:                []int{404, 401},
			UnknownParametersDetection: true,
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.SetBodyString("firstname=test&lastname=test&job=test&email=test@wallarm.com&url=http://wallarm.com")
	req.Header.SetContentType("application/x-www-form-urlencoded")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	req.SetBodyString("firstname=test&lastname=test&email=test@wallarm.com&url=http://wallarm.com&test=hello")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) unknownParamJSONParam(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList:                []int{404, 401},
			UnknownParametersDetection: true,
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

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

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	p, err = json.Marshal(map[string]interface{}{
		"firstname": "test",
		"lastname":  "test",
		"job":       "test",
		"email":     "test@wallarm.com",
		"url":       "http://wallarm.com",
		"test":      "hello",
	})

	if err != nil {
		t.Fatal(err)
	}

	req.SetBodyStream(bytes.NewReader(p), -1)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) unknownParamUnsupportedMimeType(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList:                []int{404, 401},
			UnknownParametersDetection: true,
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.SetBodyString("firstname=test&lastname=test&job=test&email=test@wallarm.com&url=http://wallarm.com")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil)

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	req.Header.SetContentType("application/unsupported-type")

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

}

func (s *ServiceTests) testConflictPaths(t *testing.T) {

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}

	handler := proxy2.Handlers(s.lock, &cfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.dbSpec, nil, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/path/testValue1")
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

	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/path/value1.php")
	req.Header.SetMethod("GET")

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

}

func checkHostHeaderEndpoint(ctx *fasthttp.RequestCtx) {
	if bytes.EqualFold(ctx.Request.Header.Host(), []byte("testCustomHost")) {
		ctx.SetStatusCode(fasthttp.StatusOK)
	} else {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
	}
}

func checkCustomHeaderEndpoint(ctx *fasthttp.RequestCtx) {
	if bytes.Equal(ctx.Request.Header.Peek("x-test"), []byte("testValue")) {
		ctx.SetStatusCode(fasthttp.StatusOK)
		ctx.Response.SetBody(strconv.S2B(openAPISpecTest))
	} else {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
	}
}

func (s *ServiceTests) testCustomHostHeader(t *testing.T) {

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/get/test")
	req.Header.SetMethod("GET")
	req.Header.SetHost("wrongHost")

	port := 28290
	defer startServerOnPort(t, port, checkHostHeaderEndpoint).Close()

	serverConf := config.Server{
		Backend: config.Backend{
			URL:                "http://localhost:28290",
			RequestHostHeader:  "testCustomHost",
			ClientPoolCapacity: 1000,
			InsecureConnection: false,
			RootCA:             "",
			MaxConnsPerHost:    512,
			ReadTimeout:        time.Second * 5,
			WriteTimeout:       time.Second * 5,
			DialTimeout:        time.Second * 5,
		},
	}

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		Server: serverConf,
	}

	options := proxy.Options{
		InitialPoolCapacity: 100,
		ClientPoolCapacity:  cfg.Server.ClientPoolCapacity,
		InsecureConnection:  cfg.Server.InsecureConnection,
		RootCA:              cfg.Server.RootCA,
		MaxConnsPerHost:     cfg.Server.MaxConnsPerHost,
		ReadTimeout:         cfg.Server.ReadTimeout,
		WriteTimeout:        cfg.Server.WriteTimeout,
		DialTimeout:         cfg.Server.DialTimeout,
	}
	pool, err := proxy.NewChanPool("localhost:28290", &options)
	if err != nil {
		t.Fatal(err)
	}

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	if err := proxy.Perform(&reqCtx, pool, cfg.Server.RequestHostHeader); err != nil {
		t.Fatal(err)
	}

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testCustomHeaderOASviaURL(t *testing.T) {

	customHeader := config.CustomHeader{
		Name:  "x-test",
		Value: "testValue",
	}

	port := 28291
	defer startServerOnPort(t, port, checkCustomHeaderEndpoint).Close()

	specStorage, err := storage.NewOpenAPIFromURL("http://localhost:28291", &customHeader)
	if err != nil {
		t.Fatal(err)
	}

	if !specStorage.IsReady() {
		t.Errorf("Incorrect ready state. Expected: true and got %t",
			specStorage.IsReady())
	}

	if !bytes.Equal(specStorage.SpecificationRawContent(-1), strconv.S2B(openAPISpecTest)) {
		t.Error("Incorrect spec raw bytes")
	}

}
