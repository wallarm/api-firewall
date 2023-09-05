package proxy

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/fasthttp/websocket"
	"github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wundergraph/graphql-go-tools/pkg/graphql"
)

var _ WebSocketConn = (*FastHTTPWebSocketConn)(nil)

// WebSocketConn defines the interface for WebSocket connections
type WebSocketConn interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteMessage(messageType int, data []byte) error
	SendError(messageType int, msgID string, requestErrors error) error
	SendComplete(messageType int, id string) error
	SendCloseConnection(closeType int) error
	Close() error
}

// FastHTTPWebSocketConn implements the WebSocketConn interface
type FastHTTPWebSocketConn struct {
	Conn   *websocket.Conn
	Logger *logrus.Logger
	Ctx    *fasthttp.RequestCtx
	mu     sync.Mutex
}

type GqlWSErrorMessage struct {
	ID      string                `json:"id"`
	Type    string                `json:"type"`
	Payload graphql.RequestErrors `json:"payload,omitempty"`
}

func (f *FastHTTPWebSocketConn) ReadMessage() (messageType int, p []byte, err error) {
	if f.Logger != nil && f.Ctx != nil && f.Logger.Level == logrus.TraceLevel {
		f.Logger.WithFields(logrus.Fields{
			"protocol":     "websocket",
			"local_addr":   f.Conn.LocalAddr().String(),
			"remote_addr":  f.Conn.RemoteAddr().String(),
			"message":      strconv.B2S(p),
			"message_type": messageType,
			"request_id":   fmt.Sprintf("#%016X", f.Ctx.ID()),
		}).Trace("read message")
	}
	return f.Conn.ReadMessage()
}

func (f *FastHTTPWebSocketConn) WriteMessage(messageType int, data []byte) error {
	if f.Logger != nil && f.Ctx != nil && f.Logger.Level == logrus.TraceLevel {
		f.Logger.WithFields(logrus.Fields{
			"protocol":     "websocket",
			"local_addr":   f.Conn.LocalAddr().String(),
			"remote_addr":  f.Conn.RemoteAddr().String(),
			"message":      strconv.B2S(data),
			"message_type": messageType,
			"request_id":   fmt.Sprintf("#%016X", f.Ctx.ID()),
		}).Trace("write message")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Conn.WriteMessage(messageType, data)
}

func (f *FastHTTPWebSocketConn) Close() error {
	return f.Conn.Close()
}

func (f *FastHTTPWebSocketConn) SendError(messageType int, msgID string, requestErrors error) error {

	wsMsg := GqlWSErrorMessage{
		ID:      msgID,
		Type:    "error",
		Payload: graphql.RequestErrorsFromError(requestErrors),
	}

	msg, err := json.Marshal(wsMsg)
	if err != nil {
		return err
	}

	if err := f.WriteMessage(messageType, msg); err != nil {
		return err
	}

	return nil
}

func (f *FastHTTPWebSocketConn) SendComplete(messageType int, id string) error {

	completeMsg := []byte(fmt.Sprintf("{\"id\":%q,\"type\":\"complete\"}", id))
	if err := f.WriteMessage(messageType, completeMsg); err != nil {
		return err
	}

	return nil
}

func (f *FastHTTPWebSocketConn) SendCloseConnection(closeType int) error {
	errMsg := websocket.FormatCloseMessage(closeType, "")
	if err := f.WriteMessage(websocket.CloseMessage, errMsg); err != nil {
		return err
	}

	return nil
}
