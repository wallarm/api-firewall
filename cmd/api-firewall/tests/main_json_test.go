package tests

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	proxyHandler "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/proxy"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/router"
)

const openAPIJSONSpecTest = `
openapi: 3.0.1
info:
  title: Minimal integer field example
  version: 0.0.1
paths:
  /test:
    post:
      requestBody:
        content:
          application/json:
            schema:
              oneOf:
                - $ref: '#/components/schemas/Obj'
                - $ref: '#/components/schemas/Arr'
      responses:
        '200':
          description: Success

components:
  schemas:
    Obj:
      type: object
      properties:
        valueNum:
          type: number
        valueInt:
          type: integer
        valueStr:
          type: string
        valueBool:
          type: boolean
        valueNumMultipleOf:
          type: number
          multipleOf: 2.5
        valueIntMinMax:
          type: integer
          minimum: 1
          maximum: 20
        valueStringMinMax:
          type: string
          minLength: 3
          maxLength: 20
        ValueStringEnum:
          type: string
          enum: [ testValue1, testValue2, testValue3 ]
    Arr:
      type: array
      items:
        $ref: '#/components/schemas/Obj'
`

var (
	// basic APIFW configuration
	apifwCfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
	}
)

func TestJSONBasic(t *testing.T) {

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

	swagger, err := openapi3.NewLoader().LoadFromData([]byte(openAPIJSONSpecTest))
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
	}

	// basic test
	t.Run("basicObjJSONFieldValidation", apifwTests.testBasicObjJSONFieldValidation)
	t.Run("basicArrJSONFieldValidation", apifwTests.testBasicArrJSONFieldValidation)
	t.Run("basicJSONFieldValidation", apifwTests.testNegativeJSONFieldValidation)

}

func (s *ServiceTests) testBasicObjJSONFieldValidation(t *testing.T) {

	handler := proxyHandler.Handlers(&apifwCfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil)

	// basic object check
	p, err := json.Marshal(map[string]interface{}{
		"valueNum":           10.1,
		"valueInt":           10,
		"valueStr":           "testStringValue",
		"valueBool":          true,
		"valueNumMultipleOf": 10.0,
		"valueIntMinMax":     1,
		"valueStringMinMax":  "test",
		"ValueStringEnum":    "testValue1",
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test")
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

func (s *ServiceTests) testBasicArrJSONFieldValidation(t *testing.T) {

	handler := proxyHandler.Handlers(&apifwCfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil)

	p, err := json.Marshal([]map[string]interface{}{{
		"valueNum":           10.1,
		"valueInt":           10,
		"valueStr":           "testStringValue",
		"valueBool":          true,
		"valueNumMultipleOf": 10.0,
		"valueIntMinMax":     1,
		"valueStringMinMax":  "test",
		"ValueStringEnum":    "testValue1",
	},
	})

	if err != nil {
		t.Fatal(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test")
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
		t.Errorf("basic array validation test: incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceTests) testNegativeJSONFieldValidation(t *testing.T) {

	handler := proxyHandler.Handlers(&apifwCfg, s.serverUrl, s.shutdown, s.logger, s.proxy, s.swagRouter, nil)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/test")
	req.Header.SetMethod("POST")
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte("{\"status\":\"success\"}"))

	// negative tests
	negativeTests := []struct {
		name        string
		requestBody map[string]interface{}
	}{
		// request with invalid valueNum
		{
			name: "invalid_value_num",
			requestBody: map[string]interface{}{
				"valueNum":           "wrongType",
				"valueInt":           10,
				"valueStr":           "testStringValue",
				"valueBool":          true,
				"valueNumMultipleOf": 10.0,
				"valueIntMinMax":     1,
				"valueStringMinMax":  "test",
				"ValueStringEnum":    "testValue1",
			},
		},
		// request with invalid valueInt
		{
			name: "invalid_value_int",
			requestBody: map[string]interface{}{
				"valueNum":           10.1,
				"valueInt":           10.1,
				"valueStr":           "testStringValue",
				"valueBool":          true,
				"valueNumMultipleOf": 10.0,
				"valueIntMinMax":     1,
				"valueStringMinMax":  "test",
				"ValueStringEnum":    "testValue1",
			},
		},
		// request with invalid valueStr
		{
			name: "invalid_value_str",
			requestBody: map[string]interface{}{
				"valueNum":           10.1,
				"valueInt":           10,
				"valueStr":           10,
				"valueBool":          true,
				"valueNumMultipleOf": 10.0,
				"valueIntMinMax":     1,
				"valueStringMinMax":  "test",
				"ValueStringEnum":    "testValue1",
			},
		},
		// request with invalid valueBool
		{
			name: "invalid_value_bool",
			requestBody: map[string]interface{}{
				"valueNum":           10.1,
				"valueInt":           10,
				"valueStr":           "testStringValue",
				"valueBool":          "test",
				"valueNumMultipleOf": 10.0,
				"valueIntMinMax":     1,
				"valueStringMinMax":  "test",
				"ValueStringEnum":    "testValue1",
			},
		},
		// request with invalid valueNumMultipleOf
		{
			name: "invalid_value_num_multiple_of",
			requestBody: map[string]interface{}{
				"valueNum":           10.1,
				"valueInt":           10,
				"valueStr":           "testStringValue",
				"valueBool":          true,
				"valueNumMultipleOf": 9.2,
				"valueIntMinMax":     1,
				"valueStringMinMax":  "test",
				"ValueStringEnum":    "testValue1",
			},
		},
		// request with invalid valueIntMinMax
		{
			name: "invalid_value_int_min_max",
			requestBody: map[string]interface{}{
				"valueNum":           10.1,
				"valueInt":           10,
				"valueStr":           "testStringValue",
				"valueBool":          true,
				"valueNumMultipleOf": 10.0,
				"valueIntMinMax":     100,
				"valueStringMinMax":  "test",
				"ValueStringEnum":    "testValue1",
			},
		},
		// request with invalid valueStringMinMax
		{
			name: "invalid_value_string_min_max",
			requestBody: map[string]interface{}{
				"valueNum":           10.1,
				"valueInt":           10,
				"valueStr":           "testStringValue",
				"valueBool":          true,
				"valueNumMultipleOf": 10.0,
				"valueIntMinMax":     1,
				"valueStringMinMax":  "t",
				"ValueStringEnum":    "testValue1",
			},
		},
		// request with invalid ValueStringEnum
		{
			name: "invalid_value_string_enum",
			requestBody: map[string]interface{}{
				"valueNum":           10.1,
				"valueInt":           10,
				"valueStr":           "testStringValue",
				"valueBool":          true,
				"valueNumMultipleOf": 10.0,
				"valueIntMinMax":     1,
				"valueStringMinMax":  "test",
				"ValueStringEnum":    "testWrongEnum",
			},
		},
	}

	for _, test := range negativeTests {

		reqInvalidEmail, err := json.Marshal(test.requestBody)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}

		req.SetBodyStream(bytes.NewReader(reqInvalidEmail), -1)

		reqCtx := fasthttp.RequestCtx{
			Request: *req,
		}

		handler(&reqCtx)

		if reqCtx.Response.StatusCode() != 403 {
			t.Errorf("%s: incorrect response status code. Expected: 403 and got %d",
				test.name, reqCtx.Response.StatusCode())
		}
	}

}
