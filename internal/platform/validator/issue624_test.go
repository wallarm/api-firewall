package validator

import (
	"net/http"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fastjson"
)

func TestIssue624(t *testing.T) {
	loader := openapi3.NewLoader()
	ctx := loader.Context
	spec := `
openapi: 3.0.0
info:
  version: 1.0.0
  title: Sample API
paths:
  /items:
    get:
      description: Returns a list of stuff
      parameters:
      - description: "test non object"
        explode: true
        style: form
        in: query
        name: test
        required: false
        content:
          application/json:
            schema:
              anyOf:
              - type: string
              - type: integer
      responses:
        '200':
          description: Successful response
`[1:]

	doc, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)

	err = doc.Validate(ctx)
	require.NoError(t, err)

	router, err := gorillamux.NewRouter(doc)
	require.NoError(t, err)

	for _, testcase := range []string{`test1`, `test[1`} {
		t.Run(testcase, func(t *testing.T) {
			httpReq, err := http.NewRequest(http.MethodGet, `/items?test=`+testcase, nil)
			require.NoError(t, err)

			route, pathParams, err := router.FindRoute(httpReq)
			require.NoError(t, err)

			requestValidationInput := &openapi3filter.RequestValidationInput{
				Request:    httpReq,
				PathParams: pathParams,
				Route:      route,
			}

			jsonParser := &fastjson.Parser{}

			err = ValidateRequest(ctx, requestValidationInput, jsonParser)
			require.NoError(t, err)
		})
	}
}
