package loader

import (
	"context"
	"errors"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

var (
	ErrOASValidation = errors.New("OpenAPI specification validation error")
	ErrOASParsing    = errors.New("OpenAPI specification parsing error")
)

func validateOAS(spec *openapi3.T) error {

	if err := spec.Validate(
		context.Background(),
		openapi3.DisableExamplesValidation(),
		openapi3.DisableSchemaFormatValidation(),
		openapi3.DisableSchemaDefaultsValidation(),
		openapi3.DisableSchemaPatternValidation(),
	); err != nil {
		return err
	}

	return nil
}

func ParseOAS(schema []byte, SchemaVersion string, schemaID int) (*openapi3.T, error) {

	// parse specification
	loader := openapi3.NewLoader()
	parsedSpec, err := loader.LoadFromData(schema)
	if err != nil {
		return nil, fmt.Errorf("%w: schema version %s, schema ID %d: %w", ErrOASParsing, SchemaVersion, schemaID, err)
	}

	if err := validateOAS(parsedSpec); err != nil {
		return nil, fmt.Errorf("%w: schema version %s, schema ID %d: %w: ", ErrOASValidation, SchemaVersion, schemaID, err)
	}

	return parsedSpec, nil
}
