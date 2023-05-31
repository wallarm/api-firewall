package tests

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	handlersAPI "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/api"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/mid"
	"github.com/wallarm/api-firewall/internal/platform/database"
	"github.com/wallarm/api-firewall/internal/platform/router"
)

const (
	DefaultSchemaID    = 0
	DefaultSpecVersion = "1.1.0"
)

const apiModeOpenAPISpecAPIModeTest = `
openapi: 3.0.1
info:
  title: Service
  version: 1.1.0
servers:
  - url: /
paths:
  /test/security/basic:
    get:
      responses:
        '200':
          description: Static page
          content: {}
        '403':
          description: operation forbidden
          content: {}
      security:
        - basicAuth: []
  /test/security/bearer:
    get:
      responses:
        '200':
          description: Static page
          content: {}
        '403':
          description: operation forbidden
          content: {}
      security:
        - bearerAuth: []
  /test/security/cookie:
    get:
      responses:
        '200':
          description: Static page
          content: {}
        '403':
          description: operation forbidden
          content: {}
      security:
        - cookieAuth: []
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
  '/test/query':
    get:
      parameters:
        - name: id
          in: query
          required: true
          schema:
            type: string
            format: uuid
            pattern: '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
      responses:
        '200':
          description: Static page
          content: {}
        '403':
          description: operation forbidden
          content: {}
  /test/headers/request:
    get:
      summary: Get Request to test Request Headers validation
      parameters:
        - in: header
          name: X-Request-Test
          schema:
            type: string
            format: uuid
            pattern: '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
          required: true
      responses:
        200:
          description: Ok
          content: { }
  /test/cookies/request:
    get:
      summary: Get Request to test Request Cookies presence
      parameters:
        - in: cookie
          name: cookie_test
          schema:
            type: string
            format: uuid
            pattern: '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
          required: true
      responses:
        200:
          description: Ok
          content: { }
  /test/body/request:
    post:
      summary: Post Request to test Request Body presence
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - status
              properties:
                status:
                  type: string
                  format: uuid
                  pattern: '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
                error:
                  type: string
      responses:
        200:
          description: Ok
          content: { }
components:
  securitySchemes:
    basicAuth:
      type: http
      scheme: basic
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT   
    cookieAuth:
      type: apiKey
      in: cookie
      name: MyAuthHeader
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
	testDeleteMethod = "DELETE"
	testUnknownPath  = "/unknown/path/test"

	testRequestCookie = "cookie_test"
	testSecCookieName = "MyAuthHeader"
)

type APIModeServiceTests struct {
	serverUrl  *url.URL
	shutdown   chan os.Signal
	logger     *logrus.Logger
	swagRouter *router.Router
}

func TestAPIModeBasic(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dbSpec := database.NewMockDBOpenAPILoader(mockCtrl)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	serverUrl, err := url.ParseRequestURI("http://127.0.0.1:80")
	if err != nil {
		t.Fatalf("parsing API Host URL: %s", err.Error())
	}

	swagger, err := openapi3.NewLoader().LoadFromData([]byte(apiModeOpenAPISpecAPIModeTest))
	if err != nil {
		t.Fatalf("loading swagwaf file: %s", err.Error())
	}

	dbSpec.EXPECT().Specification().Return(swagger)
	dbSpec.EXPECT().SpecificationVersion().Return(DefaultSpecVersion)
	dbSpec.EXPECT().SchemaID().Return(DefaultSchemaID)

	swagRouter, err := router.NewRouterDBLoader(dbSpec)
	if err != nil {
		t.Fatalf("parsing swagwaf file: %s", err.Error())
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	apifwTests := APIModeServiceTests{
		serverUrl:  serverUrl,
		shutdown:   shutdown,
		logger:     logger,
		swagRouter: swagRouter,
	}

	// basic test
	t.Run("testAPIModeSuccess", apifwTests.testAPIModeSuccess)
	t.Run("testAPIModeNoXWallarmSchemaIDHeader", apifwTests.testAPIModeNoXWallarmSchemaIDHeader)

	t.Run("testAPIModeMethodAndPathNotFound", apifwTests.testAPIModeMethodAndPathNotFound)

	t.Run("testAPIModeRequiredQueryParameterMissed", apifwTests.testAPIModeRequiredQueryParameterMissed)
	t.Run("testAPIModeRequiredHeaderParameterMissed", apifwTests.testAPIModeRequiredHeaderParameterMissed)
	t.Run("testAPIModeRequiredCookieParameterMissed", apifwTests.testAPIModeRequiredCookieParameterMissed)
	t.Run("testAPIModeRequiredBodyMissed", apifwTests.testAPIModeRequiredBodyMissed)
	t.Run("testAPIModeRequiredBodyParameterMissed", apifwTests.testAPIModeRequiredBodyParameterMissed)

	t.Run("testAPIModeRequiredQueryParameterInvalidValue", apifwTests.testAPIModeRequiredQueryParameterInvalidValue)
	t.Run("testAPIModeRequiredHeaderParameterInvalidValue", apifwTests.testAPIModeRequiredHeaderParameterInvalidValue)
	t.Run("testAPIModeRequiredCookieParameterInvalidValue", apifwTests.testAPIModeRequiredCookieParameterInvalidValue)
	t.Run("testAPIModeRequiredBodyParameterInvalidValue", apifwTests.testAPIModeRequiredBodyParameterInvalidValue)

	t.Run("testAPIModeBasicAuthFailed", apifwTests.testAPIModeBasicAuthFailed)
	t.Run("testAPIModeBasicAuthFailed", apifwTests.testAPIModeBearerTokenFailed)
	//t.Run("testAPIModeAPITokenHeaderFailed", apifwTests.testAPIModeAPITokenHeaderFailed)
	//t.Run("testAPIModeAPITokenQueryFailed", apifwTests.testAPIModeAPITokenQueryFailed)
	t.Run("testAPIModeAPITokenCookieFailed", apifwTests.testAPIModeAPITokenCookieFailed)

}

func (s *APIModeServiceTests) testAPIModeSuccess(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		APIMode: true,
	}

	handler := handlersAPI.APIModeHandlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.swagRouter)

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
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

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

func (s *APIModeServiceTests) testAPIModeNoXWallarmSchemaIDHeader(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		APIMode: true,
	}

	handler := handlersAPI.APIModeHandlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.swagRouter)

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

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 500 {
		t.Errorf("Incorrect response status code. Expected: 500 and got %d",
			reqCtx.Response.StatusCode())
	}
}

func (s *APIModeServiceTests) testAPIModeMethodAndPathNotFound(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		APIMode: true,
	}

	handler := handlersAPI.APIModeHandlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.swagRouter)

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
	req.Header.SetMethod(testDeleteMethod)
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.ResponseWithError{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Code != handlersAPI.ErrCodeMethodAndPathNotFound {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeMethodAndPathNotFound, apifwResponse.Code)
	}

	// check path
	req.Header.SetMethod("POST")
	req.Header.SetRequestURI(testUnknownPath)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse = handlersAPI.ResponseWithError{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Code != handlersAPI.ErrCodeMethodAndPathNotFound {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeMethodAndPathNotFound, apifwResponse.Code)
	}

}

func (s *APIModeServiceTests) testAPIModeRequiredQueryParameterMissed(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		APIMode: true,
	}

	handler := handlersAPI.APIModeHandlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.swagRouter)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/query?id=" + uuid.New().String())
	req.Header.SetMethod("GET")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	req.SetRequestURI("/test/query?wrong_q_parameter=test")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.ResponseWithError{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Code != handlersAPI.ErrCodeRequiredQueryParameterMissed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredQueryParameterMissed, apifwResponse.Code)
	}
}

func (s *APIModeServiceTests) testAPIModeRequiredHeaderParameterMissed(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		APIMode: true,
	}

	handler := handlersAPI.APIModeHandlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.swagRouter)

	xReqTestValue := uuid.New()

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/headers/request")
	req.Header.SetMethod("GET")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.Add(testRequestHeader, xReqTestValue.String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	req.Header.Del(testRequestHeader)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.ResponseWithError{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Code != handlersAPI.ErrCodeRequiredHeaderMissed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredHeaderMissed, apifwResponse.Code)
	}
}

func (s *APIModeServiceTests) testAPIModeRequiredCookieParameterMissed(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		APIMode: true,
	}

	handler := handlersAPI.APIModeHandlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.swagRouter)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/cookies/request")
	req.Header.SetMethod("GET")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.SetCookie(testRequestCookie, uuid.New().String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	req.Header.DelAllCookies()

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.ResponseWithError{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Code != handlersAPI.ErrCodeRequiredCookieParameterMissed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredCookieParameterMissed, apifwResponse.Code)
	}
}

func (s *APIModeServiceTests) testAPIModeRequiredBodyMissed(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		APIMode: true,
	}

	handler := handlersAPI.APIModeHandlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.swagRouter)

	p, err := json.Marshal(map[string]interface{}{
		"status": uuid.New().String(),
		"error":  "test",
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/body/request")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/body/request")
	req.Header.SetMethod("POST")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.ResponseWithError{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Code != handlersAPI.ErrCodeRequiredBodyMissed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredBodyMissed, apifwResponse.Code)
	}
}

func (s *APIModeServiceTests) testAPIModeRequiredBodyParameterMissed(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		APIMode: true,
	}

	handler := handlersAPI.APIModeHandlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.swagRouter)

	p, err := json.Marshal(map[string]interface{}{
		"status": uuid.New().String(),
		"error":  "test",
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/body/request")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// body without required parameter
	p, err = json.Marshal(map[string]interface{}{
		"error": "test",
	})

	if err != nil {
		t.Fatal(err)
	}

	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/body/request")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.ResponseWithError{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Code != handlersAPI.ErrCodeRequiredBodyParameterMissed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredBodyParameterMissed, apifwResponse.Code)
	}
}

// Invalid parameters errors
func (s *APIModeServiceTests) testAPIModeRequiredQueryParameterInvalidValue(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		APIMode: true,
	}

	handler := handlersAPI.APIModeHandlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.swagRouter)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/query?id=" + uuid.New().String())
	req.Header.SetMethod("GET")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	req.SetRequestURI("/test/query?id=invalid_value_test")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.ResponseWithError{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Code != handlersAPI.ErrCodeRequiredQueryParameterInvalidValue {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredQueryParameterInvalidValue, apifwResponse.Code)
	}
}

func (s *APIModeServiceTests) testAPIModeRequiredHeaderParameterInvalidValue(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		APIMode: true,
	}

	handler := handlersAPI.APIModeHandlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.swagRouter)

	xReqTestValue := uuid.New()

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/headers/request")
	req.Header.SetMethod("GET")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.Add(testRequestHeader, xReqTestValue.String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	req.Header.Del(testRequestHeader)
	req.Header.Add(testRequestHeader, "invalid_value")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.ResponseWithError{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Code != handlersAPI.ErrCodeRequiredHeaderInvalidValue {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredHeaderInvalidValue, apifwResponse.Code)
	}
}

func (s *APIModeServiceTests) testAPIModeRequiredCookieParameterInvalidValue(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		APIMode: true,
	}

	handler := handlersAPI.APIModeHandlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.swagRouter)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/cookies/request")
	req.Header.SetMethod("GET")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.SetCookie(testRequestCookie, uuid.New().String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	req.Header.SetCookie(testRequestCookie, "invalid_test_value")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.ResponseWithError{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Code != handlersAPI.ErrCodeRequiredCookieParameterInvalidValue {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredCookieParameterInvalidValue, apifwResponse.Code)
	}
}

func (s *APIModeServiceTests) testAPIModeRequiredBodyParameterInvalidValue(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		APIMode: true,
	}

	handler := handlersAPI.APIModeHandlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.swagRouter)

	p, err := json.Marshal(map[string]interface{}{
		"status": uuid.New().String(),
		"error":  "test",
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/body/request")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	// body without required parameter
	p, err = json.Marshal(map[string]interface{}{
		"status": "invalid_test_value",
		"error":  "test",
	})

	if err != nil {
		t.Fatal(err)
	}

	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/body/request")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.ResponseWithError{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Code != handlersAPI.ErrCodeRequiredBodyParameterInvalidValue {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredBodyParameterInvalidValue, apifwResponse.Code)
	}
}

// security requirements
func (s *APIModeServiceTests) testAPIModeBasicAuthFailed(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		APIMode: true,
	}

	handler := handlersAPI.APIModeHandlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.swagRouter)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/security/basic")
	req.Header.SetMethod("GET")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("user1:password1")))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	req.Header.Del("Authorization")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.ResponseWithError{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Code != handlersAPI.ErrCodeSecRequirementsFailed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeSecRequirementsFailed, apifwResponse.Code)
	}
}

func (s *APIModeServiceTests) testAPIModeBearerTokenFailed(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		APIMode: true,
	}

	handler := handlersAPI.APIModeHandlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.swagRouter)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/security/bearer")
	req.Header.SetMethod("GET")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.Add("Authorization", "Bearer "+uuid.New().String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	req.Header.Del("Authorization")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.ResponseWithError{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Code != handlersAPI.ErrCodeSecRequirementsFailed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeSecRequirementsFailed, apifwResponse.Code)
	}
}

func (s *APIModeServiceTests) testAPIModeAPITokenCookieFailed(t *testing.T) {

	var cfg = config.APIFWConfiguration{
		APIMode: true,
	}

	handler := handlersAPI.APIModeHandlers(&cfg, s.serverUrl, s.shutdown, s.logger, s.swagRouter)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/security/cookie")
	req.Header.SetMethod("GET")
	req.Header.Add(mid.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.SetCookie(testSecCookieName, uuid.New().String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	req.Header.DelAllCookies()

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.ResponseWithError{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Code != handlersAPI.ErrCodeSecRequirementsFailed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeSecRequirementsFailed, apifwResponse.Code)
	}
}
