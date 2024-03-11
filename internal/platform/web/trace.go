package web

import (
	"strings"

	"github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

const responseBodyOmitted = "<response body is omitted>"

func LogRequestResponseAtTraceLevel(ctx *fasthttp.RequestCtx, logger *logrus.Logger) {

	if logger.Level < logrus.TraceLevel {
		return
	}

	var strBuild strings.Builder
	ctx.Request.Header.VisitAll(func(key, value []byte) {
		strBuild.WriteString(strconv.B2S(key))
		strBuild.WriteString(":")
		strBuild.WriteString(strconv.B2S(value))
		strBuild.WriteString("\n")
	})

	logger.WithFields(logrus.Fields{
		"request_id":     ctx.UserValue(RequestID),
		"method":         strconv.B2S(ctx.Request.Header.Method()),
		"uri":            strconv.B2S(ctx.Request.URI().RequestURI()),
		"headers":        strings.ReplaceAll(strBuild.String(), "\n", `\r\n`),
		"body":           strings.ReplaceAll(strconv.B2S(ctx.Request.Body()), "\n", `\r\n`),
		"client_address": ctx.RemoteAddr(),
	}).Trace("new request")

	strBuild.Reset()
	ctx.Response.Header.VisitAll(func(key, value []byte) {
		strBuild.WriteString(strconv.B2S(key))
		strBuild.WriteString(":")
		strBuild.WriteString(strconv.B2S(value))
		strBuild.WriteString("\n")
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
		"request_id":     ctx.UserValue(RequestID),
		"status_code":    ctx.Response.StatusCode(),
		"headers":        strings.ReplaceAll(strBuild.String(), "\n", `\r\n`),
		"body":           strings.ReplaceAll(body, "\n", `\r\n`),
		"client_address": ctx.RemoteAddr(),
	}).Trace("response from the API-Firewall")
}
