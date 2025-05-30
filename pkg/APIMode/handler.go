package APIMode

import (
	"fmt"
	strconv2 "strconv"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/platform/loader"
	apiMode "github.com/wallarm/api-firewall/internal/platform/validator"
	"github.com/wallarm/api-firewall/pkg/APIMode/validator"
)

type RequestValidator struct {
	CustomRoute   *loader.CustomRoute
	OpenAPIRouter *loader.Router
	ParserPool    *fastjson.ParserPool
	SchemaID      int
	Options       *Configuration
}

// APIModeHandler finds route in the OpenAPI spec and validates request
func (rv *RequestValidator) APIModeHandler(ctx *fasthttp.RequestCtx) (err error) {

	// handle panic
	defer func() {
		if r := recover(); r != nil {

			switch e := r.(type) {
			case error:
				err = e
			default:
				err = fmt.Errorf("panic: %v", r)
			}

			return
		}
	}()

	keyValidationErrors := strconv2.Itoa(rv.SchemaID) + validator.APIModePostfixValidationErrors
	keyStatusCode := strconv2.Itoa(rv.SchemaID) + validator.APIModePostfixStatusCode

	// Route not found
	if rv.CustomRoute == nil {
		ctx.SetUserValue(keyValidationErrors, []*validator.ValidationError{{Message: validator.ErrMethodAndPathNotFound.Error(), Code: validator.ErrCodeMethodAndPathNotFound, SchemaID: &rv.SchemaID}})
		ctx.SetUserValue(keyStatusCode, fasthttp.StatusForbidden)
		return nil
	}

	validationErrors, err := apiMode.APIModeValidateRequest(ctx, rv.SchemaID, rv.ParserPool, rv.CustomRoute, rv.Options.UnknownParametersDetection)
	if err != nil {
		return err
	}

	// Respond 403 with errors
	if len(validationErrors) > 0 {
		// add schema IDs to the validation error messages
		for _, r := range validationErrors {
			r.SchemaID = &rv.SchemaID
			r.SchemaVersion = rv.OpenAPIRouter.SchemaVersion
		}

		ctx.SetUserValue(keyValidationErrors, validationErrors)
		ctx.SetUserValue(keyStatusCode, fasthttp.StatusForbidden)
		return nil
	}

	// request successfully validated
	ctx.SetUserValue(keyStatusCode, fasthttp.StatusOK)
	return nil
}
