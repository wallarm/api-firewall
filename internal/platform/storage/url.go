package storage

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"

	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/loader"
)

type URL struct {
	isReady      bool
	url          string
	customHeader *config.CustomHeader
	RawSpec      string
	LastUpdate   time.Time
	OpenAPISpec  *openapi3.T
	lock         *sync.RWMutex
	client       *fasthttp.Client
}

const (
	currentURLVersion = 0
	readTimeout       = 10 * time.Second
	writeTimeout      = 5 * time.Second

	userAgent = "Wallarm/API-Firewall"
)

var _ DBOpenAPILoader = (*URL)(nil)

func NewOpenAPIFromURL(url string, customHeader *config.CustomHeader) (DBOpenAPILoader, error) {

	var err error

	tlsConfig := &tls.Config{}

	client := fasthttp.Client{
		TLSConfig:    tlsConfig,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	urlObj := URL{
		lock:         &sync.RWMutex{},
		isReady:      false,
		url:          url,
		customHeader: customHeader,
		client:       &client,
	}

	urlObj.isReady, err = urlObj.Load(url)

	return &urlObj, err
}

func (u *URL) Load(url string) (bool, error) {

	var parsingErrs error
	var isReady bool

	req := fasthttp.AcquireRequest()
	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodGet)
	req.Header.SetUserAgent(userAgent)

	// add custom header to request
	customHeaderName := strings.TrimSpace(u.customHeader.Name)
	customHeaderValue := strings.TrimSpace(u.customHeader.Value)
	if customHeaderName != "" && customHeaderValue != "" {
		req.Header.Set(customHeaderName, customHeaderValue)
	}

	resp := fasthttp.AcquireResponse()

	if err := u.client.Do(req, resp); err != nil {
		log.Fatal(err)
	}

	if resp.StatusCode() != fasthttp.StatusOK {
		log.Fatal(fmt.Errorf("OAS loading via URL: unexpected status code: %d", resp.StatusCode()))
	}

	rawSpec := resp.Body()

	parsedSpec, err := loader.ParseOAS(rawSpec, "", undefinedSchemaID)
	if err != nil {
		parsingErrs = errors.Join(parsingErrs, err)
	}

	u.lock.Lock()
	defer u.lock.Unlock()

	u.RawSpec = strconv.B2S(rawSpec)
	u.OpenAPISpec = parsedSpec
	isReady = true

	return isReady, parsingErrs
}

func (u *URL) Specification(_ int) *openapi3.T {
	u.lock.RLock()
	defer u.lock.RUnlock()

	return u.OpenAPISpec
}

func (u *URL) SpecificationRaw(_ int) any {
	u.lock.RLock()
	defer u.lock.RUnlock()

	return u.RawSpec
}

func (u *URL) SpecificationRawContent(_ int) []byte {
	u.lock.RLock()
	defer u.lock.RUnlock()

	return getSpecBytes(u.RawSpec)
}

func (u *URL) SpecificationVersion(_ int) string {
	return ""
}

func (u *URL) IsLoaded(_ int) bool {
	u.lock.RLock()
	defer u.lock.RUnlock()

	return u.OpenAPISpec != nil
}

func (u *URL) SchemaIDs() []int {
	return []int{}
}

func (u *URL) IsReady() bool {
	u.lock.RLock()
	defer u.lock.RUnlock()

	return u.isReady
}

func (u *URL) Version() int {
	return currentURLVersion
}

func (u *URL) AfterLoad(_ string) error {
	return nil
}

func (u *URL) ShouldUpdate(newStorage DBOpenAPILoader) bool {

	beforeUpdateSpecs := getChecksum(u.SpecificationRawContent(undefinedSchemaID))
	afterUpdateSpecs := getChecksum(newStorage.SpecificationRawContent(undefinedSchemaID))

	return !bytes.Equal(beforeUpdateSpecs, afterUpdateSpecs)
}
