package router

import (
	"fmt"
	"log"
	"testing"

	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

func TestTree(t *testing.T) {
	hStub := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hIndex := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hFavicon := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hArticleList := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hArticleNear := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hArticleShow := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hArticleShowRelated := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hArticleShowOpts := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hArticleSlug := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hArticleByUser := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hUserList := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hUserShow := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hAdminCatchall := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hAdminAppShow := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hAdminAppShowCatchall := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hUserProfile := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hUserSuper := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hUserAll := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hHubView1 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hHubView2 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hHubView3 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })

	tr := &node{}

	if _, err := tr.InsertRoute(mGET, "/", hIndex); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/favicon.ico", hFavicon); err != nil {
		t.Fatal(err)
	}

	if _, err := tr.InsertRoute(mGET, "/pages/*", hStub); err != nil {
		t.Fatal(err)
	}

	if _, err := tr.InsertRoute(mGET, "/article", hArticleList); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/article/", hArticleList); err != nil {
		t.Fatal(err)
	}

	if _, err := tr.InsertRoute(mGET, "/article/near", hArticleNear); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/article/{id}", hStub); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/article/{id}", hArticleShow); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/article/{id}", hArticleShow); err != nil {
		t.Fatal(err)
	} // duplicate will have no effect

	if _, err := tr.InsertRoute(mGET, "/article/@{user}", hArticleByUser); err != nil {
		t.Fatal(err)
	}

	if _, err := tr.InsertRoute(mGET, "/article/{sup}/{opts}", hArticleShowOpts); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/article/{id}/{opts}", hArticleShowOpts); err != nil {
		t.Fatal(err)
	} // overwrite above route, latest wins

	if _, err := tr.InsertRoute(mGET, "/article/{iffd}/edit", hStub); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/article/{id}//related", hArticleShowRelated); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/article/slug/{month}/-/{day}/{year}", hArticleSlug); err != nil {
		t.Fatal(err)
	}

	if _, err := tr.InsertRoute(mGET, "/admin/user", hUserList); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/admin/user/", hStub); err != nil {
		t.Fatal(err)
	} // will get replaced by next route

	if _, err := tr.InsertRoute(mGET, "/admin/user/", hUserList); err != nil {
		t.Fatal(err)
	}

	if _, err := tr.InsertRoute(mGET, "/admin/user//{id}", hUserShow); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/admin/user/{id}", hUserShow); err != nil {
		t.Fatal(err)
	}

	if _, err := tr.InsertRoute(mGET, "/admin/apps/{id}", hAdminAppShow); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/admin/apps/{id}/*", hAdminAppShowCatchall); err != nil {
		t.Fatal(err)
	}

	if _, err := tr.InsertRoute(mGET, "/admin/*", hStub); err != nil {
		t.Fatal(err)
	} // catchall segment will get replaced by next route

	if _, err := tr.InsertRoute(mGET, "/admin/*", hAdminCatchall); err != nil {
		t.Fatal(err)
	}

	if _, err := tr.InsertRoute(mGET, "/users/{userID}/profile", hUserProfile); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/users/super/*", hUserSuper); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/users/*", hUserAll); err != nil {
		t.Fatal(err)
	}

	if _, err := tr.InsertRoute(mGET, "/hubs/{hubID}/view", hHubView1); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/hubs/{hubID}/view/*", hHubView2); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/hubs/{hubID}/users", hHubView3); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		r string      // input request path
		h web.Handler // output matched handler
		k []string    // output param keys
		v []string    // output param values
	}{
		{r: "/", h: hIndex, k: []string{}, v: []string{}},
		{r: "/favicon.ico", h: hFavicon, k: []string{}, v: []string{}},

		{r: "/pages", h: nil, k: []string{}, v: []string{}},
		{r: "/pages/", h: hStub, k: []string{"*"}, v: []string{""}},
		{r: "/pages/yes", h: hStub, k: []string{"*"}, v: []string{"yes"}},

		{r: "/article", h: hArticleList, k: []string{}, v: []string{}},
		{r: "/article/", h: hArticleList, k: []string{}, v: []string{}},
		{r: "/article/near", h: hArticleNear, k: []string{}, v: []string{}},
		{r: "/article/neard", h: hArticleShow, k: []string{"id"}, v: []string{"neard"}},
		{r: "/article/123", h: hArticleShow, k: []string{"id"}, v: []string{"123"}},
		{r: "/article/123/456", h: hArticleShowOpts, k: []string{"id", "opts"}, v: []string{"123", "456"}},
		{r: "/article/@peter", h: hArticleByUser, k: []string{"user"}, v: []string{"peter"}},
		{r: "/article/22//related", h: hArticleShowRelated, k: []string{"id"}, v: []string{"22"}},
		{r: "/article/111/edit", h: hStub, k: []string{"iffd"}, v: []string{"111"}},
		{r: "/article/slug/sept/-/4/2015", h: hArticleSlug, k: []string{"month", "day", "year"}, v: []string{"sept", "4", "2015"}},
		{r: "/article/:id", h: hArticleShow, k: []string{"id"}, v: []string{":id"}},

		{r: "/admin/user", h: hUserList, k: []string{}, v: []string{}},
		{r: "/admin/user/", h: hUserList, k: []string{}, v: []string{}},
		{r: "/admin/user/1", h: hUserShow, k: []string{"id"}, v: []string{"1"}},
		{r: "/admin/user//1", h: hUserShow, k: []string{"id"}, v: []string{"1"}},
		{r: "/admin/hi", h: hAdminCatchall, k: []string{"*"}, v: []string{"hi"}},
		{r: "/admin/lots/of/:fun", h: hAdminCatchall, k: []string{"*"}, v: []string{"lots/of/:fun"}},
		{r: "/admin/apps/333", h: hAdminAppShow, k: []string{"id"}, v: []string{"333"}},
		{r: "/admin/apps/333/woot", h: hAdminAppShowCatchall, k: []string{"id", "*"}, v: []string{"333", "woot"}},

		{r: "/hubs/123/view", h: hHubView1, k: []string{"hubID"}, v: []string{"123"}},
		{r: "/hubs/123/view/index.html", h: hHubView2, k: []string{"hubID", "*"}, v: []string{"123", "index.html"}},
		{r: "/hubs/123/users", h: hHubView3, k: []string{"hubID"}, v: []string{"123"}},

		{r: "/users/123/profile", h: hUserProfile, k: []string{"userID"}, v: []string{"123"}},
		{r: "/users/super/123/okay/yes", h: hUserSuper, k: []string{"*"}, v: []string{"123/okay/yes"}},
		{r: "/users/123/okay/yes", h: hUserAll, k: []string{"*"}, v: []string{"123/okay/yes"}},
	}

	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")
	// debugPrintTree(0, 0, tr, 0)
	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")

	for i, tt := range tests {
		rctx := NewRouteContext()

		_, handlers, _ := tr.FindRoute(rctx, mGET, tt.r)

		var handler web.Handler
		if methodHandler, ok := handlers[mGET]; ok {
			handler = methodHandler.handler
		}

		paramKeys := rctx.routeParams.Keys
		paramValues := rctx.routeParams.Values

		if fmt.Sprintf("%v", tt.h) != fmt.Sprintf("%v", handler) {
			t.Errorf("input [%d]: find '%s' expecting handler:%v , got:%v", i, tt.r, tt.h, handler)
		}
		if !stringSliceEqual(tt.k, paramKeys) {
			t.Errorf("input [%d]: find '%s' expecting paramKeys:(%d)%v , got:(%d)%v", i, tt.r, len(tt.k), tt.k, len(paramKeys), paramKeys)
		}
		if !stringSliceEqual(tt.v, paramValues) {
			t.Errorf("input [%d]: find '%s' expecting paramValues:(%d)%v , got:(%d)%v", i, tt.r, len(tt.v), tt.v, len(paramValues), paramValues)
		}
	}
}

func TestTreeMoar(t *testing.T) {
	hStub := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub1 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub2 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub3 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub4 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub5 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub6 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub7 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub8 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub9 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub10 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub11 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub12 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub13 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub14 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub15 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub16 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })

	// TODO: panic if we see {id}{x} because we're missing a delimiter, its not possible.
	// also {:id}* is not possible.

	tr := &node{}

	if _, err := tr.InsertRoute(mGET, "/articlefun", hStub5); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{id}", hStub); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mDELETE, "/articles/{slug}", hStub8); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/search", hStub1); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{id}:delete", hStub8); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{iidd}!sup", hStub4); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{id}:{op}", hStub3); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{id}:{op}", hStub2); err != nil {
		t.Fatal(err) // this route sets a new handler for the above route
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{slug:^[a-z]+}/posts", hStub); err != nil { // up to tail '/' will only match if contents match the rex
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{id}/posts/{pid}", hStub6); err != nil { // /articles/123/posts/1
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{id}/posts/{month}/{day}/{year}/{slug}", hStub7); err != nil { // /articles/123/posts/09/04/1984/juice
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{id}.json", hStub10); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{id}/data.json", hStub11); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/files/{file}.{ext}", hStub12); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mPUT, "/articles/me", hStub13); err != nil {
		t.Fatal(err)
	}

	// TODO: make a separate test case for this one..
	// tr.InsertRoute(mGET, "/articles/{id}/{id}", hStub1)                              // panic expected, we're duplicating param keys

	tr.InsertRoute(mGET, "/pages/*", hStub)
	tr.InsertRoute(mGET, "/pages/*", hStub9)

	tr.InsertRoute(mGET, "/users/{id}", hStub14)
	tr.InsertRoute(mGET, "/users/{id}/settings/{key}", hStub15)
	tr.InsertRoute(mGET, "/users/{id}/settings/*", hStub16)

	tests := []struct {
		h web.Handler
		r string
		k []string
		v []string
		m methodTyp
	}{
		{m: mGET, r: "/articles/search", h: hStub1, k: []string{}, v: []string{}},
		{m: mGET, r: "/articlefun", h: hStub5, k: []string{}, v: []string{}},
		{m: mGET, r: "/articles/123", h: hStub, k: []string{"id"}, v: []string{"123"}},
		{m: mDELETE, r: "/articles/123mm", h: hStub8, k: []string{"slug"}, v: []string{"123mm"}},
		{m: mGET, r: "/articles/789:delete", h: hStub8, k: []string{"id"}, v: []string{"789"}},
		{m: mGET, r: "/articles/789!sup", h: hStub4, k: []string{"iidd"}, v: []string{"789"}},
		{m: mGET, r: "/articles/123:sync", h: hStub2, k: []string{"id", "op"}, v: []string{"123", "sync"}},
		{m: mGET, r: "/articles/456/posts/1", h: hStub6, k: []string{"id", "pid"}, v: []string{"456", "1"}},
		{m: mGET, r: "/articles/456/posts/09/04/1984/juice", h: hStub7, k: []string{"id", "month", "day", "year", "slug"}, v: []string{"456", "09", "04", "1984", "juice"}},
		{m: mGET, r: "/articles/456.json", h: hStub10, k: []string{"id"}, v: []string{"456"}},
		{m: mGET, r: "/articles/456/data.json", h: hStub11, k: []string{"id"}, v: []string{"456"}},

		{m: mGET, r: "/articles/files/file.zip", h: hStub12, k: []string{"file", "ext"}, v: []string{"file", "zip"}},
		{m: mGET, r: "/articles/files/photos.tar.gz", h: hStub12, k: []string{"file", "ext"}, v: []string{"photos", "tar.gz"}},
		{m: mGET, r: "/articles/files/photos.tar.gz", h: hStub12, k: []string{"file", "ext"}, v: []string{"photos", "tar.gz"}},

		{m: mPUT, r: "/articles/me", h: hStub13, k: []string{}, v: []string{}},
		{m: mGET, r: "/articles/me", h: hStub, k: []string{"id"}, v: []string{"me"}},
		{m: mGET, r: "/pages", h: nil, k: []string{}, v: []string{}},
		{m: mGET, r: "/pages/", h: hStub9, k: []string{"*"}, v: []string{""}},
		{m: mGET, r: "/pages/yes", h: hStub9, k: []string{"*"}, v: []string{"yes"}},

		{m: mGET, r: "/users/1", h: hStub14, k: []string{"id"}, v: []string{"1"}},
		{m: mGET, r: "/users/", h: nil, k: []string{}, v: []string{}},
		{m: mGET, r: "/users/2/settings/password", h: hStub15, k: []string{"id", "key"}, v: []string{"2", "password"}},
		{m: mGET, r: "/users/2/settings/", h: hStub16, k: []string{"id", "*"}, v: []string{"2", ""}},
	}

	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")
	// debugPrintTree(0, 0, tr, 0)
	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")

	for i, tt := range tests {
		rctx := NewRouteContext()

		_, handlers, _ := tr.FindRoute(rctx, tt.m, tt.r)

		var handler web.Handler
		if methodHandler, ok := handlers[tt.m]; ok {
			handler = methodHandler.handler
		}

		paramKeys := rctx.routeParams.Keys
		paramValues := rctx.routeParams.Values

		if fmt.Sprintf("%v", tt.h) != fmt.Sprintf("%v", handler) {
			t.Errorf("input [%d]: find '%s' expecting handler:%v , got:%v", i, tt.r, tt.h, handler)
		}
		if !stringSliceEqual(tt.k, paramKeys) {
			t.Errorf("input [%d]: find '%s' expecting paramKeys:(%d)%v , got:(%d)%v", i, tt.r, len(tt.k), tt.k, len(paramKeys), paramKeys)
		}
		if !stringSliceEqual(tt.v, paramValues) {
			t.Errorf("input [%d]: find '%s' expecting paramValues:(%d)%v , got:(%d)%v", i, tt.r, len(tt.v), tt.v, len(paramValues), paramValues)
		}
	}
}

func TestTreeRegexp(t *testing.T) {
	hStub1 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub2 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub3 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub4 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub5 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub6 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub7 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })

	tr := &node{}
	if _, err := tr.InsertRoute(mGET, "/articles/{rid:^[0-9]{5,6}}", hStub7); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{zid:^0[0-9]+}", hStub3); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{name:^@[a-z]+}/posts", hStub4); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{op:^[0-9]+}/run", hStub5); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{id:^[0-9]+}", hStub1); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{id:^[1-9]+}-{aux}", hStub6); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{slug}", hStub2); err != nil {
		t.Fatal(err)
	}

	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")
	// debugPrintTree(0, 0, tr, 0)
	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")

	tests := []struct {
		r string      // input request path
		h web.Handler // output matched handler
		k []string    // output param keys
		v []string    // output param values
	}{
		{r: "/articles", h: nil, k: []string{}, v: []string{}},
		{r: "/articles/12345", h: hStub7, k: []string{"rid"}, v: []string{"12345"}},
		{r: "/articles/123", h: hStub1, k: []string{"id"}, v: []string{"123"}},
		{r: "/articles/how-to-build-a-router", h: hStub2, k: []string{"slug"}, v: []string{"how-to-build-a-router"}},
		{r: "/articles/0456", h: hStub3, k: []string{"zid"}, v: []string{"0456"}},
		{r: "/articles/@pk/posts", h: hStub4, k: []string{"name"}, v: []string{"@pk"}},
		{r: "/articles/1/run", h: hStub5, k: []string{"op"}, v: []string{"1"}},
		{r: "/articles/1122", h: hStub1, k: []string{"id"}, v: []string{"1122"}},
		{r: "/articles/1122-yes", h: hStub6, k: []string{"id", "aux"}, v: []string{"1122", "yes"}},
	}

	for i, tt := range tests {
		rctx := NewRouteContext()

		_, handlers, _ := tr.FindRoute(rctx, mGET, tt.r)

		var handler web.Handler
		if methodHandler, ok := handlers[mGET]; ok {
			handler = methodHandler.handler
		}

		paramKeys := rctx.routeParams.Keys
		paramValues := rctx.routeParams.Values

		if fmt.Sprintf("%v", tt.h) != fmt.Sprintf("%v", handler) {
			t.Errorf("input [%d]: find '%s' expecting handler:%v , got:%v", i, tt.r, tt.h, handler)
		}
		if !stringSliceEqual(tt.k, paramKeys) {
			t.Errorf("input [%d]: find '%s' expecting paramKeys:(%d)%v , got:(%d)%v", i, tt.r, len(tt.k), tt.k, len(paramKeys), paramKeys)
		}
		if !stringSliceEqual(tt.v, paramValues) {
			t.Errorf("input [%d]: find '%s' expecting paramValues:(%d)%v , got:(%d)%v", i, tt.r, len(tt.v), tt.v, len(paramValues), paramValues)
		}
	}
}

func TestTreeRegexpRecursive(t *testing.T) {
	hStub1 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub2 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })

	tr := &node{}
	if _, err := tr.InsertRoute(mGET, "/one/{firstId:[a-z0-9-]+}/{secondId:[a-z0-9-]+}/first", hStub1); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/one/{firstId:[a-z0-9-_]+}/{secondId:[a-z0-9-_]+}/second", hStub2); err != nil {
		t.Fatal(err)
	}

	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")
	// debugPrintTree(0, 0, tr, 0)
	// log.Println("~~~~~~~~~")
	// log.Println("~~~~~~~~~")

	tests := []struct {
		r string      // input request path
		h web.Handler // output matched handler
		k []string    // output param keys
		v []string    // output param values
	}{
		{r: "/one/hello/world/first", h: hStub1, k: []string{"firstId", "secondId"}, v: []string{"hello", "world"}},
		{r: "/one/hi_there/ok/second", h: hStub2, k: []string{"firstId", "secondId"}, v: []string{"hi_there", "ok"}},
		{r: "/one///first", h: nil, k: []string{}, v: []string{}},
		{r: "/one/hi/123/second", h: hStub2, k: []string{"firstId", "secondId"}, v: []string{"hi", "123"}},
	}

	for i, tt := range tests {
		rctx := NewRouteContext()

		_, handlers, _ := tr.FindRoute(rctx, mGET, tt.r)

		var handler web.Handler
		if methodHandler, ok := handlers[mGET]; ok {
			handler = methodHandler.handler
		}

		paramKeys := rctx.routeParams.Keys
		paramValues := rctx.routeParams.Values

		if fmt.Sprintf("%v", tt.h) != fmt.Sprintf("%v", handler) {
			t.Errorf("input [%d]: find '%s' expecting handler:%v , got:%v", i, tt.r, tt.h, handler)
		}
		if !stringSliceEqual(tt.k, paramKeys) {
			t.Errorf("input [%d]: find '%s' expecting paramKeys:(%d)%v , got:(%d)%v", i, tt.r, len(tt.k), tt.k, len(paramKeys), paramKeys)
		}
		if !stringSliceEqual(tt.v, paramValues) {
			t.Errorf("input [%d]: find '%s' expecting paramValues:(%d)%v , got:(%d)%v", i, tt.r, len(tt.v), tt.v, len(paramValues), paramValues)
		}
	}
}

func TestTreeRegexMatchWholeParam(t *testing.T) {
	hStub1 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })

	rctx := NewRouteContext()
	tr := &node{}
	if _, err := tr.InsertRoute(mGET, "/{id:[0-9]+}", hStub1); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/{x:.+}/foo", hStub1); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/{param:[0-9]*}/test", hStub1); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		expectedHandler web.Handler
		url             string
	}{
		{url: "/13", expectedHandler: hStub1},
		{url: "/a13", expectedHandler: nil},
		{url: "/13.jpg", expectedHandler: nil},
		{url: "/a13.jpg", expectedHandler: nil},
		{url: "/a/foo", expectedHandler: hStub1},
		{url: "//foo", expectedHandler: nil},
		{url: "//test", expectedHandler: hStub1},
	}

	for _, tc := range tests {
		_, _, handler := tr.FindRoute(rctx, mGET, tc.url)
		if fmt.Sprintf("%v", tc.expectedHandler) != fmt.Sprintf("%v", handler) {
			t.Errorf("url %v: expecting handler:%v , got:%v", tc.url, tc.expectedHandler, handler)
		}
	}
}

func TestTreeFindPattern(t *testing.T) {
	hStub1 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub2 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	hStub3 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })

	tr := &node{}
	if _, err := tr.InsertRoute(mGET, "/pages/*", hStub1); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{id}/*", hStub2); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/articles/{slug}/{uid}/*", hStub3); err != nil {
		t.Fatal(err)
	}

	if ok, err := tr.findPattern("/pages"); ok != false {
		t.Errorf("find /pages failed: %v", err)
	}
	if ok, err := tr.findPattern("/pages*"); ok != false {
		t.Errorf("find /pages* failed - should be nil: %v", err)
	}
	if ok, err := tr.findPattern("/pages/*"); ok == false {
		t.Errorf("find /pages/* failed: %v", err)
	}
	if ok, err := tr.findPattern("/articles/{id}/*"); ok == false {
		t.Errorf("find /articles/{id}/* failed: %v", err)
	}
	if ok, err := tr.findPattern("/articles/{something}/*"); ok == false {
		t.Errorf("find /articles/{something}/* failed: %v", err)
	}
	if ok, err := tr.findPattern("/articles/{slug}/{uid}/*"); ok == false {
		t.Errorf("find /articles/{slug}/{uid}/* failed: %v", err)
	}
}

func debugPrintTree(parent int, i int, n *node, label byte) bool {
	numEdges := 0
	for _, nds := range n.children {
		numEdges += len(nds)
	}

	if n.endpoints != nil {
		log.Printf("[node %d parent:%d] typ:%d prefix:%s label:%s tail:%s numEdges:%d isLeaf:%v handler:%v\n", i, parent, n.typ, n.prefix, string(label), string(n.tail), numEdges, n.isLeaf(), n.endpoints)
	} else {
		log.Printf("[node %d parent:%d] typ:%d prefix:%s label:%s tail:%s numEdges:%d isLeaf:%v\n", i, parent, n.typ, n.prefix, string(label), string(n.tail), numEdges, n.isLeaf())
	}
	parent = i
	for _, nds := range n.children {
		for _, e := range nds {
			i++
			if debugPrintTree(parent, i, e, e.label) {
				return true
			}
		}
	}
	return false
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if b[i] != a[i] {
			return false
		}
	}
	return true
}

func BenchmarkTreeGet(b *testing.B) {
	h1 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })
	h2 := web.Handler(func(ctx *fasthttp.RequestCtx) error { return nil })

	tr := &node{}
	if _, err := tr.InsertRoute(mGET, "/", h1); err != nil {
		b.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/ping", h2); err != nil {
		b.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/pingall", h2); err != nil {
		b.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/ping/{id}", h2); err != nil {
		b.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/ping/{id}/woop", h2); err != nil {
		b.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/ping/{id}/{opt}", h2); err != nil {
		b.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/pinggggg", h2); err != nil {
		b.Fatal(err)
	}
	if _, err := tr.InsertRoute(mGET, "/hello", h1); err != nil {
		b.Fatal(err)
	}

	mctx := NewRouteContext()

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mctx.Reset()
		tr.FindRoute(mctx, mGET, "/ping/123/456")
	}
}
