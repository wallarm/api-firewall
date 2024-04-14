package validator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fastjson"
)

func setupTestRouter(t *testing.T, spec string) routers.Router {
	t.Helper()
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)

	err = doc.Validate(loader.Context)
	require.NoError(t, err)

	router, err := gorillamux.NewRouter(doc)
	require.NoError(t, err)

	return router
}

func TestValidateRequest(t *testing.T) {
	const spec = `
openapi: 3.0.0
info:
  title: 'Validator'
  version: 0.0.2
paths:
  /category:
    post:
      parameters:
        - name: category
          in: query
          schema:
            type: string
          required: true
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - subCategory
              properties:
                subCategory:
                  type: string
                category:
                  type: string
                  default: Sweets
                subCategoryInt:
                  type: integer
                  minimum: 100
                  maximum: 1000
                categoryFloat:
                  type: number
                  minimum: 123.10
                  maximum: 123.20
      responses:
        '201':
          description: Created
      security:
      - apiKey: []
components:
  securitySchemes:
    apiKey:
      type: apiKey
      name: Api-Key
      in: header
`

	router := setupTestRouter(t, spec)

	verifyAPIKeyPresence := func(c context.Context, input *openapi3filter.AuthenticationInput) error {
		if input.SecurityScheme.Type == "apiKey" {
			var found bool
			switch input.SecurityScheme.In {
			case "query":
				_, found = input.RequestValidationInput.GetQueryParams()[input.SecurityScheme.Name]
			case "header":
				_, found = input.RequestValidationInput.Request.Header[http.CanonicalHeaderKey(input.SecurityScheme.Name)]
			case "cookie":
				_, err := input.RequestValidationInput.Request.Cookie(input.SecurityScheme.Name)
				found = !errors.Is(err, http.ErrNoCookie)
			}
			if !found {
				return fmt.Errorf("%v not found in %v", input.SecurityScheme.Name, input.SecurityScheme.In)
			}
		}
		return nil
	}

	type testRequestBody struct {
		SubCategory    string  `json:"subCategory"`
		Category       string  `json:"category,omitempty"`
		SubCategoryInt int     `json:"subCategoryInt,omitempty"`
		CategoryFloat  float32 `json:"categoryFloat,omitempty"`
	}
	type args struct {
		requestBody *testRequestBody
		url         string
		apiKey      string
	}
	tests := []struct {
		name                 string
		args                 args
		expectedModification bool
		expectedErr          error
	}{
		{
			name: "Valid request with all fields set",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", Category: "Food", SubCategoryInt: 123, CategoryFloat: 123.12},
				url:         "/category?category=cookies",
				apiKey:      "SomeKey",
			},
			expectedModification: false,
			expectedErr:          nil,
		},
		{
			name: "Valid request without certain fields",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate"},
				url:         "/category?category=cookies",
				apiKey:      "SomeKey",
			},
			expectedModification: true,
			expectedErr:          nil,
		},
		{
			name: "Invalid operation params",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate"},
				url:         "/category?invalidCategory=badCookie",
				apiKey:      "SomeKey",
			},
			expectedModification: false,
			expectedErr:          &openapi3filter.RequestError{},
		},
		{
			name: "Invalid request body",
			args: args{
				requestBody: nil,
				url:         "/category?category=cookies",
				apiKey:      "SomeKey",
			},
			expectedModification: false,
			expectedErr:          &openapi3filter.RequestError{},
		},
		{
			name: "Invalid security",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate"},
				url:         "/category?category=cookies",
				apiKey:      "",
			},
			expectedModification: false,
			expectedErr:          &openapi3filter.SecurityRequirementsError{},
		},
		{
			name: "Invalid request body and security",
			args: args{
				requestBody: nil,
				url:         "/category?category=cookies",
				apiKey:      "",
			},
			expectedModification: false,
			expectedErr:          &openapi3filter.SecurityRequirementsError{},
		},
		{
			name: "Invalid SubCategoryInt value",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", Category: "Food", SubCategoryInt: 1, CategoryFloat: 123.12},
				url:         "/category?category=cookies",
				apiKey:      "SomeKey",
			},
			expectedModification: false,
			expectedErr:          &openapi3filter.RequestError{},
		},
		{
			name: "Invalid CategoryFloat value",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", Category: "Food", SubCategoryInt: 123, CategoryFloat: 123.21},
				url:         "/category?category=cookies",
				apiKey:      "SomeKey",
			},
			expectedModification: false,
			expectedErr:          &openapi3filter.RequestError{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var requestBody io.Reader
			var originalBodySize int
			if tc.args.requestBody != nil {
				testingBody, err := json.Marshal(tc.args.requestBody)
				require.NoError(t, err)
				requestBody = bytes.NewReader(testingBody)
				originalBodySize = len(testingBody)
			}
			req, err := http.NewRequest(http.MethodPost, tc.args.url, requestBody)
			require.NoError(t, err)
			req.Header.Add("Content-Type", "application/json")
			if tc.args.apiKey != "" {
				req.Header.Add("Api-Key", tc.args.apiKey)
			}

			route, pathParams, err := router.FindRoute(req)
			require.NoError(t, err)

			validationInput := &openapi3filter.RequestValidationInput{
				Request:    req,
				PathParams: pathParams,
				Route:      route,
				Options: &openapi3filter.Options{
					AuthenticationFunc: verifyAPIKeyPresence,
				},
			}
			err = ValidateRequest(context.Background(), validationInput, &fastjson.Parser{})
			assert.IsType(t, tc.expectedErr, err, "ValidateRequest(): error = %v, expectedError %v", err, tc.expectedErr)
			if tc.expectedErr != nil {
				return
			}
			body, err := io.ReadAll(validationInput.Request.Body)
			contentLen := int(validationInput.Request.ContentLength)
			bodySize := len(body)
			assert.NoError(t, err, "unable to read request body: %v", err)
			assert.Equal(t, contentLen, bodySize, "expect ContentLength %d to equal body size %d", contentLen, bodySize)
			bodyModified := originalBodySize != bodySize
			assert.Equal(t, bodyModified, tc.expectedModification, "expect request body modification happened: %t, expected %t", bodyModified, tc.expectedModification)

			validationInput.Request.Body, err = validationInput.Request.GetBody()
			assert.NoError(t, err, "unable to re-generate body by GetBody(): %v", err)
			body2, err := io.ReadAll(validationInput.Request.Body)
			assert.NoError(t, err, "unable to read request body: %v", err)
			assert.Equal(t, body, body2, "body by GetBody() is not matched")
		})
	}
}

func TestValidateQueryParams(t *testing.T) {
	type testCase struct {
		name  string
		param *openapi3.Parameter
		query string
		want  map[string]interface{}
		err   *openapi3.SchemaError // test ParseError in decoder tests
	}

	testCases := []testCase{
		{
			name: "deepObject explode additionalProperties with object properties - missing required property",
			param: &openapi3.Parameter{
				Name: "param", In: "query", Style: "deepObject", Explode: explode,
				Schema: objectOf(
					"obj", additionalPropertiesObjectOf(func() *openapi3.SchemaRef {
						s := objectOf(
							"item1", integerSchema,
							"requiredProp", stringSchema,
						)
						s.Value.Required = []string{"requiredProp"}

						return s
					}()),
					"objIgnored", objectOf("items", stringArraySchema),
				),
			},
			query: "param[obj][prop1][item1]=1",
			err:   &openapi3.SchemaError{SchemaField: "required", Reason: "property \"requiredProp\" is missing"},
		},
		{
			// XXX should this error out?
			name: "deepObject explode additionalProperties with object properties - extraneous nested param property ignored",
			param: &openapi3.Parameter{
				Name: "param", In: "query", Style: "deepObject", Explode: explode,
				Schema: objectOf(
					"obj", additionalPropertiesObjectOf(objectOf(
						"item1", integerSchema,
						"requiredProp", stringSchema,
					)),
					"objIgnored", objectOf("items", stringArraySchema),
				),
			},
			query: "param[obj][prop1][inexistent]=1",
			want: map[string]interface{}{
				"obj": map[string]interface{}{
					"prop1": map[string]interface{}{},
				},
			},
		},
		{
			name: "deepObject explode additionalProperties with object properties",
			param: &openapi3.Parameter{
				Name: "param", In: "query", Style: "deepObject", Explode: explode,
				Schema: objectOf(
					"obj", additionalPropertiesObjectOf(objectOf(
						"item1", numberSchema,
						"requiredProp", stringSchema,
					)),
					"objIgnored", objectOf("items", stringArraySchema),
				),
			},
			query: "param[obj][prop1][item1]=1.123",
			want: map[string]interface{}{
				"obj": map[string]interface{}{
					"prop1": map[string]interface{}{
						"item1": float64(1.123),
					},
				},
			},
		},
		{
			name: "deepObject explode nested objects - misplaced parameter",
			param: &openapi3.Parameter{
				Name: "param", In: "query", Style: "deepObject", Explode: explode,
				Schema: objectOf(
					"obj", objectOf("nestedObjOne", objectOf("items", stringArraySchema)),
				),
			},
			query: "param[obj][nestedObjOne]=baz",
			err: &openapi3.SchemaError{
				SchemaField: "type", Reason: "value must be an object", Value: "baz", Schema: objectOf("items", stringArraySchema).Value,
			},
		},
		{
			name: "deepObject explode nested object - extraneous param ignored",
			param: &openapi3.Parameter{
				Name: "param", In: "query", Style: "deepObject", Explode: explode,
				Schema: objectOf(
					"obj", objectOf("nestedObjOne", stringSchema, "nestedObjTwo", stringSchema),
				),
			},
			query: "anotherparam=bar",
			want:  map[string]interface{}(nil),
		},
		{
			name: "deepObject explode additionalProperties with object properties - multiple properties",
			param: &openapi3.Parameter{
				Name: "param", In: "query", Style: "deepObject", Explode: explode,
				Schema: objectOf(
					"obj", additionalPropertiesObjectOf(objectOf("item1", integerSchema, "item2", stringArraySchema)),
					"objIgnored", objectOf("items", stringArraySchema),
				),
			},
			query: "param[obj][prop1][item1]=1&param[obj][prop1][item2][0]=abc&param[obj][prop2][item1]=2&param[obj][prop2][item2][0]=def",
			want: map[string]interface{}{
				"obj": map[string]interface{}{
					"prop1": map[string]interface{}{
						"item1": int64(1),
						"item2": []interface{}{"abc"},
					},
					"prop2": map[string]interface{}{
						"item1": int64(2),
						"item2": []interface{}{"def"},
					},
				},
			},
		},

		//
		//
		{
			name: "deepObject explode nested object anyOf",
			param: &openapi3.Parameter{
				Name: "param", In: "query", Style: "deepObject", Explode: explode,
				Schema: objectOf(
					"obj", anyofSchema,
				),
			},
			query: "param[obj]=1",
			want: map[string]interface{}{
				"obj": int64(1),
			},
		},
		{
			name: "deepObject explode nested object allOf",
			param: &openapi3.Parameter{
				Name: "param", In: "query", Style: "deepObject", Explode: explode,
				Schema: objectOf(
					"obj", allofSchema,
				),
			},
			query: "param[obj]=1",
			want: map[string]interface{}{
				"obj": int64(1),
			},
		},
		{
			name: "deepObject explode nested object oneOf",
			param: &openapi3.Parameter{
				Name: "param", In: "query", Style: "deepObject", Explode: explode,
				Schema: objectOf(
					"obj", oneofSchema,
				),
			},
			query: "param[obj]=true",
			want: map[string]interface{}{
				"obj": true,
			},
		},
		{
			name: "deepObject explode nested object oneOf - object",
			param: &openapi3.Parameter{
				Name: "param", In: "query", Style: "deepObject", Explode: explode,
				Schema: objectOf(
					"obj", oneofSchemaObject,
				),
			},
			query: "param[obj][id2]=1&param[obj][name2]=abc",
			want: map[string]interface{}{
				"obj": map[string]interface{}{
					"id2":   "1",
					"name2": "abc",
				},
			},
		},
		{
			name: "deepObject explode nested object oneOf - object - more than one match",
			param: &openapi3.Parameter{
				Name: "param", In: "query", Style: "deepObject", Explode: explode,
				Schema: objectOf(
					"obj", oneofSchemaObject,
				),
			},
			query: "param[obj][id]=1&param[obj][id2]=2",
			err: &openapi3.SchemaError{
				SchemaField: "oneOf",
				Value:       map[string]interface{}{"id": "1", "id2": "2"},
				Reason:      "value matches more than one schema from \"oneOf\" (matches schemas at indices [0 1])",
				Schema:      oneofSchemaObject.Value,
			},
		},
		{
			name: "deepObject explode nested object oneOf - array",
			param: &openapi3.Parameter{
				Name: "param", In: "query", Style: "deepObject", Explode: explode,
				Schema: objectOf(
					"obj", oneofSchemaArrayObject,
				),
			},
			query: "param[obj][0]=a&param[obj][1]=b",
			want: map[string]interface{}{
				"obj": []interface{}{
					"a",
					"b",
				},
			},
		},
		//
		//
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			info := &openapi3.Info{
				Title:   "MyAPI",
				Version: "0.1",
			}
			doc := &openapi3.T{OpenAPI: "3.0.0", Info: info, Paths: openapi3.NewPaths()}
			op := &openapi3.Operation{
				OperationID: "test",
				Parameters:  []*openapi3.ParameterRef{{Value: tc.param}},
				Responses:   openapi3.NewResponses(),
			}
			doc.AddOperation("/test", http.MethodGet, op)
			err := doc.Validate(context.Background())
			require.NoError(t, err)
			router, err := legacyrouter.NewRouter(doc)
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodGet, "http://test.org/test?"+tc.query, nil)
			route, pathParams, err := router.FindRoute(req)
			require.NoError(t, err)

			input := &openapi3filter.RequestValidationInput{Request: req, PathParams: pathParams, Route: route}
			err = ValidateParameter(context.Background(), input, tc.param)

			if tc.err != nil {
				require.Error(t, err)
				re, ok := err.(*openapi3filter.RequestError)
				if !ok {
					t.Errorf("error is not a RequestError")

					return
				}

				gErr, ok := re.Unwrap().(*openapi3.SchemaError)
				if !ok {
					t.Errorf("unknown RequestError wrapped error type")
				}
				matchSchemaError(t, gErr, tc.err)

				return
			}

			require.NoError(t, err)

			got, _, err := decodeStyledParameter(tc.param, input)
			require.EqualValues(t, tc.want, got)
		})
	}
}

func matchSchemaError(t *testing.T, got, want error) {
	t.Helper()

	wErr, ok := want.(*openapi3.SchemaError)
	if !ok {
		t.Errorf("want error is not a SchemaError")
		return
	}
	gErr, ok := got.(*openapi3.SchemaError)
	if !ok {
		t.Errorf("got error is not a SchemaError")
		return
	}
	assert.Equalf(t, wErr.SchemaField, gErr.SchemaField, "SchemaError SchemaField differs")
	assert.Equalf(t, wErr.Reason, gErr.Reason, "SchemaError Reason differs")

	if wErr.Schema != nil {
		assert.EqualValuesf(t, wErr.Schema, gErr.Schema, "SchemaError Schema differs")
	}
	if wErr.Value != nil {
		assert.EqualValuesf(t, wErr.Value, gErr.Value, "SchemaError Value differs")
	}

	if gErr.Origin == nil && wErr.Origin != nil {
		t.Errorf("expected error origin but got nothing")
	}
	if gErr.Origin != nil && wErr.Origin != nil {
		switch gErrOrigin := gErr.Origin.(type) {
		case *openapi3.SchemaError:
			matchSchemaError(t, gErrOrigin, wErr.Origin)
		case *ParseError:
			matchParseError(t, gErrOrigin, wErr.Origin)
		default:
			t.Errorf("unknown origin error")
		}
	}
}
