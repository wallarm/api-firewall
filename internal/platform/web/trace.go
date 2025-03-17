package web

import (
	"strings"

	"github.com/rs/zerolog"
	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
)

const responseBodyOmitted = "<response body is omitted>"

func LogRequestResponseAtTraceLevel(ctx *fasthttp.RequestCtx, logger zerolog.Logger) {

	if logger.GetLevel() != zerolog.TraceLevel {
		return
	}

	var strBuild strings.Builder
	ctx.Request.Header.VisitAll(func(key, value []byte) {
		strBuild.WriteString(strconv.B2S(key))
		strBuild.WriteString(":")
		strBuild.WriteString(strconv.B2S(value))
		strBuild.WriteString("\n")
	})

	logger.Trace().
		Str("request_id", ctx.UserValue(RequestID).(string)).
		Str("method", strconv.B2S(ctx.Request.Header.Method())).
		Str("uri", strconv.B2S(ctx.Request.URI().RequestURI())).
		Str("headers", strings.ReplaceAll(strBuild.String(), "\n", `\r\n`)).
		Str("body", strings.ReplaceAll(strconv.B2S(ctx.Request.Body()), "\n", `\r\n`)).
		Str("client_address", ctx.RemoteAddr().String()).
		Msg("new request")

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

	logger.Trace().
		Str("request_id", ctx.UserValue(RequestID).(string)).
		Int("status_code", ctx.Response.StatusCode()).
		Str("headers", strings.ReplaceAll(strBuild.String(), "\n", `\r\n`)).
		Str("body", strings.ReplaceAll(body, "\n", `\r\n`)).
		Str("client_address", ctx.RemoteAddr().String()).
		Msg("response from the API-Firewall")
}
