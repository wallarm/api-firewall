package database

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const currentSqlVersion = 1

type OpenAPISchemas struct {
	SchemaID      int    `db:"schema_id"`
	SchemaVersion string `db:"schema_version"`
	SchemaFormat  string `db:"schema_format"`
	SchemaContent string `db:"schema_content"`
}

type DBOpenAPILoader interface {
	Load(dbStoragePath string) error
	SpecificationRaw() []byte
	SpecificationVersion() string
	SchemaID() int
	Specification() *openapi3.T
}

type SQLLite struct {
	CurrentVersion int
	Log            *logrus.Logger
	RawOpenAPISpec *OpenAPISchemas
	LastUpdate     time.Time
	OpenAPISpec    *openapi3.T
}

func getSpecBytes(spec string) []byte {
	return bytes.NewBufferString(spec).Bytes()
}

func NewOpenAPIDB(log *logrus.Logger, dbStoragePath string) (DBOpenAPILoader, error) {

	sqlObj := SQLLite{
		Log: log,
	}

	if err := sqlObj.Load(dbStoragePath); err != nil {
		return nil, err
	}

	return &sqlObj, nil
}

func (s *SQLLite) Load(dbStoragePath string) error {

	s.CurrentVersion = currentSqlVersion

	currentDBPath := dbStoragePath
	if currentDBPath == "" {
		currentDBPath = fmt.Sprintf("/var/wallarm/%d/wallarm_api.db", currentSqlVersion)
	}
	db, err := sql.Open("sqlite3", currentDBPath)
	if err != nil {
		s.Log.Fatal(err)
		return err
	}
	defer db.Close()

	entry := OpenAPISchemas{}

	rows, err := db.Query("select schema_id,schema_version,schema_format,schema_content from openapi_schemas ORDER BY schema_id LIMIT 1")
	if err != nil {
		s.Log.Fatal(err)
		return err
	}
	defer rows.Close()

	for rows.Next() {

		err = rows.Scan(&entry.SchemaID, &entry.SchemaVersion, &entry.SchemaFormat, &entry.SchemaContent)
		if err != nil {
			log.Fatal(err)
		}
	}

	if err = rows.Err(); err != nil {
		s.Log.Fatal(err)
		return err
	}

	s.RawOpenAPISpec = &entry
	s.LastUpdate = time.Now().UTC()

	// parse specification
	swagger, err := openapi3.NewLoader().LoadFromData(getSpecBytes(s.RawOpenAPISpec.SchemaContent))
	if err != nil {
		return errors.Wrap(err, "error: parse OpenAPI specification")
	}

	s.OpenAPISpec = swagger

	return nil
}

func (s *SQLLite) Specification() *openapi3.T {
	return s.OpenAPISpec
}

func (s *SQLLite) SpecificationRaw() []byte {
	return getSpecBytes(s.RawOpenAPISpec.SchemaContent)
}

func (s *SQLLite) SchemaID() int {
	return currentSqlVersion
}

func (s *SQLLite) SpecificationVersion() string {
	return s.RawOpenAPISpec.SchemaVersion
}
