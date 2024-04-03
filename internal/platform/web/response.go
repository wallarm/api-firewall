package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/klauspost/compress/flate"
	"github.com/klauspost/compress/zlib"
	"github.com/valyala/fasthttp"
	"github.com/wundergraph/graphql-go-tools/pkg/graphql"
	"golang.org/x/exp/slices"
)

// List of the supported compression schemes
var (
	supportedEncodings = []string{"gzip", "deflate", "br"}
)

// GetDecompressedResponseBody function returns the Reader of the decompressed response body
func GetDecompressedResponseBody(resp *fasthttp.Response, contentEncoding string) (io.ReadCloser, error) {

	bodyBytes := resp.Body()

	if contentEncoding != "" {
		if slices.Contains(supportedEncodings, contentEncoding) {
			var body []byte
			var err error
			if body, err = resp.BodyUncompressed(); err != nil {
				if errors.Is(zlib.ErrHeader, err) && contentEncoding == "deflate" {
					// deflate rfc 1951 implementation
					return flate.NewReader(bytes.NewReader(bodyBytes)), nil
				}
				// got error while body decompression
				return nil, err
			}
			// body has been successfully uncompressed
			return io.NopCloser(bytes.NewReader(body)), nil
		}
		// body compression schema not supported
		return nil, fasthttp.ErrContentEncodingUnsupported
	}

	// body without compression
	return io.NopCloser(bytes.NewReader(bodyBytes)), nil
}

// GetDecompressedRequestBody function returns the Reader of the decompressed request body
func GetDecompressedRequestBody(req *fasthttp.Request, contentEncoding string) (io.ReadCloser, error) {

	bodyBytes := req.Body()

	if contentEncoding != "" {
		if slices.Contains(supportedEncodings, contentEncoding) {
			var body []byte
			var err error
			if body, err = req.BodyUncompressed(); err != nil {
				if errors.Is(zlib.ErrHeader, err) && contentEncoding == "deflate" {
					// deflate rfc 1951 implementation
					return flate.NewReader(bytes.NewReader(bodyBytes)), nil
				}
				// got error while body decompression
				return nil, err
			}
			// body has been successfully uncompressed
			return io.NopCloser(bytes.NewReader(body)), nil
		}
		// body compression schema not supported
		return nil, fasthttp.ErrContentEncodingUnsupported
	}

	// body without compression
	return io.NopCloser(bytes.NewReader(bodyBytes)), nil
}

// Respond converts a Go value to JSON and sends it to the client.
func Respond(ctx *fasthttp.RequestCtx, data interface{}, statusCode int) error {
	// If there is nothing to marshal then set status code and return.
	if statusCode == http.StatusNoContent {
		ctx.SetStatusCode(statusCode)
		return nil
	}

	// Convert the response value to JSON.
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Set the content type and headers once we know marshaling has succeeded.
	ctx.SetContentType("application/json")

	// Write the status code to the response.
	ctx.SetStatusCode(statusCode)

	// Send the result back to the client.
	if _, err := ctx.Write(jsonData); err != nil {
		return err
	}

	return nil
}

// RespondError sends an error response back to the client.
func RespondError(ctx *fasthttp.RequestCtx, statusCode int, statusHeader string) error {

	ctx.Error("", statusCode)

	// Add validation status header
	if statusHeader != "" {
		ctx.Response.Header.Add(ValidationStatus, statusHeader)
	}

	return nil
}

// RespondGraphQLErrors sends errors back to the client via GraphQL
func RespondGraphQLErrors(ctx *fasthttp.Response, errors error) error {

	gqlErrors := graphql.RequestErrorsFromError(errors)

	ctx.Header.Set("Content-Type", "application/json")

	if _, err := gqlErrors.WriteResponse(ctx.BodyWriter()); err != nil {
		return err
	}

	return nil
}

// RespondAPIModeErrors sends API mode specific response back to the client
func RespondAPIModeErrors(ctx *fasthttp.RequestCtx, code, message string) error {

	schemaIDstr, ok := ctx.UserValue(RequestSchemaID).(string)
	if !ok {
		ctx.SetUserValue(GlobalResponseStatusCodeKey, fasthttp.StatusInternalServerError)
		return errors.New("request schema ID not found in the request")
	}
	schemaID, err := strconv.Atoi(schemaIDstr)
	if err != nil {
		ctx.SetUserValue(GlobalResponseStatusCodeKey, fasthttp.StatusInternalServerError)
		return err
	}

	keyValidationErrors := schemaIDstr + APIModePostfixValidationErrors
	keyStatusCode := schemaIDstr + APIModePostfixStatusCode
	ctx.SetUserValue(keyValidationErrors, []*ValidationError{{Message: message, Code: code, SchemaID: &schemaID}})
	ctx.SetUserValue(keyStatusCode, fasthttp.StatusForbidden)

	return nil
}
