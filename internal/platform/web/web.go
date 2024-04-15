package web

import (
	"bytes"
	"os"
	"runtime/debug"
	"syscall"

	"github.com/google/uuid"
	"github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/router"
)

const (
	Playground = "playground"

	ValidationStatus = "APIFW-Validation-Status"

	XWallarmSchemaIDHeader = "X-WALLARM-SCHEMA-ID"

	ValidationDisable = "disable"
	ValidationBlock   = "block"
	ValidationLog     = "log_only"

	PassRequestOPTIONS     = "proxy_request_with_options_method"
	RequestProxyFailed     = "proxy_failed"
	RequestProxyNoRoute    = "proxy_no_route"
	RequestBlocked         = "request_blocked"
	ResponseBlocked        = "response_blocked"
	ResponseStatusNotFound = "response_status_not_found"

	APIMode     = "api"
	ProxyMode   = "proxy"
	GraphQLMode = "graphql"

	RequestID = "__wallarm_apifw_request_id"
)

// App is the entrypoint into our application and what configures our context
// object for each of our http handlers. Feel free to add any configuration
// data/logic on this App struct
type App struct {
	Router   *router.Mux
	Log      *logrus.Logger
	shutdown chan os.Signal
	mw       []Middleware
	Options  *AppAdditionalOptions
}

type AppAdditionalOptions struct {
	Mode                  string
	PassOptions           bool
	RequestValidation     string
	ResponseValidation    string
	CustomBlockStatusCode int
	OptionsHandler        fasthttp.RequestHandler
	DefaultHandler        router.Handler
}

// NewApp creates an App value that handle a set of routes for the application.
func NewApp(options *AppAdditionalOptions, shutdown chan os.Signal, logger *logrus.Logger, mw ...Middleware) *App {

	app := App{
		Router:   router.NewRouter(),
		shutdown: shutdown,
		mw:       mw,
		Log:      logger,
		Options:  options,
	}

	return &app
}

// Handle is our mechanism for mounting Handlers for a given HTTP verb and path
// pair, this makes for really easy, convenient routing.
func (a *App) Handle(method string, path string, handler router.Handler, mw ...Middleware) error {

	// First wrap handler specific middleware around this handler.
	handler = WrapMiddleware(mw, handler)

	// Add the application's general middleware to the handler chain.
	handler = WrapMiddleware(a.mw, handler)

	// The function to execute for each request.
	h := func(ctx *fasthttp.RequestCtx) error {

		// Add request ID
		ctx.SetUserValue(RequestID, uuid.NewString())

		if err := handler(ctx); err != nil {
			a.SignalShutdown()
			return err
		}

		return nil
	}

	// Add this handler for the specified verb and route.
	//a.Router.Handle(method, path, h)
	if err := a.Router.AddEndpoint(method, path, h); err != nil {
		return err
	}

	return nil
}

// MainHandler routes request to the OpenAPI validator (handler)
func (a *App) MainHandler(ctx *fasthttp.RequestCtx) {

	// handle panic
	defer func() {
		if r := recover(); r != nil {
			a.Log.Errorf("panic: %v", r)

			// Log the Go stack trace for this panic'd goroutine.
			a.Log.Debugf("%s", debug.Stack())
			return
		}
	}()

	// Add request ID
	ctx.SetUserValue(RequestID, uuid.NewString())

	// find the handler with the OAS information
	rctx := router.NewRouteContext()
	handler := a.Router.Find(rctx, strconv.B2S(ctx.Method()), strconv.B2S(ctx.Request.URI().Path()))

	// handler not found in the OAS
	if handler == nil {

		// OPTIONS methods are passed if the passOPTIONS is set to true
		if a.Options.PassOptions == true && strconv.B2S(ctx.Method()) == fasthttp.MethodOptions {

			ctx.SetUserValue(PassRequestOPTIONS, true)

			a.Log.WithFields(logrus.Fields{
				"host":       strconv.B2S(ctx.Request.Header.Host()),
				"path":       strconv.B2S(ctx.Path()),
				"method":     strconv.B2S(ctx.Request.Header.Method()),
				"request_id": ctx.UserValue(RequestID),
			}).Debug("Pass request with OPTIONS method")

			// proxy request if passOptions flag is set to true and request method is OPTIONS
			if err := a.Options.DefaultHandler(ctx); err != nil {
				a.Log.WithFields(logrus.Fields{
					"error":      err,
					"host":       strconv.B2S(ctx.Request.Header.Host()),
					"path":       strconv.B2S(ctx.Path()),
					"method":     strconv.B2S(ctx.Request.Header.Method()),
					"request_id": ctx.UserValue(RequestID),
				}).Error("Error in the request handler")
			}
			return
		}

		a.Log.WithFields(logrus.Fields{
			"request_id":     ctx.UserValue(RequestID),
			"method":         bytes.NewBuffer(ctx.Request.Header.Method()).String(),
			"host":           string(ctx.Request.Header.Host()),
			"path":           string(ctx.Path()),
			"client_address": ctx.RemoteAddr(),
		}).Info("Path or method not found")

		// block request if the GraphQL endpoint not found
		if a.Options.Mode == GraphQLMode {
			RespondError(ctx, fasthttp.StatusForbidden, "")
			return
		}

		// handle request by default handler in the endpoint not found in Proxy mode
		// Default handler is used to handle request and response validation logic
		if err := a.Options.DefaultHandler(ctx); err != nil {
			a.Log.WithFields(logrus.Fields{
				"error":      err,
				"host":       strconv.B2S(ctx.Request.Header.Host()),
				"path":       strconv.B2S(ctx.Path()),
				"method":     strconv.B2S(ctx.Request.Header.Method()),
				"request_id": ctx.UserValue(RequestID),
			}).Error("Error in the request handler")
		}

		return
	}

	// add router context to get URL params in the Handler
	ctx.SetUserValue(router.RouteCtxKey, rctx)

	if err := handler(ctx); err != nil {
		a.Log.WithFields(logrus.Fields{
			"error":      err,
			"host":       strconv.B2S(ctx.Request.Header.Host()),
			"path":       strconv.B2S(ctx.Path()),
			"method":     strconv.B2S(ctx.Request.Header.Method()),
			"request_id": ctx.UserValue(RequestID),
		}).Error("Error in the request handler")
	}

	// delete Allow header which is set by the router
	ctx.Response.Header.Del(fasthttp.HeaderAllow)
}

// SignalShutdown is used to gracefully shutdown the app when an integrity
// issue is identified.
func (a *App) SignalShutdown() {
	a.shutdown <- syscall.SIGTERM
}
