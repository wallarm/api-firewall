package tests

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	graphqlHandler "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/graphql"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wundergraph/graphql-go-tools/pkg/graphql"
)

var (
	msg                  = []byte("{\"id\":\"1\",\"type\":\"start\",\"payload\":{\"variables\":{},\"extensions\":{},\"operationName\":\"NewMessageInGeneralChat\",\"query\":\"subscription NewMessageInGeneralChat {\\n  messageAdded(roomName: \\\"GeneralChat\\\") {\\n    id\\n    text\\n    createdBy\\n    createdAt\\n  }\\n}\\n\"}}")
	gqlBenchQuery        = []byte("{\"query\":\"\\n\\t\\tquery {\\n    room(name: \\\"GeneralChat\\\") {\\n        name\\n        messages {\\n            id\\n            text\\n            createdBy\\n            createdAt\\n        }\\n    }\\n}\\n\\t\"}")
	gqlBenchResponseBody = []byte(`{
    "data": {
        "room": {
            "name": "GeneralChat",
            "messages": [
                {
                    "id": "TrsXJcKa",
                    "text": "Hello, world!",
                    "createdBy": "TestUser",
                    "createdAt": "2023-01-01T00:00:00+00:00"
                }
            ]
        }
    }
}`)
	benchBackendURL  = "http://localhost:29090/graphql"
	benchBackendHost = "localhost:29090"
	benchHandlerURL  = "http://localhost:29091/graphql"
	benchHandlerHost = "localhost:29091"
)

func StartGqlBenchBackendServer(t testing.TB, addr string) *fasthttp.Server {
	upgrader := websocket.FastHTTPUpgrader{
		Subprotocols: []string{"graphql-ws"},
		CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
			return true
		},
	}

	gqlHandler := func(ctx *fasthttp.RequestCtx) {
		if websocket.FastHTTPIsWebSocketUpgrade(ctx) {
			if err := upgrader.Upgrade(ctx, func(ws *websocket.Conn) {
				defer ws.Close()
				for {
					mt, message, err := ws.ReadMessage()
					if err != nil {
						t.Error(err)
						break
					}

					err = ws.WriteMessage(mt, message)
					if err != nil {
						t.Error(err)
						break
					}
				}
			}); err != nil {
				if _, ok := err.(websocket.HandshakeError); ok {
					assert.Errorf(t, err, "websocket handshake")
				}
				return
			}
			return
		}

		if bytes.Equal(ctx.Request.Body(), gqlBenchQuery) {
			ctx.Response.SetStatusCode(fasthttp.StatusOK)
			ctx.Response.Header.SetContentType("application/json")
			ctx.Response.SetBody(gqlBenchResponseBody)
			return
		}
		ctx.Response.SetStatusCode(fasthttp.StatusForbidden)
		return
	}

	// backend server initializing
	server := fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			switch string(ctx.Path()) {
			case "/graphql":
				gqlHandler(ctx)
			}
		},
	}

	go func() {
		// backend websocket server
		if err := server.ListenAndServe(addr); err != nil {
			assert.Errorf(t, err, "websocket backend server `ListenAndServe` quit")
		}
	}()

	return &server
}

func BenchmarkGraphQL(b *testing.B) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// start backend
	server := StartGqlBenchBackendServer(b, benchBackendHost)
	defer server.Shutdown()

	serverURL, err := url.ParseRequestURI(benchBackendURL)
	if err != nil {
		b.Fatalf("parsing API Host URL: %s", err.Error())
	}
	host := serverURL.Host

	initialCap := 100

	options := proxy.Options{
		InitialPoolCapacity: initialCap,
		ClientPoolCapacity:  1000,
		InsecureConnection:  true,
		MaxConnsPerHost:     512,
		ReadTimeout:         5 * time.Second,
		WriteTimeout:        5 * time.Second,
		DialTimeout:         5 * time.Second,
	}
	pool, err := proxy.NewChanPool(host, &options)
	if err != nil {
		b.Fatalf("proxy pool init: %v", err)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	wsConnPoolOptions := &proxy.WSClientOptions{
		Scheme:             "ws",
		Host:               serverURL.Host,
		Path:               serverURL.Path,
		InsecureConnection: true,
		DialTimeout:        500 * time.Millisecond,
	}

	wsPool, err := proxy.NewWSClient(logger, wsConnPoolOptions)
	if err != nil {
		b.Fatalf("ws connections pool init: %v", err)
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		b.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	gqlCfg := config.GraphQL{
		MaxQueryComplexity: 20,
		MaxQueryDepth:      20,
		NodeCountLimit:     20,
		Playground:         false,
		Introspection:      false,
		Schema:             "",
		WSCheckOrigin:      true,
		WSOrigin:           []string{"http://" + benchHandlerHost},
		RequestValidation:  "BLOCK",
	}
	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		Server: config.Backend{
			URL: benchBackendURL,
		},
		APIHost: benchHandlerURL,
	}

	handler := graphqlHandler.Handlers(&cfg, schema, serverURL, shutdown, logger, pool, wsPool, nil, nil)

	srv := fasthttp.Server{
		Handler: handler,
	}

	go func() {
		if err := srv.ListenAndServe(benchHandlerHost); err != nil {
			b.Errorf("websocket proxy server `ListenAndServe` quit, err=%v\n", err)
		}
	}()
	defer srv.Shutdown()

	time.Sleep(1 * time.Second)

	headers := http.Header{}
	headers.Set("Sec-WebSocket-Protocol", "graphql-ws")
	headers.Set("Origin", "http://"+benchHandlerHost)

	b.Run("ws_client_echo", func(b *testing.B) {
		wsClientConn, _, err := websocket.DefaultDialer.Dial("ws://"+benchHandlerHost+"/graphql", headers)
		if err != nil {
			b.Fatal(err)
		}
		for i := 0; i < b.N; i++ {
			// client send
			err = wsClientConn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				b.Fatal(err)
			}
			_, _, err = wsClientConn.ReadMessage()
			if err != nil {
				b.Fatal(err)
			}
		}
		wsClientConn.Close()
	})

	b.Run("http_query", func(b *testing.B) {
		req := fasthttp.AcquireRequest()
		req.SetRequestURI("/graphql")
		req.Header.SetMethod("POST")
		req.SetBody(gqlBenchQuery)
		req.Header.SetContentType("application/json")
		req.SetRequestURI(benchHandlerURL)
		resp := fasthttp.AcquireResponse()

		for i := 0; i < b.N; i++ {
			if err := fasthttp.Do(req, resp); err != nil {
				b.Fatal(err)
			}
			if resp.StatusCode() != fasthttp.StatusOK {
				b.Fatal(errors.New("wrong status code"))
			}
			resp.Reset()
		}
	})
}
