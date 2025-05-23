package graphql

import (
	"errors"
	"strings"
	"sync"

	"github.com/fasthttp/websocket"
	"github.com/rs/zerolog"
	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wundergraph/graphql-go-tools/pkg/graphql"

	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/validator"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

func closeWSConn(ctx *fasthttp.RequestCtx, logger zerolog.Logger, conn proxy.WebSocketConn) {
	if err := conn.SendCloseConnection(websocket.CloseNormalClosure); err != nil {
		logger.Debug().
			Err(err).
			Str("protocol", "websocket").
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Msg("Send close message")
	}

	if err := conn.Close(); err != nil {
		logger.Error().
			Err(err).
			Str("protocol", "websocket").
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Msg("Closing connection")
	}
}

func (h *Handler) HandleWebSocketProxy(ctx *fasthttp.RequestCtx) error {

	// Connect to backend
	backendWSConnect, err := h.wsClient.GetConn(ctx)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("protocol", "websocket").
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Msg("Connecting to the server WS error")

		return web.RespondError(ctx, fasthttp.StatusServiceUnavailable, "")
	}

	// Get fastjson parser
	jsonParser := h.parserPool.Get()
	defer h.parserPool.Put(jsonParser)

	var wg sync.WaitGroup
	wg.Add(2)

	errClient := make(chan struct{}, 1)
	errBackend := make(chan struct{}, 1)

	err = h.upgrader.Upgrade(ctx, func(clientConnPub *websocket.Conn) {

		clientConn := &proxy.FastHTTPWebSocketConn{Conn: clientConnPub, Logger: h.logger, Ctx: ctx}

		// Close client WS connection
		defer closeWSConn(ctx, h.logger, clientConn)
		// Close backend WS connection
		defer closeWSConn(ctx, h.logger, backendWSConnect)

		// Send messages from client to backend
		go func() {
			defer wg.Done()
			for {
				select {
				case <-errBackend:
					close(errClient)
					return
				default:
					// Read message from the client
					messageType, p, err := clientConn.ReadMessage()
					if err != nil {
						if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
							h.logger.Debug().
								Err(err).
								Str("protocol", "websocket").
								Interface("request_id", ctx.UserValue(web.RequestID)).
								Msg("Read from client")
						}

						close(errClient)
						return
					}

					// Write to backend server if request validation is disabled OR
					// websocket message type is not TextMessage or BinaryMessage OR
					// received an empty message
					if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationDisable) || len(p) == 0 ||
						messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {

						if err := backendWSConnect.WriteMessage(messageType, p); err != nil {
							h.logger.Debug().
								Err(err).
								Str("protocol", "websocket").
								Interface("request_id", ctx.UserValue(web.RequestID)).
								Msg("Write to backend")

							close(errClient)
							return
						}
						continue
					}

					var msg *fastjson.Value

					// Try to parse graphql WS message
					if msg, err = jsonParser.ParseBytes(p); err != nil {
						h.logger.Error().
							Err(err).
							Str("protocol", "websocket").
							Interface("request_id", ctx.UserValue(web.RequestID)).
							Msg("read from client: request unmarshal")

						// If validation is in log_only mode then the request should be proxied to the backend server
						if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationLog) {
							if err := backendWSConnect.WriteMessage(messageType, p); err != nil {
								h.logger.Debug().
									Err(err).
									Str("protocol", "websocket").
									Interface("request_id", ctx.UserValue(web.RequestID)).
									Msg("Write to backend")

								close(errClient)
								return
							}
						}

						continue
					}

					msgType := strconv.B2S(msg.Get("type").GetStringBytes())
					msgID := strconv.B2S(msg.Get("id").GetStringBytes())

					// Skip message types that do not contain payload
					if msgType != "subscribe" && msgType != "start" {
						if err := backendWSConnect.WriteMessage(messageType, p); err != nil {
							h.logger.Debug().
								Err(err).
								Str("protocol", "websocket").
								Interface("request_id", ctx.UserValue(web.RequestID)).
								Msg("Write to backend")

							close(errClient)
							return
						}
						continue
					}

					request := new(graphql.Request)

					msgPayload := msg.Get("payload")

					query := msgPayload.GetStringBytes("query")
					opName := msgPayload.GetStringBytes("operationName")

					request.OperationName = strconv.B2S(opName)
					request.Query = strconv.B2S(query)
					request.Variables = msgPayload.Get("variables").GetStringBytes()

					// Validate request
					// Send error and complete messages to the client in case of the APIFW can't validate the request
					// and do not proxy request to the backend
					validationResult, err := validator.ValidateGraphQLRequest(&h.cfg.Graphql, h.schema, request)
					if err != nil {
						h.logger.Error().
							Err(err).
							Str("protocol", "websocket").
							Interface("request_id", ctx.UserValue(web.RequestID)).
							Msg("GraphQL query validation")

						// Block request and respond by error in BLOCK mode
						if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationBlock) {

							if err := clientConn.SendError(messageType, msgID, errors.New("invalid graphql request")); err != nil {
								h.logger.Debug().
									Err(err).
									Str("protocol", "websocket").
									Interface("request_id", ctx.UserValue(web.RequestID)).
									Msg("Write to client")
							}

							if err := clientConn.SendComplete(messageType, msgID); err != nil {
								h.logger.Debug().
									Err(err).
									Str("protocol", "websocket").
									Interface("request_id", ctx.UserValue(web.RequestID)).
									Msg("Write to client")
							}
							continue
						}
						// Send request to the backend server
						if err := backendWSConnect.WriteMessage(messageType, p); err != nil {
							h.logger.Debug().
								Err(err).
								Str("protocol", "websocket").
								Interface("request_id", ctx.UserValue(web.RequestID)).
								Msg("Write to backend")

							close(errClient)
							return
						}
					}

					// Send error and complete messages to the client in case of the validation has been failed
					// and do not proxy request to the backend
					if !validationResult.Valid {
						h.logger.Error().
							Err(validationResult.Errors).
							Str("protocol", "websocket").
							Interface("request_id", ctx.UserValue(web.RequestID)).
							Msg("GraphQL query validation")

						// Block request and respond by error in BLOCK mode
						if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationBlock) {

							if err := clientConn.SendError(messageType, msgID, validationResult.Errors); err != nil {
								h.logger.Debug().
									Err(err).
									Str("protocol", "websocket").
									Interface("request_id", ctx.UserValue(web.RequestID)).
									Msg("Write to client")
							}

							if err := clientConn.SendComplete(messageType, msgID); err != nil {
								h.logger.Debug().
									Err(err).
									Str("protocol", "websocket").
									Interface("request_id", ctx.UserValue(web.RequestID)).
									Msg("Write to client")
							}
							continue
						}
					}

					// Send request to the backend server
					if err := backendWSConnect.WriteMessage(messageType, p); err != nil {
						h.logger.Debug().
							Err(err).
							Str("protocol", "websocket").
							Interface("request_id", ctx.UserValue(web.RequestID)).
							Msg("Write to backend")

						close(errClient)
						return
					}
				}
			}
		}()

		// Send messages from backend to client
		go func() {
			defer wg.Done()
			for {
				select {
				case <-errClient:
					close(errBackend)
					return
				default:
					messageType, p, err := backendWSConnect.ReadMessage()
					if err != nil {
						h.logger.Debug().
							Err(err).
							Str("protocol", "websocket").
							Interface("request_id", ctx.UserValue(web.RequestID)).
							Msg("Read from backend")

						close(errBackend)
						return
					}

					if err := clientConn.WriteMessage(messageType, p); err != nil {
						h.logger.Debug().
							Err(err).
							Str("protocol", "websocket").
							Interface("request_id", ctx.UserValue(web.RequestID)).
							Msg("Write to client")

						close(errBackend)
						return
					}
				}
			}
		}()

		wg.Wait()
	})

	// The upgrader sets response status code and adds the error message
	if err != nil {
		h.logger.Error().
			Err(err).
			Interface("request_id", ctx.UserValue(web.RequestID)).
			Msg("WebSocket handler")

		return nil
	}

	return nil
}
