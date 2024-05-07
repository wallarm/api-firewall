package apifw

import (
	"bufio"
	"errors"
	"fmt"
	strconv2 "strconv"
	"sync"

	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/platform/APImode"
	"github.com/wallarm/api-firewall/internal/platform/database"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

var (
	StatusOK                  int = fasthttp.StatusOK
	StatusForbidden           int = fasthttp.StatusForbidden
	StatusInternalServerError int = fasthttp.StatusInternalServerError

	ErrSchemaNotFound = fmt.Errorf("schema not found")
	ErrRequestParsing = fmt.Errorf("request parsing error")
	ErrSpecParsing    = fmt.Errorf("OpenAPI specification parsing error")
	ErrSpecValidation = fmt.Errorf("OpenAPI specification validation error")
	ErrSpecLoading    = fmt.Errorf("OpenAPI specifications reading from database error")
	ErrHandlersInit   = fmt.Errorf("handlers initialization error")
)

type APIFirewall interface {
	ValidateRequestFromReader(schemaID int, r *bufio.Reader) (*web.APIModeResponse, error)
	ValidateRequest(schemaID int, uri, method, body []byte, headers map[string][]string) (*web.APIModeResponse, error)
	UpdateSpecsStorage() ([]int, bool, error)
}

type APIMode struct {
	routers      map[int]*router.Mux
	specsStorage database.DBOpenAPILoader
	parserPool   *fastjson.ParserPool
	lock         *sync.RWMutex
	options      *Configuration
}

type Configuration struct {
	PathToSpecDB               string
	DBVersion                  int
	UnknownParametersDetection bool
	PassOptionsRequests        bool
}

type Option func(*Configuration)

// WithDBVersion is a functional option to set the SQLLite DB structure version
func WithDBVersion(dbVersion int) Option {
	return func(c *Configuration) {
		c.DBVersion = dbVersion
	}
}

// WithPathToDB is a functional option to set path to the SQLLite DB with the OpenAPI specifications
func WithPathToDB(path string) Option {
	return func(c *Configuration) {
		c.PathToSpecDB = path
	}
}

// DisableUnknownParameters is a functional option to disable following redirects.
func DisableUnknownParameters() Option {
	return func(c *Configuration) {
		c.UnknownParametersDetection = false
	}
}

// DisablePassOptionsRequests is a functional option to disable requests with method OPTIONS
func DisablePassOptionsRequests() Option {
	return func(c *Configuration) {
		c.PassOptionsRequests = false
	}
}

func NewAPIFirewall(options ...Option) (APIFirewall, error) {

	// db usage lock
	var dbLock sync.RWMutex

	// define FastJSON parsers pool
	var parserPool fastjson.ParserPool

	apiMode := APIMode{
		parserPool: &parserPool,
		lock:       &dbLock,
		options: &Configuration{
			PathToSpecDB:               "",
			DBVersion:                  0,
			UnknownParametersDetection: true,
			PassOptionsRequests:        true,
		},
	}

	// apply all the functional options
	for _, opt := range options {
		opt(apiMode.options)
	}

	var err error

	// load spec from the database
	specsStorage, errLoadDB := database.NewOpenAPIDB(apiMode.options.PathToSpecDB, apiMode.options.DBVersion)
	if errLoadDB != nil {
		err = errors.Join(err, wrapOASpecErrs(errLoadDB))
	}

	apiMode.specsStorage = specsStorage

	// init routers
	routers, errRouters := getRouters(apiMode.specsStorage, &parserPool, apiMode.options)
	if errRouters != nil {
		err = errors.Join(err, fmt.Errorf("%w: %w", ErrHandlersInit, errRouters))
	}

	apiMode.routers = routers

	return &apiMode, err
}

// UpdateSpecsStorage method reloads data from SQLite DB with specs
func (a *APIMode) UpdateSpecsStorage() ([]int, bool, error) {

	var isUpdated bool

	// load new schemes
	newSpecDB, err := database.NewOpenAPIDB(a.options.PathToSpecDB, a.options.DBVersion)
	if err != nil {
		return a.specsStorage.SchemaIDs(), isUpdated, wrapOASpecErrs(err)
	}

	// do not downgrade the db version
	if a.specsStorage.Version() > newSpecDB.Version() {
		return a.specsStorage.SchemaIDs(), isUpdated, fmt.Errorf("%w: version of the new DB structure is lower then current one (V2)", ErrSpecLoading)
	}

	if a.specsStorage.ShouldUpdate(newSpecDB) {
		a.lock.Lock()
		defer a.lock.Unlock()

		routers, err := getRouters(newSpecDB, a.parserPool, a.options)
		if err != nil {
			return a.specsStorage.SchemaIDs(), isUpdated, fmt.Errorf("%w: %w", ErrHandlersInit, err)
		}

		a.routers = routers
		a.specsStorage = newSpecDB

		isUpdated = true

		if err := a.specsStorage.AfterLoad(a.options.PathToSpecDB); err != nil {
			return a.specsStorage.SchemaIDs(), isUpdated, wrapOASpecErrs(err)
		}
	}

	return a.specsStorage.SchemaIDs(), isUpdated, nil
}

// ValidateRequest method validates request against the spec with provided schema ID
func (a *APIMode) ValidateRequest(schemaID int, uri, method, body []byte, headers map[string][]string) (*web.APIModeResponse, error) {

	// build fasthttp RequestCTX
	ctx := new(fasthttp.RequestCtx)

	ctx.Request.Header.SetRequestURIBytes(uri)
	ctx.Request.Header.SetMethodBytes(method)
	ctx.Request.SetBody(body)

	for hName, hValues := range headers {
		for _, hValue := range hValues {
			ctx.Request.Header.Add(hName, hValue)
		}
	}

	return a.processRequest(schemaID, ctx)
}

// ValidateRequestFromReader method validates request against the spec with provided schema ID
func (a *APIMode) ValidateRequestFromReader(schemaID int, r *bufio.Reader) (response *web.APIModeResponse, err error) {

	// build fasthttp RequestCTX
	ctx := new(fasthttp.RequestCtx)
	if err := ctx.Request.Read(r); err != nil {
		return &web.APIModeResponse{
			Summary: []*web.APIModeResponseSummary{
				{
					SchemaID:   &schemaID,
					StatusCode: &StatusInternalServerError,
				},
			},
		}, fmt.Errorf("%w: %w", ErrRequestParsing, err)
	}

	return a.processRequest(schemaID, ctx)
}

func (a *APIMode) processRequest(schemaID int, ctx *fasthttp.RequestCtx) (response *web.APIModeResponse, err error) {

	// handle panic
	defer func() {
		if r := recover(); r != nil {

			switch e := r.(type) {
			case error:
				err = e
			default:
				err = fmt.Errorf("%w: panic: %v", ErrRequestParsing, r)
			}

			response = &web.APIModeResponse{
				Summary: []*web.APIModeResponseSummary{
					{
						SchemaID:   &schemaID,
						StatusCode: &StatusInternalServerError,
					},
				},
			}

			return
		}
	}()

	// find handler
	rctx := router.NewRouteContext()
	handler, err := a.find(rctx, schemaID, strconv.B2S(ctx.Method()), strconv.B2S(ctx.Request.URI().Path()))
	if err != nil {
		return &web.APIModeResponse{
			Summary: []*web.APIModeResponseSummary{
				{
					SchemaID:   &schemaID,
					StatusCode: &StatusInternalServerError,
				},
			},
		}, fmt.Errorf("%w: %w", ErrSchemaNotFound, err)
	}

	// handler not found in the existing OAS
	if handler == nil {
		// OPTIONS methods are passed if the passOPTIONS is set to true
		if a.options.PassOptionsRequests == true && strconv.B2S(ctx.Method()) == fasthttp.MethodOptions {
			return &web.APIModeResponse{
				Summary: []*web.APIModeResponseSummary{
					{
						SchemaID:   &schemaID,
						StatusCode: &StatusOK,
					},
				},
			}, nil
		}

		// method or path were not found
		return &web.APIModeResponse{
			Summary: []*web.APIModeResponseSummary{
				{
					SchemaID:   &schemaID,
					StatusCode: &StatusForbidden,
				},
			},
			Errors: []*web.ValidationError{{Message: APImode.ErrMethodAndPathNotFound.Error(), Code: APImode.ErrCodeMethodAndPathNotFound, SchemaID: &schemaID}},
		}, nil
	}

	// add router context to get URL params in the Handler
	ctx.SetUserValue(router.RouteCtxKey, rctx)

	if err := handler(ctx); err != nil {
		return &web.APIModeResponse{
			Summary: []*web.APIModeResponseSummary{
				{
					SchemaID:   &schemaID,
					StatusCode: &StatusInternalServerError,
				},
			},
		}, fmt.Errorf("%w: %w", ErrRequestParsing, err)
	}

	responseSummary := make([]*web.APIModeResponseSummary, 0, 1)
	responseErrors := make([]*web.ValidationError, 0)

	statusCode, ok := ctx.UserValue(strconv2.Itoa(schemaID) + web.APIModePostfixStatusCode).(int)
	if !ok {
		// Didn't receive the response code. It means that the router respond to the request because it was not valid.
		// The API Firewall should respond by 500 status code in this case.
		return &web.APIModeResponse{
			Summary: []*web.APIModeResponseSummary{
				{
					SchemaID:   &schemaID,
					StatusCode: &StatusInternalServerError,
				},
			},
		}, fmt.Errorf("%w: unknown error while request processing", ErrRequestParsing)
	}

	responseSummary = append(responseSummary, &web.APIModeResponseSummary{
		SchemaID:   &schemaID,
		StatusCode: &statusCode,
	})

	if validationErrors, ok := ctx.UserValue(strconv2.Itoa(schemaID) + web.APIModePostfixValidationErrors).([]*web.ValidationError); ok && validationErrors != nil {
		responseErrors = append(responseErrors, validationErrors...)
	}

	return &web.APIModeResponse{Summary: responseSummary, Errors: responseErrors}, nil
}

// Find function searches for the handler by path and method
func (a *APIMode) find(rctx *router.Context, schemaID int, method, path string) (router.Handler, error) {

	a.lock.RLock()
	defer a.lock.RUnlock()

	// Find the handler with the OAS information
	schemaRouter, ok := a.routers[schemaID]
	if !ok {
		return nil, fmt.Errorf("provided schema ID %d, list of loaded schema IDs %v ", schemaID, a.specsStorage.SchemaIDs())
	}

	return schemaRouter.Find(rctx, method, path), nil
}
