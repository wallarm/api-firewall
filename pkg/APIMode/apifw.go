package APIMode

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ardanlabs/conf"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"

	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/metrics"
	"github.com/wallarm/api-firewall/internal/platform/router"
	"github.com/wallarm/api-firewall/internal/platform/storage"
	"github.com/wallarm/api-firewall/internal/version"
	"github.com/wallarm/api-firewall/pkg/APIMode/validator"
)

type APIFirewall interface {
	ValidateRequestFromReader(schemaIDs []int, r *bufio.Reader) (*validator.ValidationResponse, error)
	ValidateRequest(schemaIDs []int, uri, method, body []byte, headers map[string][]string) (*validator.ValidationResponse, error)
	UpdateSpecsStorage() ([]int, bool, error)
}

type APIFWModeAPI struct {
	routers      map[int]*router.Mux
	specsStorage storage.DBOpenAPILoader
	parserPool   *fastjson.ParserPool
	lock         *sync.RWMutex
	options      *Configuration
}

var _ APIFirewall = (*APIFWModeAPI)(nil)

type Configuration struct {
	PathToSpecDB               string
	DBVersion                  int
	UnknownParametersDetection bool
	PassOptionsRequests        bool
	MaxErrorsInResponse        int
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

// WithMaxErrorsInResponse is a functional option to set max errors in the response
func WithMaxErrorsInResponse(limit int) Option {
	return func(c *Configuration) {
		c.MaxErrorsInResponse = limit
	}
}

// DisableUnknownParameters is a functional option to disable following redirects.
func DisableUnknownParameters() Option {
	return func(c *Configuration) {
		c.UnknownParametersDetection = false
	}
}

// EnablePassOptionsRequests is a functional option to enable requests with method OPTIONS
func EnablePassOptionsRequests() Option {
	return func(c *Configuration) {
		c.PassOptionsRequests = true
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

	apiMode := APIFWModeAPI{
		parserPool: &parserPool,
		lock:       &dbLock,
		options: &Configuration{
			PathToSpecDB:               "",
			DBVersion:                  0,
			UnknownParametersDetection: true,
			PassOptionsRequests:        false,
			MaxErrorsInResponse:        0,
		},
	}

	var cfg config.APIMode
	cfg.Version.SVN = version.Version
	cfg.Version.Desc = version.ProjectName

	if err := conf.Parse(os.Args[1:], version.Namespace, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// apply env var params
	apiMode.options.PassOptionsRequests = cfg.PassOptionsRequests
	apiMode.options.MaxErrorsInResponse = cfg.MaxErrorsInResponse
	apiMode.options.UnknownParametersDetection = cfg.UnknownParametersDetection
	apiMode.options.PathToSpecDB = cfg.PathToSpecDB
	apiMode.options.DBVersion = cfg.DBVersion

	// apply all the functional options
	for _, opt := range options {
		opt(apiMode.options)
	}

	var err error

	// load spec from the database
	specsStorage, errLoadDB := storage.NewOpenAPIDB(apiMode.options.PathToSpecDB, apiMode.options.DBVersion)
	if errLoadDB != nil {
		err = errors.Join(err, wrapOASpecErrs(errLoadDB))
	}

	apiMode.specsStorage = specsStorage

	// init routers
	routers, errRouters := getRouters(apiMode.specsStorage, &parserPool, apiMode.options)
	if errRouters != nil {
		err = errors.Join(err, fmt.Errorf("%w: %w", validator.ErrHandlersInit, errRouters))
	}

	apiMode.routers = routers

	return &apiMode, err
}

// UpdateSpecsStorage method reloads data from SQLite DB with specs
func (a *APIFWModeAPI) UpdateSpecsStorage() ([]int, bool, error) {

	var isUpdated bool

	// load new schemes
	newSpecDB, err := storage.NewOpenAPIDB(a.options.PathToSpecDB, a.options.DBVersion)
	if err != nil {
		return a.specsStorage.SchemaIDs(), isUpdated, wrapOASpecErrs(err)
	}

	// do not downgrade the db version
	if a.specsStorage.Version() > newSpecDB.Version() {
		return a.specsStorage.SchemaIDs(), isUpdated, fmt.Errorf("%w: version of the new DB structure is lower then current one (V2)", validator.ErrSpecLoading)
	}

	if a.specsStorage.ShouldUpdate(newSpecDB) {
		a.lock.Lock()
		defer a.lock.Unlock()

		routers, err := getRouters(newSpecDB, a.parserPool, a.options)
		if err != nil {
			return a.specsStorage.SchemaIDs(), isUpdated, fmt.Errorf("%w: %w", validator.ErrHandlersInit, err)
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
func (a *APIFWModeAPI) ValidateRequest(schemaIDs []int, uri, method, body []byte, headers map[string][]string) (*validator.ValidationResponse, error) {

	resp := validator.ValidationResponse{}
	var respErr error

	var wg sync.WaitGroup
	var m sync.Mutex

	// Request handling start time
	start := time.Now()

	for _, schemaID := range schemaIDs {

		// build fasthttp RequestCTX
		ctxReq := new(fasthttp.RequestCtx)

		ctxReq.Request.Header.SetRequestURIBytes(uri)
		ctxReq.Request.Header.SetMethodBytes(method)
		ctxReq.Request.SetBody(body)

		for hName, hValues := range headers {
			for _, hValue := range hValues {
				ctxReq.Request.Header.Add(hName, hValue)
			}
		}

		wg.Add(1)

		go func(ctx *fasthttp.RequestCtx, sID int) {
			defer wg.Done()
			defer metrics.IncHTTPRequestStat(start, schemaID, ctx.Response.StatusCode())

			pReqResp, pReqErrs := validator.ProcessRequest(sID, ctx, a.routers, a.lock, a.options.PassOptionsRequests, a.options.MaxErrorsInResponse)

			m.Lock()
			defer m.Unlock()

			if pReqResp != nil {
				resp.Summary = append(resp.Summary, pReqResp.Summary...)
				resp.Errors = append(resp.Errors, pReqResp.Errors...)
			}

			if pReqErrs != nil {
				if respErr == nil {
					respErr = pReqErrs
				} else {
					respErr = fmt.Errorf("%w; %w", respErr, pReqErrs)
				}
			}

		}(ctxReq, schemaID)

	}

	wg.Wait()

	return &resp, respErr
}

// ValidateRequestFromReader method validates request against the spec with provided schema ID
func (a *APIFWModeAPI) ValidateRequestFromReader(schemaIDs []int, r *bufio.Reader) (*validator.ValidationResponse, error) {

	resp := validator.ValidationResponse{}
	var respErr error

	var wg sync.WaitGroup
	var m sync.Mutex

	// Request handling start time
	start := time.Now()

	for _, schemaID := range schemaIDs {

		// build fasthttp RequestCTX
		ctx := new(fasthttp.RequestCtx)
		if err := ctx.Request.Read(r); err != nil {
			resp.Summary = append(resp.Summary, []*validator.ValidationResponseSummary{{SchemaID: &schemaID, StatusCode: &validator.StatusInternalServerError}}...)
			if respErr == nil {
				respErr = fmt.Errorf("%w: %w", validator.ErrRequestParsing, err)
				continue
			}

			respErr = fmt.Errorf("%w; %w: %w", respErr, validator.ErrRequestParsing, err)

			continue
		}

		wg.Add(1)

		go func(sID int) {
			defer wg.Done()
			defer metrics.IncHTTPRequestStat(start, schemaID, ctx.Response.StatusCode())

			pReqResp, pReqErrs := validator.ProcessRequest(sID, ctx, a.routers, a.lock, a.options.PassOptionsRequests, a.options.MaxErrorsInResponse)

			m.Lock()
			defer m.Unlock()

			if pReqResp != nil {
				resp.Summary = append(resp.Summary, pReqResp.Summary...)
				resp.Errors = append(resp.Errors, pReqResp.Errors...)
			}

			if pReqErrs != nil {
				if respErr == nil {
					respErr = pReqErrs
				} else {
					respErr = fmt.Errorf("%w; %w", respErr, pReqErrs)
				}
			}

		}(schemaID)
	}

	wg.Wait()

	return &resp, respErr
}
