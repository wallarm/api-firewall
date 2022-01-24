package openapi3filter

import (
	"bytes"
	"encoding/json"
	"github.com/valyala/fastjson"
	"testing"

	"github.com/savsgio/gotils/strconv"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"

	"github.com/wallarm/api-firewall/internal/platform/openapi3"
	"github.com/wallarm/api-firewall/internal/platform/router"
)

func TestValidatingRequestBodyWithReadOnlyProperty(t *testing.T) {
	const spec = `{
  "openapi": "3.0.3",
  "info": {
    "version": "1.0.0",
    "title": "title",
    "description": "desc",
    "contact": {
      "email": "email"
    }
  },
  "paths": {
    "/accounts": {
      "post": {
        "description": "Create a new account",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "required": ["_id"],
                "properties": {
                  "_id": {
                    "type": "string",
                    "description": "Unique identifier for this object.",
                    "pattern": "[0-9a-v]+$",
                    "minLength": 20,
                    "maxLength": 20,
                    "readOnly": true
                  }
                }
              }
            }
          }
        },
        "responses": {
          "201": {
            "description": "Successfully created a new account"
          },
          "400": {
            "description": "The server could not understand the request due to invalid syntax",
          }
        }
      }
    }
  }
}
`

	type Request struct {
		ID string `json:"_id"`
	}

	sl := openapi3.NewSwaggerLoader()
	doc, err := sl.LoadSwaggerFromData([]byte(spec))
	require.NoError(t, err)
	err = doc.Validate(sl.Context)
	require.NoError(t, err)

	b, err := json.Marshal(Request{ID: "bt6kdc3d0cvp6u8u3ft0"})
	require.NoError(t, err)

	swagRouter, err := router.NewRouter(doc)
	require.NoError(t, err)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/accounts")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(b), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	reqCtx := fasthttp.RequestCtx{
		Request:  *req,
		Response: *resp,
	}

	pathParamLength := 0
	if getOp := swagRouter.Routes[0].Route.PathItem.GetOperation("GET"); getOp != nil {
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

	err = ValidateRequest(sl.Context, &RequestValidationInput{
		RequestCtx: &reqCtx,
		ParserJson: &parserPool,
		PathParams: pathParams,
		Route:      swagRouter.Routes[0].Route,
	})
	require.NoError(t, err)
}
