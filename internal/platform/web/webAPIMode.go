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
	"github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/database"
)

const (
	APIModePostfixStatusCode       = "_status_code"
	APIModePostfixValidationErrors = "_validation_errors"
)

type FieldTypeError struct {
	Name         string `json:"name"`
	ExpectedType string `json:"expected_type,omitempty"`
	Pattern      string `json:"pattern,omitempty"`
	CurrentValue string `json:"current_value"`
}

type ValidationError struct {
	Message       string           `json:"message"`
	Code          string           `json:"code"`
	SchemaVersion string           `json:"schema_version,omitempty"`
	SchemaID      string           `json:"schema_id,omitempty"`
	Fields        []string         `json:"related_fields,omitempty"`
	FieldsDetails []FieldTypeError `json:"related_fields_details,omitempty"`
}

type APIModeResponse struct {
	Errors []*ValidationError `json:"errors"`
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

func getWallarmSchemaID(ctx *fasthttp.RequestCtx, storedSpecs database.DBOpenAPILoader) ([]int, error) {

	// Get Wallarm Schema ID
	xWallarmSchemaIDsStr := string(ctx.Request.Header.Peek(XWallarmSchemaIDHeader))
	if xWallarmSchemaIDsStr == "" {
		return nil, errors.New("required X-WALLARM-SCHEMA-ID header is missing")
	}

	xWallarmSchemaIDs := strings.Split(xWallarmSchemaIDsStr, ",")

	schemaIDsMap := make(map[int]struct{})

	for _, schemaIDStr := range xWallarmSchemaIDs {
		// Get schema version
		schemaID, err := strconv2.Atoi(strings.TrimSpace(schemaIDStr))
		if err != nil {
			return nil, fmt.Errorf("error parsing  value: %v", err)
		}

		// Check if schema ID is loaded
		if !storedSpecs.IsLoaded(schemaID) {
			return nil, fmt.Errorf("provided via X-WALLARM-SCHEMA-ID header schema ID %d not found", schemaID)
		}

		schemaIDsMap[schemaID] = struct{}{}
	}

	schemaIDs := make([]int, 0, len(schemaIDsMap))
	for id := range schemaIDsMap {
		schemaIDs = append(schemaIDs, id)
	}

	return schemaIDs, nil
}

// APIModeHandler routes request to the appropriate handler according to the OpenAPI specification schema ID
func (a *APIModeApp) APIModeHandler(ctx *fasthttp.RequestCtx) {

	defer func() {
		// If pass request with OPTIONS method is enabled then log request
		if ctx.Response.StatusCode() == fasthttp.StatusOK && a.passOPTIONS && strconv.B2S(ctx.Method()) == fasthttp.MethodOptions {
			a.Log.WithFields(logrus.Fields{
				"request_id": fmt.Sprintf("#%016X", ctx.ID()),
			}).Debug("pass request with OPTIONS method")
		}
	}()

	schemaIDs, err := getWallarmSchemaID(ctx, a.storedSpecs)
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

	// Delete internal header
	ctx.Request.Header.Del(XWallarmSchemaIDHeader)

	a.lock.RLock()
	defer a.lock.RUnlock()

	// validate requests against list of schemas
	for _, schemaID := range schemaIDs {
		a.Routers[schemaID].Handler(ctx)
	}

	response := APIModeResponse{}
	for _, schemaID := range schemaIDs {
		statusCode, ok := ctx.UserValue(strconv2.Itoa(schemaID) + APIModePostfixStatusCode).(int)
		if !ok {
			return
		}
		switch statusCode {
		case fasthttp.StatusOK:
			continue
		case fasthttp.StatusInternalServerError:
			if err := RespondError(ctx, fasthttp.StatusInternalServerError, ""); err != nil {
				a.Log.WithFields(logrus.Fields{
					"request_id": fmt.Sprintf("#%016X", ctx.ID()),
					"error":      err,
				}).Error("respond error")
			}
			return
		case fasthttp.StatusForbidden:
			if validationErrors, ok := ctx.UserValue(strconv2.Itoa(schemaID) + APIModePostfixValidationErrors).([]*ValidationError); ok && validationErrors != nil {
				response.Errors = append(response.Errors, validationErrors...)
			}
		}
	}

	if len(response.Errors) == 0 {
		if err := RespondError(ctx, fasthttp.StatusOK, ""); err != nil {
			a.Log.WithFields(logrus.Fields{
				"request_id": fmt.Sprintf("#%016X", ctx.ID()),
				"error":      err,
			}).Error("respond error")
		}
		return
	}

	if err := Respond(ctx, response, fasthttp.StatusForbidden); err != nil {
		a.Log.WithFields(logrus.Fields{
			"request_id": fmt.Sprintf("#%016X", ctx.ID()),
			"error":      err,
		}).Error("respond error")
	}
}

// SignalShutdown is used to gracefully shutdown the app when an integrity
// issue is identified.
func (a *APIModeApp) SignalShutdown() {
	a.shutdown <- syscall.SIGTERM
}
