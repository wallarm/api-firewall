package mid

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"

	utils "github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	coraza "github.com/wallarm/api-firewall/internal/modsec"
	"github.com/wallarm/api-firewall/internal/modsec/experimental"
	"github.com/wallarm/api-firewall/internal/modsec/types"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

// processRequest fills all transaction variables from an http.Request object
// Most implementations of Coraza will probably use http.Request objects
// so this will implement all phase 0, 1 and 2 variables
// Note: This function will stop after an interruption
// Note: Do not manually fill any request variables
func processRequest(tx types.Transaction, ctx *fasthttp.RequestCtx) (*types.Interruption, error) {
	var (
		client string
		cport  int
	)

	// IMPORTANT: Some http.Request.RemoteAddr implementations will not contain port or contain IPV6: [2001:db8::1]:8080
	client, cportStr, err := net.SplitHostPort(ctx.RemoteAddr().String())
	if err != nil {
		return nil, err
	}
	cport, _ = strconv.Atoi(cportStr)

	var in *types.Interruption
	// There is no socket access in the request object, so we neither know the server client nor port.
	tx.ProcessConnection(client, cport, "", 0)
	//tx.ProcessURI(req.URL.String(), req.Method, req.Proto)
	tx.ProcessURI(ctx.Request.URI().String(), utils.B2S(ctx.Request.Header.Method()), utils.B2S(ctx.Request.Header.Protocol()))
	ctx.Request.Header.VisitAll(func(k, v []byte) {
		tx.AddRequestHeader(utils.B2S(k), utils.B2S(v))
	})

	// Host will always be removed from req.Headers() and promoted to the
	// Request.Host field, so we manually add it
	if host := utils.B2S(ctx.Request.Host()); host != "" {
		tx.AddRequestHeader("Host", host)
		// This connector relies on the host header (now host field) to populate ServerName
		tx.SetServerName(host)
	}

	// Transfer-Encoding header is removed by go/http
	// We manually add it to make rules relying on it work (E.g. CRS rule 920171)
	if te := ctx.Request.Header.Peek(fasthttp.HeaderTransferEncoding); te != nil {
		tx.AddRequestHeader("Transfer-Encoding", utils.B2S(te))
	}

	in = tx.ProcessRequestHeaders()
	if in != nil {
		return in, nil
	}

	if tx.IsRequestBodyAccessible() {
		// We only do body buffering if the transaction requires request
		// body inspection, otherwise we just let the request follow its
		// regular flow.
		bodyRaw := ctx.Request.Body()
		bodyReader := io.NopCloser(ctx.Request.BodyStream())
		if bodyRaw != nil {
			it, _, err := tx.ReadRequestBodyFrom(bodyReader)
			if err != nil {
				return nil, fmt.Errorf("failed to append request body: %s", err.Error())
			}

			if it != nil {
				return it, nil
			}

			rbr, err := tx.RequestBodyReader()
			if err != nil {
				return nil, fmt.Errorf("failed to get the request body: %s", err.Error())
			}

			bodyReader = io.NopCloser(ctx.Request.BodyStream())

			// Adds all remaining bytes beyond the coraza limit to its buffer
			// It happens when the partial body has been processed and it did not trigger an interruption
			body := io.MultiReader(rbr, bodyReader)
			// req.Body is transparently reinizialied with a new io.ReadCloser.
			// The http handler will be able to read it.
			// Prior to Go 1.19 NopCloser does not implement WriterTo if the reader implements it.
			// - https://github.com/golang/go/issues/51566
			// - https://tip.golang.org/doc/go1.19#minor_library_changes
			// This avoid errors like "failed to process request: malformed chunked encoding" when
			// using io.Copy.
			// In Go 1.19 we just do `req.Body = io.NopCloser(reader)`
			ctx.Request.SetBodyStream(body, -1)
			//if rwt, ok := body.(io.WriterTo); ok {
			//	req.Body = struct {
			//		io.Reader
			//		io.WriterTo
			//		io.Closer
			//	}{body, rwt, req.Body}
			//} else {
			//	req.Body = struct {
			//		io.Reader
			//		io.Closer
			//	}{body, req.Body}
			//}
		}
	}

	return tx.ProcessRequestBody()
}

func WAFModSecurity(waf coraza.WAF, logger *logrus.Logger) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before web.Handler) web.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {

			if waf == nil {
				err := before(ctx)

				// Return the error, so it can be handled further up the chain.
				return err
			}

			newTX := func(requestCtx *fasthttp.RequestCtx) types.Transaction {
				return waf.NewTransaction()
			}

			if ctxwaf, ok := waf.(experimental.WAFWithOptions); ok {
				newTX = func(requestCtx *fasthttp.RequestCtx) types.Transaction {
					return ctxwaf.NewTransactionWithOptions(experimental.Options{
						Context: requestCtx,
					})
				}
			}

			tx := newTX(ctx)
			defer func() {
				// We run phase 5 rules and create audit logs (if enabled)
				tx.ProcessLogging()
				// we remove temporary files and free some memory
				if err := tx.Close(); err != nil {
					tx.DebugLogger().Error().Err(err).Msg("Failed to close the transaction")
				}
			}()

			// Early return, Coraza is not going to process any rule
			if !tx.IsRuleEngineOff() {
				// ProcessRequest is just a wrapper around ProcessConnection, ProcessURI,
				// ProcessRequestHeaders and ProcessRequestBody.
				// It fails if any of these functions returns an error and it stops on interruption.
				if it, err := processRequest(tx, ctx); err != nil {
					tx.DebugLogger().Error().Err(err).Msg("Failed to process request")
					return web.RespondError(ctx, fasthttp.StatusBadRequest, "")
				} else if it != nil {
					if err := web.RespondError(ctx, obtainStatusCodeFromInterruptionOrDefault(it, http.StatusOK), ""); err != nil {
						return err
					}
					return nil
				}
			}

			// Run the handler chain and catch any propagated error.
			if err := before(ctx); err != nil {

				// Log the error.
				logger.WithFields(logrus.Fields{
					"request_id": fmt.Sprintf("#%016X", ctx.ID()),
					"error":      err,
				}).Error("common error")

				// Respond to the error.
				if err := web.RespondError(ctx, fasthttp.StatusInternalServerError, ""); err != nil {
					return err
				}

				// If we receive the shutdown err we need to return it
				// back to the base handler to shutdown the service.
				if ok := web.IsShutdown(err); ok {
					return err
				}
			}

			if tx.IsResponseBodyAccessible() && tx.IsResponseBodyProcessable() {
				if it, err := tx.ProcessResponseBody(); err != nil {
					web.RespondError(ctx, fasthttp.StatusInternalServerError, "")
					return err
				} else if it != nil {
					ctx.Response.Header.SetContentLength(0)
					web.RespondError(ctx, obtainStatusCodeFromInterruptionOrDefault(it, ctx.Response.Header.StatusCode()), "")
					return nil
				}

				// we release the buffer
				reader, err := tx.ResponseBodyReader()
				if err != nil {
					web.RespondError(ctx, fasthttp.StatusInternalServerError, "")
					return fmt.Errorf("failed to release the response body reader: %v", err)
				}

				// this is the last opportunity we have to report the resolved status code
				// as next step is write into the response writer (triggering a 200 in the
				// response status code.)
				if _, err := io.Copy(ctx.Response.BodyWriter(), reader); err != nil {
					return fmt.Errorf("failed to copy the response body: %v", err)
				}
			}

			// The error has been handled so we can stop propagating it.
			return nil
		}

		return h
	}

	return m
}

// obtainStatusCodeFromInterruptionOrDefault returns the desired status code derived from the interruption
// on a "deny" action or a default value.
func obtainStatusCodeFromInterruptionOrDefault(it *types.Interruption, defaultStatusCode int) int {
	if it.Action == "deny" {
		statusCode := it.Status
		if statusCode == 0 {
			statusCode = 403
		}

		return statusCode
	}

	return defaultStatusCode
}
