package graphql

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/fasthttp/websocket"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/validator"
	"github.com/wallarm/api-firewall/internal/platform/web"
	"github.com/wundergraph/graphql-go-tools/pkg/graphql"
)

func closeWSConn(ctx *fasthttp.RequestCtx, logger *logrus.Logger, conn proxy.WebSocketConn) {
	if err := conn.SendCloseConnection(websocket.CloseNormalClosure); err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err,
			"protocol":   "websocket",
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
		}).Debug("send close message")
	}

	if err := conn.Close(); err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err,
			"protocol":   "websocket",
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
		}).Error("closing connection")
	}
}

func (h *Handler) HandleWebSocketProxy(ctx *fasthttp.RequestCtx) error {

	// connect to backend
	backendWSConnect, err := h.wsClient.GetConn(ctx)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"error":      err,
			"protocol":   "websocket",
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
		}).Error("Connecting to the server WS error")

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

		// close client WS connection
		defer closeWSConn(ctx, h.logger, clientConn)
		// close backend WS connection
		defer closeWSConn(ctx, h.logger, backendWSConnect)

		// sends messages from client to backend
		go func() {
			defer wg.Done()
			for {
				select {
				case <-errBackend:
					close(errClient)
					return
				default:
					// read message from the client
					messageType, p, err := clientConn.ReadMessage()
					if err != nil {
						if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
							h.logger.WithFields(logrus.Fields{
								"error":      err,
								"protocol":   "websocket",
								"request_id": fmt.Sprintf("#%016X", ctx.ID()),
							}).Debug("read from client")
						}

						close(errClient)
						return
					}

					// write to backend server if request validation is disabled OR
					// websocket message type is not TextMessage or BinaryMessage OR
					// received an empty message
					if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationDisable) || len(p) == 0 ||
						messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {

						if err := backendWSConnect.WriteMessage(messageType, p); err != nil {
							h.logger.WithFields(logrus.Fields{
								"error":      err,
								"protocol":   "websocket",
								"request_id": fmt.Sprintf("#%016X", ctx.ID()),
							}).Debug("write to backend")

							close(errClient)
							return
						}
						continue
					}

					var msg *fastjson.Value

					// try to parse graphql ws message
					if msg, err = jsonParser.ParseBytes(p); err != nil {
						h.logger.WithFields(logrus.Fields{
							"error":      err,
							"protocol":   "websocket",
							"request_id": fmt.Sprintf("#%016X", ctx.ID()),
						}).Error("read from client: request unmarshal")

						// if validation is in log_only mode then the request should be proxied to the backend server
						if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationLog) {
							if err := backendWSConnect.WriteMessage(messageType, p); err != nil {
								h.logger.WithFields(logrus.Fields{
									"error":      err,
									"protocol":   "websocket",
									"request_id": fmt.Sprintf("#%016X", ctx.ID()),
								}).Debug("write to backend")

								close(errClient)
								return
							}
						}

						continue
					}

					msgType := string(msg.Get("type").GetStringBytes())
					msgID := string(msg.Get("id").GetStringBytes())

					// skip message types that do not contain payload
					if msgType != "subscribe" && msgType != "start" {
						if err := backendWSConnect.WriteMessage(messageType, p); err != nil {
							h.logger.WithFields(logrus.Fields{
								"error":      err,
								"protocol":   "websocket",
								"request_id": fmt.Sprintf("#%016X", ctx.ID()),
							}).Debug("write to backend")

							close(errClient)
							return
						}
						continue
					}

					request := new(graphql.Request)

					msgPayload := msg.Get("payload").String()

					// unmarshal the graphql request.
					// send error and complete messages to the client in case of an error occurred and do not proxy request to the backend in BLOCK mode
					// log error and proxy request to the backend server in LOG_ONLY mode
					if err := graphql.UnmarshalRequest(io.NopCloser(strings.NewReader(msgPayload)), request); err != nil {

						h.logger.WithFields(logrus.Fields{
							"error":      err,
							"protocol":   "websocket",
							"payload":    msgPayload,
							"request_id": fmt.Sprintf("#%016X", ctx.ID()),
						}).Error("GraphQL request unmarshal")

						// block request and respond by error in BLOCK mode
						if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationBlock) {
							if err := clientConn.SendError(messageType, msgID, errors.New("invalid graphql request")); err != nil {
								h.logger.WithFields(logrus.Fields{
									"error":      err,
									"protocol":   "websocket",
									"request_id": fmt.Sprintf("#%016X", ctx.ID()),
								}).Debug("write to client")
							}

							if err := clientConn.SendComplete(messageType, msgID); err != nil {
								h.logger.WithFields(logrus.Fields{
									"error":      err,
									"protocol":   "websocket",
									"request_id": fmt.Sprintf("#%016X", ctx.ID()),
								}).Debug("write to client")
							}

							continue
						}
						// send request to the backend server (LOG_ONLY mode)
						if err := backendWSConnect.WriteMessage(messageType, p); err != nil {
							h.logger.WithFields(logrus.Fields{
								"error":      err,
								"protocol":   "websocket",
								"request_id": fmt.Sprintf("#%016X", ctx.ID()),
							}).Debug("write to backend")

							close(errClient)
							return
						}
					}

					// validate request
					// send error and complete messages to the client in case of the APIFW can't validate the request
					// and do not proxy request to the backend
					validationResult, err := validator.ValidateGraphQLRequest(&h.cfg.Graphql, h.schema, request)
					if err != nil {
						h.logger.WithFields(logrus.Fields{
							"error":      err,
							"protocol":   "websocket",
							"request_id": fmt.Sprintf("#%016X", ctx.ID()),
						}).Error("GraphQL query validation")

						// block request and respond by error in BLOCK mode
						if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationBlock) {

							if err := clientConn.SendError(messageType, msgID, errors.New("invalid graphql request")); err != nil {
								h.logger.WithFields(logrus.Fields{
									"error":      err,
									"protocol":   "websocket",
									"request_id": fmt.Sprintf("#%016X", ctx.ID()),
								}).Debug("write to client")
							}

							if err := clientConn.SendComplete(messageType, msgID); err != nil {
								h.logger.WithFields(logrus.Fields{
									"error":      err,
									"protocol":   "websocket",
									"request_id": fmt.Sprintf("#%016X", ctx.ID()),
								}).Debug("write to client")
							}
							continue
						}
						// send request to the backend server
						if err := backendWSConnect.WriteMessage(messageType, p); err != nil {
							h.logger.WithFields(logrus.Fields{
								"error":      err,
								"protocol":   "websocket",
								"request_id": fmt.Sprintf("#%016X", ctx.ID()),
							}).Debug("write to backend")

							close(errClient)
							return
						}
					}

					// send error and complete messages to the client in case of the validation has been failed
					// and do not proxy request to the backend
					if !validationResult.Valid {
						h.logger.WithFields(logrus.Fields{
							"error":      validationResult.Errors,
							"protocol":   "websocket",
							"request_id": fmt.Sprintf("#%016X", ctx.ID()),
						}).Error("GraphQL query validation")

						// block request and respond by error in BLOCK mode
						if strings.EqualFold(h.cfg.Graphql.RequestValidation, web.ValidationBlock) {

							if err := clientConn.SendError(messageType, msgID, validationResult.Errors); err != nil {
								h.logger.WithFields(logrus.Fields{
									"error":      err,
									"protocol":   "websocket",
									"request_id": fmt.Sprintf("#%016X", ctx.ID()),
								}).Debug("write to client")
							}

							if err := clientConn.SendComplete(messageType, msgID); err != nil {
								h.logger.WithFields(logrus.Fields{
									"error":      err,
									"protocol":   "websocket",
									"request_id": fmt.Sprintf("#%016X", ctx.ID()),
								}).Debug("write to client")
							}
							continue
						}
					}

					// send request to the backend server
					if err := backendWSConnect.WriteMessage(messageType, p); err != nil {
						h.logger.WithFields(logrus.Fields{
							"error":      err,
							"protocol":   "websocket",
							"request_id": fmt.Sprintf("#%016X", ctx.ID()),
						}).Debug("write to backend")

						close(errClient)
						return
					}
				}
			}
		}()

		// sends messages from backend to client
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
						h.logger.WithFields(logrus.Fields{
							"error":      err,
							"protocol":   "websocket",
							"request_id": fmt.Sprintf("#%016X", ctx.ID()),
						}).Debug("read from backend")

						close(errBackend)
						return
					}

					if err := clientConn.WriteMessage(messageType, p); err != nil {
						h.logger.WithFields(logrus.Fields{
							"error":      err,
							"protocol":   "websocket",
							"request_id": fmt.Sprintf("#%016X", ctx.ID()),
						}).Debug("write to client")

						close(errBackend)
						return
					}
				}
			}
		}()

		wg.Wait()
	})

	// upgrader will set response status code and add the error message
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"error":      err,
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
		}).Error("WebSocket handler")

		return nil
	}

	return nil
}
