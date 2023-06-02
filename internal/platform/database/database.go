package database

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const currentSQLSchemaVersion = 1

type OpenAPISpecStorage struct {
	Specs []SpecificationEntry
}

type SpecificationEntry struct {
	SchemaID      int    `db:"schema_id"`
	SchemaVersion string `db:"schema_version"`
	SchemaFormat  string `db:"schema_format"`
	SchemaContent string `db:"schema_content"`
}

type DBOpenAPILoader interface {
	Load(dbStoragePath string) error
	SpecificationRaw(schemaID int) []byte
	SpecificationVersion(schemaID int) string
	Specification(schemaID int) *openapi3.T
	IsLoaded(schemaID int) bool
	SchemaIDs() []int
}

type SQLLite struct {
	Log         *logrus.Logger
	RawSpecs    map[int]*SpecificationEntry
	LastUpdate  time.Time
	OpenAPISpec map[int]*openapi3.T
	lock        *sync.RWMutex
}

func getSpecBytes(spec string) []byte {
	return bytes.NewBufferString(spec).Bytes()
}

func NewOpenAPIDB(log *logrus.Logger, dbStoragePath string) (DBOpenAPILoader, error) {

	sqlObj := SQLLite{
		Log:  log,
		lock: &sync.RWMutex{},
	}

	if err := sqlObj.Load(dbStoragePath); err != nil {
		return nil, err
	}

	log.Debugf("OpenAPI specifications with the following IDs has been loaded: %v", sqlObj.SchemaIDs())

	return &sqlObj, nil
}

func (s *SQLLite) Load(dbStoragePath string) error {

	entries := make(map[int]*SpecificationEntry)
	specs := make(map[int]*openapi3.T)

	currentDBPath := dbStoragePath
	if currentDBPath == "" {
		currentDBPath = fmt.Sprintf("/var/lib/wallarm-api/%d/wallarm_api.db", currentSQLSchemaVersion)
	}

	// check if file exists
	if _, err := os.Stat(currentDBPath); errors.Is(err, os.ErrNotExist) {
		return err
	}

	db, err := sql.Open("sqlite3", currentDBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query("select schema_id,schema_version,schema_format,schema_content from openapi_schemas")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		entry := SpecificationEntry{}
		err = rows.Scan(&entry.SchemaID, &entry.SchemaVersion, &entry.SchemaFormat, &entry.SchemaContent)
		if err != nil {
			return err
		}
		entries[entry.SchemaID] = &entry
	}

	if err = rows.Err(); err != nil {
		return err
	}

	s.RawSpecs = entries
	s.LastUpdate = time.Now().UTC()

	for schemaID, spec := range s.RawSpecs {

		// parse specification
		loader := openapi3.NewLoader()
		parsedSpec, err := loader.LoadFromData(getSpecBytes(spec.SchemaContent))
		if err != nil {
			s.Log.Errorf("error: parsing of the OpenAPI specification %s (schema ID %d): %v", spec.SchemaVersion, schemaID, err)
			delete(s.RawSpecs, schemaID)
			continue
		}

		if err := parsedSpec.Validate(loader.Context); err != nil {
			s.Log.Errorf("error: validation of the OpenAPI specification %s (schema ID %d): %v", spec.SchemaVersion, schemaID, err)
			delete(s.RawSpecs, schemaID)
			continue
		}

		specs[spec.SchemaID] = parsedSpec
	}

	if len(specs) == 0 {
		return errors.New("no OpenAPI specs has been loaded")
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	s.RawSpecs = entries
	s.OpenAPISpec = specs

	return nil
}

func (s *SQLLite) Specification(schemaID int) *openapi3.T {
	s.lock.RLock()
	defer s.lock.RUnlock()

	spec, ok := s.OpenAPISpec[schemaID]
	if !ok {
		return nil
	}
	return spec
}

func (s *SQLLite) SpecificationRaw(schemaID int) []byte {
	s.lock.RLock()
	defer s.lock.RUnlock()

	rawSpec, ok := s.RawSpecs[schemaID]
	if !ok {
		return nil
	}
	return getSpecBytes(rawSpec.SchemaContent)
}

func (s *SQLLite) SpecificationVersion(schemaID int) string {
	s.lock.RLock()
	defer s.lock.RUnlock()

	rawSpec, ok := s.RawSpecs[schemaID]
	if !ok {
		return ""
	}
	return rawSpec.SchemaVersion
}

func (s *SQLLite) IsLoaded(schemaID int) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	_, ok := s.OpenAPISpec[schemaID]
	return ok
}

func (s *SQLLite) SchemaIDs() []int {
	s.lock.RLock()
	defer s.lock.RUnlock()

	var schemaIDs []int
	for _, spec := range s.RawSpecs {
		schemaIDs = append(schemaIDs, spec.SchemaID)
	}

	return schemaIDs
}
