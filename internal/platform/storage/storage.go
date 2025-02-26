package storage

import (
	"bytes"
	"net/url"

	"github.com/getkin/kin-openapi/openapi3"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/wallarm/api-firewall/internal/config"
)

type DBOpenAPILoader interface {
	Load(dbStoragePath string) (bool, error)
	AfterLoad(dbStoragePath string) error
	SpecificationRaw(schemaID int) interface{}
	SpecificationRawContent(schemaID int) []byte
	SpecificationVersion(schemaID int) string
	Specification(schemaID int) *openapi3.T
	IsLoaded(schemaID int) bool
	SchemaIDs() []int
	IsReady() bool
	ShouldUpdate(newStorage DBOpenAPILoader) bool
	Version() int
}

func getSpecBytes(spec string) []byte {
	return bytes.NewBufferString(spec).Bytes()
}

// NewOpenAPIDB loads OAS specs from the database and returns the struct with the parsed specs WITH database entry status update
func NewOpenAPIDB(dbStoragePath string, version int) (DBOpenAPILoader, error) {
	return loadOpenAPIDBVersion(dbStoragePath, version, true)
}

// LoadOpenAPIDB loads OAS specs from the database and returns the struct with the parsed specs WITHOUT database entry status update
func LoadOpenAPIDB(dbStoragePath string, version int) (DBOpenAPILoader, error) {
	return loadOpenAPIDBVersion(dbStoragePath, version, false)
}

// loadOpenAPIDBVersion chooses the right database schema version and then loads OAS specs from the database
func loadOpenAPIDBVersion(dbStoragePath string, version int, applyAfterLoad bool) (DBOpenAPILoader, error) {
	switch version {
	case 1:
		return NewOpenAPIDBV1(dbStoragePath)
	case 2:
		return NewOpenAPIDBV2(dbStoragePath, applyAfterLoad)
	default:
		// first trying to load db v2
		storageV2, errV2 := NewOpenAPIDBV2(dbStoragePath, applyAfterLoad)
		if errV2 == nil {
			return storageV2, errV2
		}

		return NewOpenAPIDBV1(dbStoragePath)

	}
}

// NewOpenAPIFromFileOrURL loads OAS specs from the file or URL and returns the struct with the parsed specs
func NewOpenAPIFromFileOrURL(specPath string, header *config.CustomHeader) (DBOpenAPILoader, error) {

	var specStorage DBOpenAPILoader
	var err error

	// try to parse path or URL
	apiSpecURL, err := url.ParseRequestURI(specPath)

	// can't parse string as URL. Try to load spec from file
	if err != nil || apiSpecURL == nil || apiSpecURL.Scheme == "" {
		specStorage, err = NewOpenAPIFromFile(specPath)
		if err != nil {
			return nil, errors.Wrap(err, "loading OpenAPI specification from file")
		}

		return specStorage, err
	}

	// try to load spec from
	specStorage, err = NewOpenAPIFromURL(specPath, header)
	if err != nil {
		return nil, errors.Wrap(err, "loading OpenAPI specification from URL")
	}

	return specStorage, err
}
