package web

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

func LogRequestResponseAtTraceLevel(ctx *fasthttp.RequestCtx, logger *logrus.Logger) {
	if logger.Level == logrus.TraceLevel {
		requestHeaders := ""
		ctx.Request.Header.VisitAll(func(key, value []byte) {
			requestHeaders += string(key) + ":" + string(value) + "\n"
		})

		logger.WithFields(logrus.Fields{
			"request_id":     fmt.Sprintf("#%016X", ctx.ID()),
			"method":         string(ctx.Request.Header.Method()),
			"uri":            string(ctx.Request.URI().RequestURI()),
			"headers":        requestHeaders,
			"body":           string(ctx.Request.Body()),
			"client_address": ctx.RemoteAddr(),
		}).Trace("new request")

		responseHeaders := ""
		ctx.Response.Header.VisitAll(func(key, value []byte) {
			responseHeaders += string(key) + ":" + string(value) + "\n"
		})

		logger.WithFields(logrus.Fields{
			"request_id":     fmt.Sprintf("#%016X", ctx.ID()),
			"status_code":    ctx.Response.StatusCode(),
			"headers":        responseHeaders,
			"body":           string(ctx.Response.Body()),
			"client_address": ctx.RemoteAddr(),
		}).Trace("response from the API-Firewall")
	}
}
