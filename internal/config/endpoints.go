package config

import (
	"fmt"
	"strings"
)

type Endpoint struct {
	ValidationMode `mapstructure:",squash"`
	Path           string `conf:"required" validate:"url"`
	Method         string `conf:"" validate:"required"`
}

type ValidationMode struct {
	RequestValidation  string `conf:"required" validate:"required,oneof=DISABLE BLOCK LOG_ONLY"`
	ResponseValidation string `conf:"required" validate:"required,oneof=DISABLE BLOCK LOG_ONLY"`
}

type EndpointList []Endpoint

// Set method parses list of Endpoints string to the list of Endpoint objects
func (e *EndpointList) Set(value string) error {
	if value == "" {
		return nil
	}

	items := strings.Split(value, ",")
	for _, item := range items {
		parts := strings.Split(item, "|")
		if len(parts) != 3 {
			return fmt.Errorf("invalid endpoint format, expected [METHOD:]PATH|REQ|RESP")
		}

		method := ""
		path := ""

		if strings.Contains(parts[0], ":") {
			split := strings.SplitN(parts[0], ":", 2)
			method = split[0]
			path = split[1]
		} else {
			path = parts[0]
		}

		endpoint := Endpoint{
			Method: method,
			Path:   strings.TrimSpace(path),
			ValidationMode: ValidationMode{
				RequestValidation:  strings.TrimSpace(parts[1]),
				ResponseValidation: strings.TrimSpace(parts[2]),
			},
		}

		if endpoint.Path == "" || endpoint.RequestValidation == "" || endpoint.ResponseValidation == "" {
			return fmt.Errorf("invalid endpoint format, expected [METHOD:]PATH|REQ|RESP")
		}

		*e = append(*e, endpoint)
	}

	return nil
}

// String method returns a string representation of the Endpoint objects list
func (e EndpointList) String() string {
	var entries []string
	for _, ep := range e {
		entry := fmt.Sprintf("%s:%s|%s|%s", ep.Method, ep.Path, ep.RequestValidation, ep.ResponseValidation)
		entries = append(entries, entry)
	}
	return strings.Join(entries, ",")
}
