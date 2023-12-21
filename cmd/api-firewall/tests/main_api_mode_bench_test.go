package tests

import (
	"bytes"
	"encoding/json"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	handlersAPI "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/api"
	"github.com/wallarm/api-firewall/internal/platform/database"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

func BenchmarkAPIModeBasic(b *testing.B) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var lock sync.RWMutex

	// load spec from the database
	specStorage, err := database.NewOpenAPIDB(logger, "../../../resources/test/database/wallarm_api.db")
	if err != nil {
		b.Fatalf("trying to load API Spec value from SQLLite Database : %v\n", err.Error())
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	handler := handlersAPI.Handlers(&lock, &cfg, shutdown, logger, specStorage)

	p, err := json.Marshal(map[string]interface{}{
		"firstname": "test",
		"lastname":  "test",
		"job":       "test",
		"email":     "test@wallarm.com",
		"url":       "http://wallarm.com",
	})

	if err != nil {
		b.Fatal(err)
	}

	// basic test
	b.Run("api_basic", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			req := fasthttp.AcquireRequest()
			req.SetRequestURI("/test/signup")
			req.Header.SetMethod("POST")
			req.SetBodyStream(bytes.NewReader(p), -1)
			req.Header.SetContentType("application/json")
			req.Header.Add(web.XWallarmSchemaIDHeader, "2")

			reqCtx := fasthttp.RequestCtx{
				Request: *req,
			}
			handler(&reqCtx)
			if reqCtx.Response.StatusCode() != 200 {
				b.Errorf("Incorrect response status code. Expected: 200 and got %d",
					reqCtx.Response.StatusCode())
			}
		}
	})

}
