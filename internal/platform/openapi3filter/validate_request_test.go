package openapi3filter

import (
	"bytes"
	"encoding/json"
	"github.com/valyala/fastjson"
	"testing"

	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"

	"github.com/wallarm/api-firewall/internal/platform/openapi3"
	"github.com/wallarm/api-firewall/internal/platform/router"
)

const spec = `
openapi: 3.0.0
info:
  title: My API
  version: 0.0.1
paths:
  /:
    post:
      responses:
        default:
          description: ''
      requestBody:
        required: true
        content:
          application/json:
            schema:
              oneOf:
              - $ref: '#/components/schemas/Cat'
              - $ref: '#/components/schemas/Dog'
              discriminator:
                propertyName: pet_type

components:
  schemas:
    Pet:
      type: object
      required: [pet_type]
      properties:
        pet_type:
          type: string
      discriminator:
        propertyName: pet_type

    Dog:
      allOf:
      - $ref: '#/components/schemas/Pet'
      - type: object
        properties:
          breed:
            type: string
            enum: [Dingo, Husky, Retriever, Shepherd]
    Cat:
      allOf:
      - $ref: '#/components/schemas/Pet'
      - type: object
        properties:
          hunts:
            type: boolean
          age:
            type: integer
`

//TODO: check oneOf
func TestRequestValidationOneof(t *testing.T) {
	loader := openapi3.NewSwaggerLoader()
	doc, err := loader.LoadSwaggerFromData([]byte(spec))
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

	p, err := json.Marshal(map[string]interface{}{
		"pet_type": "Cat",
		"hunts":    true,
		"age":      5,
	})
	if err != nil {
		panic(err)
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(p), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	reqCtx := fasthttp.RequestCtx{
		Request:  *req,
		Response: *resp,
	}

	pathParamLength := 0
	if getOp := router.Routes[0].Route.PathItem.GetOperation("GET"); getOp != nil {
		pathParamLength = len(getOp.Parameters)
	}

	var pathParams map[string]string

	if pathParamLength > 0 {
		pathParams = make(map[string]string, pathParamLength)

		reqCtx.VisitUserValues(func(key []byte, value interface{}) {
			keyStr := strconv.B2S(key)
			pathParams[keyStr] = value.(string)
		})
	}

	var parserPool fastjson.ParserPool
	requestValidationInput := &RequestValidationInput{
		RequestCtx: &reqCtx,
		PathParams: pathParams,
		ParserJson: &parserPool,
		Route:      router.Routes[0].Route,
	}

	if err := ValidateRequest(loader.Context, requestValidationInput); err != nil {
		t.Log(err)
	}
	// Output:
	// request body has an error: doesn't match the schema: Doesn't match schema "oneOf"
	// Schema:
	//   {
	//     "discriminator": {
	//       "propertyName": "pet_type"
	//     },
	//     "oneOf": [
	//       {
	//         "$ref": "#/components/schemas/Cat"
	//       },
	//       {
	//         "$ref": "#/components/schemas/Dog"
	//       }
	//     ]
	//   }
	//
	// Value:
	//   {
	//     "bark": true,
	//     "breed": "Dingo",
	//     "pet_type": "Cat"
	//   }
}
