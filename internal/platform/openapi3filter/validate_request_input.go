package openapi3filter

import (
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"

	"github.com/wallarm/api-firewall/internal/platform/openapi3"
	"github.com/wallarm/api-firewall/internal/platform/routers"
)

// A ContentParameterDecoder takes a parameter definition from the swagger spec,
// and the value which we received for it. It is expected to return the
// value unmarshaled into an interface which can be traversed for
// validation, it should also return the schema to be used for validating the
// object, since there can be more than one in the content spec.
//
// If a query parameter appears multiple times, values[] will have more
// than one  value, but for all other parameter types it should have just
// one.
type ContentParameterDecoder func(param *openapi3.Parameter, values []string) (interface{}, *openapi3.Schema, error)

type RequestValidationInput struct {
	RequestCtx   *fasthttp.RequestCtx
	PathParams   map[string]string
	QueryParams  *fasthttp.Args
	Route        *routers.Route
	Options      *Options
	ParamDecoder ContentParameterDecoder
	ParserJson   *fastjson.ParserPool
}

func (input *RequestValidationInput) GetQueryParams() *fasthttp.Args {
	q := input.QueryParams
	if q == nil {
		q = input.RequestCtx.QueryArgs()
		input.QueryParams = q
	}
	return q
}
