package apifw

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	strconv2 "strconv"
	"sync"

	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/platform/APImode"
	"github.com/wallarm/api-firewall/internal/platform/database"
	"github.com/wallarm/api-firewall/internal/platform/loader"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

var (
	StatusOK                  int = fasthttp.StatusOK
	StatusForbidden           int = fasthttp.StatusForbidden
	StatusInternalServerError int = fasthttp.StatusInternalServerError
)

type APIFirewall interface {
	ValidateRequest(schemaID int, r *bufio.Reader) (*web.APIModeResponse, error)
	UpdateSpecsStorage() (bool, error)
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

	// DB Usage Lock
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

	// Apply all the functional options
	for _, opt := range options {
		opt(apiMode.options)
	}

	var err error

	// load spec from the database
	specsStorage, errLoadDB := database.NewOpenAPIDB(apiMode.options.PathToSpecDB, apiMode.options.DBVersion)
	if errLoadDB != nil {
		err = errors.Join(err, errLoadDB)
	}

	apiMode.specsStorage = specsStorage

	// init routers
	routers, errRouters := getRouters(apiMode.specsStorage, &parserPool)
	if err != nil {
		err = errors.Join(err, errRouters)
	}

	apiMode.routers = routers

	return &apiMode, err
}

func getRouters(specStorage database.DBOpenAPILoader, parserPool *fastjson.ParserPool) (map[int]*router.Mux, error) {

	// Init routers
	routers := make(map[int]*router.Mux)
	for _, schemaID := range specStorage.SchemaIDs() {
		routers[schemaID] = router.NewRouter()

		serverURLStr := "/"
		spec := specStorage.Specification(schemaID)
		servers := spec.Servers
		if servers != nil {
			var err error
			if serverURLStr, err = servers.BasePath(); err != nil {
				return nil, fmt.Errorf("getting server URL from OpenAPI specification with ID %d: %w", schemaID, err)
			}
		}

		serverURL, err := url.Parse(serverURLStr)
		if err != nil {
			return nil, fmt.Errorf("parsing server URL from OpenAPI specification with ID %d: %w", schemaID, err)
		}

		// get new router
		newSwagRouter, err := loader.NewRouterDBLoader(specStorage.SpecificationVersion(schemaID), specStorage.Specification(schemaID))
		if err != nil {
			return nil, fmt.Errorf("new router creation failed for specification with ID %d: %w", schemaID, err)
		}

		for i := 0; i < len(newSwagRouter.Routes); i++ {

			s := RequestValidator{
				CustomRoute:   &newSwagRouter.Routes[i],
				ParserPool:    parserPool,
				OpenAPIRouter: newSwagRouter,
				SchemaID:      schemaID,
			}
			updRoutePathEsc, err := url.JoinPath(serverURL.Path, newSwagRouter.Routes[i].Path)
			if err != nil {
				return nil, fmt.Errorf("join path error for route %s in specification with ID %d: %w", newSwagRouter.Routes[i].Path, schemaID, err)
			}

			updRoutePath, err := url.PathUnescape(updRoutePathEsc)
			if err != nil {
				return nil, fmt.Errorf("path unescape error for route %s in specification with ID %d: %w", newSwagRouter.Routes[i].Path, schemaID, err)
			}

			if err := routers[schemaID].AddEndpoint(newSwagRouter.Routes[i].Method, updRoutePath, s.APIModeHandler); err != nil {
				return nil, fmt.Errorf("the OAS endpoint registration failed: method %s, path %s: %w", newSwagRouter.Routes[i].Method, updRoutePath, err)
			}
		}
	}

	return routers, nil
}

func (a *APIMode) UpdateSpecsStorage() (bool, error) {

	var isUpdated bool

	// load new schemes
	newSpecDB, err := database.NewOpenAPIDB(a.options.PathToSpecDB, a.options.DBVersion)
	if err != nil {
		return isUpdated, fmt.Errorf("loading specifications: %w", err)
	}

	// do not downgrade the db version
	if a.specsStorage.Version() > newSpecDB.Version() {
		return isUpdated, fmt.Errorf("version of the new DB structure is lower then current one (V2)")
	}

	if a.specsStorage.ShouldUpdate(newSpecDB) {
		a.lock.Lock()
		defer a.lock.Unlock()

		routers, err := getRouters(newSpecDB, a.parserPool)
		if err != nil {
			return isUpdated, err
		}

		a.routers = routers
		a.specsStorage = newSpecDB

		isUpdated = true

		if err := a.specsStorage.AfterLoad(a.options.PathToSpecDB); err != nil {
			return isUpdated, fmt.Errorf("error in after specification loading function: %w", err)
		}
	}

	return isUpdated, nil
}

func (a *APIMode) ValidateRequest(schemaID int, r *bufio.Reader) (response *web.APIModeResponse, err error) {

	// handle panic
	defer func() {
		if r := recover(); r != nil {

			switch e := r.(type) {
			case error:
				err = e
			default:
				err = fmt.Errorf("panic: %v", r)
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
		}, err
	}

	// find handler
	rctx := router.NewRouteContext()
	handler, err := a.Find(rctx, schemaID, strconv.B2S(ctx.Method()), strconv.B2S(ctx.Request.URI().Path()))
	if err != nil {
		return &web.APIModeResponse{
			Summary: []*web.APIModeResponseSummary{
				{
					SchemaID:   &schemaID,
					StatusCode: &StatusInternalServerError,
				},
			},
		}, err
	}

	// handler not found in the OAS
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

		// Method or Path were not found
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
		}, err
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
		}, nil
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
func (a *APIMode) Find(rctx *router.Context, schemaID int, method, path string) (router.Handler, error) {

	a.lock.RLock()
	defer a.lock.RUnlock()

	// Find the handler with the OAS information
	schemaRouter, ok := a.routers[schemaID]
	if !ok {
		return nil, fmt.Errorf("router not found: provided schema ID %d, list of loaded schema IDs %v ", schemaID, a.specsStorage.SchemaIDs())
	}

	return schemaRouter.Find(rctx, method, path), nil
}
