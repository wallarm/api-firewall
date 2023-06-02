package web

import (
	"errors"
	"fmt"
	"os"
	strconv2 "strconv"
	"sync"
	"syscall"

	"github.com/fasthttp/router"
	"github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/database"
)

// Apps is the entrypoint into our application and what configures our context
// object for each of our http handlers. Feel free to add any configuration
// data/logic on this App struct
type Apps struct {
	Routers     map[int]*router.Router
	Log         *logrus.Logger
	passOPTIONS bool
	shutdown    chan os.Signal
	mw          []Middleware
	storedSpecs database.DBOpenAPILoader
	lock        *sync.RWMutex
}

func (a *Apps) SetDefaultBehavior(schemaID int, handler Handler, mw ...Middleware) {
	// First wrap handler specific middleware around this handler.
	handler = wrapMiddleware(mw, handler)

	// Add the application's general middleware to the handler chain.
	handler = wrapMiddleware(a.mw, handler)

	customHandler := func(ctx *fasthttp.RequestCtx) {

		if err := handler(ctx); err != nil {
			a.SignalShutdown()
			return
		}

	}

	//Set NOT FOUND behavior
	a.Routers[schemaID].NotFound = customHandler

	// Set Method Not Allowed behavior
	a.Routers[schemaID].MethodNotAllowed = customHandler
}

// NewApps creates an Apps value that handle a set of routes for the set of application.
func NewApps(lock *sync.RWMutex, passOPTIONS bool, storedSpecs database.DBOpenAPILoader, shutdown chan os.Signal, logger *logrus.Logger, mw ...Middleware) *Apps {

	schemaIDs := storedSpecs.SchemaIDs()

	// init routers
	routers := make(map[int]*router.Router)
	for _, schemaID := range schemaIDs {
		routers[schemaID] = router.New()
		routers[schemaID].HandleOPTIONS = passOPTIONS
	}

	app := Apps{
		Routers:     routers,
		shutdown:    shutdown,
		mw:          mw,
		Log:         logger,
		storedSpecs: storedSpecs,
		lock:        lock,
		passOPTIONS: passOPTIONS,
	}

	return &app
}

// Handle is our mechanism for mounting Handlers for a given HTTP verb and path
// pair, this makes for really easy, convenient routing.
func (a *Apps) Handle(schemaID int, method string, path string, handler Handler, mw ...Middleware) {

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
	}

	// Add this handler for the specified verb and route.
	a.Routers[schemaID].Handle(method, path, h)
}

func getWallarmSchemaID(ctx *fasthttp.RequestCtx, storedSpecs database.DBOpenAPILoader) (int, error) {

	// get Wallarm Schema ID
	xWallarmSchemaID := string(ctx.Request.Header.Peek(XWallarmSchemaIDHeader))
	if xWallarmSchemaID == "" {
		return 0, errors.New("required X-WALLARM-SCHEMA-ID header is missing")
	}

	// get schema version
	schemaID, err := strconv2.Atoi(xWallarmSchemaID)
	if err != nil {
		return 0, fmt.Errorf("error parsing  value: %v", err)
	}

	// check if schema ID is loaded
	if !storedSpecs.IsLoaded(schemaID) {
		return 0, fmt.Errorf("provided via X-WALLARM-SCHEMA-ID header schema ID %d not found", schemaID)
	}

	return schemaID, nil
}

// APIModeHandler routes request to the appropriate handler according to the OpenAPI specification schema ID
func (a *Apps) APIModeHandler(ctx *fasthttp.RequestCtx) {

	schemaID, err := getWallarmSchemaID(ctx, a.storedSpecs)
	if err != nil {
		defer LogRequestResponseAtTraceLevel(ctx, a.Log)

		a.Log.WithFields(logrus.Fields{
			"error":      err,
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
		}).Error("error while getting schema ID")

		if err := RespondError(ctx, fasthttp.StatusInternalServerError, ""); err != nil {
			a.Log.WithFields(logrus.Fields{
				"error":      err,
				"request_id": fmt.Sprintf("#%016X", ctx.ID()),
			}).Error("error while sending response")
		}

		return
	}

	// add internal header to the context
	ctx.SetUserValue(WallarmSchemaID, schemaID)

	// delete internal header
	ctx.Request.Header.Del(XWallarmSchemaIDHeader)

	a.lock.RLock()
	defer a.lock.RUnlock()

	a.Routers[schemaID].Handler(ctx)

	// if pass request with OPTIONS method is enabled then log request
	if ctx.Response.StatusCode() == fasthttp.StatusOK && a.passOPTIONS && strconv.B2S(ctx.Method()) == fasthttp.MethodOptions {
		a.Log.WithFields(logrus.Fields{
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
		}).Debug("pass request with OPTIONS method")
	}
}

// SignalShutdown is used to gracefully shutdown the app when an integrity
// issue is identified.
func (a *Apps) SignalShutdown() {
	a.shutdown <- syscall.SIGTERM
}
