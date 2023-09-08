package web

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

const responseBodyOmitted = "<response body is omitted>"

func LogRequestResponseAtTraceLevel(ctx *fasthttp.RequestCtx, logger *logrus.Logger) {

	if logger.Level < logrus.TraceLevel {
		return
	}

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

	isPlayground := false
	if ctx.UserValue(Playground) != nil {
		isPlayground = true
	}

	body := responseBodyOmitted
	if !isPlayground {
		body = string(ctx.Response.Body())
	}

	logger.WithFields(logrus.Fields{
		"request_id":     fmt.Sprintf("#%016X", ctx.ID()),
		"status_code":    ctx.Response.StatusCode(),
		"headers":        responseHeaders,
		"body":           body,
		"client_address": ctx.RemoteAddr(),
	}).Trace("response from the API-Firewall")
}
