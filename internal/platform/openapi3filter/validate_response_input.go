package openapi3filter

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/valyala/fasthttp"
)

type ResponseValidationInput struct {
	RequestValidationInput *RequestValidationInput
	Status                 int
	ResponseHeader         *fasthttp.ResponseHeader
	Body                   io.ReadCloser
	Options                *Options
}

func (input *ResponseValidationInput) SetBodyBytes(value []byte) *ResponseValidationInput {
	input.Body = ioutil.NopCloser(bytes.NewReader(value))
	return input
}

var JSONPrefixes = []string{
	")]}',\n",
}

// TrimJSONPrefix trims one of the possible prefixes
func TrimJSONPrefix(data []byte) []byte {
search:
	for _, prefix := range JSONPrefixes {
		if len(data) < len(prefix) {
			continue
		}
		for i, b := range data[:len(prefix)] {
			if b != prefix[i] {
				continue search
			}
		}
		return data[len(prefix):]
	}
	return data
}
