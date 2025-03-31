package config

import "time"

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

type ProtectedAPI struct {
	URL                  string        `conf:"default:http://localhost:3000/v1/" validate:"required,url"`
	RequestHostHeader    string        `conf:""`
	ClientPoolCapacity   int           `conf:"default:1000" validate:"gt=0"`
	InsecureConnection   bool          `conf:"default:false"`
	RootCA               string        `conf:""`
	MaxConnsPerHost      int           `conf:"default:512"`
	ReadTimeout          time.Duration `conf:"default:5s"`
	WriteTimeout         time.Duration `conf:"default:5s"`
	DialTimeout          time.Duration `conf:"default:200ms"`
	ReadBufferSize       int           `conf:"default:8192"`
	WriteBufferSize      int           `conf:"default:8192"`
	MaxResponseBodySize  int           `conf:"default:0"`
	DeleteAcceptEncoding bool          `conf:"default:false"`
}

type Backend struct {
	ProtectedAPI
	Oauth Oauth
}
