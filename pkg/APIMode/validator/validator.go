package validator

import (
	"fmt"
	strconv2 "strconv"
	"sync"

	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/router"
)

const (
	APIModePostfixStatusCode       = "_status_code"
	APIModePostfixValidationErrors = "_validation_errors"
)

var (
	StatusOK                  int = fasthttp.StatusOK
	StatusForbidden           int = fasthttp.StatusForbidden
	StatusInternalServerError int = fasthttp.StatusInternalServerError
)

func ProcessRequest(schemaID int, ctx *fasthttp.RequestCtx, routers map[int]*router.Mux, lock *sync.RWMutex, passOptionsRequests bool) (resp *ValidationResponse, err error) {

	// handle panic
	defer func() {
		if r := recover(); r != nil {

			switch e := r.(type) {
			case error:
				err = e
			default:
				err = fmt.Errorf("%w: panic: %v", ErrRequestParsing, r)
			}

			resp = &ValidationResponse{
				Summary: []*ValidationResponseSummary{
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
	handler, err := find(routers, lock, rctx, schemaID, strconv.B2S(ctx.Method()), strconv.B2S(ctx.Request.URI().Path()))
	if err != nil {
		return &ValidationResponse{
			Summary: []*ValidationResponseSummary{
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
		if passOptionsRequests == true && strconv.B2S(ctx.Method()) == fasthttp.MethodOptions {
			return &ValidationResponse{
				Summary: []*ValidationResponseSummary{
					{
						SchemaID:   &schemaID,
						StatusCode: &StatusOK,
					},
				},
			}, nil
		}

		// method or path were not found
		return &ValidationResponse{
			Summary: []*ValidationResponseSummary{
				{
					SchemaID:   &schemaID,
					StatusCode: &StatusForbidden,
				},
			},
			Errors: []*ValidationError{{Message: ErrMethodAndPathNotFound.Error(), Code: ErrCodeMethodAndPathNotFound, SchemaID: &schemaID}},
		}, nil
	}

	// add router context to get URL params in the Handler
	ctx.SetUserValue(router.RouteCtxKey, rctx)

	if err := handler(ctx); err != nil {
		return &ValidationResponse{
			Summary: []*ValidationResponseSummary{
				{
					SchemaID:   &schemaID,
					StatusCode: &StatusInternalServerError,
				},
			},
		}, fmt.Errorf("%w: %w", ErrRequestParsing, err)
	}

	responseSummary := make([]*ValidationResponseSummary, 0, 1)
	responseErrors := make([]*ValidationError, 0)

	statusCode, ok := ctx.UserValue(strconv2.Itoa(schemaID) + APIModePostfixStatusCode).(int)
	if !ok {
		// Didn't receive the response code. It means that the router respond to the request because it was not valid.
		// The API Firewall should respond by 500 status code in this case.
		return &ValidationResponse{
			Summary: []*ValidationResponseSummary{
				{
					SchemaID:   &schemaID,
					StatusCode: &StatusInternalServerError,
				},
			},
		}, fmt.Errorf("%w: unknown error while request processing", ErrRequestParsing)
	}

	responseSummary = append(responseSummary, &ValidationResponseSummary{
		SchemaID:   &schemaID,
		StatusCode: &statusCode,
	})

	if validationErrors, ok := ctx.UserValue(strconv2.Itoa(schemaID) + APIModePostfixValidationErrors).([]*ValidationError); ok && validationErrors != nil {
		responseErrors = append(responseErrors, validationErrors...)
	}

	return &ValidationResponse{Summary: responseSummary, Errors: responseErrors}, nil
}

// Find function searches for the handler by path and method
func find(routers map[int]*router.Mux, lock *sync.RWMutex, rctx *router.Context, schemaID int, method, path string) (router.Handler, error) {

	lock.RLock()
	defer lock.RUnlock()

	// Find the handler with the OAS information
	schemaRouter, ok := routers[schemaID]
	if !ok {
		return nil, fmt.Errorf("router not found: provided schema ID %d", schemaID)
	}

	return schemaRouter.Find(rctx, method, path), nil
}
