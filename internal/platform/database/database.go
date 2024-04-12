package database

import (
	"bytes"

	"github.com/getkin/kin-openapi/openapi3"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

type DBOpenAPILoader interface {
	Load(dbStoragePath string) error
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

// NewOpenAPIDB loads OAS specs from the database and returns the struct with the parsed specs
func NewOpenAPIDB(log *logrus.Logger, dbStoragePath string, version int) (DBOpenAPILoader, error) {

	switch version {
	case 1:
		return NewOpenAPIDBV1(log, dbStoragePath)
	case 2:
		return NewOpenAPIDBV2(log, dbStoragePath)
	default:
		// first trying to load db v2
		storageV2, errV2 := NewOpenAPIDBV2(log, dbStoragePath)
		if errV2 == nil {
			return storageV2, errV2
		}

		return NewOpenAPIDBV1(log, dbStoragePath)

	}

}
