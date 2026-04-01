package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/rs/zerolog"
	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
)

var _ WebSocketClient = (*FastHTTPWebSocketClient)(nil)

type WSClientOptions struct {
	Scheme             string
	Host               string
	Path               string
	InsecureConnection bool
	RootCA             string
	DialTimeout        time.Duration
}

// WebSocketClient defines the interface for WebSocket connections Pool
type WebSocketClient interface {
	GetConn(ctx *fasthttp.RequestCtx) (*FastHTTPWebSocketConn, error)
}

// FastHTTPWebSocketClient implements the WebSocketClient interface
type FastHTTPWebSocketClient struct {
	Dialer  *websocket.Dialer
	ConnStr string
	Logger  zerolog.Logger
}

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// wsCopyResponse copies WS first response header from the backend server
func wsCopyResponse(dst *fasthttp.Response, src *http.Response) error {
	for k, vv := range src.Header {
		for _, v := range vv {
			dst.Header.Add(k, v)
		}
	}

	dst.SetStatusCode(src.StatusCode)
	defer src.Body.Close()

	buf := bufferPool.Get().(*bytes.Buffer)
	if _, err := io.Copy(buf, src.Body); err != nil {
		return err
	}
	dst.SetBody(buf.Bytes())

	buf.Reset()
	bufferPool.Put(buf)

	return nil
}

// builtinForwardHeaderHandler built in handler for dealing forward request headers
func builtinForwardHeaderHandler(ctx *fasthttp.RequestCtx) (forwardHeader http.Header) {
	forwardHeader = make(http.Header, 2)

	// Pass headers from the incoming request to the dialer to forward them to
	// the final destinations
	origin := strconv.B2S(ctx.Request.Header.Peek("Origin"))
	if origin != "" {
		forwardHeader.Add("Origin", origin)
	}

	cookie := strconv.B2S(ctx.Request.Header.Peek("Cookie"))
	if cookie != "" {
		forwardHeader.Add("Cookie", cookie)
	}

	return
}

func NewWSClient(logger zerolog.Logger, options *WSClientOptions) (WebSocketClient, error) {

	tlsConfig, err := BuildTLSConfig(options.InsecureConnection, options.RootCA)
	if err != nil {
		return nil, err
	}

	dialer := websocket.Dialer{
		TLSClientConfig:  tlsConfig,
		HandshakeTimeout: options.DialTimeout,
		Subprotocols:     []string{"graphql-ws"},
	}

	return &FastHTTPWebSocketClient{
		Dialer:  &dialer,
		ConnStr: fmt.Sprintf("%s://%s", options.Scheme, path.Join(options.Host, options.Path)),
		Logger:  logger,
	}, nil
}

func (f *FastHTTPWebSocketClient) GetConn(ctx *fasthttp.RequestCtx) (*FastHTTPWebSocketConn, error) {
	backendConn, backendResp, err := f.Dialer.Dial(f.ConnStr, builtinForwardHeaderHandler(ctx))
	if err != nil {
		return nil, err
	}

	// copy response from ws to client response
	if err := wsCopyResponse(&ctx.Response, backendResp); err != nil {
		return nil, err
	}

	return &FastHTTPWebSocketConn{Conn: backendConn, Logger: f.Logger, Ctx: ctx}, nil
}
