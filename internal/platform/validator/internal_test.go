package validator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseMediaType(t *testing.T) {
	type response struct {
		mediaType string
		suffix    string
	}
	tests := []struct {
		name        string
		contentType string
		response    response
	}{
		{
			name:        "json",
			contentType: "application/json",
			response: response{
				mediaType: "application/json",
				suffix:    "",
			},
		},
		{
			name:        "json with charset",
			contentType: "application/json; charset=utf-8",
			response: response{
				mediaType: "application/json",
				suffix:    "",
			},
		},
		{
			name:        "json with suffix",
			contentType: "application/vnd.mycompany.myapp.v2+json",
			response: response{
				mediaType: "application/vnd.mycompany.myapp.v2+json",
				suffix:    "+json",
			},
		},
		{
			name:        "xml",
			contentType: "application/xml",
			response: response{
				mediaType: "application/xml",
				suffix:    "",
			},
		},
		{
			name:        "xml with charset",
			contentType: "application/xml; charset=utf-8",
			response: response{
				mediaType: "application/xml",
				suffix:    "",
			},
		},
		{
			name:        "xml with suffix",
			contentType: "application/vnd.openstreetmap.data+xml",
			response: response{
				mediaType: "application/vnd.openstreetmap.data+xml",
				suffix:    "+xml",
			},
		},
		{
			name:        "json with suffix 2",
			contentType: "application/test+myapp+json; charset=utf8",
			response: response{
				mediaType: "application/test+myapp+json",
				suffix:    "+json",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			mt, suffix := parseMediaType(tt.contentType)
			if tt.response.mediaType != mt {
				require.Error(t, fmt.Errorf("test name - %s: content type is invalid. Expected: %s. Got: %s", tt.name, tt.response.mediaType, mt))
			}

			if tt.response.suffix != suffix {
				require.Error(t, fmt.Errorf("test name - %s: content type suffix is invalid. Expected: %s. Got: %s", tt.name, tt.response.suffix, suffix))
			}
		})
	}
}
