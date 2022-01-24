package web

import (
	"encoding/json"
	"net/http"

	"github.com/valyala/fasthttp"
)

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
