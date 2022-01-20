package router

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"

	"github.com/wallarm/api-firewall/internal/platform/openapi3"
	"github.com/wallarm/api-firewall/internal/platform/routers"
)

// Router helps link http.Request.s and an OpenAPIv3 spec
type Router struct {
	Routes []Route
}

type Route struct {
	Route  *routers.Route
	Path   string
	Method string
}

// NewRouter creates a new router.
//
// If the given Swagger has servers, router will use them.
// All operations of the Swagger will be added to the router.
func NewRouter(doc *openapi3.Swagger) (*Router, error) {
	if err := doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("validating OpenAPI failed: %v", err)
	}
	var router Router

	for path, pathItem := range doc.Paths {
		for method, operation := range pathItem.Operations() {
			method = strings.ToUpper(method)
			route := routers.Route{
				Swagger:   doc,
				Path:      path,
				PathItem:  pathItem,
				Method:    method,
				Operation: operation,
			}
			router.Routes = append(router.Routes, Route{
				Route:  &route,
				Path:   path,
				Method: method,
			})
		}
	}
	return &router, nil
}

func NewReqB2S(ctx *fasthttp.RequestCtx) (*http.Request, error) {
	var r http.Request

	body := ctx.PostBody()
	r.Method = strconv.B2S(ctx.Method())
	r.Proto = strconv.B2S(ctx.Request.Header.Protocol())
	r.ProtoMajor = 1
	r.ProtoMinor = 1
	r.RequestURI = strconv.B2S(ctx.RequestURI())
	r.ContentLength = int64(len(body))
	r.Host = strconv.B2S(ctx.Host())
	r.RemoteAddr = ctx.RemoteAddr().String()

	hdr := make(http.Header)
	ctx.Request.Header.VisitAll(func(k, v []byte) {
		sk := strconv.B2S(k)
		sv := strconv.B2S(v)
		switch sk {
		case "Transfer-Encoding":
			r.TransferEncoding = append(r.TransferEncoding, sv)
		default:
			hdr.Set(sk, sv)
		}
	})
	r.Header = hdr
	r.Body = &netHTTPBody{body}
	rURL, err := url.ParseRequestURI(r.RequestURI)
	if err != nil {
		//ctx.Logger().Printf("cannot parse requestURI %q: %s", r.RequestURI, err)
		ctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		return nil, err
	}
	r.URL = rURL
	return &r, nil
}

type netHTTPBody struct {
	b []byte
}

func (r *netHTTPBody) Read(p []byte) (int, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.b)
	r.b = r.b[n:]
	return n, nil
}

func (r *netHTTPBody) Close() error {
	r.b = r.b[:0]
	return nil
}
