package database

import (
	"bytes"
	"sort"
	"testing"

	"github.com/sirupsen/logrus"
)

const (
	testSchemaID1    = 1
	testSpecVersion1 = "1"
	testSchemaID2    = 4
	testSpecVersion2 = "2"

	expectedSchemaNum   = 3
	expectedSchemaNumV2 = 1

	testDBVersion1 = 1
	testDBVersion2 = 2
)

const (
	dbVersion           = 1
	dbVersion2          = 2
	testOpenAPISchemeV2 = `openapi: 3.0.1
info:
  title: Service
  version: 1.1.1
servers:
  - url: /
paths:
  /:
    get:
      tags:
        - Redirects
      summary: Absolutely 302 Redirects n times.
      responses:
        '200':
          description: A redirection.
          content: {}`
	testOpenAPIScheme1 = `openapi: 3.0.1
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
                    type: string`
	testOpenAPIScheme2 = `{
  "openapi": "3.0.1",
  "info": {
    "title": "Minimal integer field example",
    "version": "0.0.1"
  },
  "paths": {
    "/wrong": {
      "get": {
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "required": [
                    "status"
                  ],
                  "properties": {
                    "status": {
                      "type": "string",
                      "example": "example"
                    },
                    "error": {
                      "type": "string"
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}`
)

func TestBasicDBSpecsLoading(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	dbSpec, err := NewOpenAPIDB("../../../resources/test/database/wallarm_api.db", dbVersion)
	if err != nil {
		t.Fatal(err)
	}

	// test first OpenAPI spec
	openAPISpec := bytes.Trim(dbSpec.SpecificationRawContent(testSchemaID1), "\xef\xbb\xbf")
	if !bytes.Equal(openAPISpec, bytes.NewBufferString(testOpenAPIScheme1).Bytes()) {
		t.Error("loaded and the original specifications are not equal")
	}

	loadedSchemaIDs := dbSpec.SchemaIDs()
	sort.Ints(loadedSchemaIDs)

	if len(loadedSchemaIDs) != expectedSchemaNum || loadedSchemaIDs[0] != testSchemaID1 {
		t.Error("loaded and the original schema IDs are not equal")
	}

	if testSpecVersion1 != dbSpec.SpecificationVersion(testSchemaID1) {
		t.Error("loaded and the original specifications versions are not equal")
	}

	// test second OpenAPI spec
	openAPISpec = bytes.Trim(dbSpec.SpecificationRawContent(testSchemaID2), "\xef\xbb\xbf")
	if !bytes.Equal(openAPISpec, bytes.NewBufferString(testOpenAPIScheme2).Bytes()) {
		t.Error("loaded and the original specifications are not equal")
	}

	if len(loadedSchemaIDs) != expectedSchemaNum || loadedSchemaIDs[2] != testSchemaID2 {
		t.Error("loaded and the original schema IDs are not equal")
	}

	if testSpecVersion2 != dbSpec.SpecificationVersion(testSchemaID2) {
		t.Error("loaded and the original specifications versions are not equal")
	}

	if !dbSpec.IsReady() {
		t.Error("loaded db is not ready")
	}

	if testDBVersion1 != dbSpec.Version() {
		t.Errorf("the DB versions are not equal. Expected %d, got %d", testDBVersion1, dbSpec.Version())
	}
}

func TestBasicDBSpecsLoadingV2(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	dbSpec, err := NewOpenAPIDB("../../../resources/test/database/wallarm_api_v2.db", dbVersion2)
	if err != nil {
		t.Fatal(err)
	}

	// test first OpenAPI spec
	openAPISpec := bytes.Trim(dbSpec.SpecificationRawContent(testSchemaID1), "\xef\xbb\xbf")
	if !bytes.Equal(openAPISpec, bytes.NewBufferString(testOpenAPISchemeV2).Bytes()) {
		t.Error("loaded and the original specifications are not equal")
	}

	loadedSchemaIDs := dbSpec.SchemaIDs()
	sort.Ints(loadedSchemaIDs)

	if len(loadedSchemaIDs) != expectedSchemaNumV2 || loadedSchemaIDs[0] != testSchemaID1 {
		t.Error("loaded and the original schema IDs are not equal")
	}

	if testSpecVersion1 != dbSpec.SpecificationVersion(testSchemaID1) {
		t.Error("loaded and the original specifications versions are not equal")
	}

	if !dbSpec.IsReady() {
		t.Error("loaded db is not ready")
	}

	if testDBVersion2 != dbSpec.Version() {
		t.Errorf("the DB versions are not equal. Expected %d, got %d", testDBVersion2, dbSpec.Version())
	}
}
