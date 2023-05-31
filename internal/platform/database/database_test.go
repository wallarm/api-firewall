package database

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
)

const (
	testSchemaID    = 1
	testSpecVersion = "1"
)

const testOpenAPIScheme = `openapi: 3.0.1
info:
  title: Minimal integer field example
  version: 0.0.1
paths:
  /ok:
    get:
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                required:
                  - status
                properties:
                  status:
                    type: string
                    example: "success"
                  error:
                    type: string
  /wrong:
    get:
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                required:
                  - status
                properties:
                  status:
                    type: string
                    example: "example"
                  error:
                    type: string`

func TestDatabaseLoad(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	dbSpec, err := NewOpenAPIDB(logger, "../../../resources/test/database/wallarm_api.db")
	if err != nil {
		t.Fatal(err)
	}

	openAPISpec := bytes.Trim(dbSpec.SpecificationRaw(), "\xef\xbb\xbf")
	if !bytes.Equal(openAPISpec, bytes.NewBufferString(testOpenAPIScheme).Bytes()) {
		t.Error("loaded and the original specifications are not equal")
	}

	if testSchemaID != dbSpec.SchemaID() {
		t.Error("loaded and the original schema IDs are not equal")
	}

	if testSpecVersion != dbSpec.SpecificationVersion() {
		t.Error("loaded and the original specifications versions are not equal")
	}

}
