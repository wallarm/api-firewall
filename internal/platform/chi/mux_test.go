package chi

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/valyala/fasthttp"
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

//func TestMuxBig(t *testing.T) {
//	r := bigMux()
//
//	ts := httptest.NewServer(r)
//	defer ts.Close()
//
//	var body, expected string
//
//	_, body = testRequest(t, ts, "GET", "/favicon.ico", nil)
//	if body != "fav" {
//		t.Fatalf("got '%s'", body)
//	}
//	_, body = testRequest(t, ts, "GET", "/hubs/4/view", nil)
//	if body != "/hubs/4/view reqid:1 session:anonymous" {
//		t.Fatalf("got '%v'", body)
//	}
//	_, body = testRequest(t, ts, "GET", "/hubs/4/view/index.html", nil)
//	if body != "/hubs/4/view/index.html reqid:1 session:anonymous" {
//		t.Fatalf("got '%s'", body)
//	}
//	_, body = testRequest(t, ts, "POST", "/hubs/ethereumhub/view/index.html", nil)
//	if body != "/hubs/ethereumhub/view/index.html reqid:1 session:anonymous" {
//		t.Fatalf("got '%s'", body)
//	}
//	_, body = testRequest(t, ts, "GET", "/", nil)
//	if body != "/ reqid:1 session:elvis" {
//		t.Fatalf("got '%s'", body)
//	}
//	_, body = testRequest(t, ts, "GET", "/suggestions", nil)
//	if body != "/suggestions reqid:1 session:elvis" {
//		t.Fatalf("got '%s'", body)
//	}
//	_, body = testRequest(t, ts, "GET", "/woot/444/hiiii", nil)
//	if body != "/woot/444/hiiii" {
//		t.Fatalf("got '%s'", body)
//	}
//	_, body = testRequest(t, ts, "GET", "/hubs/123", nil)
//	expected = "/hubs/123 reqid:1 session:elvis"
//	if body != expected {
//		t.Fatalf("expected:%s got:%s", expected, body)
//	}
//	_, body = testRequest(t, ts, "GET", "/hubs/123/touch", nil)
//	if body != "/hubs/123/touch reqid:1 session:elvis" {
//		t.Fatalf("got '%s'", body)
//	}
//	_, body = testRequest(t, ts, "GET", "/hubs/123/webhooks", nil)
//	if body != "/hubs/123/webhooks reqid:1 session:elvis" {
//		t.Fatalf("got '%s'", body)
//	}
//	_, body = testRequest(t, ts, "GET", "/hubs/123/posts", nil)
//	if body != "/hubs/123/posts reqid:1 session:elvis" {
//		t.Fatalf("got '%s'", body)
//	}
//	_, body = testRequest(t, ts, "GET", "/folders", nil)
//	if body != "404 page not found\n" {
//		t.Fatalf("got '%s'", body)
//	}
//	_, body = testRequest(t, ts, "GET", "/folders/", nil)
//	if body != "/folders/ reqid:1 session:elvis" {
//		t.Fatalf("got '%s'", body)
//	}
//	_, body = testRequest(t, ts, "GET", "/folders/public", nil)
//	if body != "/folders/public reqid:1 session:elvis" {
//		t.Fatalf("got '%s'", body)
//	}
//	_, body = testRequest(t, ts, "GET", "/folders/nothing", nil)
//	if body != "404 page not found\n" {
//		t.Fatalf("got '%s'", body)
//	}
//}
//
//func TestSingleHandler(t *testing.T) {
//	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		name := URLParam(r, "name")
//		w.Write([]byte("hi " + name))
//	})
//
//	r, _ := http.NewRequest("GET", "/", nil)
//	rctx := NewRouteContext()
//	r = r.WithContext(context.WithValue(r.Context(), RouteCtxKey, rctx))
//	rctx.URLParams.Add("name", "joe")
//
//	w := httptest.NewRecorder()
//	h.ServeHTTP(w, r)
//
//	body := w.Body.String()
//	expected := "hi joe"
//	if body != expected {
//		t.Fatalf("expected:%s got:%s", expected, body)
//	}
//}
//
//func TestNestedGroups(t *testing.T) {
//	handlerPrintCounter := func(w http.ResponseWriter, r *http.Request) {
//		counter, _ := r.Context().Value(ctxKey{"counter"}).(int)
//		w.Write([]byte(fmt.Sprintf("%v", counter)))
//	}
//
//	mwIncreaseCounter := func(next http.Handler) http.Handler {
//		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//			ctx := r.Context()
//			counter, _ := ctx.Value(ctxKey{"counter"}).(int)
//			counter++
//			ctx = context.WithValue(ctx, ctxKey{"counter"}, counter)
//			next.ServeHTTP(w, r.WithContext(ctx))
//		})
//	}
//
//	// Each route represents value of its counter (number of applied middlewares).
//	r := NewRouter() // counter == 0
//	r.Get("/0", handlerPrintCounter)
//	r.Group(func(r Router) {
//		r.Use(mwIncreaseCounter) // counter == 1
//		r.Get("/1", handlerPrintCounter)
//
//		// r.Handle(GET, "/2", Chain(mwIncreaseCounter).HandlerFunc(handlerPrintCounter))
//		r.With(mwIncreaseCounter).Get("/2", handlerPrintCounter)
//
//		r.Group(func(r Router) {
//			r.Use(mwIncreaseCounter, mwIncreaseCounter) // counter == 3
//			r.Get("/3", handlerPrintCounter)
//		})
//		r.Route("/", func(r Router) {
//			r.Use(mwIncreaseCounter, mwIncreaseCounter) // counter == 3
//
//			// r.Handle(GET, "/4", Chain(mwIncreaseCounter).HandlerFunc(handlerPrintCounter))
//			r.With(mwIncreaseCounter).Get("/4", handlerPrintCounter)
//
//			r.Group(func(r Router) {
//				r.Use(mwIncreaseCounter, mwIncreaseCounter) // counter == 5
//				r.Get("/5", handlerPrintCounter)
//				// r.Handle(GET, "/6", Chain(mwIncreaseCounter).HandlerFunc(handlerPrintCounter))
//				r.With(mwIncreaseCounter).Get("/6", handlerPrintCounter)
//
//			})
//		})
//	})
//
//	ts := httptest.NewServer(r)
//	defer ts.Close()
//
//	for _, route := range []string{"0", "1", "2", "3", "4", "5", "6"} {
//		if _, body := testRequest(t, ts, "GET", "/"+route, nil); body != route {
//			t.Errorf("expected %v, got %v", route, body)
//		}
//	}
//}
//
//func TestMuxEmptyParams(t *testing.T) {
//	r := NewRouter()
//	r.Get(`/users/{x}/{y}/{z}`, func(w http.ResponseWriter, r *http.Request) {
//		x := URLParam(r, "x")
//		y := URLParam(r, "y")
//		z := URLParam(r, "z")
//		w.Write([]byte(fmt.Sprintf("%s-%s-%s", x, y, z)))
//	})
//
//	ts := httptest.NewServer(r)
//	defer ts.Close()
//
//	if _, body := testRequest(t, ts, "GET", "/users/a/b/c", nil); body != "a-b-c" {
//		t.Fatalf(body)
//	}
//	if _, body := testRequest(t, ts, "GET", "/users///c", nil); body != "--c" {
//		t.Fatalf(body)
//	}
//}
//
//func TestMuxMissingParams(t *testing.T) {
//	r := NewRouter()
//	r.Get(`/user/{userId:\d+}`, func(w http.ResponseWriter, r *http.Request) {
//		userID := URLParam(r, "userId")
//		w.Write([]byte(fmt.Sprintf("userId = '%s'", userID)))
//	})
//	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
//		w.WriteHeader(404)
//		w.Write([]byte("nothing here"))
//	})
//
//	ts := httptest.NewServer(r)
//	defer ts.Close()
//
//	if _, body := testRequest(t, ts, "GET", "/user/123", nil); body != "userId = '123'" {
//		t.Fatalf(body)
//	}
//	if _, body := testRequest(t, ts, "GET", "/user/", nil); body != "nothing here" {
//		t.Fatalf(body)
//	}
//}
//
//func TestMuxWildcardRoute(t *testing.T) {
//	handler := func(w http.ResponseWriter, r *http.Request) {}
//
//	defer func() {
//		if recover() == nil {
//			t.Error("expected panic()")
//		}
//	}()
//
//	r := NewRouter()
//	r.Get("/*/wildcard/must/be/at/end", handler)
//}
//
//func TestMuxWildcardRouteCheckTwo(t *testing.T) {
//	handler := func(w http.ResponseWriter, r *http.Request) {}
//
//	defer func() {
//		if recover() == nil {
//			t.Error("expected panic()")
//		}
//	}()
//
//	r := NewRouter()
//	r.Get("/*/wildcard/{must}/be/at/end", handler)
//}
//
//func TestMuxRegexp(t *testing.T) {
//	r := NewRouter()
//	r.Route("/{param:[0-9]*}/test", func(r Router) {
//		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
//			w.Write([]byte(fmt.Sprintf("Hi: %s", URLParam(r, "param"))))
//		})
//	})
//
//	ts := httptest.NewServer(r)
//	defer ts.Close()
//
//	if _, body := testRequest(t, ts, "GET", "//test", nil); body != "Hi: " {
//		t.Fatalf(body)
//	}
//}
//
//func TestMuxRegexp2(t *testing.T) {
//	r := NewRouter()
//	r.Get("/foo-{suffix:[a-z]{2,3}}.json", func(w http.ResponseWriter, r *http.Request) {
//		w.Write([]byte(URLParam(r, "suffix")))
//	})
//	ts := httptest.NewServer(r)
//	defer ts.Close()
//
//	if _, body := testRequest(t, ts, "GET", "/foo-.json", nil); body != "" {
//		t.Fatalf(body)
//	}
//	if _, body := testRequest(t, ts, "GET", "/foo-abc.json", nil); body != "abc" {
//		t.Fatalf(body)
//	}
//}
//
//func TestMuxRegexp3(t *testing.T) {
//	r := NewRouter()
//	r.Get("/one/{firstId:[a-z0-9-]+}/{secondId:[a-z]+}/first", func(w http.ResponseWriter, r *http.Request) {
//		w.Write([]byte("first"))
//	})
//	r.Get("/one/{firstId:[a-z0-9-_]+}/{secondId:[0-9]+}/second", func(w http.ResponseWriter, r *http.Request) {
//		w.Write([]byte("second"))
//	})
//	r.Delete("/one/{firstId:[a-z0-9-_]+}/{secondId:[0-9]+}/second", func(w http.ResponseWriter, r *http.Request) {
//		w.Write([]byte("third"))
//	})
//
//	r.Route("/one", func(r Router) {
//		r.Get("/{dns:[a-z-0-9_]+}", func(writer http.ResponseWriter, request *http.Request) {
//			writer.Write([]byte("_"))
//		})
//		r.Get("/{dns:[a-z-0-9_]+}/info", func(writer http.ResponseWriter, request *http.Request) {
//			writer.Write([]byte("_"))
//		})
//		r.Delete("/{id:[0-9]+}", func(writer http.ResponseWriter, request *http.Request) {
//			writer.Write([]byte("forth"))
//		})
//	})
//
//	ts := httptest.NewServer(r)
//	defer ts.Close()
//
//	if _, body := testRequest(t, ts, "GET", "/one/hello/peter/first", nil); body != "first" {
//		t.Fatalf(body)
//	}
//	if _, body := testRequest(t, ts, "GET", "/one/hithere/123/second", nil); body != "second" {
//		t.Fatalf(body)
//	}
//	if _, body := testRequest(t, ts, "DELETE", "/one/hithere/123/second", nil); body != "third" {
//		t.Fatalf(body)
//	}
//	if _, body := testRequest(t, ts, "DELETE", "/one/123", nil); body != "forth" {
//		t.Fatalf(body)
//	}
//}
//
//func TestMuxSubrouterWildcardParam(t *testing.T) {
//	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//		fmt.Fprintf(w, "param:%v *:%v", URLParam(r, "param"), URLParam(r, "*"))
//	})
//
//	r := NewRouter()
//
//	r.Get("/bare/{param}", h)
//	r.Get("/bare/{param}/*", h)
//
//	r.Route("/case0", func(r Router) {
//		r.Get("/{param}", h)
//		r.Get("/{param}/*", h)
//	})
//
//	ts := httptest.NewServer(r)
//	defer ts.Close()
//
//	if _, body := testRequest(t, ts, "GET", "/bare/hi", nil); body != "param:hi *:" {
//		t.Fatalf(body)
//	}
//	if _, body := testRequest(t, ts, "GET", "/bare/hi/yes", nil); body != "param:hi *:yes" {
//		t.Fatalf(body)
//	}
//	if _, body := testRequest(t, ts, "GET", "/case0/hi", nil); body != "param:hi *:" {
//		t.Fatalf(body)
//	}
//	if _, body := testRequest(t, ts, "GET", "/case0/hi/yes", nil); body != "param:hi *:yes" {
//		t.Fatalf(body)
//	}
//}
//
//func TestMuxContextIsThreadSafe(t *testing.T) {
//	router := NewRouter()
//	router.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
//		ctx, cancel := context.WithTimeout(r.Context(), 1*time.Millisecond)
//		defer cancel()
//
//		<-ctx.Done()
//	})
//
//	wg := sync.WaitGroup{}
//
//	for i := 0; i < 100; i++ {
//		wg.Add(1)
//		go func() {
//			defer wg.Done()
//			for j := 0; j < 10000; j++ {
//				w := httptest.NewRecorder()
//				r, err := http.NewRequest("GET", "/ok", nil)
//				if err != nil {
//					t.Error(err)
//					return
//				}
//
//				ctx, cancel := context.WithCancel(r.Context())
//				r = r.WithContext(ctx)
//
//				go func() {
//					cancel()
//				}()
//				router.ServeHTTP(w, r)
//			}
//		}()
//	}
//	wg.Wait()
//}
//
//func TestEscapedURLParams(t *testing.T) {
//	m := NewRouter()
//	m.Get("/api/{identifier}/{region}/{size}/{rotation}/*", func(w http.ResponseWriter, r *http.Request) {
//		w.WriteHeader(200)
//		rctx := RouteContext(r.Context())
//		if rctx == nil {
//			t.Error("no context")
//			return
//		}
//		identifier := URLParam(r, "identifier")
//		if identifier != "http:%2f%2fexample.com%2fimage.png" {
//			t.Errorf("identifier path parameter incorrect %s", identifier)
//			return
//		}
//		region := URLParam(r, "region")
//		if region != "full" {
//			t.Errorf("region path parameter incorrect %s", region)
//			return
//		}
//		size := URLParam(r, "size")
//		if size != "max" {
//			t.Errorf("size path parameter incorrect %s", size)
//			return
//		}
//		rotation := URLParam(r, "rotation")
//		if rotation != "0" {
//			t.Errorf("rotation path parameter incorrect %s", rotation)
//			return
//		}
//		w.Write([]byte("success"))
//	})
//
//	ts := httptest.NewServer(m)
//	defer ts.Close()
//
//	if _, body := testRequest(t, ts, "GET", "/api/http:%2f%2fexample.com%2fimage.png/full/max/0/color.png", nil); body != "success" {
//		t.Fatalf(body)
//	}
//}
//
//func TestCustomHTTPMethod(t *testing.T) {
//	// first we must register this method to be accepted, then we
//	// can define method handlers on the router below
//	RegisterMethod("BOO")
//
//	r := NewRouter()
//	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
//		w.Write([]byte("."))
//	})
//
//	// note the custom BOO method for route /hi
//	r.MethodFunc("BOO", "/hi", func(w http.ResponseWriter, r *http.Request) {
//		w.Write([]byte("custom method"))
//	})
//
//	ts := httptest.NewServer(r)
//	defer ts.Close()
//
//	if _, body := testRequest(t, ts, "GET", "/", nil); body != "." {
//		t.Fatalf(body)
//	}
//	if _, body := testRequest(t, ts, "BOO", "/hi", nil); body != "custom method" {
//		t.Fatalf(body)
//	}
//}
//
//func TestMuxMatch(t *testing.T) {
//	r := NewRouter()
//	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
//		w.Header().Set("X-Test", "yes")
//		w.Write([]byte("bye"))
//	})
//	r.Route("/articles", func(r Router) {
//		r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
//			id := URLParam(r, "id")
//			w.Header().Set("X-Article", id)
//			w.Write([]byte("article:" + id))
//		})
//	})
//	r.Route("/users", func(r Router) {
//		r.Head("/{id}", func(w http.ResponseWriter, r *http.Request) {
//			w.Header().Set("X-User", "-")
//			w.Write([]byte("user"))
//		})
//		r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
//			id := URLParam(r, "id")
//			w.Header().Set("X-User", id)
//			w.Write([]byte("user:" + id))
//		})
//	})
//
//	tctx := NewRouteContext()
//
//	tctx.Reset()
//	if r.Match(tctx, "GET", "/users/1") == false {
//		t.Fatal("expecting to find match for route:", "GET", "/users/1")
//	}
//
//	tctx.Reset()
//	if r.Match(tctx, "HEAD", "/articles/10") == true {
//		t.Fatal("not expecting to find match for route:", "HEAD", "/articles/10")
//	}
//}
//
//func TestServerBaseContext(t *testing.T) {
//	r := NewRouter()
//	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
//		baseYes := r.Context().Value(ctxKey{"base"}).(string)
//		if _, ok := r.Context().Value(http.ServerContextKey).(*http.Server); !ok {
//			panic("missing server context")
//		}
//		if _, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); !ok {
//			panic("missing local addr context")
//		}
//		w.Write([]byte(baseYes))
//	})
//
//	// Setup http Server with a base context
//	ctx := context.WithValue(context.Background(), ctxKey{"base"}, "yes")
//	ts := httptest.NewUnstartedServer(r)
//	ts.Config.BaseContext = func(_ net.Listener) context.Context {
//		return ctx
//	}
//	ts.Start()
//
//	defer ts.Close()
//
//	if _, body := testRequest(t, ts, "GET", "/", nil); body != "yes" {
//		t.Fatalf(body)
//	}
//}

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
