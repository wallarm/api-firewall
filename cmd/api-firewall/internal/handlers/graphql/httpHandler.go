package graphql

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/fasthttp/websocket"
	"github.com/rs/zerolog"
	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"golang.org/x/sync/errgroup"

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
	logger     zerolog.Logger
	schema     *graphql.Schema
	parserPool *fastjson.ParserPool
	wsClient   proxy.WebSocketClient
	upgrader   *websocket.FastHTTPUpgrader
	mu         sync.Mutex
}

var (
	ErrNetworkConnection = errors.New("network connection error")
	ErrInvalidQuery      = errors.New("invalid query")
)

// GraphQLHandle performs complexity checks to the GraphQL query and proxy request to the backend if all checks are passed
func (h *Handler) GraphQLHandle(ctx *fasthttp.RequestCtx) error {

	// update the path to the API endpoint (server URL path)
	ctx.Request.URI().SetPath(h.serverURL.Path)

	// handle WS
	if websocket.FastHTTPIsWebSocketUpgrade(ctx) {
		return h.HandleWebSocketProxy(ctx)
	}

	// respond with 403 status code in case of content-type of POST request is not application/json
	if strconv.B2S(ctx.Request.Header.Method()) == fasthttp.MethodPost &&
		!strings.EqualFold(strconv.B2S(ctx.Request.Header.ContentType()), "application/json") {
		h.logger.Debug().
			Str("protocol", "HTTP").
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Msg("POST request without application/json content-type is received")

		return web.RespondError(ctx, fasthttp.StatusForbidden, "")
	}

	// respond with 403 status code in case of lack of "query" query parameter in GET request
	if strconv.B2S(ctx.Request.Header.Method()) == fasthttp.MethodGet &&
		len(ctx.Request.URI().QueryArgs().Peek("query")) == 0 {
		h.logger.Debug().
			Str("protocol", "HTTP").
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Msg("GET request without \"query\" query parameter is received")

		ctx.Response.SetStatusCode(fasthttp.StatusBadRequest)
		return web.RespondGraphQLErrors(&ctx.Response, ErrInvalidQuery)
	}

	// Proxy request if the validation is disabled
	if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationDisable) {
		if err := proxy.Perform(ctx, h.proxyPool, h.cfg.Server.RequestHostHeader); err != nil {
			h.logger.Error().
				Err(err).
				Str("protocol", "HTTP").
				Interface("request_id", ctx.UserValue(web.RequestID)).
				Msg("request proxying")

			ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
			return web.RespondGraphQLErrors(&ctx.Response, ErrNetworkConnection)
		}
		return nil
	}

	gqlRequest, err := validator.ParseGraphQLRequest(ctx, h.parserPool)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("protocol", "HTTP").
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Msg("GraphQL request unmarshal")

		if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationBlock) {
			return web.RespondGraphQLErrors(&ctx.Response, ErrInvalidQuery)
		}
	}

	// batch query limit
	if h.cfg.Graphql.BatchQueryLimit > 0 && h.cfg.Graphql.BatchQueryLimit < len(gqlRequest) {
		h.logger.Error().
			Err(fmt.Errorf("the batch query limit has been exceeded. The number of queries in the batch is %d. The current batch query limit is %d", len(gqlRequest), h.cfg.Graphql.BatchQueryLimit)).
			Str("protocol", "HTTP").
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Msg("GraphQL query validation")

		if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationBlock) {
			return web.RespondGraphQLErrors(&ctx.Response, ErrInvalidQuery)
		}
	}

	eg := errgroup.Group{}

	for _, req := range gqlRequest {
		eg.Go(func() error {
			// validate request
			if gqlRequest != nil {
				validationResult, err := validator.ValidateGraphQLRequest(&h.cfg.Graphql, h.schema, &req)
				// internal errors
				if err != nil {
					h.logger.Error().
						Err(err).
						Str("protocol", "HTTP").
						Interface("request_id", ctx.UserValue(web.RequestID)).
						Msg("GraphQL query validation")
					return err
				}

				// validation failed
				if !validationResult.Valid && validationResult.Errors != nil {
					h.logger.Error().
						Err(validationResult.Errors).
						Str("protocol", "HTTP").
						Interface("request_id", ctx.UserValue(web.RequestID)).
						Msg("GraphQL query validation")

					return validationResult.Errors
				}
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationBlock) {
			return web.RespondGraphQLErrors(&ctx.Response, ErrInvalidQuery)
		}
	}

	if err := proxy.Perform(ctx, h.proxyPool, h.cfg.Server.RequestHostHeader); err != nil {
		h.logger.Error().
			Err(err).
			Str("protocol", "HTTP").
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Msg("request proxying")

		ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
		return web.RespondGraphQLErrors(&ctx.Response, ErrNetworkConnection)
	}

	return nil
}
