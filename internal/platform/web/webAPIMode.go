package web

import (
	"errors"
	"fmt"
	"os"
	strconv2 "strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/fasthttp/router"
	"github.com/google/uuid"
	"github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/database"
)

const (
	APIModePostfixStatusCode       = "_status_code"
	APIModePostfixValidationErrors = "_validation_errors"
)

var (
	statusOK            = fasthttp.StatusOK
	statusInternalError = fasthttp.StatusInternalServerError
)

type FieldTypeError struct {
	Name         string `json:"name"`
	ExpectedType string `json:"expected_type,omitempty"`
	Pattern      string `json:"pattern,omitempty"`
	CurrentValue string `json:"current_value,omitempty"`
}

type ValidationError struct {
	Message       string           `json:"message"`
	Code          string           `json:"code"`
	SchemaVersion string           `json:"schema_version,omitempty"`
	SchemaID      *int             `json:"schema_id"`
	Fields        []string         `json:"related_fields,omitempty"`
	FieldsDetails []FieldTypeError `json:"related_fields_details,omitempty"`
}

type APIModeResponseSummary struct {
	SchemaID   *int `json:"schema_id"`
	StatusCode *int `json:"status_code"`
}

type APIModeResponse struct {
	Summary []*APIModeResponseSummary `json:"summary"`
	Errors  []*ValidationError        `json:"errors,omitempty"`
}

// APIModeApp is the entrypoint into our application and what configures our context
// object for each of our http handlers. Feel free to add any configuration
// data/logic on this App struct
type APIModeApp struct {
	Routers     map[int]*router.Router
	Log         *logrus.Logger
	passOPTIONS bool
	shutdown    chan os.Signal
	mw          []Middleware
	storedSpecs database.DBOpenAPILoader
	lock        *sync.RWMutex
}

func (a *APIModeApp) SetDefaultBehavior(schemaID int, handler Handler, mw ...Middleware) {
	// First wrap handler specific middleware around this handler.
	handler = wrapMiddleware(mw, handler)

	// Add the application's general middleware to the handler chain.
	handler = wrapMiddleware(a.mw, handler)

	customHandler := func(ctx *fasthttp.RequestCtx) {

		// Add request ID
		ctx.SetUserValue(RequestID, uuid.NewString())

		if err := handler(ctx); err != nil {
			a.SignalShutdown()
			return
		}

	}

	// Set NOT FOUND behavior
	a.Routers[schemaID].NotFound = customHandler

	// Set Method Not Allowed behavior
	a.Routers[schemaID].MethodNotAllowed = customHandler
}

// NewAPIModeApp creates an APIModeApp value that handle a set of routes for the set of application.
func NewAPIModeApp(lock *sync.RWMutex, passOPTIONS bool, storedSpecs database.DBOpenAPILoader, shutdown chan os.Signal, logger *logrus.Logger, mw ...Middleware) *APIModeApp {

	schemaIDs := storedSpecs.SchemaIDs()

	// Init routers
	routers := make(map[int]*router.Router)
	for _, schemaID := range schemaIDs {
		routers[schemaID] = router.New()
		routers[schemaID].HandleOPTIONS = passOPTIONS
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
func (a *APIModeApp) Handle(schemaID int, method string, path string, handler Handler, mw ...Middleware) {

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

// getWallarmSchemaID returns lists of found schema IDs in the DB, not found schema IDs in the DB and errors
func getWallarmSchemaID(ctx *fasthttp.RequestCtx, storedSpecs database.DBOpenAPILoader) (found []int, notFound []int, err error) {

	if !storedSpecs.IsReady() {
		return nil, nil, errors.New("DB with schemas has not loaded")
	}

	// Get Wallarm Schema ID
	xWallarmSchemaIDsStr := string(ctx.Request.Header.Peek(XWallarmSchemaIDHeader))
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

// APIModeHandler routes request to the appropriate handler according to the OpenAPI specification schema ID
func (a *APIModeApp) APIModeHandler(ctx *fasthttp.RequestCtx) {

	// Add request ID
	ctx.SetUserValue(RequestID, uuid.NewString())

	defer func() {
		// If pass request with OPTIONS method is enabled then log request
		if ctx.Response.StatusCode() == fasthttp.StatusOK && a.passOPTIONS && strconv.B2S(ctx.Method()) == fasthttp.MethodOptions {
			a.Log.WithFields(logrus.Fields{
				"request_id": ctx.UserValue(RequestID),
			}).Debug("pass request with OPTIONS method")
		}
	}()

	schemaIDs, notFoundSchemaIDs, err := getWallarmSchemaID(ctx, a.storedSpecs)
	if err != nil {
		defer LogRequestResponseAtTraceLevel(ctx, a.Log)

		a.Log.WithFields(logrus.Fields{
			"error":      err,
			"request_id": ctx.UserValue(RequestID),
		}).Error("error while getting schema ID")

		if err := RespondError(ctx, fasthttp.StatusInternalServerError, ""); err != nil {
			a.Log.WithFields(logrus.Fields{
				"error":      err,
				"request_id": ctx.UserValue(RequestID),
			}).Error("error while sending response")
		}

		return
	}

	// Delete internal header
	ctx.Request.Header.Del(XWallarmSchemaIDHeader)

	a.lock.RLock()
	defer a.lock.RUnlock()

	// Validate requests against list of schemas
	for _, schemaID := range schemaIDs {
		a.Routers[schemaID].Handler(ctx)
	}

	responseSummary := make([]*APIModeResponseSummary, 0, len(schemaIDs))
	responseErrors := make([]*ValidationError, 0)

	for i := 0; i < len(schemaIDs); i++ {
		statusCode, ok := ctx.UserValue(strconv2.Itoa(schemaIDs[i]) + APIModePostfixStatusCode).(int)
		if !ok {
			// set summary for the schema ID in pass Options mode
			if a.passOPTIONS && strconv.B2S(ctx.Method()) == fasthttp.MethodOptions {
				responseSummary = append(responseSummary, &APIModeResponseSummary{
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

		responseSummary = append(responseSummary, &APIModeResponseSummary{
			SchemaID:   &schemaIDs[i],
			StatusCode: &statusCode,
		})

		if validationErrors, ok := ctx.UserValue(strconv2.Itoa(schemaIDs[i]) + APIModePostfixValidationErrors).([]*ValidationError); ok && validationErrors != nil {
			responseErrors = append(responseErrors, validationErrors...)
		}
	}

	// Add schema IDs that were not found in the DB to the response
	for i := 0; i < len(notFoundSchemaIDs); i++ {
		responseSummary = append(responseSummary, &APIModeResponseSummary{
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

	if err := Respond(ctx, APIModeResponse{Summary: responseSummary, Errors: responseErrors}, fasthttp.StatusOK); err != nil {
		a.Log.WithFields(logrus.Fields{
			"request_id": ctx.UserValue(RequestID),
			"error":      err,
		}).Error("respond error")
	}
}

// SignalShutdown is used to gracefully shutdown the app when an integrity
// issue is identified.
func (a *APIModeApp) SignalShutdown() {
	a.shutdown <- syscall.SIGTERM
}
