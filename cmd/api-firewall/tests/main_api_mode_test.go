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
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"

	handlersAPI "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/api"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/metrics"
	"github.com/wallarm/api-firewall/internal/platform/storage"
	"github.com/wallarm/api-firewall/internal/platform/web"
	"github.com/wallarm/api-firewall/pkg/APIMode/validator"
)

const secondApiModeOpenAPISpecAPIModeTest = `
openapi: 3.0.1
info:
  title: Service
  version: 1.1.0
servers:
  - url: /
paths:
  /test/signup:
    post:
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - email
              properties:
                email:
                  type: string
                  format: email
                  pattern: '^[0-9a-zA-Z]+@[0-9a-zA-Z\.]+$'
                  example: example@mail.com
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
`

const apiModeOpenAPISpecAPIModeTest = `
openapi: 3.0.1
info:
  title: Service
  version: 1.1.0
servers:
  - url: /
paths:
  /test/methods:
    patch:
      responses:
        '200':
          description: Successful Response
          content: {}
    get:
      responses:
        '200':
          description: Successful Response
          content: {}
    delete:
      responses:
        '200':
          description: Successful Response
          content: {}
    put:
      responses:
        '200':
          description: Successful Response
          content: {}
    post:
      responses:
        '200':
          description: Successful Response
          content: {}
    options:
      responses:
        '200':
          description: Successful Response
          content: {}
    head:
      responses:
        '200':
          description: Successful Response
          content: {}
    trace:
      responses:
        '200':
          description: Successful Response
          content: {}
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
              type: object 
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
        - name: int
          in: query
          schema:
            type: integer
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
          name: X-Request-Test-Int
          schema:
            type: integer
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
          name: cookie_test_int
          schema:
            type: integer
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
  /query/paramsObject:
    get:
      parameters:
        - name: f.0
          required: true
          in: query
          style: deepObject
          schema:
            type: object
            properties:
              f:
                type: object
                properties:
                  '0':
                    type: string
      summary: Get Test Info
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
                - testInt
              properties:
                status:
                  type: string
                  format: uuid
                  pattern: '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
                testInt:
                  type: integer
                  minimum: 10
                  maximum: 100
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

	testRequestCookie    = "cookie_test"
	testRequestIntCookie = "cookie_test_int"
	testSecCookieName    = "MyAuthHeader"

	testRequestIntHeader = "X-Request-Test-Int"

	MissedSchemaID      = 11
	DefaultSchemaID     = 0
	DefaultCopySchemaID = 1
	SecondSchemaID      = 2
	DefaultSpecVersion  = "1.1.0"
)

var cfg = config.APIMode{
	APIFWInit:                  config.APIFWInit{Mode: web.APIMode},
	SpecificationUpdatePeriod:  2 * time.Second,
	UnknownParametersDetection: true,
	PassOptionsRequests:        false,
}

type APIModeServiceTests struct {
	serverUrl *url.URL
	shutdown  chan os.Signal
	logger    zerolog.Logger
	dbSpec    *storage.MockDBOpenAPILoader
	lock      *sync.RWMutex
}

func TestAPIModeBasic(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dbSpec := storage.NewMockDBOpenAPILoader(mockCtrl)

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	logger = logger.Level(zerolog.ErrorLevel)

	var lock sync.RWMutex

	serverUrl, err := url.ParseRequestURI("http://127.0.0.1:80")
	if err != nil {
		t.Fatalf("parsing API Host URL: %s", err.Error())
	}

	swagger, err := openapi3.NewLoader().LoadFromData([]byte(apiModeOpenAPISpecAPIModeTest))
	if err != nil {
		t.Fatalf("loading OpenAPI specification file: %s", err.Error())
	}

	secondSwagger, err := openapi3.NewLoader().LoadFromData([]byte(secondApiModeOpenAPISpecAPIModeTest))
	if err != nil {
		t.Fatalf("loading OpenAPI specification file: %s", err.Error())
	}

	dbSpec.EXPECT().SchemaIDs().Return([]int{DefaultSchemaID, DefaultCopySchemaID, SecondSchemaID}).AnyTimes()
	dbSpec.EXPECT().Specification(DefaultSchemaID).Return(swagger).AnyTimes()
	dbSpec.EXPECT().Specification(DefaultCopySchemaID).Return(swagger).AnyTimes()
	dbSpec.EXPECT().Specification(SecondSchemaID).Return(secondSwagger).AnyTimes()
	dbSpec.EXPECT().SpecificationVersion(DefaultSchemaID).Return(DefaultSpecVersion).AnyTimes()
	dbSpec.EXPECT().SpecificationVersion(DefaultCopySchemaID).Return(DefaultSpecVersion).AnyTimes()
	dbSpec.EXPECT().SpecificationVersion(SecondSchemaID).Return(DefaultSpecVersion).AnyTimes()
	dbSpec.EXPECT().IsLoaded(DefaultSchemaID).Return(true).AnyTimes()
	dbSpec.EXPECT().IsLoaded(DefaultCopySchemaID).Return(true).AnyTimes()
	dbSpec.EXPECT().IsLoaded(SecondSchemaID).Return(true).AnyTimes()
	dbSpec.EXPECT().IsLoaded(MissedSchemaID).Return(false).AnyTimes()
	dbSpec.EXPECT().IsReady().Return(true).AnyTimes()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	apifwTests := APIModeServiceTests{
		serverUrl: serverUrl,
		shutdown:  shutdown,
		logger:    logger,
		dbSpec:    dbSpec,
		lock:      &lock,
	}

	// basic run test
	t.Run("basicRunAPIService", apifwTests.testAPIRunBasic)

	// basic test
	t.Run("testAPIModeSuccess", apifwTests.testAPIModeSuccess)
	t.Run("testAPIModeMissedMultipleReqParams", apifwTests.testAPIModeMissedMultipleReqParams)
	t.Run("testAPIModeNoXWallarmSchemaIDHeader", apifwTests.testAPIModeNoXWallarmSchemaIDHeader)

	t.Run("testAPIModeOneSchemeMultipleIDs", apifwTests.testAPIModeOneSchemeMultipleIDs)
	t.Run("testAPIModeTwoSchemesMultipleIDs", apifwTests.testAPIModeTwoSchemesMultipleIDs)
	t.Run("testAPIModeTwoDifferentSchemesMultipleIDs", apifwTests.testAPIModeTwoDifferentSchemesMultipleIDs)

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

	// invalid route
	t.Run("testAPIModeInvalidRouteInRequest", apifwTests.testAPIModeInvalidRouteInRequest)
	t.Run("testAPIModeInvalidRouteInRequestInMultipleSchemas", apifwTests.testAPIModeInvalidRouteInRequestInMultipleSchemas)

	// check all supported methods: GET POST PUT PATCH DELETE TRACE OPTIONS HEAD
	t.Run("testAPIModeAllMethods", apifwTests.testAPIModeAllMethods)

	// check conflicts in the Path
	t.Run("testConflictsInThePath", apifwTests.testConflictsInThePath)
	t.Run("testObjectInQuery", apifwTests.testObjectInQuery)

	// check limited response (maxErrorsInResponse param)
	t.Run("testAPIModeMissedMultipleReqParamsLimitedResponse", apifwTests.testAPIModeMissedMultipleReqParamsLimitedResponse)
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

func checkResponseOkStatusCode(t *testing.T, reqCtx *fasthttp.RequestCtx, schemaID int) {

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) != 1 {
		t.Errorf("Incorrect amount of responses. Expected: 1 and got %d",
			len(apifwResponse.Summary))
		return
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != schemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				schemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusOK {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusOK, *apifwResponse.Summary[0].StatusCode)
		}
	}

}

func checkResponseForbiddenStatusCode(t *testing.T, reqCtx *fasthttp.RequestCtx, schemaID int, expectedErrCodes []string) {

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) != 1 {
		t.Errorf("Incorrect amount of responses. Expected: 1 and got %d",
			len(apifwResponse.Summary))
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].SchemaID != schemaID {
			t.Errorf("Incorrect error code. Expected: %d and got %d",
				schemaID, *apifwResponse.Summary[0].SchemaID)
		}
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}
	}

	// check amount of entries in the related_fields
	for _, err := range apifwResponse.Errors {
		if len(err.Fields) > 1 {
			t.Error("The amount of related fields is more than 1")
		}
	}

	if expectedErrCodes != nil {
		if len(apifwResponse.Errors) > 0 {
			for _, e := range apifwResponse.Errors {
				// The list of the codes that doesn't contain details field
				if len(e.FieldsDetails) == 0 && (e.Code != "required_body_missed" && e.Code != "required_body_parse_error" && e.Code != "unknown_parameter_found" && e.Code != "method_and_path_not_found") {
					t.Error("The field details were not found in the error")
				}
				if !slices.Contains(expectedErrCodes, e.Code) {
					t.Errorf("The error code not found in the list of expected error codes. Expected: %v and got %s",
						expectedErrCodes, e.Code)
				}
			}
		}
	}

}

func (s *APIModeServiceTests) testAPIRunBasic(t *testing.T) {

	t.Setenv("APIFW_MODE", "api")
	t.Setenv("APIFW_API_MODE_UNKNOWN_PARAMETERS_DETECTION", "false")

	t.Setenv("APIFW_URL", "http://0.0.0.0:25869")
	t.Setenv("APIFW_HEALTH_HOST", "127.0.0.1:10669")
	t.Setenv("APIFW_API_MODE_DEBUG_PATH_DB", "../../../resources/test/database/wallarm_api.db")

	// start GQL handler
	go func() {
		if err := handlersAPI.Run(s.logger); err != nil {
			t.Fatal(err)
		}
	}()

	// wait for 3 secs to init the handler
	time.Sleep(3 * time.Second)
}

func (s *APIModeServiceTests) testAPIModeSuccess(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	p, err := json.Marshal(map[string]any{
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	// Repeat request with invalid email
	reqInvalidEmail, err := json.Marshal(map[string]any{
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

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredBodyParameterInvalidValue})
}

func (s *APIModeServiceTests) testAPIModeMissedMultipleReqParams(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	p, err := json.Marshal(map[string]any{
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	// Repeat request with invalid email
	reqInvalidEmail, err := json.Marshal(map[string]any{
		"email": "test@wallarm.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	req.SetBodyStream(bytes.NewReader(reqInvalidEmail), -1)

	missedParams := map[string]any{
		"firstname": struct{}{},
		"lastname":  struct{}{},
	}

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Errors) != 2 {
		t.Errorf("wrong number of errors. Expected: 2. Got: %d", len(apifwResponse.Errors))
	}

	for _, apifwErr := range apifwResponse.Errors {

		if apifwErr.Code != validator.ErrCodeRequiredBodyParameterMissed {
			t.Errorf("Incorrect error code. Expected: %s and got %s",
				validator.ErrCodeRequiredBodyParameterMissed, apifwErr.Code)
		}

		if len(apifwErr.Fields) != 1 {
			t.Errorf("wrong number of related fields. Expected: 1. Got: %d", len(apifwErr.Fields))
		}

		if _, ok := missedParams[apifwErr.Fields[0]]; !ok {
			t.Errorf("Invalid missed field. Expected: firstname or lastname but got %s",
				apifwErr.Fields[0])
		}

	}

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

}

func (s *APIModeServiceTests) testAPIModeSuccessEmptyPathParameter(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI(fmt.Sprintf("/absolute-redirect/%d", rand.Uint32()))
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	req.SetRequestURI("/absolute-redirect/testString")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

}

func (s *APIModeServiceTests) testAPIModeSuccessMultipartStringParameter(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredBodyParseError})

}

func (s *APIModeServiceTests) testAPIModeOneSchemeMultipleIDs(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	// one schema
	p, err := json.Marshal(map[string]any{
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

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	req.Header.Set(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d, %d", DefaultSchemaID, MissedSchemaID))
	req.SetBodyStream(bytes.NewReader(p), -1)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Errors) != 0 {
		t.Errorf("Wrong number of errors. Expected: 0. Got: %d", len(apifwResponse.Errors))
	}

	if len(apifwResponse.Summary) != 2 {
		t.Errorf("Wrong number of results in summary. Expected: 2 and got: %d", len(apifwResponse.Summary))
	}

	if len(apifwResponse.Summary) == 2 {
		for _, s := range apifwResponse.Summary {
			switch *s.SchemaID {
			case DefaultSchemaID:
				if *s.StatusCode != fasthttp.StatusOK {
					t.Errorf("Incorrect result status code for schema ID %d. Expected: 200 and got %d",
						DefaultSchemaID, s.StatusCode)
				}
			case MissedSchemaID:
				if *s.StatusCode != fasthttp.StatusInternalServerError {
					t.Errorf("Incorrect result status code for schema ID %d. Expected: 200 and got %d",
						MissedSchemaID, s.StatusCode)
				}
			}
		}

	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

}

func (s *APIModeServiceTests) testAPIModeTwoDifferentSchemesMultipleIDs(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	// one schema
	p, err := json.Marshal(map[string]any{
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

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	req.Header.Set(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", SecondSchemaID))

	p, err = json.Marshal(map[string]any{
		"email": "test@wallarm.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	req.SetBodyStream(bytes.NewReader(p), -1)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, SecondSchemaID)

	req.Header.Set(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d, %d, %d", DefaultSchemaID, DefaultCopySchemaID, SecondSchemaID))

	req.SetBodyStream(bytes.NewReader(p), -1)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

	response := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &response); err != nil {
		t.Errorf("error while unmarshal response: %v", err)
	}

	if len(response.Summary) != 3 {
		t.Errorf("Incorrect number of results. Expected: 3 and got %d",
			len(response.Summary))
		return
	}

	for _, s := range response.Summary {
		if *s.SchemaID != DefaultSchemaID && *s.SchemaID != DefaultCopySchemaID && *s.SchemaID != SecondSchemaID {
			t.Errorf("Incorrect schema ID in the results. Expected one of: %d, %d, %d and got %d",
				DefaultSchemaID, DefaultCopySchemaID, SecondSchemaID, len(response.Summary))
		}
	}

	if len(response.Errors) != 4 {
		t.Errorf("Incorrect number of errors. Expected: 4 and got %d",
			len(response.Errors))
		return
	}

	if !((*response.Errors[0].SchemaID == DefaultSchemaID && *response.Errors[1].SchemaID == DefaultSchemaID &&
		*response.Errors[2].SchemaID == DefaultCopySchemaID && *response.Errors[3].SchemaID == DefaultCopySchemaID) ||
		(*response.Errors[3].SchemaID == DefaultSchemaID && *response.Errors[2].SchemaID == DefaultSchemaID &&
			*response.Errors[1].SchemaID == DefaultCopySchemaID && *response.Errors[0].SchemaID == DefaultCopySchemaID)) {
		t.Errorf("Incorrect errors. Expected the following list of schema IDs: %d, %d and got the following errors: %v",
			DefaultSchemaID, DefaultCopySchemaID, response.Errors)
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

}

func (s *APIModeServiceTests) testAPIModeTwoSchemesMultipleIDs(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	p, err := json.Marshal(map[string]any{
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
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d, %d", DefaultSchemaID, DefaultCopySchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	response := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &response); err != nil {
		t.Errorf("error while unmarshal response: %v", err)
	}

	if len(response.Errors) != 0 {
		t.Errorf("Incorrect number of errors. Expected: 0 and got %d",
			len(response.Errors))
		return
	}

	for _, s := range response.Summary {
		if *s.SchemaID != DefaultSchemaID && *s.SchemaID != DefaultCopySchemaID {
			t.Errorf("Incorrect schema ID in the results. Expected one of: %d, %d and got %d",
				DefaultSchemaID, DefaultCopySchemaID, len(response.Summary))
		}
	}

	// Repeat request with invalid email
	reqInvalidEmail, err := json.Marshal(map[string]any{
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

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	response = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &response); err != nil {
		t.Errorf("error while unmarshal response: %v", err)
	}

	if len(response.Errors) != 2 {
		t.Errorf("Incorrect number of errors. Expected: 2 and got %d",
			len(response.Errors))
		return
	}

	for _, s := range response.Summary {
		if *s.SchemaID != DefaultSchemaID && *s.SchemaID != DefaultCopySchemaID {
			t.Errorf("Incorrect schema ID in the results. Expected one of: %d, %d and got %d",
				DefaultSchemaID, DefaultCopySchemaID, len(response.Summary))
		}
	}

	if !(*response.Errors[0].SchemaID == DefaultSchemaID && *response.Errors[1].SchemaID == DefaultCopySchemaID ||
		*response.Errors[1].SchemaID == DefaultSchemaID && *response.Errors[0].SchemaID == DefaultCopySchemaID) {
		t.Errorf("Incorrect errors. Expected the following list of schema IDs: %d, %d and got the following errors: %v",
			DefaultSchemaID, DefaultCopySchemaID, response.Errors)
	}

}

func (s *APIModeServiceTests) testAPIModeJSONParseError(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredBodyParseError})
}

func (s *APIModeServiceTests) testAPIModeInvalidCTParseError(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	p, err := json.Marshal(map[string]any{
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredBodyParseError})
}

func (s *APIModeServiceTests) testAPIModeCTNotInSpec(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	p, err := json.Marshal(map[string]any{
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredBodyParseError})
}

func (s *APIModeServiceTests) testAPIModeEmptyBody(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredBodyMissed})
}

func (s *APIModeServiceTests) testAPIModeNoXWallarmSchemaIDHeader(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	p, err := json.Marshal(map[string]any{
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 500 {
		t.Errorf("Incorrect response status code. Expected: 500 and got %d",
			reqCtx.Response.StatusCode())
	}

	if len(reqCtx.Response.Body()) > 0 {
		t.Errorf("Incorrect response body size. Expected: 0 and got %d",
			len(reqCtx.Response.Body()))
		t.Logf("Response body: %s", string(reqCtx.Response.Body()))
	}

	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// invalid values
	for _, headerValue := range []string{"invalidValue", "", " ", " , , "} {

		req.Header.Set(web.XWallarmSchemaIDHeader, headerValue)

		reqCtx = fasthttp.RequestCtx{
			Request: *req,
		}

		handler(&reqCtx)

		if reqCtx.Response.StatusCode() != 500 {
			t.Errorf("Incorrect response status code. Expected: 500 and got %d",
				reqCtx.Response.StatusCode())
		}

		t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	}

	// missed values
	for _, headerValue := range []int{MissedSchemaID} {

		req.Header.Set(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", headerValue))

		reqCtx = fasthttp.RequestCtx{
			Request: *req,
		}

		handler(&reqCtx)

		if reqCtx.Response.StatusCode() != 200 {
			t.Errorf("Incorrect response status code. Expected: 200 and got %d",
				reqCtx.Response.StatusCode())
		}

		apifwResponse := validator.ValidationResponse{}
		if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
			t.Errorf("Error while JSON response parsing: %v", err)
		}

		if len(apifwResponse.Summary) > 0 {
			if *apifwResponse.Summary[0].SchemaID != headerValue {
				t.Errorf("Incorrect error code. Expected: %d and got %d",
					headerValue, *apifwResponse.Summary[0].SchemaID)
			}
			if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusInternalServerError {
				t.Errorf("Incorrect result status. Expected: %d and got %d",
					fasthttp.StatusInternalServerError, *apifwResponse.Summary[0].StatusCode)
			}
		}

		t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	}

}

func (s *APIModeServiceTests) testAPIModeMethodAndPathNotFound(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	p, err := json.Marshal(map[string]any{
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := validator.ValidationResponse{}
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

	if len(apifwResponse.Errors) > 0 {
		if apifwResponse.Errors[0].Code != validator.ErrCodeMethodAndPathNotFound {
			t.Errorf("Incorrect error code. Expected: %s and got %s",
				validator.ErrCodeMethodAndPathNotFound, apifwResponse.Errors[0].Code)
		}
	}

	// check path
	req.Header.SetMethod("POST")
	req.Header.SetRequestURI(testUnknownPath)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse = validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Errors) > 0 {
		if apifwResponse.Errors[0].Code != validator.ErrCodeMethodAndPathNotFound {
			t.Errorf("Incorrect error code. Expected: %s and got %s",
				validator.ErrCodeMethodAndPathNotFound, apifwResponse.Errors[0].Code)
		}
	}
}

func (s *APIModeServiceTests) testAPIModeRequiredQueryParameterMissed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/query?id=" + uuid.New().String())
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	req.SetRequestURI("/test/query?wrong_q_parameter=test")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredQueryParameterMissed, validator.ErrCodeUnknownParameterFound})
}

func (s *APIModeServiceTests) testAPIModeRequiredHeaderParameterMissed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	req.Header.Del(testRequestHeader)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredHeaderMissed})
}

func (s *APIModeServiceTests) testAPIModeRequiredCookieParameterMissed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/cookies/request")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.SetCookie(testRequestCookie, uuid.New().String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	req.Header.DelAllCookies()

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredCookieParameterMissed})
}

func (s *APIModeServiceTests) testAPIModeRequiredBodyMissed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	p, err := json.Marshal(map[string]any{
		"status":  uuid.New().String(),
		"testInt": 50,
		"error":   "test",
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/body/request")
	req.Header.SetMethod("POST")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredBodyMissed})
}

func (s *APIModeServiceTests) testAPIModeRequiredBodyParameterMissed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	p, err := json.Marshal(map[string]any{
		"status":  uuid.New().String(),
		"testInt": 50,
		"error":   "test",
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	// body without required parameter
	p, err = json.Marshal(map[string]any{
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredBodyParameterMissed})
}

// Invalid parameters errors
func (s *APIModeServiceTests) testAPIModeRequiredQueryParameterInvalidValue(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/query?id=" + uuid.New().String())
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	req.SetRequestURI("/test/query?id=invalid_value_test")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredQueryParameterInvalidValue})

	req.SetRequestURI("/test/query?id=" + uuid.New().String() + "&int=wrongvalue")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredQueryParameterInvalidValue})
}

func (s *APIModeServiceTests) testAPIModeRequiredHeaderParameterInvalidValue(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	req.Header.Del(testRequestHeader)
	req.Header.Add(testRequestHeader, "invalid_value")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredHeaderInvalidValue})

	req.Header.Del(testRequestHeader)
	req.Header.Add(testRequestHeader, xReqTestValue.String())
	req.Header.Add(testRequestIntHeader, "invalid_value")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredHeaderInvalidValue})
}

func (s *APIModeServiceTests) testAPIModeRequiredCookieParameterInvalidValue(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/cookies/request")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.SetCookie(testRequestCookie, uuid.New().String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	req.Header.SetCookie(testRequestCookie, "invalid_test_value")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredCookieParameterInvalidValue})

	req.Header.SetCookie(testRequestCookie, uuid.New().String())
	req.Header.SetCookie(testRequestIntCookie, "invalid_test_value")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredCookieParameterInvalidValue})
}

func (s *APIModeServiceTests) testAPIModeRequiredBodyParameterInvalidValue(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	p, err := json.Marshal(map[string]any{
		"status":  uuid.New().String(),
		"testInt": 50,
		"error":   "test",
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	// body without required parameter
	p, err = json.Marshal(map[string]any{
		"status":  "invalid_test_value",
		"testInt": 50,
		"error":   "test",
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredBodyParameterInvalidValue})

	// body with parameter which has invalid type
	p, err = json.Marshal(map[string]any{
		"status":  uuid.New().String(),
		"testInt": "invalid_type_str",
		"error":   "test",
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredBodyParameterInvalidValue})

	// body with required parameter that has value less than minimum threshold
	p, err = json.Marshal(map[string]any{
		"status":  uuid.New().String(),
		"testInt": 1,
		"error":   "test",
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredBodyParameterInvalidValue})

	// body with required parameter that has value more than maximum threshold
	p, err = json.Marshal(map[string]any{
		"status":  uuid.New().String(),
		"testInt": 1000,
		"error":   "test",
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredBodyParameterInvalidValue})
}

// security requirements
func (s *APIModeServiceTests) testAPIModeBasicAuthFailed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/security/basic")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("user1:password1")))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	headerName := "Authorization"

	req.Header.Del(headerName)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status code. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}

		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect schema ID in the response. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
	}

	if len(apifwResponse.Errors) > 0 {
		if apifwResponse.Errors[0].Code != validator.ErrCodeSecRequirementsFailed {
			t.Errorf("Incorrect error code. Expected: %s and got %s",
				validator.ErrCodeSecRequirementsFailed, apifwResponse.Errors[0].Code)
		}
		if apifwResponse.Errors[0].Fields[0] != headerName {
			t.Errorf("Incorrect header name. Expected: %s and got %s",
				headerName, apifwResponse.Errors[0].Fields[0])
		}
	}
}

func (s *APIModeServiceTests) testAPIModeBearerTokenFailed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/security/bearer")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.Add("Authorization", "Bearer "+uuid.New().String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	headerName := "Authorization"
	req.Header.Del(headerName)

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status code. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}

		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect schema ID in the response. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
	}

	if len(apifwResponse.Errors) > 0 {
		if apifwResponse.Errors[0].Code != validator.ErrCodeSecRequirementsFailed {
			t.Errorf("Incorrect error code. Expected: %s and got %s",
				validator.ErrCodeSecRequirementsFailed, apifwResponse.Errors[0].Code)
		}

		if apifwResponse.Errors[0].Fields[0] != headerName {
			t.Errorf("Incorrect header name. Expected: %s and got %s",
				headerName, apifwResponse.Errors[0].Fields[0])
		}
	}
}

func (s *APIModeServiceTests) testAPIModeAPITokenCookieFailed(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/security/cookie")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))
	req.Header.SetCookie(testSecCookieName, uuid.New().String())

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	req.Header.DelAllCookies()

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		if *apifwResponse.Summary[0].StatusCode != fasthttp.StatusForbidden {
			t.Errorf("Incorrect result status code. Expected: %d and got %d",
				fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
		}

		if *apifwResponse.Summary[0].SchemaID != DefaultSchemaID {
			t.Errorf("Incorrect schema ID in the response. Expected: %d and got %d",
				DefaultSchemaID, *apifwResponse.Summary[0].SchemaID)
		}
	}

	if len(apifwResponse.Errors) > 0 {
		if apifwResponse.Errors[0].Code != validator.ErrCodeSecRequirementsFailed {
			t.Errorf("Incorrect error code. Expected: %s and got %s",
				validator.ErrCodeSecRequirementsFailed, apifwResponse.Errors[0].Code)
		}

		if apifwResponse.Errors[0].Fields[0] != testSecCookieName {
			t.Errorf("Incorrect header name. Expected: %s and got %s",
				testSecCookieName, apifwResponse.Errors[0].Fields[0])
		}
	}
}

// unknown parameters
func (s *APIModeServiceTests) testAPIModeUnknownParameterBodyJSON(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	p, err := json.Marshal(map[string]any{
		"firstname":     "test",
		"lastname":      "test",
		"job":           "test",
		"unknownParam":  "test",
		"unknownParam2": "test",
		"":              "test",
		"email":         "test@wallarm.com",
		"url":           "http://wallarm.com",
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeUnknownParameterFound})

	p, err = json.Marshal(map[string]any{
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)
}

func (s *APIModeServiceTests) testAPIModeUnknownParameterBodyPost(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeUnknownParameterFound})

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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)
}

func (s *APIModeServiceTests) testAPIModeUnknownParameterQuery(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/query?uparam=test&id=" + uuid.New().String())
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeUnknownParameterFound})

	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/test/query?id=" + uuid.New().String())
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)
}

func (s *APIModeServiceTests) testAPIModeUnknownParameterTextPlainCT(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)
}

func (s *APIModeServiceTests) testAPIModeUnknownParameterInvalidCT(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := validator.ValidationResponse{}
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

func (s *APIModeServiceTests) testAPIModePassOptionsRequest(t *testing.T) {

	cfgPassOptions := config.APIMode{
		APIFWInit:                  config.APIFWInit{Mode: web.APIMode},
		SpecificationUpdatePeriod:  2 * time.Second,
		UnknownParametersDetection: true,
		PassOptionsRequests:        true,
	}

	handler := handlersAPI.Handlers(s.lock, &cfgPassOptions, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/signup")
	req.Header.SetMethod("OPTIONS")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)
}

func (s *APIModeServiceTests) testAPIModeMultipartOptionalParams(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredBodyParseError})
}

func (s *APIModeServiceTests) testAPIModeInvalidRouteInRequest(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	p, err := json.Marshal(map[string]any{
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
	// invalid route (with a slash in the end)
	req.SetRequestURI("/test/signup/")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := validator.ValidationResponse{}
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

func (s *APIModeServiceTests) testAPIModeInvalidRouteInRequestInMultipleSchemas(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	p, err := json.Marshal(map[string]any{
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
	// invalid route (with a slash in the end)
	req.SetRequestURI("/test/signup/")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d, %d", DefaultSchemaID, DefaultCopySchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Summary) > 0 {
		for i := range apifwResponse.Summary {
			if *apifwResponse.Summary[i].SchemaID != DefaultSchemaID && *apifwResponse.Summary[i].SchemaID != DefaultCopySchemaID {
				t.Errorf("Incorrect error code. Expected: %d or %d and got %d",
					DefaultSchemaID, DefaultCopySchemaID, *apifwResponse.Summary[0].SchemaID)
			}
			if *apifwResponse.Summary[i].StatusCode != fasthttp.StatusForbidden {
				t.Errorf("Incorrect result status. Expected: %d and got %d",
					fasthttp.StatusForbidden, *apifwResponse.Summary[0].StatusCode)
			}
		}
	}
}

func (s *APIModeServiceTests) testAPIModeAllMethods(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	// check all supported methods: GET POST PUT PATCH DELETE TRACE OPTIONS HEAD
	for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "TRACE", "OPTIONS", "HEAD"} {
		req := fasthttp.AcquireRequest()
		req.SetRequestURI("/test/methods")
		req.Header.SetMethod(m)
		req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

		reqCtx := fasthttp.RequestCtx{
			Request: *req,
		}

		handler(&reqCtx)

		t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
		t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

		// check response status code and response body
		checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test/methods")
	req.Header.SetMethod("INVALID_METHOD")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeMethodAndPathNotFound})
}

func (s *APIModeServiceTests) testConflictsInThePath(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	// check all related paths
	for _, path := range []string{"/path/testValue1", "/path/value1.php"} {
		req := fasthttp.AcquireRequest()
		req.SetRequestURI(path)
		req.Header.SetMethod("GET")
		req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

		reqCtx := fasthttp.RequestCtx{
			Request: *req,
		}

		handler(&reqCtx)

		t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
		t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

		// check response status code and response body
		checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/path/valueNotExist.php")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredPathParameterInvalidValue})
}

func (s *APIModeServiceTests) testObjectInQuery(t *testing.T) {

	handler := handlersAPI.Handlers(s.lock, &cfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	for _, path := range []string{"/query/paramsObject?f.0%5Bf%5D%5B0%5D=test"} {

		req := fasthttp.AcquireRequest()
		req.SetRequestURI(path)
		req.Header.SetMethod("GET")
		req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

		reqCtx := fasthttp.RequestCtx{
			Request: *req,
		}

		handler(&reqCtx)

		t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
		t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

		// check response status code and response body
		checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/query/paramsObject")
	req.Header.SetMethod("GET")
	req.Header.Add(web.XWallarmSchemaIDHeader, fmt.Sprintf("%d", DefaultSchemaID))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseForbiddenStatusCode(t, &reqCtx, DefaultSchemaID, []string{validator.ErrCodeRequiredQueryParameterMissed})
}

func (s *APIModeServiceTests) testAPIModeMissedMultipleReqParamsLimitedResponse(t *testing.T) {

	updatedCfg := config.APIMode{
		APIFWInit:                  config.APIFWInit{Mode: web.APIMode},
		SpecificationUpdatePeriod:  2 * time.Second,
		UnknownParametersDetection: true,
		PassOptionsRequests:        false,
		MaxErrorsInResponse:        1,
	}

	handler := handlersAPI.Handlers(s.lock, &updatedCfg, s.shutdown, s.logger, metrics.NewPrometheusMetrics(false), s.dbSpec, nil, nil)

	p, err := json.Marshal(map[string]any{
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

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

	// check response status code and response body
	checkResponseOkStatusCode(t, &reqCtx, DefaultSchemaID)

	// Repeat request with invalid email
	reqInvalidEmail, err := json.Marshal(map[string]any{
		"email": "test@wallarm.com",
	})

	if err != nil {
		t.Fatal(err)
	}

	req.SetBodyStream(bytes.NewReader(reqInvalidEmail), -1)

	missedParams := map[string]any{
		"firstname": struct{}{},
		"lastname":  struct{}{},
	}

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	apifwResponse := validator.ValidationResponse{}
	if err := json.Unmarshal(reqCtx.Response.Body(), &apifwResponse); err != nil {
		t.Errorf("Error while JSON response parsing: %v", err)
	}

	if len(apifwResponse.Errors) != 1 {
		t.Errorf("wrong number of errors. Expected: 1. Got: %d", len(apifwResponse.Errors))
	}

	for _, apifwErr := range apifwResponse.Errors {

		if apifwErr.Code != validator.ErrCodeRequiredBodyParameterMissed {
			t.Errorf("Incorrect error code. Expected: %s and got %s",
				validator.ErrCodeRequiredBodyParameterMissed, apifwErr.Code)
		}

		if len(apifwErr.Fields) != 1 {
			t.Errorf("wrong number of related fields. Expected: 1. Got: %d", len(apifwErr.Fields))
		}

		if _, ok := missedParams[apifwErr.Fields[0]]; !ok {
			t.Errorf("Invalid missed field. Expected: firstname or lastname but got %s",
				apifwErr.Fields[0])
		}

	}

	t.Logf("Name of the test: %s; request method: %s; request uri: %s; request body: %s", t.Name(), string(reqCtx.Request.Header.Method()), string(reqCtx.Request.RequestURI()), string(reqCtx.Request.Body()))
	t.Logf("Name of the test: %s; status code: %d; response body: %s", t.Name(), reqCtx.Response.StatusCode(), string(reqCtx.Response.Body()))

}
