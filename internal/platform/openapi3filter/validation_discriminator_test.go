package openapi3filter

import (
	"bytes"
	"testing"

	"github.com/savsgio/gotils/strconv"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"

	"github.com/wallarm/api-firewall/internal/platform/openapi3"
	"github.com/wallarm/api-firewall/internal/platform/router"
)

func TestValidationWithDiscriminatorSelection(t *testing.T) {
	const spec = `
openapi: 3.0.0
info:
  version: 0.2.0
  title: yaAPI

paths:

  /blob:
    put:
      operationId: SetObj
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/blob'
      responses:
        '200':
          description: Ok

components:
  schemas:
    blob:
      oneOf:
        - $ref: '#/components/schemas/objA'
        - $ref: '#/components/schemas/objB'
      discriminator:
        propertyName: discr
        mapping:
          objA: '#/components/schemas/objA'
          objB: '#/components/schemas/objB'
    genericObj:
      type: object
      required:
        - discr
      properties:
        discr:
          type: string
          enum:
            - objA
            - objB
      discriminator:
        propertyName: discr
        mapping:
          objA: '#/components/schemas/objA'
          objB: '#/components/schemas/objB'
    objA:
      allOf:
      - $ref: '#/components/schemas/genericObj'
      - type: object
        properties:
          base64:
            type: string

    objB:
      allOf:
      - $ref: '#/components/schemas/genericObj'
      - type: object
        properties:
          value:
            type: integer
`

	loader := openapi3.NewSwaggerLoader()
	doc, err := loader.LoadSwaggerFromData([]byte(spec))
	require.NoError(t, err)

	router, err := router.NewRouter(doc)
	require.NoError(t, err)

	body := bytes.NewReader([]byte(`{"discr": "objA", "base64": "S25vY2sgS25vY2ssIE5lbyAuLi4="}`))

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/blob")
	req.Header.SetMethod("PUT")
	req.SetBodyStream(body, -1)
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
	err = ValidateRequest(loader.Context, requestValidationInput)
	require.NoError(t, err)
}
