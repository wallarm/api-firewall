package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	strconv2 "strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/google/uuid"
	"github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"github.com/wallarm/api-firewall/internal/platform/database"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

var (
	statusOK            = fasthttp.StatusOK
	statusInternalError = fasthttp.StatusInternalServerError
)

// APIModeApp is the entrypoint into our application and what configures our context
// object for each of our http handlers. Feel free to add any configuration
// data/logic on this App struct
type APIModeApp struct {
	Routers     map[int]*router.Mux
	Log         *logrus.Logger
	passOPTIONS bool
	shutdown    chan os.Signal
	mw          []web.Middleware
	storedSpecs database.DBOpenAPILoader
	lock        *sync.RWMutex
}

// NewAPIModeApp creates an APIModeApp value that handle a set of routes for the set of application.
func NewAPIModeApp(lock *sync.RWMutex, passOPTIONS bool, storedSpecs database.DBOpenAPILoader, shutdown chan os.Signal, logger *logrus.Logger, mw ...web.Middleware) *APIModeApp {

	schemaIDs := storedSpecs.SchemaIDs()

	// Init routers
	routers := make(map[int]*router.Mux)
	for _, schemaID := range schemaIDs {
		//routers[schemaID] = make(map[string]*mux.Router)
		routers[schemaID] = router.NewRouter()
		//routers[schemaID].HandleOPTIONS = passOPTIONS
	}

	app := APIModeApp{
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
func (a *APIModeApp) Handle(schemaID int, method string, path string, handler web.Handler, mw ...web.Middleware) error {

	// First wrap handler specific middleware around this handler.
	handler = web.WrapMiddleware(mw, handler)

	// Add the application's general middleware to the handler chain.
	handler = web.WrapMiddleware(a.mw, handler)

	// The function to execute for each request.
	h := func(ctx *fasthttp.RequestCtx) error {

		if err := handler(ctx); err != nil {
			a.SignalShutdown()
			return err
		}
		return nil
	}

	// Add this handler for the specified verb and route.
	if err := a.Routers[schemaID].AddEndpoint(method, path, h); err != nil {
		return err
	}
	return nil
}

// getWallarmSchemaID returns lists of found schema IDs in the DB, not found schema IDs in the DB and errors
func getWallarmSchemaID(ctx *fasthttp.RequestCtx, storedSpecs database.DBOpenAPILoader) (found []int, notFound []int, err error) {

	if !storedSpecs.IsReady() {
		return nil, nil, errors.New("DB with schemas has not loaded")
	}

	// Get Wallarm Schema ID
	xWallarmSchemaIDsStr := string(ctx.Request.Header.Peek(web.XWallarmSchemaIDHeader))
	if xWallarmSchemaIDsStr == "" {
		return nil, nil, errors.New("required X-WALLARM-SCHEMA-ID header is missing")
	}

	xWallarmSchemaIDs := strings.Split(xWallarmSchemaIDsStr, ",")

	schemaIDsMap := make(map[int]struct{})

	for _, schemaIDStr := range xWallarmSchemaIDs {
		// Get schema version
		schemaID, err := strconv2.Atoi(strings.TrimSpace(schemaIDStr))
		if err != nil {
			return nil, nil, fmt.Errorf("error parsing  value: %v", err)
		}

		// Check if schema ID is loaded
		if !storedSpecs.IsLoaded(schemaID) {
			notFound = append(notFound, schemaID)
			continue
		}

		schemaIDsMap[schemaID] = struct{}{}
	}

	for id := range schemaIDsMap {
		found = append(found, id)
	}

	return
}

// APIModeRouteHandler routes request to the appropriate handler according to the OpenAPI specification schema ID
func (a *APIModeApp) APIModeRouteHandler(ctx *fasthttp.RequestCtx) {

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
	ctx.SetUserValue(web.RequestID, uuid.NewString())

	schemaIDs, notFoundSchemaIDs, err := getWallarmSchemaID(ctx, a.storedSpecs)
	if err != nil {
		defer web.LogRequestResponseAtTraceLevel(ctx, a.Log)

		a.Log.WithFields(logrus.Fields{
			"error":      err,
			"host":       string(ctx.Request.Header.Host()),
			"path":       string(ctx.Path()),
			"method":     string(ctx.Request.Header.Method()),
			"request_id": ctx.UserValue(web.RequestID),
		}).Error("error while getting schema ID")

		if err := web.RespondError(ctx, fasthttp.StatusInternalServerError, ""); err != nil {
			a.Log.WithFields(logrus.Fields{
				"error":      err,
				"host":       string(ctx.Request.Header.Host()),
				"path":       string(ctx.Path()),
				"method":     string(ctx.Request.Header.Method()),
				"request_id": ctx.UserValue(web.RequestID),
			}).Error("error while sending response")
		}

		return
	}

	// Delete internal header
	ctx.Request.Header.Del(web.XWallarmSchemaIDHeader)

	a.lock.RLock()
	defer a.lock.RUnlock()
	//w := NewFastHTTPResponseAdapter(ctx)

	// Validate requests against list of schemas
	for _, sID := range schemaIDs {
		schemaID := sID
		// Save schema IDs
		ctx.SetUserValue(web.RequestSchemaID, strconv2.Itoa(schemaID))
		var r http.Request
		if err := fasthttpadaptor.ConvertRequest(ctx, &r, true); err != nil {
			a.Log.WithFields(logrus.Fields{
				"error":      err,
				"host":       strconv.B2S(ctx.Request.Header.Host()),
				"path":       strconv.B2S(ctx.Path()),
				"method":     strconv.B2S(ctx.Request.Header.Method()),
				"request_id": ctx.UserValue(web.RequestID),
			}).Error("error converting request")
			return
		}

		// find the handler with the OAS information
		rctx := router.NewRouteContext()
		handler := a.Routers[schemaID].Find(rctx, strconv.B2S(ctx.Method()), strconv.B2S(ctx.Request.URI().Path()))

		// handler not found in the OAS
		if handler == nil {
			keyValidationErrors := strconv2.Itoa(schemaID) + web.APIModePostfixValidationErrors
			keyStatusCode := strconv2.Itoa(schemaID) + web.APIModePostfixStatusCode

			// OPTIONS methods are passed if the passOPTIONS is set to true
			if a.passOPTIONS == true && strconv.B2S(ctx.Method()) == fasthttp.MethodOptions {
				ctx.SetUserValue(keyStatusCode, fasthttp.StatusOK)
				a.Log.WithFields(logrus.Fields{
					"host":       strconv.B2S(ctx.Request.Header.Host()),
					"path":       strconv.B2S(ctx.Path()),
					"method":     strconv.B2S(ctx.Request.Header.Method()),
					"request_id": ctx.UserValue(web.RequestID),
				}).Debug("Pass request with OPTIONS method")
				continue
			}

			// Method or Path were not found
			a.Log.WithFields(logrus.Fields{
				"host":       strconv.B2S(ctx.Request.Header.Host()),
				"path":       strconv.B2S(ctx.Path()),
				"method":     strconv.B2S(ctx.Request.Header.Method()),
				"request_id": ctx.UserValue(web.RequestID),
			}).Debug("Method or path were not found")
			ctx.SetUserValue(keyValidationErrors, []*web.ValidationError{{Message: ErrMethodAndPathNotFound.Error(), Code: ErrCodeMethodAndPathNotFound, SchemaID: &schemaID}})
			ctx.SetUserValue(keyStatusCode, fasthttp.StatusForbidden)
			continue
		}

		// add router context to get URL params in the Handler
		ctx.SetUserValue(router.RouteCtxKey, rctx)

		if err := handler(ctx); err != nil {
			a.Log.WithFields(logrus.Fields{
				"error":      err,
				"host":       strconv.B2S(ctx.Request.Header.Host()),
				"path":       strconv.B2S(ctx.Path()),
				"method":     strconv.B2S(ctx.Request.Header.Method()),
				"request_id": ctx.UserValue(web.RequestID),
			}).Error("error in the request handler")
		}
	}

	responseSummary := make([]*web.APIModeResponseSummary, 0, len(schemaIDs))
	responseErrors := make([]*web.ValidationError, 0)

	for i := 0; i < len(schemaIDs); i++ {

		if statusCode, ok := ctx.UserValue(web.GlobalResponseStatusCodeKey).(int); ok {
			ctx.Response.Header.Reset()
			ctx.Response.Header.SetStatusCode(statusCode)
			return
		}

		statusCode, ok := ctx.UserValue(strconv2.Itoa(schemaIDs[i]) + web.APIModePostfixStatusCode).(int)
		if !ok {
			// set summary for the schema ID in pass Options mode
			if a.passOPTIONS && strconv.B2S(ctx.Method()) == fasthttp.MethodOptions {
				responseSummary = append(responseSummary, &web.APIModeResponseSummary{
					SchemaID:   &schemaIDs[i],
					StatusCode: &statusOK,
				})
				continue
			}

			// Didn't receive the response code. It means that the router respond to the request because it was not valid.
			// The API Firewall should respond by 500 status code in this case.
			ctx.Response.Header.Reset()
			statusCode = fasthttp.StatusInternalServerError
		}

		responseSummary = append(responseSummary, &web.APIModeResponseSummary{
			SchemaID:   &schemaIDs[i],
			StatusCode: &statusCode,
		})

		if validationErrors, ok := ctx.UserValue(strconv2.Itoa(schemaIDs[i]) + web.APIModePostfixValidationErrors).([]*web.ValidationError); ok && validationErrors != nil {
			responseErrors = append(responseErrors, validationErrors...)
		}
	}

	// Add schema IDs that were not found in the DB to the response
	for i := 0; i < len(notFoundSchemaIDs); i++ {
		responseSummary = append(responseSummary, &web.APIModeResponseSummary{
			SchemaID:   &notFoundSchemaIDs[i],
			StatusCode: &statusInternalError,
		})
	}

	// delete Allow header which is set by the router
	ctx.Response.Header.Del(fasthttp.HeaderAllow)

	// replace method to send response body
	if ctx.IsHead() {
		ctx.Request.Header.SetMethod(fasthttp.MethodGet)
	}

	if err := web.Respond(ctx, web.APIModeResponse{Summary: responseSummary, Errors: responseErrors}, fasthttp.StatusOK); err != nil {
		a.Log.WithFields(logrus.Fields{
			"request_id": ctx.UserValue(web.RequestID),
			"host":       string(ctx.Request.Header.Host()),
			"path":       string(ctx.Path()),
			"method":     string(ctx.Request.Header.Method()),
			"error":      err,
		}).Error("respond error")
	}
}

// SignalShutdown is used to gracefully shutdown the app when an integrity
// issue is identified.
func (a *APIModeApp) SignalShutdown() {
	a.shutdown <- syscall.SIGTERM
}
