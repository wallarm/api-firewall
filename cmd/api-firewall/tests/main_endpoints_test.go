package tests

import (
	"bytes"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"

	proxyMode "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/proxy"
	"github.com/wallarm/api-firewall/internal/config"
	proxyPool "github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/storage"
)

const openAPISpecEndpointsTest = `
openapi: 3.0.1
info:
  title: Service
  version: 1.0.0
servers:
  - url: /
paths:
  /api:
    get:
      parameters:
        - name: param
          in: query
          required: true
          schema:
            type: string
      responses:
        '200':
          description: Set cookies.
          content: {}
`

func TestEndpointConfig(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	dbSpec := storage.NewMockDBOpenAPILoader(mockCtrl)

	var lock sync.RWMutex

	serverUrl, err := url.ParseRequestURI("http://127.0.0.1:80")
	if err != nil {
		t.Fatalf("parsing API Host URL: %s", err.Error())
	}

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	logger = logger.Level(zerolog.DebugLevel)

	proxy := proxyPool.NewMockPool(mockCtrl)
	client := proxyPool.NewMockHTTPClient(mockCtrl)

	proxy.EXPECT().Get().Return(client, resolvedIP, nil).AnyTimes()
	client.EXPECT().Do(gomock.Any(), gomock.Any()).AnyTimes()
	proxy.EXPECT().Put(resolvedIP, client).Return(nil).AnyTimes()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	swagger, err := openapi3.NewLoader().LoadFromData([]byte(openAPISpecEndpointsTest))
	if err != nil {
		t.Fatalf("loading OpenAPI specification file: %s", err.Error())
	}

	dbSpec.EXPECT().SchemaIDs().Return([]int{}).AnyTimes()
	dbSpec.EXPECT().Specification(gomock.Any()).Return(swagger).AnyTimes()
	dbSpec.EXPECT().SpecificationVersion(gomock.Any()).Return("").AnyTimes()
	dbSpec.EXPECT().IsLoaded(gomock.Any()).Return(true).AnyTimes()
	dbSpec.EXPECT().IsReady().Return(true).AnyTimes()

	tests := []struct {
		name    string
		request struct {
			URI    string
			Method string
			Body   []byte
			CT     string
		}
		GlobalRequestValidation  string
		GlobalResponseValidation string
		endpoints                []config.Endpoint
		ExpectedStatusCode       int
	}{
		{
			name: "Valid no custom endpoints test",
			request: struct {
				URI    string
				Method string
				Body   []byte
				CT     string
			}{
				URI:    "/api",
				Method: "GET",
			},
			endpoints:                []config.Endpoint{},
			GlobalRequestValidation:  "LOG_ONLY",
			GlobalResponseValidation: "LOG_ONLY",
			ExpectedStatusCode:       200,
		},
		{
			name: "Valid single endpoint BLOCK",
			request: struct {
				URI    string
				Method string
				Body   []byte
				CT     string
			}{
				URI:    "/api",
				Method: "GET",
			},
			endpoints: []config.Endpoint{
				{
					ValidationMode: config.ValidationMode{
						RequestValidation:  "BLOCK",
						ResponseValidation: "BLOCK",
					},
					Path:   "/api",
					Method: "GET",
				},
			},
			GlobalRequestValidation:  "DISABLE",
			GlobalResponseValidation: "DISABLE",
			ExpectedStatusCode:       403,
		},
		{
			name: "Multiple invalid endpoints conf",
			request: struct {
				URI    string
				Method string
				Body   []byte
				CT     string
			}{
				URI:    "/api",
				Method: "GET",
			},
			endpoints: []config.Endpoint{
				{
					ValidationMode: config.ValidationMode{
						RequestValidation:  "BLOCK",
						ResponseValidation: "BLOCK",
					},
					Path:   "/api",
					Method: "POST",
				},
				{
					ValidationMode: config.ValidationMode{
						RequestValidation:  "BLOCK",
						ResponseValidation: "BLOCK",
					},
					Path:   "/apiInvalid/test",
					Method: "GET",
				},
			},
			GlobalRequestValidation:  "DISABLE",
			GlobalResponseValidation: "DISABLE",
			ExpectedStatusCode:       200,
		},
		{
			name: "Invalid URL in endpoint",
			endpoints: []config.Endpoint{
				{
					ValidationMode: config.ValidationMode{
						RequestValidation:  "DISABLE",
						ResponseValidation: "DISABLE",
					},
					Path:   "invalid-url",
					Method: "GET",
				},
			},
			request: struct {
				URI    string
				Method string
				Body   []byte
				CT     string
			}{
				URI:    "/api",
				Method: "GET",
			},
			GlobalRequestValidation:  "BLOCK",
			GlobalResponseValidation: "BLOCK",
			ExpectedStatusCode:       403,
		},
		{
			name: "Invalid validation mode",
			endpoints: []config.Endpoint{
				{
					ValidationMode: config.ValidationMode{
						RequestValidation:  "INVALID",
						ResponseValidation: "BLOCK",
					},
					Path:   "/",
					Method: "GET",
				},
			},
			GlobalRequestValidation:  "LOG_ONLY",
			GlobalResponseValidation: "LOG_ONLY",
			ExpectedStatusCode:       200,
		},
		{
			name: "Missing HTTP method",
			endpoints: []config.Endpoint{
				{
					ValidationMode: config.ValidationMode{
						RequestValidation:  "BLOCK",
						ResponseValidation: "BLOCK",
					},
					Path: "/api",
				},
			},
			request: struct {
				URI    string
				Method string
				Body   []byte
				CT     string
			}{
				URI:    "/api",
				Method: "GET",
			},
			GlobalRequestValidation:  "DISABLE",
			GlobalResponseValidation: "DISABLE",
			ExpectedStatusCode:       403,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var cfg = config.ProxyMode{
				CustomBlockStatusCode: 403,
				RequestValidation:     tt.GlobalRequestValidation,
				ResponseValidation:    tt.GlobalResponseValidation,
				Endpoints:             tt.endpoints,
			}

			handler := proxyMode.Handlers(&lock, &cfg, serverUrl, shutdown, logger, proxy, dbSpec, nil, nil, nil)

			req := fasthttp.AcquireRequest()
			req.SetRequestURI(tt.request.URI)
			req.Header.SetMethod(tt.request.Method)
			if tt.request.Body != nil {
				req.SetBodyStream(bytes.NewReader(tt.request.Body), -1)
			}
			if tt.request.CT != "" {
				req.Header.SetContentType(tt.request.CT)
			}

			resp := fasthttp.AcquireResponse()
			resp.SetStatusCode(fasthttp.StatusOK)

			reqCtx := fasthttp.RequestCtx{
				Request: *req,
			}

			handler(&reqCtx)

			if reqCtx.Response.StatusCode() != tt.ExpectedStatusCode {
				t.Errorf("Incorrect response status code. Expected: %d and got %d",
					tt.ExpectedStatusCode, reqCtx.Response.StatusCode())
			}

		})
	}
}
