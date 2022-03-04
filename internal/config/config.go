package config

import (
	"time"

	"github.com/ardanlabs/conf"
)

type TLS struct {
	CertsPath string `conf:"default:certs"`
	CertFile  string `conf:"default:localhost.crt"`
	CertKey   string `conf:"default:localhost.key"`
}

type Server struct {
	URL                string        `conf:"default:http://localhost:3000/v1/" validate:"required,url"`
	ClientPoolCapacity int           `conf:"default:1000" validate:"gt=0"`
	InsecureConnection bool          `conf:"default:false"`
	RootCA             string        `conf:""`
	MaxConnsPerHost    int           `conf:"default:512"`
	ReadTimeout        time.Duration `conf:"default:5s"`
	WriteTimeout       time.Duration `conf:"default:5s"`
	DialTimeout        time.Duration `conf:"default:200ms"`
	Oauth              Oauth
}

type JWT struct {
	SignatureAlgorithm string `conf:"default:RS256"`
	PubCertFile        string `conf:""`
	SecretKey          string `conf:""`
}

type Introspection struct {
	ClientAuthBearerToken string        `conf:""`
	Endpoint              string        `conf:""`
	EndpointParams        string        `conf:""`
	TokenParamName        string        `conf:""`
	ContentType           string        `conf:""`
	EndpointMethod        string        `conf:"default:GET"`
	RefreshInterval       time.Duration `conf:"default:10m"`
}

type Oauth struct {
	ValidationType string `conf:"default:JWT"`
	JWT            JWT
	Introspection  Introspection
}

type ShadowAPI struct {
	ExcludeList []int `conf:"default:404,env:SHADOW_API_EXCLUDE_LIST" validate:"HttpStatusCodes"`
}

type APIFWConfiguration struct {
	conf.Version
	TLS    TLS
	Server Server

	APIHost                   string        `conf:"default:http://0.0.0.0:8282,env:URL" validate:"required,url"`
	HealthAPIHost             string        `conf:"default:0.0.0.0:9667,env:HEALTH_HOST" validate:"required"`
	ReadTimeout               time.Duration `conf:"default:5s"`
	WriteTimeout              time.Duration `conf:"default:5s"`
	LogLevel                  string        `conf:"default:DEBUG" validate:"required,oneof=DEBUG INFO ERROR WARNING"`
	LogFormat                 string        `conf:"default:TEXT" validate:"required,oneof=TEXT JSON"`
	RequestValidation         string        `conf:"required" validate:"required,oneof=DISABLE BLOCK LOG_ONLY"`
	ResponseValidation        string        `conf:"required" validate:"required,oneof=DISABLE BLOCK LOG_ONLY"`
	CustomBlockStatusCode     int           `conf:"default:403" validate:"HttpStatusCodes"`
	AddValidationStatusHeader bool          `conf:"default:false"`
	APISpecs                  string        `conf:"default:swagger.json,env:API_SPECS"`
	ShadowAPI                 ShadowAPI
}
