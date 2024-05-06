package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/demo/interface/api-mode/internal/updater"
	"github.com/wallarm/api-firewall/pkg/apifw"
)

const logMainPrefix = "Wallarm API-Firewall"

var (
	updateTime = 60 * time.Second
	apiHost    = "0.0.0.0:8080"
)

func main() {

	logger := logrus.New()

	logger.Infof("%s : Started : Application initializing", logMainPrefix)
	defer logger.Infof("%s: Completed", logMainPrefix)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	apiFirewall, err := apifw.NewAPIFirewall(
		apifw.WithPathToDB("./wallarm_apifw_test.db"),
	)
	if err != nil {
		logger.Error(err)
	}

	handler := func(ctx *fasthttp.RequestCtx) {

		schemaID, err := strconv.Atoi(string(ctx.Request.Header.Peek("X-Wallarm-Schema-ID")))
		if err != nil {
			logger.Error(err)
		}

		w := new(bytes.Buffer)
		bw := bufio.NewWriter(w)

		if err := ctx.Request.Write(bw); err != nil {
			logger.Error(err)
		}
		if err := bw.Flush(); err != nil {
			logger.Error(err)
		}

		result, err := apiFirewall.ValidateRequest(schemaID, bufio.NewReader(w))
		if err != nil {
			logger.Error(err)
		}

		response, err := json.Marshal(result)
		if err != nil {
			logger.Error(err)

			ctx.Response.Reset()
			ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
		}

		ctx.Response.SetStatusCode(fasthttp.StatusOK)
		ctx.Response.SetBody(response)
	}

	// =========================================================================
	// Init Regular Update Controller

	updSpecErrors := make(chan error, 1)

	updOpenAPISpec := updater.NewUpdater(logger, apiFirewall, updateTime)

	go func() {
		logger.Infof("%s: starting specification regular update process every %.0f seconds", logMainPrefix, updateTime.Seconds())
		updSpecErrors <- updOpenAPISpec.Start()
	}()

	serverErrors := make(chan error, 1)

	api := fasthttp.Server{
		Handler:               handler,
		ReadTimeout:           5 * time.Second,
		WriteTimeout:          5 * time.Second,
		Logger:                logger,
		NoDefaultServerHeader: true,
	}

	// Start the service listening for requests.
	go func() {
		logger.Infof("%s: API listening on %s", logMainPrefix, apiHost)
		serverErrors <- api.ListenAndServe(apiHost)
	}()

	// Blocking main and waiting for shutdown.
	select {
	case err := <-serverErrors:
		logger.Error(errors.Wrap(err, "server error"))
		return
	case err := <-updSpecErrors:
		logger.Error(errors.Wrap(err, "regular updater error"))
		return
	case sig := <-shutdown:
		logger.Infof("%s: %v: Start shutdown", logMainPrefix, sig)

		if err := updOpenAPISpec.Shutdown(); err != nil {
			logger.Error(errors.Wrap(err, "could not stop configuration updater gracefully"))
			return
		}

		// Asking listener to shutdown and shed load.
		if err := api.Shutdown(); err != nil {
			logger.Error(errors.Wrap(err, "could not stop server gracefully"))
			return
		}

		logger.Infof("%s: %v: Completed shutdown", logMainPrefix, sig)
	}

	return
}
