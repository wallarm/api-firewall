package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	strconv2 "github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"

	"github.com/wallarm/api-firewall/demo/interface/api-mode/internal/updater"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/pkg/APIMode"
)

const logMainPrefix = "Wallarm API-Firewall"

var (
	updateTime = 60 * time.Second
	apiHost    = "0.0.0.0:8080"
)

func main() {

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	logger.Info().Msgf("%s : Started : Application initializing", logMainPrefix)
	defer logger.Info().Msgf("%s: Completed", logMainPrefix)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	apiFirewall, err := APIMode.NewAPIFirewall(
		APIMode.WithPathToDB("./wallarm_apifw_test.db"),
	)
	if err != nil {
		logger.Err(err)
	}

	handler := func(ctx *fasthttp.RequestCtx) {

		schemaID, err := strconv.Atoi(string(ctx.Request.Header.Peek("X-Wallarm-Schema-ID")))
		if err != nil {
			logger.Err(err)
		}

		w := new(bytes.Buffer)
		bw := bufio.NewWriter(w)

		if err := ctx.Request.Write(bw); err != nil {
			logger.Err(err)
		}
		if err := bw.Flush(); err != nil {
			logger.Err(err)
		}

		headers := http.Header{}

		ctx.Request.Header.VisitAll(func(k, v []byte) {
			sk := strconv2.B2S(k)
			sv := strconv2.B2S(v)

			headers.Set(sk, sv)
		})

		result, err := apiFirewall.ValidateRequest([]int{schemaID}, ctx.Request.Header.RequestURI(), ctx.Request.Header.Method(), ctx.Request.Body(), headers)
		if err != nil {
			logger.Err(err)
		}

		response, err := json.Marshal(result)
		if err != nil {
			logger.Err(err)

			ctx.Response.Reset()
			ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
		}

		ctx.Response.SetStatusCode(fasthttp.StatusOK)
		ctx.Response.SetBody(response)
	}

	// =========================================================================
	// Init ZeroLogger

	zeroLogger := &config.ZerologAdapter{Logger: logger}

	// =========================================================================
	// Init Regular Update Controller

	updSpecErrors := make(chan error, 1)

	updOpenAPISpec := updater.NewUpdater(logger, apiFirewall, updateTime)

	go func() {
		logger.Info().Msgf("%s: starting specification regular update process every %.0f seconds", logMainPrefix, updateTime.Seconds())
		updSpecErrors <- updOpenAPISpec.Start()
	}()

	serverErrors := make(chan error, 1)

	api := fasthttp.Server{
		Handler:               handler,
		ReadTimeout:           5 * time.Second,
		WriteTimeout:          5 * time.Second,
		Logger:                zeroLogger,
		NoDefaultServerHeader: true,
	}

	// Start the service listening for requests.
	go func() {
		logger.Info().Msgf("%s: API listening on %s", logMainPrefix, apiHost)
		serverErrors <- api.ListenAndServe(apiHost)
	}()

	// Blocking main and waiting for shutdown.
	select {
	case err := <-serverErrors:
		logger.Err(errors.Wrap(err, "server error"))
		return
	case err := <-updSpecErrors:
		logger.Err(errors.Wrap(err, "regular updater error"))
		return
	case sig := <-shutdown:
		logger.Info().Msgf("%s: %v: Start shutdown", logMainPrefix, sig)

		if err := updOpenAPISpec.Shutdown(); err != nil {
			logger.Err(errors.Wrap(err, "could not stop configuration updater gracefully"))
			return
		}

		// Asking listener to shutdown and shed load.
		if err := api.Shutdown(); err != nil {
			logger.Err(errors.Wrap(err, "could not stop server gracefully"))
			return
		}

		logger.Info().Msgf("%s: %v: Completed shutdown", logMainPrefix, sig)
	}

}
