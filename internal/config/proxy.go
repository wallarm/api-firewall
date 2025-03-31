package config

import "time"

type ProxyMode struct {
	APIFWInit   `mapstructure:",squash"`
	APIFWServer `mapstructure:"Server"`
	ModSecurity
	TLS       TLS
	ShadowAPI ShadowAPI
	Denylist  Denylist
	Server    Backend `mapstructure:"Backend"`
	AllowIP   AllowIP
	DNS       DNS
	Endpoints EndpointList

	RequestValidation         string       `conf:"required" validate:"required,oneof=DISABLE BLOCK LOG_ONLY"`
	ResponseValidation        string       `conf:"required" validate:"required,oneof=DISABLE BLOCK LOG_ONLY"`
	CustomBlockStatusCode     int          `conf:"default:403" validate:"HttpStatusCodes"`
	AddValidationStatusHeader bool         `conf:"default:false"`
	APISpecs                  string       `conf:"required,env:API_SPECS" validate:"required"`
	APISpecsCustomHeader      CustomHeader `conf:"env:API_SPECS_CUSTOM_HEADER"`
	PassOptionsRequests       bool         `conf:"default:false,env:PASS_OPTIONS"`

	SpecificationUpdatePeriod time.Duration `conf:"default:0"`
}

type CustomHeader struct {
	Name  string
	Value string
}

type ShadowAPI struct {
	ExcludeList                []int `conf:"default:404,env:SHADOW_API_EXCLUDE_LIST" validate:"HttpStatusCodes"`
	UnknownParametersDetection bool  `conf:"default:true,env:SHADOW_API_UNKNOWN_PARAMETERS_DETECTION"`
}
