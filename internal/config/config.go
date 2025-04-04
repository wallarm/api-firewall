package config

import (
	"github.com/ardanlabs/conf"
)

type APIFWInit struct {
	conf.Version
	Mode string `conf:"default:PROXY" validate:"oneof=PROXY API GRAPHQL" mapstructure:"mode"`

	LogLevel  string `conf:"default:INFO" validate:"oneof=TRACE DEBUG INFO ERROR WARNING"`
	LogFormat string `conf:"default:TEXT" validate:"oneof=TEXT JSON"`
}

type TLS struct {
	CertsPath string `conf:"default:certs"`
	CertFile  string `conf:"default:localhost.crt"`
	CertKey   string `conf:"default:localhost.key"`
}

type Token struct {
	CookieName       string `conf:""`
	HeaderName       string `conf:""`
	TrimBearerPrefix bool   `conf:"default:true"`
	File             string `conf:""`
}

type AllowIP struct {
	File       string `conf:""`
	HeaderName string `conf:""`
}
type Denylist struct {
	Tokens Token
}

type AllowIPlist struct {
	AllowedIP AllowIP
}
