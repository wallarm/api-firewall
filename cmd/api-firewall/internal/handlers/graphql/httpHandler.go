package graphql

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/fasthttp/websocket"
	"github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/validator"
	"github.com/wallarm/api-firewall/internal/platform/web"
	"github.com/wundergraph/graphql-go-tools/pkg/graphql"
)

type Handler struct {
	cfg        *config.GraphQLMode
	serverURL  *url.URL
	proxyPool  proxy.Pool
	logger     *logrus.Logger
	schema     *graphql.Schema
	parserPool *fastjson.ParserPool
	wsClient   proxy.WebSocketClient
	upgrader   *websocket.FastHTTPUpgrader
	mu         sync.Mutex
}

var ErrNetworkConnection = errors.New("network connection error")

// GraphQLHandle performs complexity checks to the GraphQL query and proxy request to the backend if all checks are passed
func (h *Handler) GraphQLHandle(ctx *fasthttp.RequestCtx) error {

	// handle WS
	if websocket.FastHTTPIsWebSocketUpgrade(ctx) {
		return h.HandleWebSocketProxy(ctx)
	}

	// respond with 403 status code in case of content-type of POST request is not application/json
	if strconv.B2S(ctx.Request.Header.Method()) == fasthttp.MethodPost &&
		!bytes.EqualFold(ctx.Request.Header.ContentType(), []byte("application/json")) {
		h.logger.WithFields(logrus.Fields{
			"protocol":   "HTTP",
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
		}).Debug("POST request without application/json content-type is received")

		return web.RespondError(ctx, fasthttp.StatusForbidden, "")
	}

	// respond with 403 status code in case of lack of "query" query parameter in GET request
	if strconv.B2S(ctx.Request.Header.Method()) == fasthttp.MethodGet &&
		len(ctx.Request.URI().QueryArgs().Peek("query")) == 0 {
		h.logger.WithFields(logrus.Fields{
			"protocol":   "HTTP",
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
		}).Debug("GET request without \"query\" query parameter is received")

		return web.RespondError(ctx, fasthttp.StatusForbidden, "")
	}

	// Proxy request if the validation is disabled
	if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationDisable) {
		if err := proxy.Perform(ctx, h.proxyPool); err != nil {
			h.logger.WithFields(logrus.Fields{
				"error":      err,
				"protocol":   "HTTP",
				"request_id": fmt.Sprintf("#%016X", ctx.ID()),
			}).Error("request proxying")

			ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
			return web.RespondGraphQLErrors(&ctx.Response, ErrNetworkConnection)
		}
		return nil
	}

	gqlRequest, err := validator.ParseGraphQLRequest(ctx, h.schema)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"error":      err,
			"protocol":   "HTTP",
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
		}).Error("GraphQL request unmarshal")

		if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationBlock) {
			return web.RespondGraphQLErrors(&ctx.Response, err)
		}
	}

	// validate request
	if gqlRequest != nil {
		validationResult, err := validator.ValidateGraphQLRequest(&h.cfg.Graphql, h.schema, gqlRequest)
		// internal errors
		if err != nil {
			h.logger.WithFields(logrus.Fields{
				"error":      err,
				"protocol":   "HTTP",
				"request_id": fmt.Sprintf("#%016X", ctx.ID()),
			}).Error("GraphQL query validation")

			if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationBlock) {
				return web.RespondGraphQLErrors(&ctx.Response, err)
			}
		}

		// validation failed
		if !validationResult.Valid && validationResult.Errors != nil {
			h.logger.WithFields(logrus.Fields{
				"error":      validationResult.Errors,
				"protocol":   "HTTP",
				"request_id": fmt.Sprintf("#%016X", ctx.ID()),
			}).Error("GraphQL query validation")

			if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationBlock) {
				return web.RespondGraphQLErrors(&ctx.Response, validationResult.Errors)
			}
		}
	}

	if err := proxy.Perform(ctx, h.proxyPool); err != nil {
		h.logger.WithFields(logrus.Fields{
			"error":      err,
			"protocol":   "HTTP",
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
		}).Error("request proxying")

		ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
		return web.RespondGraphQLErrors(&ctx.Response, ErrNetworkConnection)
	}

	return nil
}
