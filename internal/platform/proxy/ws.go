package proxy

import (
	"bytes"
	"encoding/json"
	"sync"
	"unsafe"

	"github.com/fasthttp/websocket"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/web"
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
	Logger zerolog.Logger
	Ctx    *fasthttp.RequestCtx
	mu     sync.Mutex
}

type GqlWSErrorMessage struct {
	ID      string                `json:"id"`
	Type    string                `json:"type"`
	Payload graphql.RequestErrors `json:"payload,omitempty"`
}

func (f *FastHTTPWebSocketConn) ReadMessage() (messageType int, p []byte, err error) {
	if f.Ctx != nil && f.Logger.GetLevel() == zerolog.TraceLevel {
		f.Logger.Trace().
			Str("protocol", "websocket").
			Str("local_addr", f.Conn.LocalAddr().String()).
			Str("remote_addr", f.Conn.RemoteAddr().String()).
			Bytes("message", p).
			Int("message_type", messageType).
			Interface("request_id", f.Ctx.UserValue(web.RequestID)).
			Msg("read message")

	}
	return f.Conn.ReadMessage()
}

func (f *FastHTTPWebSocketConn) WriteMessage(messageType int, data []byte) error {
	if f.Ctx != nil && f.Logger.GetLevel() == zerolog.TraceLevel {
		f.Logger.Trace().
			Str("protocol", "websocket").
			Str("local_addr", f.Conn.LocalAddr().String()).
			Str("remote_addr", f.Conn.RemoteAddr().String()).
			Bytes("message", data).
			Int("message_type", messageType).
			Interface("request_id", f.Ctx.UserValue(web.RequestID)).
			Msg("write message")
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

	buf := bufferPool.Get().(*bytes.Buffer)
	defer buf.Reset()
	defer bufferPool.Put(buf)

	buf.WriteString("{\"id\":\"")
	buf.WriteString(id)
	buf.WriteString("\",\"type\":\"complete\"}")

	completeMsg := unsafe.Slice(unsafe.StringData(buf.String()), buf.Len())
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
