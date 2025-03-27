package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	strconv2 "strconv"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	_ "github.com/mattn/go-sqlite3"
	"github.com/wallarm/api-firewall/internal/platform/loader"
)

const currentSQLSchemaVersionV2 = 2

type SpecificationEntryV2 struct {
	SchemaID      int    `db:"schema_id"`
	SchemaVersion string `db:"schema_version"`
	SchemaFormat  string `db:"schema_format"`
	SchemaContent string `db:"schema_content"`
	Status        string `db:"status"`
}

type SQLLiteV2 struct {
	isReady     bool
	RawSpecs    map[int]*SpecificationEntryV2
	LastUpdate  time.Time
	OpenAPISpec map[int]*openapi3.T
	lock        *sync.RWMutex
}

func NewOpenAPIDBV2(dbStoragePath string, execAfterLoad bool) (DBOpenAPILoader, error) {

	sqlObj := SQLLiteV2{
		lock:        &sync.RWMutex{},
		RawSpecs:    make(map[int]*SpecificationEntryV2),
		OpenAPISpec: make(map[int]*openapi3.T),
		isReady:     true,
	}

	var err error
	sqlObj.isReady, err = sqlObj.Load(dbStoragePath)

	if execAfterLoad {
		if errAfterLoad := sqlObj.AfterLoad(dbStoragePath); errAfterLoad != nil {
			if sqlObj.isReady {
				sqlObj.isReady = false
			}
			err = errors.Join(err, errAfterLoad)
		}
	}

	return &sqlObj, err
}

func (s *SQLLiteV2) Load(dbStoragePath string) (bool, error) {

	entries := make(map[int]*SpecificationEntryV2)
	specs := make(map[int]*openapi3.T)

	var parsingErrs error
	var isReady bool

	currentDBPath := dbStoragePath
	if currentDBPath == "" {
		currentDBPath = fmt.Sprintf("/var/lib/wallarm-api/%d/wallarm_api.db", currentSQLSchemaVersionV2)
	}

	// check if file exists
	if _, err := os.Stat(currentDBPath); errors.Is(err, os.ErrNotExist) {
		return isReady, err
	}

	db, err := sql.Open("sqlite3", currentDBPath)
	if err != nil {
		return isReady, err
	}
	defer db.Close()

	rows, err := db.Query("select schema_id,schema_version,schema_format,schema_content,status from openapi_schemas")
	if err != nil {
		return isReady, err
	}
	defer rows.Close()

	for rows.Next() {
		entry := SpecificationEntryV2{}
		err = rows.Scan(&entry.SchemaID, &entry.SchemaVersion, &entry.SchemaFormat, &entry.SchemaContent, &entry.Status)
		if err != nil {
			return isReady, err
		}
		entries[entry.SchemaID] = &entry
	}

	if err := rows.Err(); err != nil {
		return isReady, err
	}

	s.RawSpecs = entries
	s.LastUpdate = time.Now().UTC()

	for schemaID, spec := range s.RawSpecs {

		parsedSpec, err := loader.ParseOAS(getSpecBytes(spec.SchemaContent), spec.SchemaVersion, schemaID)
		if err != nil {
			parsingErrs = errors.Join(parsingErrs, err)
			delete(s.RawSpecs, schemaID)
			continue
		}

		specs[spec.SchemaID] = parsedSpec
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	s.RawSpecs = entries
	s.OpenAPISpec = specs
	isReady = true

	return isReady, parsingErrs
}

func (s *SQLLiteV2) Specification(schemaID int) *openapi3.T {
	s.lock.RLock()
	defer s.lock.RUnlock()

	spec, ok := s.OpenAPISpec[schemaID]
	if !ok {
		return nil
	}
	return spec
}

func (s *SQLLiteV2) SpecificationRaw(schemaID int) any {
	s.lock.RLock()
	defer s.lock.RUnlock()

	rawSpec, ok := s.RawSpecs[schemaID]
	if !ok {
		return nil
	}
	return rawSpec
}

func (s *SQLLiteV2) SpecificationRawContent(schemaID int) []byte {
	s.lock.RLock()
	defer s.lock.RUnlock()

	rawSpec, ok := s.RawSpecs[schemaID]
	if !ok {
		return nil
	}
	return getSpecBytes(rawSpec.SchemaContent)
}

func (s *SQLLiteV2) SpecificationVersion(schemaID int) string {
	s.lock.RLock()
	defer s.lock.RUnlock()

	rawSpec, ok := s.RawSpecs[schemaID]
	if !ok {
		return ""
	}
	return rawSpec.SchemaVersion
}

func (s *SQLLiteV2) IsLoaded(schemaID int) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	_, ok := s.OpenAPISpec[schemaID]
	return ok
}

func (s *SQLLiteV2) SchemaIDs() []int {
	s.lock.RLock()
	defer s.lock.RUnlock()

	var schemaIDs = make([]int, 0)
	for _, spec := range s.RawSpecs {
		schemaIDs = append(schemaIDs, spec.SchemaID)
	}

	sort.Ints(schemaIDs)

	return schemaIDs
}

func (s *SQLLiteV2) IsReady() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.isReady
}

func (s *SQLLiteV2) Version() int {
	return currentSQLSchemaVersionV2
}

func (s *SQLLiteV2) AfterLoad(dbStoragePath string) error {

	currentDBPath := dbStoragePath
	if currentDBPath == "" {
		currentDBPath = fmt.Sprintf("/var/lib/wallarm-api/%d/wallarm_api.db", currentSQLSchemaVersionV2)
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

	var updatedSchemaIDs []string
	for id, spec := range s.RawSpecs {
		if spec.Status == "new" {
			updatedSchemaIDs = append(updatedSchemaIDs, strconv2.Itoa(id))
		}
	}

	// nothing to update
	if updatedSchemaIDs == nil {
		return nil
	}

	q := fmt.Sprintf("UPDATE openapi_schemas SET status = 'applied' WHERE schema_id IN (%s)", strings.Join(updatedSchemaIDs, ","))
	_, err = db.Exec(q)
	if err != nil {
		return err
	}

	// update current struct
	for _, specId := range updatedSchemaIDs {
		id, err := strconv2.Atoi(specId)
		if err != nil {
			return err
		}

		s.RawSpecs[id].Status = "applied"
	}

	return nil
}

func (s *SQLLiteV2) ShouldUpdate(newStorage DBOpenAPILoader) bool {

	newSchemaIDs := newStorage.SchemaIDs()
	currentSchemaIDs := s.SchemaIDs()

	// check if number of API spec in the database has changed
	if len(currentSchemaIDs) != len(newSchemaIDs) {
		return true
	}

	// check if schema ID of API specs in the database has changed
	for _, sid := range newSchemaIDs {
		if !slices.Contains(currentSchemaIDs, sid) {
			return true
		}
	}

	// check if status OR schema version of API spec in the database has changed
	for _, id := range newSchemaIDs {
		if rawSpec, ok := newStorage.SpecificationRaw(id).(*SpecificationEntryV2); ok {
			if rawSpec.Status == "new" ||
				rawSpec.SchemaVersion != s.SpecificationVersion(id) {
				return true
			}
		}
	}

	return false
}
