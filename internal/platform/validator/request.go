package validator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
)

const prefixInvalidCT = "header Content-Type has unexpected value"

// ValidateRequest is used to validate the given input according to previous
// loaded OpenAPIv3 spec. If the input does not match the OpenAPIv3 spec, a
// non-nil error will be returned.
//
// Note: One can tune the behavior of uniqueItems: true verification
// by registering a custom function with openapi3.RegisterArrayUniqueItemsChecker
func ValidateRequest(ctx context.Context, input *openapi3filter.RequestValidationInput) error {
	var (
		err error
		me  openapi3.MultiError
	)

	options := input.Options
	if options == nil {
		options = openapi3filter.DefaultOptions
	}
	route := input.Route
	operation := route.Operation
	operationParameters := operation.Parameters
	pathItemParameters := route.PathItem.Parameters

	// Security
	security := operation.Security
	// If there aren't any security requirements for the operation
	if security == nil {
		// Use the global security requirements.
		security = &route.Spec.Security
	}
	if security != nil {
		if err = openapi3filter.ValidateSecurityRequirements(ctx, input, *security); err != nil && !options.MultiError {
			return err
		}

		if err != nil {
			me = append(me, err)
		}
	}

	// For each parameter of the PathItem
	for _, parameterRef := range pathItemParameters {
		parameter := parameterRef.Value
		if operationParameters != nil {
			if override := operationParameters.GetByInAndName(parameter.In, parameter.Name); override != nil {
				continue
			}
		}

		if err = openapi3filter.ValidateParameter(ctx, input, parameter); err != nil && !options.MultiError {
			return err
		}

		if err != nil {
			me = append(me, err)
		}
	}

	// For each parameter of the Operation
	for _, parameter := range operationParameters {
		if err = openapi3filter.ValidateParameter(ctx, input, parameter.Value); err != nil && !options.MultiError {
			return err
		}

		if err != nil {
			me = append(me, err)
		}
	}

	// RequestBody
	requestBody := operation.RequestBody
	if requestBody != nil && !options.ExcludeRequestBody {
		if err = ValidateRequestBody(ctx, input, requestBody.Value); err != nil && !options.MultiError {
			return err
		}

		if err != nil {
			me = append(me, err)
		}
	}

	if len(me) > 0 {
		return me
	}

	return nil
}

// ValidateRequestBody validates data of a request's body.
//
// The function returns RequestError with ErrInvalidRequired cause when a value is required but not defined.
// The function returns RequestError with a openapi3.SchemaError cause when a value is invalid by JSON schema.
func ValidateRequestBody(ctx context.Context, input *openapi3filter.RequestValidationInput, requestBody *openapi3.RequestBody) error {
	var (
		req  = input.Request
		data []byte
	)

	options := input.Options
	if options == nil {
		options = openapi3filter.DefaultOptions
	}

	if req.Body != http.NoBody && req.Body != nil {
		defer req.Body.Close()
		var err error
		if data, err = io.ReadAll(req.Body); err != nil {
			return &openapi3filter.RequestError{
				Input:       input,
				RequestBody: requestBody,
				Reason:      "reading failed",
				Err:         err,
			}
		}
		// Put the data back into the input
		req.Body = io.NopCloser(bytes.NewReader(data))
	}

	if len(data) == 0 {
		if requestBody.Required {
			return &openapi3filter.RequestError{Input: input, RequestBody: requestBody, Err: openapi3filter.ErrInvalidRequired}
		}
		return nil
	}

	content := requestBody.Content
	if len(content) == 0 {
		// A request's body does not have declared content, so skip validation.
		return nil
	}

	inputMIME := req.Header.Get(headerCT)
	contentType := requestBody.Content.Get(inputMIME)
	if contentType == nil {
		return &openapi3filter.RequestError{
			Input:       input,
			RequestBody: requestBody,
			Reason:      fmt.Sprintf("%s %q", prefixInvalidCT, inputMIME),
		}
	}

	if contentType.Schema == nil {
		// A JSON schema that describes the received data is not declared, so skip validation.
		return nil
	}

	encFn := func(name string) *openapi3.Encoding { return contentType.Encoding[name] }
	mediaType, value, err := decodeBody(bytes.NewReader(data), req.Header, contentType.Schema, encFn)
	if err != nil {
		return &openapi3filter.RequestError{
			Input:       input,
			RequestBody: requestBody,
			Reason:      "failed to decode request body",
			Err:         err,
		}
	}

	defaultsSet := false
	opts := make([]openapi3.SchemaValidationOption, 0, 3) // 3 potential opts here
	opts = append(opts, openapi3.VisitAsRequest())
	opts = append(opts, openapi3.DefaultsSet(func() { defaultsSet = true }))
	if options.MultiError {
		opts = append(opts, openapi3.MultiErrors())
	}

	// Validate JSON with the schema
	if err := contentType.Schema.Value.VisitJSON(value, opts...); err != nil {
		return &openapi3filter.RequestError{
			Input:       input,
			RequestBody: requestBody,
			Reason:      "doesn't match the schema",
			Err:         err,
		}
	}

	if defaultsSet {
		var err error
		if data, err = encodeBody(value, mediaType); err != nil {
			return &openapi3filter.RequestError{
				Input:       input,
				RequestBody: requestBody,
				Reason:      "rewriting failed",
				Err:         err,
			}
		}
		// Put the data back into the input
		req.Body = io.NopCloser(bytes.NewReader(data))
	}

	return nil
}
