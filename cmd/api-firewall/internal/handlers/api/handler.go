package api

import (
	"runtime/debug"
	strconv2 "strconv"

	"github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/APImode"
	"github.com/wallarm/api-firewall/internal/platform/loader"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

type RequestValidator struct {
	CustomRoute   *loader.CustomRoute
	OpenAPIRouter *loader.Router
	Log           *logrus.Logger
	Cfg           *config.APIMode
	ParserPool    *fastjson.ParserPool
	SchemaID      int
}

// Handler validates request and respond with 200, 403 (with error) or 500 status code
func (s *RequestValidator) Handler(ctx *fasthttp.RequestCtx) error {

	// handle panic
	defer func() {
		if r := recover(); r != nil {
			s.Log.Errorf("panic: %v", r)

			// Log the Go stack trace for this panic'd goroutine.
			s.Log.Debugf("%s", debug.Stack())
			return
		}
	}()

	keyValidationErrors := strconv2.Itoa(s.SchemaID) + web.APIModePostfixValidationErrors
	keyStatusCode := strconv2.Itoa(s.SchemaID) + web.APIModePostfixStatusCode

	// Route not found
	if s.CustomRoute == nil {
		s.Log.WithFields(logrus.Fields{
			"host":       strconv.B2S(ctx.Request.Header.Host()),
			"path":       strconv.B2S(ctx.Path()),
			"method":     strconv.B2S(ctx.Request.Header.Method()),
			"request_id": ctx.UserValue(web.RequestID),
		}).Debug("Method or path were not found")
		ctx.SetUserValue(keyValidationErrors, []*web.ValidationError{{Message: APImode.ErrMethodAndPathNotFound.Error(), Code: APImode.ErrCodeMethodAndPathNotFound, SchemaID: &s.SchemaID}})
		ctx.SetUserValue(keyStatusCode, fasthttp.StatusForbidden)
		return nil
	}

	validationErrors, err := APImode.ValidateRequest(ctx, s.ParserPool, s.CustomRoute, s.Cfg.UnknownParametersDetection)
	if err != nil {
		s.Log.WithFields(logrus.Fields{
			"error":      err,
			"host":       strconv.B2S(ctx.Request.Header.Host()),
			"path":       strconv.B2S(ctx.Path()),
			"method":     strconv.B2S(ctx.Request.Header.Method()),
			"request_id": ctx.UserValue(web.RequestID),
		}).Error("request validation error")
		ctx.SetUserValue(keyStatusCode, fasthttp.StatusInternalServerError)
		return nil
	}

	// Respond 403 with errors
	if len(validationErrors) > 0 {
		// add schema IDs to the validation error messages
		for _, r := range validationErrors {
			r.SchemaID = &s.SchemaID
			r.SchemaVersion = s.OpenAPIRouter.SchemaVersion
		}

		s.Log.WithFields(logrus.Fields{
			"error":      validationErrors,
			"host":       strconv.B2S(ctx.Request.Header.Host()),
			"path":       strconv.B2S(ctx.Path()),
			"method":     strconv.B2S(ctx.Request.Header.Method()),
			"request_id": ctx.UserValue(web.RequestID),
		}).Debug("request validation error")

		ctx.SetUserValue(keyValidationErrors, validationErrors)
		ctx.SetUserValue(keyStatusCode, fasthttp.StatusForbidden)
		return nil
	}

	// request successfully validated
	ctx.SetUserValue(keyStatusCode, fasthttp.StatusOK)
	return nil
}
