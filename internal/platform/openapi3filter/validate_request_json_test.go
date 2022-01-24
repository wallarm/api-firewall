package openapi3filter

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"

	"github.com/wallarm/api-firewall/internal/platform/openapi3"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/routers"
)

const specJSONValidation = `
openapi: 3.0.0
info:
  title: My API
  version: 0.0.1
paths:
  /dog:
    post:
      responses:
        default:
          description: ''
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Dog'
  /item_str:
    post:
      responses:
        default:
          description: ''
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: string
              
  /item_num:
    post:
      responses:
        default:
          description: ''
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: integer
  /item_array_str:
    post:
      responses:
        default:
          description: ''
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: array
              items: 
                type: string
  /item_array_array:
    post:
      responses:
        default:
          description: ''
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: array
              items: 
                type: array
                items:
                  type: string
  /item_array_obj:
    post:
      responses:
        default:
          description: ''
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: array
              items: 
                type: object
                properties:
                  id:
                    type: integer
components:
  schemas:
    Dog:
      type: object
      properties:
        breed:
          type: string
          enum: [Dingo, Husky, Retriever, Shepherd]
        hunts:
          type: boolean
        age:
          type: integer
`

func validationTestBasic(t *testing.T, loader *openapi3.SwaggerLoader, route *routers.Route) {
	p, err := json.Marshal(map[string]interface{}{
		"breed": "Dingo",
		"hunts": true,
		"age":   5,
	})
	if err != nil {
		panic(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/dog")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	reqCtx := fasthttp.RequestCtx{
		Request:  *req,
		Response: *resp,
	}

	var pathParams map[string]string
	var parserPool fastjson.ParserPool
	requestValidationInput := &RequestValidationInput{
		RequestCtx: &reqCtx,
		PathParams: pathParams,
		ParserJson: &parserPool,
		Route:      route,
	}
	if err := ValidateRequest(loader.Context, requestValidationInput); err != nil {
		t.Fatal(err)
	}
}

func validationTestItemStr(t *testing.T, loader *openapi3.SwaggerLoader, route *routers.Route) {

	p := []byte("\"test\"")

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/item_str")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	reqCtx := fasthttp.RequestCtx{
		Request:  *req,
		Response: *resp,
	}

	var pathParams map[string]string
	var parserPool fastjson.ParserPool
	requestValidationInput := &RequestValidationInput{
		RequestCtx: &reqCtx,
		PathParams: pathParams,
		ParserJson: &parserPool,
		Route:      route,
	}
	if err := ValidateRequest(loader.Context, requestValidationInput); err != nil {
		t.Fatal(err)
	}
}

func validationTestItemInt(t *testing.T, loader *openapi3.SwaggerLoader, route *routers.Route) {

	p := []byte("12345")
	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/item_num")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	reqCtx := fasthttp.RequestCtx{
		Request:  *req,
		Response: *resp,
	}

	var pathParams map[string]string
	var parserPool fastjson.ParserPool
	requestValidationInput := &RequestValidationInput{
		RequestCtx: &reqCtx,
		PathParams: pathParams,
		ParserJson: &parserPool,
		Route:      route,
	}
	if err := ValidateRequest(loader.Context, requestValidationInput); err != nil {
		t.Fatal(err)
	}
}

func validationTestArrayStr(t *testing.T, loader *openapi3.SwaggerLoader, route *routers.Route) {
	p, err := json.Marshal([]interface{}{
		"hello",
		"world",
		"test",
	})
	if err != nil {
		panic(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/item_array_str")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	reqCtx := fasthttp.RequestCtx{
		Request:  *req,
		Response: *resp,
	}

	var pathParams map[string]string
	var parserPool fastjson.ParserPool
	requestValidationInput := &RequestValidationInput{
		RequestCtx: &reqCtx,
		PathParams: pathParams,
		ParserJson: &parserPool,
		Route:      route,
	}
	if err := ValidateRequest(loader.Context, requestValidationInput); err != nil {
		t.Fatal(err)
	}
}

func validationTestArrayArray(t *testing.T, loader *openapi3.SwaggerLoader, route *routers.Route) {
	test := []interface{}{
		"hello",
		"test",
	}
	p, err := json.Marshal([]interface{}{
		test,
		test,
	})
	if err != nil {
		panic(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/item_array_array")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	reqCtx := fasthttp.RequestCtx{
		Request:  *req,
		Response: *resp,
	}

	var pathParams map[string]string
	var parserPool fastjson.ParserPool
	requestValidationInput := &RequestValidationInput{
		RequestCtx: &reqCtx,
		PathParams: pathParams,
		ParserJson: &parserPool,
		Route:      route,
	}
	if err := ValidateRequest(loader.Context, requestValidationInput); err != nil {
		t.Fatal(err)
	}
}

func validationTestArrayObj(t *testing.T, loader *openapi3.SwaggerLoader, route *routers.Route) {
	p, err := json.Marshal([]map[string]interface{}{
		{"id": 1},
		{"id": 2},
		{"id": 3},
	})
	if err != nil {
		panic(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/item_array_obj")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	reqCtx := fasthttp.RequestCtx{
		Request:  *req,
		Response: *resp,
	}

	var pathParams map[string]string
	var parserPool fastjson.ParserPool
	requestValidationInput := &RequestValidationInput{
		RequestCtx: &reqCtx,
		PathParams: pathParams,
		ParserJson: &parserPool,
		Route:      route,
	}
	if err := ValidateRequest(loader.Context, requestValidationInput); err != nil {
		t.Fatal(err)
	}
}

func TestRequestValidationJson(t *testing.T) {
	loader := openapi3.NewSwaggerLoader()
	doc, err := loader.LoadSwaggerFromData([]byte(specJSONValidation))
	if err != nil {
		panic(err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		panic(err)
	}

	router, err := router.NewRouter(doc)
	if err != nil {
		panic(err)
	}

	for _, route := range router.Routes {
		switch route.Path {
		case "/dog":
			validationTestBasic(t, loader, route.Route)
		case "/item_str":
			validationTestItemStr(t, loader, route.Route)
		case "/item_num":
			validationTestItemInt(t, loader, route.Route)
		case "/item_array_str":
			validationTestArrayStr(t, loader, route.Route)
		case "/item_array_array":
			validationTestArrayArray(t, loader, route.Route)
		case "/item_array_obj":
			validationTestArrayObj(t, loader, route.Route)
		}
	}
}
