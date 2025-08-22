package storage

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/pb33f/libopenapi"
	"github.com/savsgio/gotils/strconv"

	"github.com/wallarm/api-firewall/internal/platform/loader"
)

const (
	currentFileVersion = 0
	undefinedSchemaID  = 0
)

type File struct {
	isReady     bool
	RawSpec     string
	LastUpdate  time.Time
	OpenAPISpec libopenapi.Document
	lock        *sync.RWMutex
}

var _ DBOpenAPILoader = (*File)(nil)

func NewOpenAPIFromFile(OASPath string) (DBOpenAPILoader, error) {

	fileObj := File{
		lock:    &sync.RWMutex{},
		isReady: false,
	}

	var err error
	fileObj.isReady, err = fileObj.Load(OASPath)

	return &fileObj, err
}

func getChecksum(oasFile []byte) []byte {
	h := sha256.New()
	return h.Sum(oasFile)
}

func (f *File) Load(OASPath string) (bool, error) {

	var parsingErrs error
	var isReady bool

	// check if file exists
	if _, err := os.Stat(OASPath); errors.Is(err, os.ErrNotExist) {
		return isReady, err
	}

	fSpec, err := os.Open(OASPath)
	if err != nil {
		log.Fatal(err)
	}
	defer fSpec.Close()

	rawSpec, err := io.ReadAll(fSpec)
	if err != nil {
		return isReady, err
	}

	parsedSpec, err := loader.LibOpenAPIParseOAS(rawSpec, "", undefinedSchemaID)
	if err != nil {
		parsingErrs = errors.Join(parsingErrs, err)
	}

	f.lock.Lock()
	defer f.lock.Unlock()

	f.RawSpec = strconv.B2S(rawSpec)
	f.OpenAPISpec = parsedSpec
	isReady = true

	return isReady, parsingErrs
}

func (s *File) Specification(_ int) any {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.OpenAPISpec
}

func (s *File) SpecificationRaw(_ int) any {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.RawSpec
}

func (s *File) SpecificationRawContent(_ int) []byte {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return getSpecBytes(s.RawSpec)
}

func (s *File) SpecificationVersion(_ int) string {
	return ""
}

func (s *File) IsLoaded(_ int) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.OpenAPISpec != nil
}

func (s *File) SchemaIDs() []int {
	return []int{}
}

func (s *File) IsReady() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.isReady
}

func (s *File) Version() int {
	return currentFileVersion
}

func (s *File) AfterLoad(_ string) error {
	return nil
}

func (s *File) ShouldUpdate(newStorage DBOpenAPILoader) bool {

	beforeUpdateSpecs := getChecksum(s.SpecificationRawContent(undefinedSchemaID))
	afterUpdateSpecs := getChecksum(newStorage.SpecificationRawContent(undefinedSchemaID))

	return !bytes.Equal(beforeUpdateSpecs, afterUpdateSpecs)
}
