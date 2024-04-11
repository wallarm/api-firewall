package chi

import (
	"bytes"
	"fmt"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/web"
	"io"
	"net/http"
	"testing"
)

func TestMuxBasic(t *testing.T) {

	cxindex := func(ctx *fasthttp.RequestCtx) error {
		ctx.SetStatusCode(200)
		ctx.SetBody([]byte("hi peter"))
		return nil
	}

	ping := func(ctx *fasthttp.RequestCtx) error {
		ctx.SetStatusCode(200)
		ctx.SetBody([]byte("."))
		return nil
	}

	headPing := func(ctx *fasthttp.RequestCtx) error {
		ctx.Response.Header.Set("X-Ping", "1")
		ctx.SetStatusCode(200)
		return nil
	}

	createPing := func(ctx *fasthttp.RequestCtx) error {
		// create ....
		ctx.SetStatusCode(201)
		return nil
	}

	pingAll := func(ctx *fasthttp.RequestCtx) error {
		ctx.SetStatusCode(200)
		ctx.SetBody([]byte("ping all"))
		return nil
	}

	pingAll2 := func(ctx *fasthttp.RequestCtx) error {
		ctx.SetStatusCode(200)
		ctx.SetBody([]byte("ping all2"))
		return nil
	}

	pingOne := func(ctx *fasthttp.RequestCtx) error {
		ctx.SetStatusCode(200)
		ctx.SetBody([]byte("ping one id: " + URLParam(ctx, "id")))
		return nil
	}

	pingWoop := func(ctx *fasthttp.RequestCtx) error {
		ctx.SetStatusCode(200)
		ctx.SetBody([]byte("woop." + URLParam(ctx, "iidd")))
		return nil
	}

	catchAll := func(ctx *fasthttp.RequestCtx) error {
		ctx.SetStatusCode(200)
		ctx.SetBody([]byte("catchall"))
		return nil
	}

	m := NewRouter()
	m.AddEndpoint("GET", "/", cxindex)
	m.AddEndpoint("GET", "/ping", ping)

	m.AddEndpoint("GET", "/pingall", pingAll)
	m.AddEndpoint("get", "/ping/all", pingAll)
	m.AddEndpoint("GET", "/ping/all2", pingAll2)
	m.AddEndpoint("HEAD", "/ping", headPing)
	m.AddEndpoint("POST", "/ping", createPing)
	m.AddEndpoint("GET", "/ping/{id}", pingWoop)
	m.AddEndpoint("POST", "/ping/{id}", pingOne)
	m.AddEndpoint("GET", "/ping/{iidd}/woop", pingWoop)
	m.AddEndpoint("POST", "/admin/*", catchAll)

	// GET /
	if _, body := testRequest(t, m, "GET", "/", nil); body != "hi peter" {
		t.Fatalf(body)
	}

	// GET /ping
	if _, body := testRequest(t, m, "GET", "/ping", nil); body != "." {
		t.Fatalf(body)
	}

	// GET /pingall
	if _, body := testRequest(t, m, "GET", "/pingall", nil); body != "ping all" {
		t.Fatalf(body)
	}

	// GET /ping/all
	if _, body := testRequest(t, m, "GET", "/ping/all", nil); body != "ping all" {
		t.Fatalf(body)
	}

	// GET /ping/all2
	if _, body := testRequest(t, m, "GET", "/ping/all2", nil); body != "ping all2" {
		t.Fatalf(body)
	}

	// POST /ping/123
	if _, body := testRequest(t, m, "POST", "/ping/123", nil); body != "ping one id: 123" {
		t.Fatalf(body)
	}

	// GET /ping/allan
	if _, body := testRequest(t, m, "POST", "/ping/allan", nil); body != "ping one id: allan" {
		t.Fatalf(body)
	}

	// GET /ping/1/woop
	if _, body := testRequest(t, m, "GET", "/ping/1/woop", nil); body != "woop.1" {
		t.Fatalf(body)
	}

	if status, _ := testRequest(t, m, "HEAD", "/ping", nil); status != 200 {
		t.Fatal("wrong status code")
	}

	// GET /admin/catch-this
	if status, body := testRequest(t, m, "GET", "/admin/catch-thazzzzz", nil); body != "" && status != 0 {
		t.Fatalf("method not found failed")
	}

	// POST /admin/catch-this
	if _, body := testRequest(t, m, "POST", "/admin/casdfsadfs", bytes.NewReader([]byte{})); body != "catchall" {
		t.Fatalf(body)
	}
}

func TestMuxHandlePatternValidation(t *testing.T) {
	testCases := []struct {
		name           string
		pattern        string
		shouldPanic    bool
		method         string // Method to be used for the test request
		path           string // Path to be used for the test request
		expectedBody   string // Expected response body
		expectedStatus int    // Expected HTTP status code
	}{
		// Valid patterns
		{
			name:           "Valid pattern without HTTP GET",
			pattern:        "/user/{id}",
			shouldPanic:    false,
			method:         "GET",
			path:           "/user/123",
			expectedBody:   "without-prefix GET",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Valid pattern with HTTP POST",
			pattern:        "POST /products/{id}",
			shouldPanic:    false,
			method:         "POST",
			path:           "/products/456",
			expectedBody:   "with-prefix POST",
			expectedStatus: http.StatusOK,
		},
		// Invalid patterns
		{
			name:        "Invalid pattern with no method",
			pattern:     "INVALID/user/{id}",
			shouldPanic: true,
		},
		{
			name:        "Invalid pattern with supported method",
			pattern:     "GET/user/{id}",
			shouldPanic: true,
		},
		{
			name:        "Invalid pattern with unsupported method",
			pattern:     "UNSUPPORTED /unsupported-method",
			shouldPanic: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil && !tc.shouldPanic {
					t.Errorf("Unexpected panic for pattern %s:\n%v", tc.pattern, r)
				}
			}()

			r := NewRouter()
			r.AddEndpoint(tc.method, tc.path, func(ctx *fasthttp.RequestCtx) error {
				ctx.SetStatusCode(200)
				ctx.SetBody([]byte(tc.expectedBody))
				return nil
			})

			if !tc.shouldPanic {
				statusCode, body := testRequest(t, r, tc.method, tc.path, nil)
				if body != tc.expectedBody || statusCode != tc.expectedStatus {
					t.Errorf("Expected status %d and body %s; got status %d and body %s for pattern %s",
						tc.expectedStatus, tc.expectedBody, statusCode, body, tc.pattern)
				}
			}
		})
	}
}

func TestMuxEmptyParams(t *testing.T) {
	r := NewRouter()
	if err := r.AddEndpoint("GET", "/users/{x}/{y}/{z}", func(ctx *fasthttp.RequestCtx) error {
		x := URLParam(ctx, "x")
		y := URLParam(ctx, "y")
		z := URLParam(ctx, "z")
		ctx.SetBody([]byte(fmt.Sprintf("%s-%s-%s", x, y, z)))

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, body := testRequest(t, r, "GET", "/users/a/b/c", nil); body != "a-b-c" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, r, "GET", "/users///c", nil); body != "--c" {
		t.Fatalf(body)
	}
}

func TestMuxWildcardRoute(t *testing.T) {
	handler := func(ctx *fasthttp.RequestCtx) error { return nil }

	r := NewRouter()
	if err := r.AddEndpoint("GET", "/*/wildcard/must/be/at/end", handler); err == nil {
		t.Fatal("expected error")
	}
}

func TestMuxWildcardRouteCheckTwo(t *testing.T) {
	handler := func(ctx *fasthttp.RequestCtx) error { return nil }

	r := NewRouter()
	if err := r.AddEndpoint("GET", "/*/wildcard/{must}/be/at/end", handler); err == nil {
		t.Fatal("expected error")
	}

}

func TestMuxRegexp(t *testing.T) {
	r := NewRouter()

	if err := r.AddEndpoint("GET", "/{param:[0-9]*}/test", func(ctx *fasthttp.RequestCtx) error {
		ctx.SetBody([]byte(fmt.Sprintf("Hi: %s", URLParam(ctx, "param"))))
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, body := testRequest(t, r, "GET", "//test", nil); body != "Hi: " {
		t.Fatal(body)
	}
}

func TestMuxRegexp2(t *testing.T) {
	r := NewRouter()
	if err := r.AddEndpoint("GET", "/foo-{suffix:[a-z]{2,3}}.json", func(ctx *fasthttp.RequestCtx) error {
		ctx.SetBody([]byte(URLParam(ctx, "suffix")))
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, body := testRequest(t, r, "GET", "/foo-.json", nil); body != "" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, r, "GET", "/foo-abc.json", nil); body != "abc" {
		t.Fatalf(body)
	}
}

func TestMuxRegexp3(t *testing.T) {
	r := NewRouter()
	if err := r.AddEndpoint("GET", "/one/{firstId:[a-z0-9-]+}/{secondId:[a-z]+}/first", func(ctx *fasthttp.RequestCtx) error {
		ctx.SetBody([]byte("first"))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.AddEndpoint("GET", "/one/{firstId:[a-z0-9-_]+}/{secondId:[0-9]+}/second", func(ctx *fasthttp.RequestCtx) error {
		ctx.SetBody([]byte("second"))
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := r.AddEndpoint("DELETE", "/one/{firstId:[a-z0-9-_]+}/{secondId:[0-9]+}/second", func(ctx *fasthttp.RequestCtx) error {
		ctx.SetBody([]byte("third"))
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, body := testRequest(t, r, "GET", "/one/hello/peter/first", nil); body != "first" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, r, "GET", "/one/hithere/123/second", nil); body != "second" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, r, "DELETE", "/one/hithere/123/second", nil); body != "third" {
		t.Fatalf(body)
	}
}

func TestMuxSubrouterWildcardParam(t *testing.T) {
	h := web.Handler(func(ctx *fasthttp.RequestCtx) error {
		ctx.SetBody([]byte(fmt.Sprintf("param:%v *:%v", URLParam(ctx, "param"), URLParam(ctx, "*"))))
		return nil
	})

	r := NewRouter()

	if err := r.AddEndpoint("GET", "/bare/{param}", h); err != nil {
		t.Fatal(err)
	}
	if err := r.AddEndpoint("GET", "/bare/{param}/*", h); err != nil {
		t.Fatal(err)
	}

	if err := r.AddEndpoint("GET", "/case0/{param}", h); err != nil {
		t.Fatal(err)
	}
	if err := r.AddEndpoint("GET", "/case0/{param}/*", h); err != nil {
		t.Fatal(err)
	}

	if _, body := testRequest(t, r, "GET", "/bare/hi", nil); body != "param:hi *:" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, r, "GET", "/bare/hi/yes", nil); body != "param:hi *:yes" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, r, "GET", "/case0/hi", nil); body != "param:hi *:" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, r, "GET", "/case0/hi/yes", nil); body != "param:hi *:yes" {
		t.Fatalf(body)
	}
}

func TestEscapedURLParams(t *testing.T) {
	m := NewRouter()
	if err := m.AddEndpoint("GET", "/api/{identifier}/{region}/{size}/{rotation}/*", func(ctx *fasthttp.RequestCtx) error {
		ctx.SetStatusCode(200)
		rctx := RouteContext(ctx)
		if rctx == nil {
			t.Error("no context")
			return nil
		}
		identifier := URLParam(ctx, "identifier")
		if identifier != "http:%2f%2fexample.com%2fimage.png" {
			t.Errorf("identifier path parameter incorrect %s", identifier)
			return nil
		}
		region := URLParam(ctx, "region")
		if region != "full" {
			t.Errorf("region path parameter incorrect %s", region)
			return nil
		}
		size := URLParam(ctx, "size")
		if size != "max" {
			t.Errorf("size path parameter incorrect %s", size)
			return nil
		}
		rotation := URLParam(ctx, "rotation")
		if rotation != "0" {
			t.Errorf("rotation path parameter incorrect %s", rotation)
			return nil
		}
		ctx.SetBody([]byte("success"))
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, body := testRequest(t, m, "GET", "/api/http:%2f%2fexample.com%2fimage.png/full/max/0/color.png", nil); body != "success" {
		t.Fatalf(body)
	}
}

func TestCustomHTTPMethod(t *testing.T) {
	// first we must register this method to be accepted, then we
	// can define method handlers on the router below
	if err := RegisterMethod("BOO"); err != nil {
		t.Fatal(err)
	}

	r := NewRouter()
	if err := r.AddEndpoint("GET", "/", func(ctx *fasthttp.RequestCtx) error {
		ctx.SetBody([]byte("."))
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// note the custom BOO method for route /hi
	if err := r.AddEndpoint("BOO", "/hi", func(ctx *fasthttp.RequestCtx) error {
		ctx.SetBody([]byte("custom method"))
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if _, body := testRequest(t, r, "GET", "/", nil); body != "." {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, r, "BOO", "/hi", nil); body != "custom method" {
		t.Fatalf(body)
	}
}

func testRequest(t *testing.T, mux *Mux, method, path string, body io.Reader) (int, string) {

	rctx := NewRouteContext()
	handler := mux.Find(rctx, method, path)

	if handler == nil {
		return 0, ""
	}

	req := fasthttp.AcquireRequest()
	req.SetRequestURI(path)
	req.Header.SetMethod(method)

	if body != nil {
		reqBody, err := io.ReadAll(body)
		if err != nil {
			t.Fatal(err)
			return 0, ""
		}
		req.SetBody(reqBody)
	}

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	// add url params
	reqCtx.SetUserValue(RouteCtxKey, rctx)

	if err := handler(&reqCtx); err != nil {
		t.Fatal(err)
		return 0, ""
	}

	return reqCtx.Response.StatusCode(), string(reqCtx.Response.Body())
}

type ctxKey struct {
	name string
}

func (k ctxKey) String() string {
	return "context value " + k.name
}

//func BenchmarkMux(b *testing.B) {
//	h1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
//	h2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
//	h3 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
//	h4 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
//	h5 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
//	h6 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
//
//	mx := NewRouter()
//	mx.Get("/", h1)
//	mx.Get("/hi", h2)
//	mx.Post("/hi-post", h2) // used to benchmark 405 responses
//	mx.Get("/sup/{id}/and/{this}", h3)
//	mx.Get("/sup/{id}/{bar:foo}/{this}", h3)
//
//	mx.Route("/sharing/{x}/{hash}", func(mx Router) {
//		mx.Get("/", h4)          // subrouter-1
//		mx.Get("/{network}", h5) // subrouter-1
//		mx.Get("/twitter", h5)
//		mx.Route("/direct", func(mx Router) {
//			mx.Get("/", h6) // subrouter-2
//			mx.Get("/download", h6)
//		})
//	})
//
//	routes := []string{
//		"/",
//		"/hi",
//		"/hi-post",
//		"/sup/123/and/this",
//		"/sup/123/foo/this",
//		"/sharing/z/aBc",                 // subrouter-1
//		"/sharing/z/aBc/twitter",         // subrouter-1
//		"/sharing/z/aBc/direct",          // subrouter-2
//		"/sharing/z/aBc/direct/download", // subrouter-2
//	}
//
//	for _, path := range routes {
//		b.Run("route:"+path, func(b *testing.B) {
//			w := httptest.NewRecorder()
//			r, _ := http.NewRequest("GET", path, nil)
//
//			b.ReportAllocs()
//			b.ResetTimer()
//
//			for i := 0; i < b.N; i++ {
//				mx.ServeHTTP(w, r)
//			}
//		})
//	}
//}
