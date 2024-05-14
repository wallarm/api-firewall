package validator

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"github.com/valyala/fastjson"
)

func TestUnknownParametersRequest(t *testing.T) {
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
          application/x-www-form-urlencoded:
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
      responses:
        '201':
          description: Created
  /unknown:
    post:
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
          application/x-www-form-urlencoded:
            schema:
              type: object
              required:
                - subCategory
              properties:
                subCategory:
                  type: string
      responses:
        '201':
          description: Created
`

	var categoryFood = "Food"

	router := setupTestRouter(t, spec)

	type testRequestBody struct {
		SubCategory      string  `json:"subCategory"`
		Category         *string `json:"category,omitempty"`
		UnknownParameter string  `json:"unknown,omitempty"`
	}
	type args struct {
		requestBody *testRequestBody
		ct          string
		url         string
	}
	tests := []struct {
		name         string
		args         args
		expectedErr  error
		expectedResp []*RequestUnknownParameterError
	}{
		{
			name: "Valid request with optional field which is equal to none",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", Category: nil},
				url:         "/category?category=cookies",
				ct:          "application/json",
			},
			expectedErr:  nil,
			expectedResp: nil,
		},
		{
			name: "Valid request with all fields set",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", Category: &categoryFood},
				url:         "/category?category=cookies",
				ct:          "application/json",
			},
			expectedErr:  nil,
			expectedResp: nil,
		},
		{
			name: "Valid request without certain fields",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate"},
				url:         "/category?category=cookies",
				ct:          "application/json",
			},
			expectedErr:  nil,
			expectedResp: nil,
		},
		{
			name: "Invalid operation params",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate"},
				url:         "/category?invalidCategory=badCookie",
				ct:          "application/json",
			},
			expectedErr: nil,
			expectedResp: []*RequestUnknownParameterError{
				{
					Parameters: []RequestParameterDetails{{
						Name:        "invalidCategory",
						Placeholder: "query",
						Type:        "string",
					}},
					Message: ErrUnknownQueryParameter.Error(),
				},
			},
		},
		{
			name: "Invalid request body",
			args: args{
				requestBody: nil,
				url:         "/category?category=cookies",
				ct:          "application/json",
			},
			expectedErr:  nil,
			expectedResp: nil,
		},
		{
			name: "Unknown query param",
			args: args{
				requestBody: nil,
				url:         "/category?category=cookies&unknown=test",
			},
			expectedErr: nil,
			expectedResp: []*RequestUnknownParameterError{
				{
					Parameters: []RequestParameterDetails{{
						Name:        "unknown",
						Placeholder: "query",
						Type:        "string",
					}},
					Message: ErrUnknownQueryParameter.Error(),
				},
			},
		},
		{
			name: "Unknown JSON param",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", Category: &categoryFood, UnknownParameter: "test"},
				url:         "/category?category=cookies",
				ct:          "application/json",
			},
			expectedErr: nil,
			expectedResp: []*RequestUnknownParameterError{
				{
					Parameters: []RequestParameterDetails{{
						Name:        "unknown",
						Placeholder: "body",
						Type:        "string",
					}},
					Message: ErrUnknownBodyParameter.Error(),
				},
			},
		},
		{
			name: "Unknown POST param",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", Category: &categoryFood, UnknownParameter: "test"},
				url:         "/category?category=cookies",
				ct:          "application/x-www-form-urlencoded",
			},
			expectedErr: nil,
			expectedResp: []*RequestUnknownParameterError{
				{
					Parameters: []RequestParameterDetails{{
						Name:        "unknown",
						Placeholder: "body",
						Type:        "string",
					}},
					Message: ErrUnknownBodyParameter.Error(),
				},
			},
		},
		{
			name: "Valid POST params",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", Category: &categoryFood},
				url:         "/category?category=cookies",
				ct:          "application/x-www-form-urlencoded",
			},
			expectedErr:  nil,
			expectedResp: nil,
		},
		{
			name: "Valid POST unknown params 0",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", UnknownParameter: "unknownValue"},
				url:         "/unknown",
				ct:          "application/x-www-form-urlencoded",
			},
			expectedErr: nil,
			expectedResp: []*RequestUnknownParameterError{
				{
					Parameters: []RequestParameterDetails{{
						Name:        "unknown",
						Placeholder: "body",
						Type:        "string",
					}},
					Message: ErrUnknownBodyParameter.Error(),
				},
			},
		},
		{
			name: "Valid POST unknown params 1",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", Category: &categoryFood, UnknownParameter: "unknownValue"},
				url:         "/unknown",
				ct:          "application/x-www-form-urlencoded",
			},
			expectedErr: nil,
			expectedResp: []*RequestUnknownParameterError{
				{
					Parameters: []RequestParameterDetails{{
						Name:        "unknown",
						Placeholder: "body",
						Type:        "string",
					}, {
						Name:        "category",
						Placeholder: "body",
						Type:        "string",
					}},
					Message: ErrUnknownBodyParameter.Error(),
				},
			},
		},
		{
			name: "Valid JSON unknown params 2",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", Category: &categoryFood, UnknownParameter: "unknownValue"},
				url:         "/unknown",
				ct:          "application/json",
			},
			expectedErr: nil,
			expectedResp: []*RequestUnknownParameterError{
				{
					Parameters: []RequestParameterDetails{{
						Name:        "unknown",
						Placeholder: "body",
						Type:        "string",
					},
						{
							Name:        "category",
							Placeholder: "body",
							Type:        "string",
						}},
					Message: ErrUnknownBodyParameter.Error(),
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := fasthttp.AcquireRequest()
			req.SetRequestURI(tc.args.url)
			req.Header.SetMethod("POST")
			req.Header.SetContentType(tc.args.ct)

			var requestBody io.Reader
			if tc.args.requestBody != nil {
				switch tc.args.ct {
				case "application/x-www-form-urlencoded":
					if tc.args.requestBody.UnknownParameter != "" {
						req.PostArgs().Add("unknown", tc.args.requestBody.UnknownParameter)
					}
					if tc.args.requestBody.SubCategory != "" {
						req.PostArgs().Add("subCategory", tc.args.requestBody.SubCategory)
					}
					if tc.args.requestBody.Category != nil {
						if *tc.args.requestBody.Category != "" {
							req.PostArgs().Add("category", *tc.args.requestBody.Category)
						}
					}
					requestBody = strings.NewReader(req.PostArgs().String())
				case "application/json":
					testingBody, err := json.Marshal(tc.args.requestBody)
					require.NoError(t, err)
					requestBody = bytes.NewReader(testingBody)
				}
			}

			req.SetBodyStream(requestBody, -1)

			ctx := fasthttp.RequestCtx{
				Request: *req,
			}

			reqHttp := http.Request{}

			err := fasthttpadaptor.ConvertRequest(&ctx, &reqHttp, false)
			require.NoError(t, err)

			route, pathParams, err := router.FindRoute(&reqHttp)
			require.NoError(t, err)

			validationInput := &openapi3filter.RequestValidationInput{
				Request:    &reqHttp,
				PathParams: pathParams,
				Route:      route,
			}
			upRes, err := ValidateUnknownRequestParameters(&ctx, validationInput.Route, validationInput.Request.Header, &fastjson.Parser{})
			assert.IsType(t, tc.expectedErr, err, "ValidateUnknownRequestParameters(): error = %v, expectedError %v", err, tc.expectedErr)
			if tc.expectedErr != nil {
				return
			}
			if tc.expectedResp != nil && len(tc.expectedResp) > 0 {
				assert.Equal(t, len(tc.expectedResp), len(upRes), "expect the number of unknown parameters: %d, got %d", len(tc.expectedResp), len(upRes))
				assert.Equal(t, true, matchUnknownParamsResp(tc.expectedResp, upRes), "expect unknown parameters: %v, got %v", tc.expectedResp, upRes)
			}
		})
	}
}

func matchUnknownParamsResp(expected []*RequestUnknownParameterError, actual []RequestUnknownParameterError) bool {
	for _, expectedValue := range expected {
		for _, expectedParam := range expectedValue.Parameters {
			var found bool
			// search for the same param in the actual resp
			for _, actualValue := range actual {
				for _, actualParam := range actualValue.Parameters {
					if expectedParam.Name == actualParam.Name &&
						expectedParam.Type == actualParam.Type &&
						expectedParam.Placeholder == actualParam.Placeholder {
						found = true
					}
				}
			}
			if !found {
				return false
			}
		}
	}

	for _, actualValue := range actual {
		for _, actualParam := range actualValue.Parameters {
			var found bool
			// search for the same param in the actual resp
			for _, expectedValue := range expected {
				for _, expectedParam := range expectedValue.Parameters {
					if expectedParam.Name == actualParam.Name &&
						expectedParam.Type == actualParam.Type &&
						expectedParam.Placeholder == actualParam.Placeholder {
						found = true
					}
				}
			}
			if !found {
				return false
			}
		}
	}

	return true
}
