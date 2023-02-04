package web

import (
	"bytes"
	"compress/flate"
	"encoding/json"
	"io"
	"net/http"

	"github.com/valyala/fasthttp"
)

var (
	gzip    = []byte("gzip")
	deflate = []byte("deflate")
	br      = []byte("br")
)

func GetResponseBodyUncompressed(ctx *fasthttp.RequestCtx) (io.ReadCloser, error) {

	bodyBytes := ctx.Response.Body()
	bodyReader := io.NopCloser(bytes.NewReader(bodyBytes))
	compression := ctx.Response.Header.ContentEncoding()

	if compression != nil {
		for _, sc := range [][]byte{gzip, deflate, br} {
			if bytes.Equal(sc, compression) {
				var body []byte
				var err error
				if body, err = ctx.Response.BodyUncompressed(); err != nil {
					if bytes.Equal(compression, deflate) {
						return flate.NewReader(bytes.NewReader(bodyBytes)), nil
					}
					return nil, err
				}
				return io.NopCloser(bytes.NewReader(body)), nil
			}
		}
	}

	return bodyReader, nil
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

// RespondError sends an error reponse back to the client.
func RespondError(ctx *fasthttp.RequestCtx, statusCode int, statusHeader *string) error {

	ctx.Error("", statusCode)

	// Add validation status header
	if statusHeader != nil {
		ctx.Response.Header.Add(ValidationStatus, *statusHeader)
	}

	return nil
}

// Redirect302 redirects client with code 302
func Redirect302(ctx *fasthttp.RequestCtx, redirectUrl string) error {

	ctx.Redirect(redirectUrl, 302)
	return nil
}
