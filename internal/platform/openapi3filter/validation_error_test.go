package openapi3filter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/openapi3"
	"github.com/wallarm/api-firewall/internal/platform/routers"
)

func newPetstoreRequest(t *testing.T, method, path string, body io.Reader) *http.Request {
	host := "petstore.swagger.io"
	pathPrefix := "v2"
	r, err := http.NewRequest(method, fmt.Sprintf("http://%s/%s%s", host, pathPrefix, path), body)
	require.NoError(t, err)
	r.Header.Set(headerCT, "application/json")
	r.Header.Set("Authorization", "Bearer magicstring")
	r.Host = host
	return r
}

type validationFields struct {
	Handler      http.Handler
	SwaggerFile  string
	ErrorEncoder ErrorEncoder
}
type validationArgs struct {
	r *http.Request
}
type validationTest struct {
	name                      string
	fields                    validationFields
	args                      validationArgs
	wantErr                   bool
	wantErrBody               string
	wantErrReason             string
	wantErrSchemaReason       string
	wantErrSchemaPath         string
	wantErrSchemaValue        interface{}
	wantErrSchemaOriginReason string
	wantErrSchemaOriginPath   string
	wantErrSchemaOriginValue  interface{}
	wantErrParam              string
	wantErrParamIn            string
	wantErrParseKind          ParseErrorKind
	wantErrParseValue         interface{}
	wantErrParseReason        string
	//wantErrResponse           *ValidationError
	wantErrResponse interface{}
}

func getValidationTests(t *testing.T) []*validationTest {
	badHost, _ := http.NewRequest(http.MethodGet, "http://unknown-host.com/v2/pet", nil)
	badPath := newPetstoreRequest(t, http.MethodGet, "/watdis", nil)
	badMethod := newPetstoreRequest(t, http.MethodTrace, "/pet", nil)

	missingBody1 := newPetstoreRequest(t, http.MethodPost, "/pet", nil)
	missingBody2 := newPetstoreRequest(t, http.MethodPost, "/pet", bytes.NewBufferString(``))

	noContentType := newPetstoreRequest(t, http.MethodPost, "/pet", bytes.NewBufferString(`{}`))
	noContentType.Header.Del(headerCT)

	noContentTypeNeeded := newPetstoreRequest(t, http.MethodGet, "/pet/findByStatus?status=sold", nil)
	noContentTypeNeeded.Header.Del(headerCT)

	unknownContentType := newPetstoreRequest(t, http.MethodPost, "/pet", bytes.NewBufferString(`{}`))
	unknownContentType.Header.Set(headerCT, "application/xml")

	unsupportedContentType := newPetstoreRequest(t, http.MethodPost, "/pet", bytes.NewBufferString(`{}`))
	unsupportedContentType.Header.Set(headerCT, "text/plain")

	unsupportedHeaderValue := newPetstoreRequest(t, http.MethodPost, "/pet", bytes.NewBufferString(`{}`))
	unsupportedHeaderValue.Header.Set("x-environment", "watdis")

	return []*validationTest{
		//
		// Basics
		//

		{
			name: "error - unknown host",
			args: validationArgs{
				r: badHost,
			},
			wantErrReason:   routers.ErrPathNotFound.Error(),
			wantErrResponse: &ValidationError{Status: http.StatusNotFound, Title: routers.ErrPathNotFound.Error()},
			//wantErrResponse: &routers.RouteError{Reason: routers.ErrPathNotFound.Error()},
		},
		{
			name: "error - unknown path",
			args: validationArgs{
				r: badPath,
			},
			wantErrReason:   routers.ErrPathNotFound.Error(),
			wantErrResponse: &ValidationError{Status: http.StatusNotFound, Title: routers.ErrPathNotFound.Error()},
			//wantErrResponse: &routers.RouteError{Reason: routers.ErrPathNotFound.Error()},
		},
		{
			name: "error - unknown method",
			args: validationArgs{
				r: badMethod,
			},
			wantErrReason: routers.ErrMethodNotAllowed.Error(),
			// TODO: By HTTP spec, this should have an Allow header with what is allowed
			// but kin-openapi doesn't provide us the requested method or path, so impossible to provide details
			wantErrResponse: &ValidationError{Status: http.StatusMethodNotAllowed, Title: routers.ErrMethodNotAllowed.Error()},
			//wantErrResponse: &routers.RouteError{Reason: routers.ErrMethodNotAllowed.Error()},
		},
		{
			name: "error - missing body on POST",
			args: validationArgs{
				r: missingBody1,
			},
			wantErrBody: "request body has an error: " + ErrInvalidRequired.Error(),
			wantErrResponse: &ValidationError{Status: http.StatusBadRequest,
				Title: "request body has an error: " + ErrInvalidRequired.Error()},
		},
		{
			name: "error - empty body on POST",
			args: validationArgs{
				r: missingBody2,
			},
			wantErrBody: "request body has an error: " + ErrInvalidRequired.Error(),
			wantErrResponse: &ValidationError{Status: http.StatusBadRequest,
				Title: "request body has an error: " + ErrInvalidRequired.Error()},
		},

		//
		// Content-Type
		//

		{
			name: "error - missing content-type on POST",
			args: validationArgs{
				r: noContentType,
			},
			wantErrReason: prefixInvalidCT + ` ""`,
			wantErrResponse: &ValidationError{Status: http.StatusUnsupportedMediaType,
				Title: "header Content-Type is required"},
		},
		{
			name: "error - unsupported content-type on POST",
			args: validationArgs{
				r: unsupportedContentType,
			},
			wantErrReason: prefixInvalidCT + ` "text/plain"`,
			wantErrResponse: &ValidationError{Status: http.StatusUnsupportedMediaType,
				Title: prefixUnsupportedCT + ` "text/plain"`},
		},
		{
			name: "success - no content-type header required on GET",
			args: validationArgs{
				r: noContentTypeNeeded,
			},
		},

		//
		// Query strings
		//

		{
			name: "error - missing required query string parameter",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodGet, "/pet/findByStatus", nil),
			},
			wantErrParam:   "status",
			wantErrParamIn: "query",
			wantErrReason:  ErrInvalidRequired.Error(),
			wantErrResponse: &ValidationError{Status: http.StatusBadRequest,
				Title: `parameter "status" in query is required`},
		},
		{
			name: "error - wrong query string parameter type",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodGet, "/pet/findByIds?ids=1,notAnInt", nil),
			},
			wantErrParam:   "ids",
			wantErrParamIn: "query",
			// This is a nested ParseError. The outer error is a KindOther with no details.
			// So we'd need to look at the inner one which is a KindInvalidFormat. So just check the error body.
			wantErrBody: `parameter "ids" in query has an error: path 1: value notAnInt: an invalid integer: ` +
				"strconv.ParseFloat: parsing \"notAnInt\": invalid syntax",
			// TODO: Should we treat query params of the wrong type like a 404 instead of a 400?
			wantErrResponse: &ValidationError{Status: http.StatusBadRequest,
				Title: `parameter "ids" in query is invalid: notAnInt is an invalid integer`},
		},
		{
			name: "success - ignores unknown query string parameter",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodGet, "/pet/findByStatus?wat=isdis", nil),
			},
		},
		{
			name: "success - normal case, query strings",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodGet, "/pet/findByStatus?status=available", nil),
			},
		},
		{
			name: "success - normal case, query strings, array",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodGet, "/pet/findByStatus?status=available&status=sold", nil),
			},
		},
		{
			name: "error - invalid query string array serialization",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodGet, "/pet/findByStatus?status=available,sold", nil),
			},
			wantErrParam:        "status",
			wantErrParamIn:      "query",
			wantErrSchemaReason: "value is not one of the allowed values",
			wantErrSchemaPath:   "/0",
			wantErrSchemaValue:  "available,sold",
			wantErrResponse: &ValidationError{Status: http.StatusBadRequest,
				Title: "value is not one of the allowed values",
				Detail: "value available,sold at /0 must be one of: available, pending, sold; " +
					// TODO: do we really want to use this heuristic to guess
					//  that they're using the wrong serialization?
					"perhaps you intended '?status=available&status=sold'",
				Source: &ValidationErrorSource{Parameter: "status"}},
		},
		{
			name: "error - invalid enum value for query string parameter",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodGet, "/pet/findByStatus?status=sold&status=watdis", nil),
			},
			wantErrParam:        "status",
			wantErrParamIn:      "query",
			wantErrSchemaReason: "value is not one of the allowed values",
			wantErrSchemaPath:   "/1",
			wantErrSchemaValue:  "watdis",
			wantErrResponse: &ValidationError{Status: http.StatusBadRequest,
				Title:  "value is not one of the allowed values",
				Detail: "value watdis at /1 must be one of: available, pending, sold",
				Source: &ValidationErrorSource{Parameter: "status"}},
		},
		{
			name: "error - invalid enum value, allowing commas (without 'perhaps you intended' recommendation)",
			args: validationArgs{
				// fish,with,commas isn't an enum value
				r: newPetstoreRequest(t, http.MethodGet, "/pet/findByKind?kind=dog|fish,with,commas", nil),
			},
			wantErrParam:        "kind",
			wantErrParamIn:      "query",
			wantErrSchemaReason: "value is not one of the allowed values",
			wantErrSchemaPath:   "/1",
			wantErrSchemaValue:  "fish,with,commas",
			wantErrResponse: &ValidationError{Status: http.StatusBadRequest,
				Title:  "value is not one of the allowed values",
				Detail: "value fish,with,commas at /1 must be one of: dog, cat, turtle, bird,with,commas",
				// No 'perhaps you intended' because its the right serialization format
				Source: &ValidationErrorSource{Parameter: "kind"}},
		},
		{
			name: "success - valid enum value, allowing commas",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodGet, "/pet/findByKind?kind=dog|bird,with,commas", nil),
			},
		},

		//
		// Request header params
		//
		{
			name: "error - invalid enum value for header string parameter",
			args: validationArgs{
				r: unsupportedHeaderValue,
			},
			wantErrParam:        "x-environment",
			wantErrParamIn:      "header",
			wantErrSchemaReason: "value is not one of the allowed values",
			wantErrSchemaPath:   "/",
			wantErrSchemaValue:  "watdis",
			wantErrResponse: &ValidationError{Status: http.StatusBadRequest,
				Title:  "value is not one of the allowed values",
				Detail: "value watdis at / must be one of: demo, prod",
				Source: &ValidationErrorSource{Parameter: "x-environment"}},
		},

		//
		// Request bodies
		//

		{
			name: "error - invalid enum value for header object attribute",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodPost, "/pet", bytes.NewBufferString(`{"status":"watdis"}`)),
			},
			wantErrReason:       "doesn't match the schema",
			wantErrSchemaReason: "value is not one of the allowed values",
			wantErrSchemaValue:  "\"watdis\"",
			wantErrSchemaPath:   "/status",
			wantErrResponse: &ValidationError{Status: http.StatusUnprocessableEntity,
				Title:  "value is not one of the allowed values",
				Detail: "value \"watdis\" at /status must be one of: available, pending, sold",
				Source: &ValidationErrorSource{Pointer: "/status"}},
		},
		{
			name: "error - missing required object attribute",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodPost, "/pet", bytes.NewBufferString(`{"name":"Bahama"}`)),
			},
			wantErrReason:       "doesn't match the schema",
			wantErrSchemaReason: `property "photoUrls" is missing`,
			//wantErrSchemaValue:  map[string]string{"name": "Bahama"},
			wantErrSchemaValue: "{\"name\":\"Bahama\"}",
			wantErrSchemaPath:  "/photoUrls",
			wantErrResponse: &ValidationError{Status: http.StatusUnprocessableEntity,
				Title:  `property "photoUrls" is missing`,
				Source: &ValidationErrorSource{Pointer: "/photoUrls"}},
		},
		{
			name: "error - missing required nested object attribute",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodPost, "/pet",
					bytes.NewBufferString(`{"name":"Bahama","photoUrls":[],"category":{}}`)),
			},
			wantErrReason:       "doesn't match the schema",
			wantErrSchemaReason: `property "name" is missing`,
			//wantErrSchemaValue:  map[string]string{},
			wantErrSchemaValue: "{}",
			wantErrSchemaPath:  "/category/name",
			wantErrResponse: &ValidationError{Status: http.StatusUnprocessableEntity,
				Title:  `property "name" is missing`,
				Source: &ValidationErrorSource{Pointer: "/category/name"}},
		},
		{
			name: "error - missing required deeply nested object attribute",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodPost, "/pet",
					bytes.NewBufferString(`{"name":"Bahama","photoUrls":[],"category":{"tags": [{}]}}`)),
			},
			wantErrReason:       "doesn't match the schema",
			wantErrSchemaReason: `property "name" is missing`,
			//wantErrSchemaValue:  map[string]string{},
			wantErrSchemaValue: "{}",
			wantErrSchemaPath:  "/category/tags/0/name",
			wantErrResponse: &ValidationError{Status: http.StatusUnprocessableEntity,
				Title:  `property "name" is missing`,
				Source: &ValidationErrorSource{Pointer: "/category/tags/0/name"}},
		},
		{
			name: "error - wrong attribute type",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodPost, "/pet",
					bytes.NewBufferString(`{"name":"Bahama","photoUrls":"http://cat"}`)),
			},
			wantErrReason:       "doesn't match the schema",
			wantErrSchemaReason: "Field must be set to array or not be present",
			wantErrSchemaPath:   "/photoUrls",
			wantErrSchemaValue:  "string",
			// TODO: this shouldn't say "or not be present", but this requires recursively resolving
			//  innerErr.JSONPointer() against e.RequestBody.Content["application/json"].Schema.Value (.Required, .Properties)
			wantErrResponse: &ValidationError{Status: http.StatusUnprocessableEntity,
				Title:  "Field must be set to array or not be present",
				Source: &ValidationErrorSource{Pointer: "/photoUrls"}},
		},
		{
			name: "error - missing required object attribute from allOf required overlay",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodPost, "/pet2", bytes.NewBufferString(`{"name":"Bahama"}`)),
			},
			wantErrReason:             "doesn't match the schema",
			wantErrSchemaPath:         "/",
			wantErrSchemaValue:        "{\"name\":\"Bahama\"}",
			wantErrSchemaOriginReason: `property "photoUrls" is missing`,
			wantErrSchemaOriginValue:  "{\"name\":\"Bahama\"}",
			wantErrSchemaOriginPath:   "/photoUrls",
			wantErrResponse: &ValidationError{Status: http.StatusUnprocessableEntity,
				Title:  `property "photoUrls" is missing`,
				Source: &ValidationErrorSource{Pointer: "/photoUrls"}},
		},
		{
			name: "success - ignores unknown object attribute",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodPost, "/pet",
					bytes.NewBufferString(`{"wat":"isdis","name":"Bahama","photoUrls":[]}`)),
			},
		},
		{
			name: "success - normal case, POST",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodPost, "/pet",
					bytes.NewBufferString(`{"name":"Bahama","photoUrls":[]}`)),
			},
		},
		{
			name: "success - required properties are not required on PATCH if required overlaid using allOf elsewhere",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodPatch, "/pet", bytes.NewBufferString(`{}`)),
			},
		},

		//
		// Path params
		//

		{
			name: "error - missing path param",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodGet, "/pet/", nil),
			},
			wantErrParam:   "petId",
			wantErrParamIn: "path",
			wantErrReason:  ErrInvalidRequired.Error(),
			wantErrResponse: &ValidationError{Status: http.StatusBadRequest,
				Title: `parameter "petId" in path is required`},
		},
		{
			name: "error - wrong path param type",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodGet, "/pet/NotAnInt", nil),
			},
			wantErrParam:       "petId",
			wantErrParamIn:     "path",
			wantErrParseKind:   KindInvalidFormat,
			wantErrParseValue:  "NotAnInt",
			wantErrParseReason: "an invalid integer",
			wantErrResponse: &ValidationError{Status: http.StatusNotFound,
				Title: `resource not found with "petId" value: NotAnInt`},
		},
		{
			name: "success - normal case, with path params",
			args: validationArgs{
				r: newPetstoreRequest(t, http.MethodGet, "/pet/23", nil),
			},
		},
	}
}

func TestValidationHandler_validateRequest(t *testing.T) {
	tests := getValidationTests(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			h, err := buildValidationHandler(tt)
			req.NoError(err)

			httpReq := fasthttp.AcquireRequest()
			httpReq.SetRequestURI(tt.args.r.URL.String())
			httpReq.Header.SetMethod(tt.args.r.Method)

			if tt.args.r.ContentLength > 0 {
				httpReq.SetBodyStream(tt.args.r.Body, -1)
			}

			if tt.args.r.Header.Get(headerCT) != "" {
				httpReq.Header.SetContentType(tt.args.r.Header.Get(headerCT))
			}

			if tt.args.r.Header.Get("x-environment") != "" {
				httpReq.Header.Set("x-environment", tt.args.r.Header.Get("x-environment"))
			}

			if tt.args.r.Header.Get("Authorization") != "" {
				httpReq.Header.Set("Authorization", tt.args.r.Header.Get("Authorization"))
			}

			httpResp := fasthttp.AcquireResponse()
			reqCtx := fasthttp.RequestCtx{
				Request:  *httpReq,
				Response: *httpResp,
			}

			err = h.validateRequest(&reqCtx)
			req.Equal(tt.wantErr, err != nil)

			if err != nil {
				if tt.wantErrBody != "" {
					req.Equal(tt.wantErrBody, err.Error())
				}

				if e, ok := err.(*routers.RouteError); ok {
					req.Equal(tt.wantErrReason, e.Error())
					return
				}

				e, ok := err.(*RequestError)
				req.True(ok, "not a RequestError: %T -- %#v", err, err)

				req.Equal(tt.wantErrReason, e.Reason)

				if e.Parameter != nil {
					req.Equal(tt.wantErrParam, e.Parameter.Name)
					req.Equal(tt.wantErrParamIn, e.Parameter.In)
				} else {
					req.False(tt.wantErrParam != "" || tt.wantErrParamIn != "",
						"error = %v, no Parameter -- %#v", e, e)
				}

				if innerErr, ok := e.Err.(*openapi3.SchemaError); ok {
					req.Equal(tt.wantErrSchemaReason, innerErr.Reason)
					pointer := toJSONPointer(innerErr.JSONPointer())
					req.Equal(tt.wantErrSchemaPath, pointer)
					req.Equal(fmt.Sprintf("%v", tt.wantErrSchemaValue), fmt.Sprintf("%v", innerErr.Value))

					if originErr, ok := innerErr.Origin.(*openapi3.SchemaError); ok {
						req.Equal(tt.wantErrSchemaOriginReason, originErr.Reason)
						pointer := toJSONPointer(originErr.JSONPointer())
						req.Equal(tt.wantErrSchemaOriginPath, pointer)
						req.Equal(fmt.Sprintf("%v", tt.wantErrSchemaOriginValue), fmt.Sprintf("%v", originErr.Value))
					}
				} else {
					req.False(tt.wantErrSchemaReason != "" || tt.wantErrSchemaPath != "",
						"error = %v, not a SchemaError -- %#v", e.Err, e.Err)
					req.False(tt.wantErrSchemaOriginReason != "" || tt.wantErrSchemaOriginPath != "",
						"error = %v, not a SchemaError with Origin -- %#v", e.Err, e.Err)
				}

				if innerErr, ok := e.Err.(*ParseError); ok {
					req.Equal(tt.wantErrParseKind, innerErr.Kind)
					req.Equal(tt.wantErrParseValue, innerErr.Value)
					req.Equal(tt.wantErrParseReason, innerErr.Reason)
				} else {
					req.False(tt.wantErrParseValue != nil || tt.wantErrParseReason != "",
						"error = %v, not a ParseError -- %#v", e.Err, e.Err)
				}
			}
		})
	}
}

func TestValidationErrorEncoder(t *testing.T) {
	tests := getValidationTests(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEncoder := &mockErrorEncoder{}
			encoder := &ValidationErrorEncoder{Encoder: mockEncoder.Encode}

			req := require.New(t)

			h, err := buildValidationHandler(tt)
			req.NoError(err)

			httpReq := fasthttp.AcquireRequest()
			httpReq.SetRequestURI(tt.args.r.URL.String())
			httpReq.Header.SetMethod(tt.args.r.Method)

			if tt.args.r.ContentLength > 0 {
				httpReq.SetBodyStream(tt.args.r.Body, -1)
			}

			if tt.args.r.Header.Get(headerCT) != "" {
				httpReq.Header.SetContentType(tt.args.r.Header.Get(headerCT))
			}

			if tt.args.r.Header.Get("x-environment") != "" {
				httpReq.Header.Set("x-environment", tt.args.r.Header.Get("x-environment"))
			}

			if tt.args.r.Header.Get("Authorization") != "" {
				httpReq.Header.Set("Authorization", tt.args.r.Header.Get("Authorization"))
			}

			httpResp := fasthttp.AcquireResponse()
			reqCtx := fasthttp.RequestCtx{
				Request:  *httpReq,
				Response: *httpResp,
			}

			err = h.validateRequest(&reqCtx)
			req.Equal(tt.wantErr, err != nil)

			if err != nil {
				encoder.Encode(tt.args.r.Context(), err, httptest.NewRecorder())
				if tt.wantErrResponse != mockEncoder.Err {
					req.Equal(tt.wantErrResponse, mockEncoder.Err)
				}
			}
		})
	}
}

func buildValidationHandler(tt *validationTest) (*ValidationHandler, error) {
	if tt.fields.SwaggerFile == "" {
		tt.fields.SwaggerFile = "fixtures/petstore.json"
	}
	h := &ValidationHandler{
		Handler:      tt.fields.Handler,
		SwaggerFile:  tt.fields.SwaggerFile,
		ErrorEncoder: tt.fields.ErrorEncoder,
	}
	tt.wantErr = tt.wantErr ||
		(tt.wantErrBody != "") ||
		(tt.wantErrReason != "") ||
		(tt.wantErrSchemaReason != "") ||
		(tt.wantErrSchemaPath != "") ||
		(tt.wantErrParseValue != nil) ||
		(tt.wantErrParseReason != "")
	err := h.Load()
	return h, err
}

type mockErrorEncoder struct {
	Called bool
	Ctx    context.Context
	Err    error
	W      http.ResponseWriter
}

func (e *mockErrorEncoder) Encode(ctx context.Context, err error, w http.ResponseWriter) {
	e.Called = true
	e.Ctx = ctx
	e.Err = err
	e.W = w
}
