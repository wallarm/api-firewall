package validator

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fastjson"
)

func TestIssue201(t *testing.T) {
	loader := openapi3.NewLoader()
	ctx := loader.Context
	spec := `
openapi: '3.0.3'
info:
  version: 1.0.0
  title: Sample API
paths:
  /_:
    get:
      description: ''
      responses:
        default:
          description: ''
          content:
            application/json:
              schema:
                type: object
          headers:
            X-Blip:
              description: ''
              required: true
              schema:
                type: string
                pattern: '^blip$'
            x-blop:
              description: ''
              schema:
                type: string
                pattern: '^blop$'
            X-Blap:
              description: ''
              required: true
              schema:
                type: string
                pattern: '^blap$'
            X-Blup:
              description: ''
              required: true
              schema:
                type: string
                pattern: '^blup$'
`[1:]

	doc, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)

	err = doc.Validate(ctx)
	require.NoError(t, err)

	for name, testcase := range map[string]struct {
		headers map[string]string
		err     string
	}{

		"no error": {
			headers: map[string]string{
				"X-Blip": "blip",
				"x-blop": "blop",
				"X-Blap": "blap",
				"X-Blup": "blup",
			},
		},

		"missing non-required header": {
			headers: map[string]string{
				"X-Blip": "blip",
				// "x-blop": "blop",
				"X-Blap": "blap",
				"X-Blup": "blup",
			},
		},

		"missing required header": {
			err: `response header "X-Blip" missing`,
			headers: map[string]string{
				// "X-Blip": "blip",
				"x-blop": "blop",
				"X-Blap": "blap",
				"X-Blup": "blup",
			},
		},

		"invalid required header": {
			err: `response header "X-Blup" doesn't match schema: string doesn't match the regular expression "^blup$"`,
			headers: map[string]string{
				"X-Blip": "blip",
				"x-blop": "blop",
				"X-Blap": "blap",
				"X-Blup": "bluuuuuup",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			router, err := gorillamux.NewRouter(doc)
			require.NoError(t, err)

			r, err := http.NewRequest(http.MethodGet, `/_`, nil)
			require.NoError(t, err)

			r.Header.Add(headerCT, "application/json")
			for k, v := range testcase.headers {
				r.Header.Add(k, v)
			}

			route, pathParams, err := router.FindRoute(r)
			require.NoError(t, err)

			jsonParser := &fastjson.Parser{}

			err = ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
				RequestValidationInput: &openapi3filter.RequestValidationInput{
					Request:    r,
					PathParams: pathParams,
					Route:      route,
				},
				Status: 200,
				Header: r.Header,
				Body:   io.NopCloser(strings.NewReader(`{}`)),
			}, jsonParser)
			if e := testcase.err; e != "" {
				require.ErrorContains(t, err, e)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
