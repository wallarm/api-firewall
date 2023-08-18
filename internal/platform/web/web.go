package web

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/fasthttp/router"
	"github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

const (
	Playground = "playground"

	ValidationStatus = "APIFW-Validation-Status"

	XWallarmSchemaIDHeader = "X-WALLARM-SCHEMA-ID"
	WallarmSchemaID        = "WallarmSchemaID"

	ValidationDisable = "disable"
	ValidationBlock   = "block"
	ValidationLog     = "log_only"

	RequestProxyNoRoute    = "proxy_no_route"
	RequestProxyFailed     = "proxy_failed"
	RequestBlocked         = "request_blocked"
	ResponseBlocked        = "response_blocked"
	ResponseStatusNotFound = "response_status_not_found"

	APIMode     = "api"
	ProxyMode   = "proxy"
	GraphQLMode = "graphql"

	AnyMethod = "any"
)

// A Handler is a type that handles an http request within our own little mini
// framework.
type Handler func(ctx *fasthttp.RequestCtx) error

// App is the entrypoint into our application and what configures our context
// object for each of our http handlers. Feel free to add any configuration
// data/logic on this App struct
type App struct {
	Router   *router.Router
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
}

func (a *App) SetDefaultBehavior(handler Handler, mw ...Middleware) {
	// First wrap handler specific middleware around this handler.
	handler = wrapMiddleware(mw, handler)

	// Add the application's general middleware to the handler chain.
	handler = wrapMiddleware(a.mw, handler)

	customHandler := func(ctx *fasthttp.RequestCtx) {

		// Block request if it's not found in the route. Not for API mode.
		if strings.EqualFold(a.Options.Mode, ProxyMode) {
			if strings.EqualFold(a.Options.RequestValidation, ValidationBlock) || strings.EqualFold(a.Options.ResponseValidation, ValidationBlock) {
				a.Log.WithFields(logrus.Fields{
					"request_id":     fmt.Sprintf("#%016X", ctx.ID()),
					"method":         bytes.NewBuffer(ctx.Request.Header.Method()).String(),
					"path":           string(ctx.Path()),
					"client_address": ctx.RemoteAddr(),
				}).Info("request blocked")

				ctx.Error("", a.Options.CustomBlockStatusCode)

				return
			}
		}

		if err := handler(ctx); err != nil {
			a.SignalShutdown()
			return
		}

	}

	//Set NOT FOUND behavior
	a.Router.NotFound = customHandler

	// Set Method Not Allowed behavior
	a.Router.MethodNotAllowed = customHandler
}

// NewApp creates an App value that handle a set of routes for the application.
func NewApp(options *AppAdditionalOptions, shutdown chan os.Signal, logger *logrus.Logger, mw ...Middleware) *App {

	app := App{
		Router:   router.New(),
		shutdown: shutdown,
		mw:       mw,
		Log:      logger,
		Options:  options,
	}

	app.Router.HandleOPTIONS = options.PassOptions

	return &app
}

// Handle is our mechanism for mounting Handlers for a given HTTP verb and path
// pair, this makes for really easy, convenient routing.
func (a *App) Handle(method string, path string, handler Handler, mw ...Middleware) {

	// First wrap handler specific middleware around this handler.
	handler = wrapMiddleware(mw, handler)

	// Add the application's general middleware to the handler chain.
	handler = wrapMiddleware(a.mw, handler)

	// The function to execute for each request.
	h := func(ctx *fasthttp.RequestCtx) {

		if err := handler(ctx); err != nil {
			a.SignalShutdown()
			return
		}

		// if pass request with OPTIONS method is enabled then log reques
		if ctx.Response.StatusCode() == fasthttp.StatusOK && a.Options.PassOptions && strconv.B2S(ctx.Method()) == fasthttp.MethodOptions {
			a.Log.WithFields(logrus.Fields{
				"request_id": fmt.Sprintf("#%016X", ctx.ID()),
			}).Debug("pass request with OPTIONS method")
		}
	}

	if method == AnyMethod {
		a.Router.ANY(path, h)
		return
	}

	// Add this handler for the specified verb and route.
	a.Router.Handle(method, path, h)
}

// SignalShutdown is used to gracefully shutdown the app when an integrity
// issue is identified.
func (a *App) SignalShutdown() {
	a.shutdown <- syscall.SIGTERM
}
