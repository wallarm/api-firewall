package mid

import (
	"github.com/gabriel-vasile/mimetype"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

// MIMETypeIdentifier identifies the MIME type of the content in case of CT header is missing
func MIMETypeIdentifier(logger *logrus.Logger) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before web.Handler) web.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {

			// get current Wallarm schema ID
			if len(ctx.Request.Header.ContentType()) == 0 && len(ctx.Request.Body()) > 0 {
				// decode request body
				requestContentEncoding := string(ctx.Request.Header.ContentEncoding())
				if requestContentEncoding != "" {
					body, err := web.GetDecompressedRequestBody(&ctx.Request, requestContentEncoding)
					if err != nil {
						logger.WithFields(logrus.Fields{
							"error":      err,
							"host":       string(ctx.Request.Header.Host()),
							"path":       string(ctx.Path()),
							"method":     string(ctx.Request.Header.Method()),
							"request_id": ctx.UserValue(web.RequestID),
						}).Error("request body decompression error")
						return web.RespondError(ctx, fasthttp.StatusInternalServerError, "")
					}
					mtype, err := mimetype.DetectReader(body)
					if err != nil {
						logger.WithFields(logrus.Fields{
							"host":       string(ctx.Request.Header.Host()),
							"path":       string(ctx.Path()),
							"method":     string(ctx.Request.Header.Method()),
							"request_id": ctx.UserValue(web.RequestID),
						}).Error("schema version mismatch")
						return web.RespondError(ctx, fasthttp.StatusInternalServerError, "")
					}

					// set the identified mime type
					ctx.Request.Header.SetContentType(mtype.String())
				}

				// set the identified mime type
				ctx.Request.Header.SetContentType(mimetype.Detect(ctx.Request.Body()).String())
			}

			err := before(ctx)

			return err
		}

		return h
	}

	return m
}
