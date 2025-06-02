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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	strconv2 "github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"

	"github.com/wallarm/api-firewall/demo/interface/api-mode/internal/updater"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/pkg/APIMode"
)

const logMainPrefix = "Wallarm API-Firewall"

var (
	updateTime = 60 * time.Second
	apiHost    = "0.0.0.0:8282"
)

func main() {

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	logger.Info().Msgf("%s : Started : Application initializing", logMainPrefix)
	defer logger.Info().Msgf("%s: Completed", logMainPrefix)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	apiFirewall, err := APIMode.NewAPIFirewall(
		APIMode.WithPathToDB("pkg/APIMode/wallarm_apifw_test.db"),
		APIMode.EnablePrometheusMetrics(),
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

	// Start Metrics logging
	metrics, err := apiFirewall.GetPrometheusCollectors()
	if err != nil {
		logger.Err(err).Msgf("%s: error getting prometheus metrics: %s", logMainPrefix, err)
	}

	metricsErrors := make(chan error, 1)

	if metrics != nil {
		reg := prometheus.NewRegistry()
		reg.MustRegister(metrics...)

		fastPrometheusHandler := fasthttpadaptor.NewFastHTTPHandler(promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
		metricsHandler := func(ctx *fasthttp.RequestCtx) {
			switch string(ctx.Path()) {
			case "/metrics":
				fastPrometheusHandler(ctx)
				return
			default:
				ctx.Error("Unsupported path", fasthttp.StatusNotFound)
			}
		}

		metricsAPI := fasthttp.Server{
			Handler:               metricsHandler,
			NoDefaultServerHeader: true,
			Logger:                &logger,
		}

		// Start the service listening for requests.
		go func() {
			logger.Info().Msgf("%s:Prometheus Metrics: API listening on 0.0.0.0:9010/metrics", logMainPrefix)
			metricsErrors <- metricsAPI.ListenAndServe("0.0.0.0:9010")
		}()
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
	case err := <-metricsErrors:
		logger.Err(errors.Wrap(err, "metrics error"))
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
