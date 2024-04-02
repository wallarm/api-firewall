package mid

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/corazawaf/coraza/v3"
	"github.com/corazawaf/coraza/v3/experimental"
	"github.com/corazawaf/coraza/v3/types"
	"github.com/pkg/errors"
	utils "github.com/savsgio/gotils/strconv"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

type ModSecurityOptions struct {
	Mode                  string
	WAF                   coraza.WAF
	Logger                *logrus.Logger
	RequestValidation     string
	ResponseValidation    string
	CustomBlockStatusCode int
}

var ErrModSecMaliciousRequest = errors.New("malicious request")

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
	rUri := ctx.Request.URI().String()
	tx.ProcessURI(rUri, utils.B2S(ctx.Request.Header.Method()), utils.B2S(ctx.Request.Header.Protocol()))
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
		bodyReader := io.NopCloser(bytes.NewReader(bodyRaw))
		if bodyRaw != nil {
			it, _, err := tx.ReadRequestBodyFrom(bodyReader)
			if err != nil {
				return nil, fmt.Errorf("failed to append request body: %s", err.Error())
			}

			if it != nil {
				return it, nil
			}
		}
	}

	return tx.ProcessRequestBody()
}

func WAFModSecurity(options *ModSecurityOptions) web.Middleware {

	// This is the actual middleware function to be executed.
	m := func(before web.Handler) web.Handler {

		// Create the handler that will be attached in the middleware chain.
		h := func(ctx *fasthttp.RequestCtx) error {

			if options.WAF == nil {
				err := before(ctx)

				// Return the error, so it can be handled further up the chain.
				return err
			}

			newTX := func(requestCtx *fasthttp.RequestCtx) types.Transaction {
				return options.WAF.NewTransaction()
			}

			if ctxwaf, ok := options.WAF.(experimental.WAFWithOptions); ok {
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
			if tx.IsRuleEngineOff() {
				return nil
			}

			if !strings.EqualFold(options.RequestValidation, web.ValidationDisable) {
				// ProcessRequest is just a wrapper around ProcessConnection, ProcessURI,
				// ProcessRequestHeaders and ProcessRequestBody.
				// It fails if any of these functions returns an error and it stops on interruption.
				if it, err := processRequest(tx, ctx); err != nil {
					tx.DebugLogger().Error().Err(err).Msg("Failed to process request")

					if options.Mode == web.APIMode {
						if err := web.RespondAPIModeErrors(ctx, "ModSecurity rules: failed to process request", err.Error()); err != nil {
							options.Logger.WithFields(logrus.Fields{
								"host":       utils.B2S(ctx.Request.Header.Host()),
								"path":       utils.B2S(ctx.Path()),
								"method":     utils.B2S(ctx.Request.Header.Method()),
								"request_id": ctx.UserValue(web.RequestID),
							}).Error(err)
						}
						return nil
					}

					if strings.EqualFold(options.RequestValidation, web.ValidationBlock) {
						return err
					}
				} else if it != nil {

					if options.Mode == web.APIMode {
						if err := web.RespondAPIModeErrors(ctx, ErrModSecMaliciousRequest.Error(), fmt.Sprintf("ModSecurity rules: request blocked due to rule %d", it.RuleID)); err != nil {
							options.Logger.WithFields(logrus.Fields{
								"host":       utils.B2S(ctx.Request.Header.Host()),
								"path":       utils.B2S(ctx.Path()),
								"method":     utils.B2S(ctx.Request.Header.Method()),
								"request_id": ctx.UserValue(web.RequestID),
							}).Error(err)
						}
						return nil
					}

					if strings.EqualFold(options.RequestValidation, web.ValidationBlock) {
						return performResponseAction(ctx, it, options.CustomBlockStatusCode)
					}
				}
			}

			// Run the handler chain and catch any propagated error.
			if err := before(ctx); err != nil {
				return err
			}

			// do not check response in the API mode
			if options.Mode == web.APIMode {
				return nil
			}

			if !strings.EqualFold(options.ResponseValidation, web.ValidationDisable) {

				ctx.Response.Header.VisitAll(func(k, v []byte) {
					tx.AddResponseHeader(utils.B2S(k), utils.B2S(v))
				})

				if it := tx.ProcessResponseHeaders(ctx.Response.StatusCode(), utils.B2S(ctx.Request.Header.Protocol())); it != nil {
					if strings.EqualFold(options.ResponseValidation, web.ValidationBlock) {
						return performResponseAction(ctx, it, options.CustomBlockStatusCode)
					}
				}

				if tx.IsResponseBodyAccessible() && tx.IsResponseBodyProcessable() {

					// read response body
					bodyRaw := ctx.Response.Body()
					bodyReader := io.NopCloser(bytes.NewReader(bodyRaw))
					if bodyRaw != nil {
						it, _, err := tx.ReadResponseBodyFrom(bodyReader)
						if err != nil {
							return fmt.Errorf("failed to append request body: %s", err.Error())
						}

						if it != nil {
							if strings.EqualFold(options.ResponseValidation, web.ValidationBlock) {
								return performResponseAction(ctx, it, options.CustomBlockStatusCode)
							}
						}
					}

					if it, err := tx.ProcessResponseBody(); err != nil {
						tx.DebugLogger().Error().Err(err).Msg("Failed to process response")

						if strings.EqualFold(options.ResponseValidation, web.ValidationBlock) {
							return err
						}
					} else if it != nil {

						if strings.EqualFold(options.ResponseValidation, web.ValidationBlock) {
							return performResponseAction(ctx, it, options.CustomBlockStatusCode)
						}
					}
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
func performResponseAction(ctx *fasthttp.RequestCtx, it *types.Interruption, blockStatusCode int) error {

	switch it.Action {
	case "deny", "drop":
		statusCode := it.Status
		if statusCode == 0 {
			statusCode = blockStatusCode
		}

		ctx.Response.Header.SetContentLength(0)
		return web.RespondError(ctx, statusCode, "")
	case "redirect":
		ctx.Response.Header.SetContentLength(0)
		ctx.Redirect(it.Data, it.Status)
		return nil
	}

	return nil
}
