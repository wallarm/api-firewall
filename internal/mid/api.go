package mid

import (
	"fmt"
	strconv2 "strconv"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

const (
	XWallarmSchemaIDHeader = "X-WALLARM-SCHEMA-ID"
	WallarmSchemaID        = "WallarmSchemaID"
)

// APIHeaders prepare
func APIHeaders(logger *logrus.Logger) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before web.Handler) web.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {

			// get current Wallarm schema ID
			xWallarmSchemaID := string(ctx.Request.Header.Peek(XWallarmSchemaIDHeader))
			if xWallarmSchemaID == "" {
				logger.WithFields(logrus.Fields{
					"error":      errors.New("required X-WALLARM-SCHEMA-ID header is missing"),
					"request_id": fmt.Sprintf("#%016X", ctx.ID()),
				}).Error("error while converting http request")
				return web.RespondError(ctx, fasthttp.StatusInternalServerError, "")
			}

			// get schema version
			schemaVersion, err := strconv2.Atoi(xWallarmSchemaID)
			if err != nil {
				logger.WithFields(logrus.Fields{
					"error":      err,
					"request_id": fmt.Sprintf("#%016X", ctx.ID()),
				}).Error("error while converting http request")
				return web.RespondError(ctx, fasthttp.StatusInternalServerError, "")
			}

			// add internal header to the context
			ctx.SetUserValue(WallarmSchemaID, schemaVersion)

			// delete internal header
			ctx.Request.Header.Del(XWallarmSchemaIDHeader)

			err = before(ctx)

			return err
		}

		return h
	}

	return m
}
