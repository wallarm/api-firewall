package validator

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
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
  version: 0.0.1
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
            schema: {}
          application/x-www-form-urlencoded:
            schema: {}
      responses:
        '201':
          description: Created
`

	router := setupTestRouter(t, spec)

	type testRequestBody struct {
		SubCategory      string `json:"subCategory"`
		Category         string `json:"category,omitempty"`
		UnknownParameter string `json:"unknown,omitempty"`
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
			name: "Valid request with all fields set",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", Category: "Food"},
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
					Parameters: []string{"invalidCategory"},
					Err:        ErrUnknownQueryParameter,
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
					Parameters: []string{"unknown"},
					Err:        ErrUnknownQueryParameter,
				},
			},
		},
		{
			name: "Unknown JSON param",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", Category: "Food", UnknownParameter: "test"},
				url:         "/category?category=cookies",
				ct:          "application/json",
			},
			expectedErr: nil,
			expectedResp: []*RequestUnknownParameterError{
				{
					Parameters: []string{"unknown"},
					Err:        ErrUnknownBodyParameter,
				},
			},
		},
		{
			name: "Unknown POST param",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", Category: "Food", UnknownParameter: "test"},
				url:         "/category?category=cookies",
				ct:          "application/x-www-form-urlencoded",
			},
			expectedErr: nil,
			expectedResp: []*RequestUnknownParameterError{
				{
					Parameters: []string{"unknown"},
					Err:        ErrUnknownBodyParameter,
				},
			},
		},
		{
			name: "Valid POST params",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", Category: "Food"},
				url:         "/category?category=cookies",
				ct:          "application/x-www-form-urlencoded",
			},
			expectedErr:  nil,
			expectedResp: nil,
		},
		{
			name: "Valid POST unknown params",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate", Category: "Food"},
				url:         "/unknown",
				ct:          "application/x-www-form-urlencoded",
			},
			expectedErr: nil,
			expectedResp: []*RequestUnknownParameterError{
				{
					Parameters: []string{"subCategory", "category"},
					Err:        ErrUnknownBodyParameter,
				},
			},
		},
		{
			name: "Valid JSON unknown params",
			args: args{
				requestBody: &testRequestBody{SubCategory: "Chocolate"},
				url:         "/unknown",
				ct:          "application/json",
			},
			expectedErr: nil,
			expectedResp: []*RequestUnknownParameterError{
				{
					Parameters: []string{"subCategory"},
					Err:        ErrUnknownBodyParameter,
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
					if tc.args.requestBody.Category != "" {
						req.PostArgs().Add("category", tc.args.requestBody.Category)
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
			if tc.expectedResp != nil || len(tc.expectedResp) > 0 {
				assert.Equal(t, len(tc.expectedResp), len(upRes), "expect the number of unknown parameters: %t, got %t", len(tc.expectedResp), len(upRes))

				if isEq := reflect.DeepEqual(tc.expectedResp, upRes); !isEq {
					assert.Errorf(t, errors.New("got unexpected unknown parameters"), "expect unknown parameters: %v, got %v", tc.expectedResp, upRes)
				}
			}
		})
	}
}
