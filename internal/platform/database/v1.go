package database

import (
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const currentSQLSchemaVersionV1 = 1

type SpecificationEntryV1 struct {
	SchemaID      int    `db:"schema_id"`
	SchemaVersion string `db:"schema_version"`
	SchemaFormat  string `db:"schema_format"`
	SchemaContent string `db:"schema_content"`
}

type SQLLiteV1 struct {
	isReady     bool
	Log         *logrus.Logger
	RawSpecs    map[int]*SpecificationEntryV1
	LastUpdate  time.Time
	OpenAPISpec map[int]*openapi3.T
	lock        *sync.RWMutex
}

func getSchemaVersions(dbSpecs DBOpenAPILoader) map[int]string {
	result := make(map[int]string)
	schemaIDs := dbSpecs.SchemaIDs()
	for _, schemaID := range schemaIDs {
		result[schemaID] = dbSpecs.SpecificationVersion(schemaID)
	}
	return result
}

func NewOpenAPIDBV1(log *logrus.Logger, dbStoragePath string) (DBOpenAPILoader, error) {

	sqlObj := SQLLiteV1{
		Log:         log,
		lock:        &sync.RWMutex{},
		RawSpecs:    make(map[int]*SpecificationEntryV1),
		OpenAPISpec: make(map[int]*openapi3.T),
		isReady:     true,
	}

	if err := sqlObj.Load(dbStoragePath); err != nil {
		sqlObj.isReady = false
		return &sqlObj, err
	}

	log.Debugf("OpenAPI specifications with the following IDs were found in the DB: %v", sqlObj.SchemaIDs())

	return &sqlObj, nil
}

func (s *SQLLiteV1) Load(dbStoragePath string) error {

	entries := make(map[int]*SpecificationEntryV1)
	specs := make(map[int]*openapi3.T)

	currentDBPath := dbStoragePath
	if currentDBPath == "" {
		currentDBPath = fmt.Sprintf("/var/lib/wallarm-api/%d/wallarm_api.db", currentSQLSchemaVersionV1)
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
		entry := SpecificationEntryV1{}
		err = rows.Scan(&entry.SchemaID, &entry.SchemaVersion, &entry.SchemaFormat, &entry.SchemaContent)
		if err != nil {
			return err
		}
		entries[entry.SchemaID] = &entry
	}

	if err := rows.Err(); err != nil {
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

	s.lock.Lock()
	defer s.lock.Unlock()

	s.RawSpecs = entries
	s.OpenAPISpec = specs

	return nil
}

func (s *SQLLiteV1) Specification(schemaID int) *openapi3.T {
	s.lock.RLock()
	defer s.lock.RUnlock()

	spec, ok := s.OpenAPISpec[schemaID]
	if !ok {
		return nil
	}
	return spec
}

func (s *SQLLiteV1) SpecificationRaw(schemaID int) interface{} {
	s.lock.RLock()
	defer s.lock.RUnlock()

	rawSpec, ok := s.RawSpecs[schemaID]
	if !ok {
		return nil
	}
	return rawSpec
}

func (s *SQLLiteV1) SpecificationRawContent(schemaID int) []byte {
	s.lock.RLock()
	defer s.lock.RUnlock()

	rawSpec, ok := s.RawSpecs[schemaID]
	if !ok {
		return nil
	}
	return getSpecBytes(rawSpec.SchemaContent)
}

func (s *SQLLiteV1) SpecificationVersion(schemaID int) string {
	s.lock.RLock()
	defer s.lock.RUnlock()

	rawSpec, ok := s.RawSpecs[schemaID]
	if !ok {
		return ""
	}
	return rawSpec.SchemaVersion
}

func (s *SQLLiteV1) IsLoaded(schemaID int) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	_, ok := s.OpenAPISpec[schemaID]
	return ok
}

func (s *SQLLiteV1) SchemaIDs() []int {
	s.lock.RLock()
	defer s.lock.RUnlock()

	var schemaIDs = make([]int, 0)
	for _, spec := range s.RawSpecs {
		schemaIDs = append(schemaIDs, spec.SchemaID)
	}

	sort.Ints(schemaIDs)

	return schemaIDs
}

func (s *SQLLiteV1) IsReady() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.isReady
}

func (s *SQLLiteV1) Version() int {
	return currentSQLSchemaVersionV1
}

func (s *SQLLiteV1) AfterLoad(dbStoragePath string) error {
	return nil
}

func (s *SQLLiteV1) ShouldUpdate(newStorage DBOpenAPILoader) bool {

	beforeUpdateSpecs := getSchemaVersions(s)
	afterUpdateSpecs := getSchemaVersions(newStorage)

	if !reflect.DeepEqual(beforeUpdateSpecs, afterUpdateSpecs) {
		return true
	}

	return false
}
