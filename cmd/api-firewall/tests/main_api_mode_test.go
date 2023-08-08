package tests

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	handlersAPI "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/api"
	"github.com/wallarm/api-firewall/cmd/api-firewall/internal/updater"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/database"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

const apiModeOpenAPISpecAPIModeTest = `
openapi: 3.0.1
info:
  title: Service
  version: 1.1.0
servers:
  - url: /
paths:
  /absolute-redirect/{n}:
    get:
      tags:
        - Redirects
      summary: Absolutely 302 Redirects n times.
      parameters:
        - name: 'n'
          in: path
          required: true
          schema: {}
      responses:
        '302':
          description: A redirection.
          content: {}
  /redirect-to:
    put:
      summary: 302/3XX Redirects to the given URL.
      requestBody:
        content:
          multipart/form-data:
            schema:
              required:
                - url
              properties:
                url:
                  type: string
                status_code: {}
        required: true
      responses:
        '302':
          description: A redirection.
          content: {}
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
        required: true
        content:
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
                job:
                  type: string
                  example: test
                url:
                  type: string
                  example: http://test.com
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
                job:
                  type: string
                  example: test
                url:
                  type: string
                  example: http://test.com
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
  /test/multipart:
    post:
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              required:
                - url
              properties:
                url:
                  type: string
                id:
                  type: integer
        required: true
      responses:
        '302':
          description: "A redirection."
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
  '/test/plain':
    post:
      requestBody:
        content:
          text/plain:
            schema:
              type: string
        required: true
      responses:
        '200':
          description: Static page
          content: {}
        '403':
          description: operation forbidden
          content: {}
  '/test/unknownCT':
    post:
      requestBody:
        content:
          application/unknownCT:
            schema:
              type: string
        required: true
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

	DefaultSchemaID    = 0
	DefaultSpecVersion = "1.1.0"
	UpdatedSpecVersion = "1.1.1"
)

const apiModeOpenAPISpecAPIModeTestUpdated = `
openapi: 3.0.1
info:
  title: Service
  version: 1.1.1
servers:
  - url: /
paths:
  /test/new:
    get:
      tags:
        - Redirects
      summary: Absolutely 302 Redirects n times.
      responses:
        '200':
          description: A redirection.
          content: {}
`

var cfg = config.APIFWConfigurationAPIMode{
	APIFWMode:                  config.APIFWMode{Mode: web.APIMode},
	SpecificationUpdatePeriod:  2 * time.Second,
	UnknownParametersDetection: true,
	PassOptionsRequests:        false,
}

type APIModeServiceTests struct {
	serverUrl *url.URL
	shutdown  chan os.Signal
	logger    *logrus.Logger
	dbSpec    *database.MockDBOpenAPILoader
	lock      *sync.RWMutex
}

func TestAPIModeBasic(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dbSpec := database.NewMockDBOpenAPILoader(mockCtrl)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	serverUrl, err := url.ParseRequestURI("http://127.0.0.1:80")
	if err != nil {
		t.Fatalf("parsing API Host URL: %s", err.Error())
	}

	swagger, err := openapi3.NewLoader().LoadFromData([]byte(apiModeOpenAPISpecAPIModeTest))
	if err != nil {
		t.Fatalf("loading swagwaf file: %s", err.Error())
	}

	dbSpec.EXPECT().SchemaIDs().Return([]int{DefaultSchemaID}).AnyTimes()
	dbSpec.EXPECT().Specification(DefaultSchemaID).Return(swagger).AnyTimes()
	dbSpec.EXPECT().SpecificationVersion(DefaultSchemaID).Return(DefaultSpecVersion).AnyTimes()
	dbSpec.EXPECT().IsLoaded(DefaultSchemaID).Return(true).AnyTimes()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	apifwTests := APIModeServiceTests{
		serverUrl: serverUrl,
		shutdown:  shutdown,
		logger:    logger,
		dbSpec:    dbSpec,
		lock:      &lock,
	}

	// basic test
	t.Run("testAPIModeSuccess", apifwTests.testAPIModeSuccess)
	t.Run("testAPIModeMissedMultipleReqParams", apifwTests.testAPIModeMissedMultipleReqParams)
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
	t.Run("testAPIModeBearerTokenFailed", apifwTests.testAPIModeBearerTokenFailed)
	t.Run("testAPIModeAPITokenCookieFailed", apifwTests.testAPIModeAPITokenCookieFailed)

	t.Run("testAPIModeSuccessEmptyPathParameter", apifwTests.testAPIModeSuccessEmptyPathParameter)
	t.Run("testAPIModeSuccessMultipartStringParameter", apifwTests.testAPIModeSuccessMultipartStringParameter)

	t.Run("testAPIModeJSONParseError", apifwTests.testAPIModeJSONParseError)
	t.Run("testAPIModeInvalidCTParseError", apifwTests.testAPIModeInvalidCTParseError)
	t.Run("testAPIModeCTNotInSpec", apifwTests.testAPIModeCTNotInSpec)
	t.Run("testAPIModeEmptyBody", apifwTests.testAPIModeEmptyBody)

	t.Run("testAPIModeUnknownParameterBodyJSON", apifwTests.testAPIModeUnknownParameterBodyJSON)
	t.Run("testAPIModeUnknownParameterBodyPost", apifwTests.testAPIModeUnknownParameterBodyPost)
	t.Run("testAPIModeUnknownParameterQuery", apifwTests.testAPIModeUnknownParameterQuery)
	t.Run("testAPIModeUnknownParameterTextPlainCT", apifwTests.testAPIModeUnknownParameterTextPlainCT)
	t.Run("testAPIModeUnknownParameterInvalidCT", apifwTests.testAPIModeUnknownParameterInvalidCT)

	t.Run("testAPIModePassOptionsRequest", apifwTests.testAPIModePassOptionsRequest)

	t.Run("testAPIModeMultipartOptionalParams", apifwTests.testAPIModeMultipartOptionalParams)
}

func createForm(form map[string]string) (string, io.Reader, error) {
	body := new(bytes.Buffer)
	mp := multipart.NewWriter(body)
	defer mp.Close()
	for key, val := range form {
		if strings.HasPrefix(val, "@") {
			val = val[1:]
			file, err := os.Open(val)
			if err != nil {
				return "", nil, err
			}
			defer file.Close()
			part, err := mp.CreateFormFile(key, val)
			if err != nil {
				return "", nil, err
			}
			io.Copy(part, file)
		} else {
			mp.WriteField(key, val)
		}
	}
	return mp.FormDataContentType(), body, nil
}

func (s *APIModeServiceTests) testAPIModeSuccess(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

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
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

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

	if err != nil {
		t.Fatal(err)
	}

	req.SetBodyStream(bytes.NewReader(reqInvalidEmail), -1)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

}

func (s *APIModeServiceTests) testAPIModeMissedMultipleReqParams(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

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
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

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
		"email": "test@wallarm.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	req.SetBodyStream(bytes.NewReader(reqInvalidEmail), -1)

	missedParams := map[string]interface{}{
		"firstname": struct{}{},
		"lastname":  struct{}{},
	}

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Errors) != 2 {
		t.Errorf("wrong number of errors. Expected: 2. Got: %d", len(apifwResponse.Errors))
	}

	for _, apifwErr := range apifwResponse.Errors {

		if apifwErr.Code != handlersAPI.ErrCodeRequiredBodyParameterMissed {
			t.Errorf("Incorrect error code. Expected: %s and got %s",
				handlersAPI.ErrCodeRequiredBodyParameterMissed, apifwErr.Code)
		}

		if len(apifwErr.Fields) != 1 {
			t.Errorf("wrong number of related fields. Expected: 1. Got: %d", len(apifwErr.Fields))
		}

		if _, ok := missedParams[apifwErr.Fields[0]]; !ok {
			t.Errorf("Invalid missed field. Expected: firstname or lastname but got %s",
				apifwErr.Fields[0])
		}

	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

}

func (s *APIModeServiceTests) testAPIModeSuccessEmptyPathParameter(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI(fmt.Sprintf("/absolute-redirect/%d", rand.Uint32()))
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	req.SetRequestURI("/absolute-redirect/testString")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

}

func (s *APIModeServiceTests) testAPIModeSuccessMultipartStringParameter(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/redirect-to")
	req.Header.SetMethod("PUT")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	form := map[string]string{"url": "test"}
	ct, body, err := createForm(form)
	if err != nil {
		t.Fatal(err)
	}

	bodyData, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.SetContentType(ct)
	req.SetBody(bodyData)

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/redirect-to")
	req.Header.SetMethod("PUT")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	form = map[string]string{"wrongKey": "test"}
	ct, body, err = createForm(form)
	if err != nil {
		t.Fatal(err)
	}

	bodyData, err = io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.SetContentType(ct)
	req.SetBody(bodyData)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

}

func (s *APIModeServiceTests) testAPIModeJSONParseError(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader([]byte("{\"test\"=\"wrongSyntax\"}")), -1)
	req.Header.SetContentType("application/json")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeRequiredBodyParseError {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredBodyParseError, apifwResponse.Errors[0].Code)
	}
}

func (s *APIModeServiceTests) testAPIModeInvalidCTParseError(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

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
	req.Header.SetContentType("invalid/mimetype")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeRequiredBodyParseError {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredBodyParseError, apifwResponse.Errors[0].Code)
	}

}

func (s *APIModeServiceTests) testAPIModeCTNotInSpec(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

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
	req.SetRequestURI("/test/multipart")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeRequiredBodyParseError {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredBodyParseError, apifwResponse.Errors[0].Code)
	}

}

func (s *APIModeServiceTests) testAPIModeEmptyBody(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	//req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeRequiredBodyMissed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredBodyMissed, apifwResponse.Errors[0].Code)
	}

}

func (s *APIModeServiceTests) testAPIModeNoXWallarmSchemaIDHeader(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

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

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

func (s *APIModeServiceTests) testAPIModeMethodAndPathNotFound(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

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
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeMethodAndPathNotFound {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeMethodAndPathNotFound, apifwResponse.Errors[0].Code)
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

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

	apifwResponse = handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeMethodAndPathNotFound {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeMethodAndPathNotFound, apifwResponse.Errors[0].Code)
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

}

func (s *APIModeServiceTests) testAPIModeRequiredQueryParameterMissed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/query?id=" + uuid.New().String())
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	req.SetRequestURI("/test/query?wrong_q_parameter=test")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeRequiredQueryParameterMissed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredQueryParameterMissed, apifwResponse.Errors[0].Code)
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

func (s *APIModeServiceTests) testAPIModeRequiredHeaderParameterMissed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	xReqTestValue := uuid.New()

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/headers/request")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.Add(testRequestHeader, xReqTestValue.String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	req.Header.Del(testRequestHeader)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeRequiredHeaderMissed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredHeaderMissed, apifwResponse.Errors[0].Code)
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

func (s *APIModeServiceTests) testAPIModeRequiredCookieParameterMissed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/cookies/request")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.SetCookie(testRequestCookie, uuid.New().String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	req.Header.DelAllCookies()

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeRequiredCookieParameterMissed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredCookieParameterMissed, apifwResponse.Errors[0].Code)
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

func (s *APIModeServiceTests) testAPIModeRequiredBodyMissed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

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
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/body/request")
	req.Header.SetMethod("POST")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeRequiredBodyMissed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredBodyMissed, apifwResponse.Errors[0].Code)
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

func (s *APIModeServiceTests) testAPIModeRequiredBodyParameterMissed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

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
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

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
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeRequiredBodyParameterMissed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredBodyParameterMissed, apifwResponse.Errors[0].Code)
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

// Invalid parameters errors
func (s *APIModeServiceTests) testAPIModeRequiredQueryParameterInvalidValue(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/query?id=" + uuid.New().String())
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	req.SetRequestURI("/test/query?id=invalid_value_test")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeRequiredQueryParameterInvalidValue {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredQueryParameterInvalidValue, apifwResponse.Errors[0].Code)
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

func (s *APIModeServiceTests) testAPIModeRequiredHeaderParameterInvalidValue(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	xReqTestValue := uuid.New()

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/headers/request")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.Add(testRequestHeader, xReqTestValue.String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

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

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeRequiredHeaderInvalidValue {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredHeaderInvalidValue, apifwResponse.Errors[0].Code)
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

func (s *APIModeServiceTests) testAPIModeRequiredCookieParameterInvalidValue(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/cookies/request")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.SetCookie(testRequestCookie, uuid.New().String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	req.Header.SetCookie(testRequestCookie, "invalid_test_value")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeRequiredCookieParameterInvalidValue {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredCookieParameterInvalidValue, apifwResponse.Errors[0].Code)
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

func (s *APIModeServiceTests) testAPIModeRequiredBodyParameterInvalidValue(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

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
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

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
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeRequiredBodyParameterInvalidValue {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeRequiredBodyParameterInvalidValue, apifwResponse.Errors[0].Code)
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

// security requirements
func (s *APIModeServiceTests) testAPIModeBasicAuthFailed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/security/basic")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("user1:password1")))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	req.Header.Del("Authorization")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeSecRequirementsFailed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeSecRequirementsFailed, apifwResponse.Errors[0].Code)
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

func (s *APIModeServiceTests) testAPIModeBearerTokenFailed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/security/bearer")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.Add("Authorization", "Bearer "+uuid.New().String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	req.Header.Del("Authorization")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeSecRequirementsFailed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeSecRequirementsFailed, apifwResponse.Errors[0].Code)
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

func (s *APIModeServiceTests) testAPIModeAPITokenCookieFailed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/security/cookie")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.SetCookie(testSecCookieName, uuid.New().String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	req.Header.DelAllCookies()

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeSecRequirementsFailed {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeSecRequirementsFailed, apifwResponse.Errors[0].Code)
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

// unknown parameters
func (s *APIModeServiceTests) testAPIModeUnknownParameterBodyJSON(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	p, err := json.Marshal(map[string]interface{}{
		"firstname":    "test",
		"lastname":     "test",
		"job":          "test",
		"unknownParam": "test",
		"email":        "test@wallarm.com",
		"url":          "http://wallarm.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeUnknownParameterFound {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeUnknownParameterFound, apifwResponse.Errors[0].Code)
	}

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

	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

func (s *APIModeServiceTests) testAPIModeUnknownParameterBodyPost(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.PostArgs().Add("firstname", "test")
	req.PostArgs().Add("lastname", "test")
	req.PostArgs().Add("job", "test")
	req.PostArgs().Add("unknownParam", "test")
	req.PostArgs().Add("email", "test@example.com")
	req.PostArgs().Add("url", "test")
	req.SetBodyString(req.PostArgs().String())
	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeUnknownParameterFound {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeUnknownParameterFound, apifwResponse.Errors[0].Code)
	}

	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("POST")
	req.PostArgs().Add("firstname", "test")
	req.PostArgs().Add("lastname", "test")
	req.PostArgs().Add("job", "test")
	req.PostArgs().Add("email", "test@example.com")
	req.PostArgs().Add("url", "test")
	req.SetBodyString(req.PostArgs().String())
	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

func (s *APIModeServiceTests) testAPIModeUnknownParameterQuery(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/query?uparam=test&id=" + uuid.New().String())
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := handlersAPI.Response{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if apifwResponse.Errors[0].Code != handlersAPI.ErrCodeUnknownParameterFound {
		t.Errorf("Incorrect error code. Expected: %s and got %s",
			handlersAPI.ErrCodeUnknownParameterFound, apifwResponse.Errors[0].Code)
	}

	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/query?id=" + uuid.New().String())
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))
}

func (s *APIModeServiceTests) testAPIModeUnknownParameterTextPlainCT(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/plain")
	req.Header.SetMethod("POST")
	req.SetBodyString("testString")
	req.Header.SetContentType("text/plain")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}
}

func (s *APIModeServiceTests) testAPIModeUnknownParameterInvalidCT(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/unknownCT")
	req.Header.SetMethod("POST")
	req.SetBodyString("testString")
	req.Header.SetContentType("application/unknownCT")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 500 {
		t.Errorf("Incorrect response status code. Expected: 500 and got %d",
			reqCtx.Response.StatusCode())
	}
}

func (s *APIModeServiceTests) testAPIModePassOptionsRequest(t *testing.T) {

	cfg.PassOptionsRequests = true
	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("OPTIONS")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

}

func (s *APIModeServiceTests) testAPIModeMultipartOptionalParams(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, s.dbSpec)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/multipart")
	req.Header.SetMethod("POST")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	form := map[string]string{"url": "test", "id": "10"}
	ct, body, err := createForm(form)
	if err != nil {
		t.Fatal(err)
	}

	bodyData, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.SetContentType(ct)
	req.SetBody(bodyData)

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/multipart")
	req.Header.SetMethod("POST")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	form = map[string]string{"url": "test", "id": "test"}
	ct, body, err = createForm(form)
	if err != nil {
		t.Fatal(err)
	}

	bodyData, err = io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.SetContentType(ct)
	req.SetBody(bodyData)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

}

func TestAPIModeMockedUpdater(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dbSpecBeforeUpdate := database.NewMockDBOpenAPILoader(mockCtrl)

	dbSpec := database.NewMockDBOpenAPILoader(mockCtrl)

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	schemaIDsBefore := dbSpec.EXPECT().SchemaIDs().Return([]int{DefaultSchemaID})
	specVersionBefore := dbSpec.EXPECT().SpecificationVersion(DefaultSchemaID).Return(DefaultSpecVersion)
	loadUpdater := dbSpec.EXPECT().Load(gomock.Any()).Return(nil)
	schemaIDsAfter := dbSpec.EXPECT().SchemaIDs().Return([]int{DefaultSchemaID})
	specVersionAfter := dbSpec.EXPECT().SpecificationVersion(DefaultSchemaID).Return(UpdatedSpecVersion)

	// updater calls
	gomock.InOrder(schemaIDsBefore, specVersionBefore, loadUpdater, schemaIDsAfter, specVersionAfter)

	swagger, err := openapi3.NewLoader().LoadFromData([]byte(apiModeOpenAPISpecAPIModeTestUpdated))
	if err != nil {
		t.Fatalf("loading swagwaf file: %s", err.Error())
	}
	specRoutes := dbSpec.EXPECT().Specification(DefaultSchemaID).Return(swagger)

	schemaIDsRoutes := dbSpec.EXPECT().SchemaIDs().Return([]int{DefaultSchemaID})
	schemaIDsApps := dbSpec.EXPECT().SchemaIDs().Return([]int{DefaultSchemaID})
	specRouter := dbSpec.EXPECT().Specification(DefaultSchemaID).Return(swagger)
	specVersionRouter := dbSpec.EXPECT().SpecificationVersion(DefaultSchemaID).Return(UpdatedSpecVersion)
	specVersionLogMsg := dbSpec.EXPECT().SpecificationVersion(DefaultSchemaID).Return(UpdatedSpecVersion)

	// router calls
	gomock.InOrder(schemaIDsRoutes, schemaIDsApps, specRoutes, specRouter, specVersionRouter, specVersionLogMsg)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	health := handlersAPI.Health{}

	var lock sync.RWMutex

	dbSpecBeforeUpdate.EXPECT().Specification(DefaultSchemaID).Return(swagger).AnyTimes()
	dbSpecBeforeUpdate.EXPECT().SchemaIDs().Return([]int{DefaultSchemaID}).AnyTimes()
	dbSpecBeforeUpdate.EXPECT().SpecificationVersion(DefaultSchemaID).Return(DefaultSpecVersion).AnyTimes()
	dbSpecBeforeUpdate.EXPECT().IsLoaded(DefaultSchemaID).Return(true).AnyTimes()

	handler := handlersAPI.Handlers(&lock, &cfg, shutdown, logger, dbSpecBeforeUpdate)
	api := fasthttp.Server{Handler: handler}

	updSpecErrors := make(chan error, 1)
	updater := updater.NewController(&lock, logger, dbSpec, &cfg, &api, shutdown, &health)
	go func() {
		t.Logf("starting specification regular update process every %.0f seconds", cfg.SpecificationUpdatePeriod.Seconds())
		updSpecErrors <- updater.Start()
	}()

	time.Sleep(3 * time.Second)

	if err := updater.Shutdown(); err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/new")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	// checker in the request handler
	dbSpec.EXPECT().IsLoaded(DefaultSchemaID).Return(true).AnyTimes()

	lock.RLock()
	api.Handler(&reqCtx)
	lock.RUnlock()

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

}
