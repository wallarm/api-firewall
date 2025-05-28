package config

import "time"

type APIMode struct {
	APIFWInit
	APIFWServer
	ModSecurity
	Metrics
	AllowIP AllowIP
	TLS     TLS

	SpecificationUpdatePeriod time.Duration `conf:"default:1m,env:API_MODE_SPECIFICATION_UPDATE_PERIOD"`
	PathToSpecDB              string        `conf:"env:API_MODE_DEBUG_PATH_DB"`
	DBVersion                 int           `conf:"default:0,env:API_MODE_DB_VERSION"`

	UnknownParametersDetection bool `conf:"default:true,env:API_MODE_UNKNOWN_PARAMETERS_DETECTION"`
	PassOptionsRequests        bool `conf:"default:false,env:PASS_OPTIONS"`

	MaxErrorsInResponse int `conf:"default:0,env:API_MODE_MAX_ERRORS_IN_RESPONSE"`
}
