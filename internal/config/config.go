package config

import (
	"time"

	"github.com/ardanlabs/conf"
)

type APIFWMode struct {
	Mode string `conf:"default:PROXY" validate:"oneof=PROXY API GRAPHQL"`
}

type APIFWServer struct {
	APIHost            string        `conf:"default:http://0.0.0.0:8282,env:URL" validate:"required,url"`
	HealthAPIHost      string        `conf:"default:0.0.0.0:9667,env:HEALTH_HOST" validate:"required"`
	ReadTimeout        time.Duration `conf:"default:5s"`
	WriteTimeout       time.Duration `conf:"default:5s"`
	ReadBufferSize     int           `conf:"default:8192"`
	WriteBufferSize    int           `conf:"default:8192"`
	MaxRequestBodySize int           `conf:"default:4194304"`
	DisableKeepalive   bool          `conf:"default:false"`
	MaxConnsPerIP      int           `conf:"default:0"`
	MaxRequestsPerConn int           `conf:"default:0"`
}

type CustomHeader struct {
	Name  string
	Value string
}

type Nameserver struct {
	Host  string `conf:""`
	Port  string `conf:"default:53"`
	Proto string `conf:"default:udp"`
}

type ProxyMode struct {
	conf.Version
	APIFWMode
	APIFWServer
	ModSecurity
	TLS       TLS
	ShadowAPI ShadowAPI
	Denylist  Denylist
	Server    Backend
	AllowIP   AllowIP
	DNS       DNS

	LogLevel  string `conf:"default:INFO" validate:"oneof=TRACE DEBUG INFO ERROR WARNING"`
	LogFormat string `conf:"default:TEXT" validate:"oneof=TEXT JSON"`

	RequestValidation         string       `conf:"required" validate:"required,oneof=DISABLE BLOCK LOG_ONLY"`
	ResponseValidation        string       `conf:"required" validate:"required,oneof=DISABLE BLOCK LOG_ONLY"`
	CustomBlockStatusCode     int          `conf:"default:403" validate:"HttpStatusCodes"`
	AddValidationStatusHeader bool         `conf:"default:false"`
	APISpecs                  string       `conf:"required,env:API_SPECS" validate:"required"`
	APISpecsCustomHeader      CustomHeader `conf:"env:API_SPECS_CUSTOM_HEADER"`
	PassOptionsRequests       bool         `conf:"default:false,env:PASS_OPTIONS"`

	SpecificationUpdatePeriod time.Duration `conf:"default:0"`
}

type APIMode struct {
	conf.Version
	APIFWMode
	APIFWServer
	ModSecurity
	AllowIP AllowIP
	TLS     TLS

	SpecificationUpdatePeriod  time.Duration `conf:"default:1m,env:API_MODE_SPECIFICATION_UPDATE_PERIOD"`
	PathToSpecDB               string        `conf:"env:API_MODE_DEBUG_PATH_DB"`
	DBVersion                  int           `conf:"default:0,env:API_MODE_DB_VERSION"`
	UnknownParametersDetection bool          `conf:"default:true,env:API_MODE_UNKNOWN_PARAMETERS_DETECTION"`

	LogLevel            string `conf:"default:INFO" validate:"oneof=TRACE DEBUG INFO ERROR WARNING"`
	LogFormat           string `conf:"default:TEXT" validate:"oneof=TEXT JSON"`
	PassOptionsRequests bool   `conf:"default:false,env:PASS_OPTIONS"`
}

type GraphQLMode struct {
	conf.Version
	APIFWMode
	APIFWServer
	Graphql  GraphQL
	TLS      TLS
	Server   ProtectedAPI
	Denylist Denylist
	AllowIP  AllowIP

	LogLevel  string `conf:"default:INFO" validate:"oneof=TRACE DEBUG INFO ERROR WARNING"`
	LogFormat string `conf:"default:TEXT" validate:"oneof=TEXT JSON"`
}

type TLS struct {
	CertsPath string `conf:"default:certs"`
	CertFile  string `conf:"default:localhost.crt"`
	CertKey   string `conf:"default:localhost.key"`
}

type Backend struct {
	ProtectedAPI
	Oauth Oauth
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
	DNSLoadBalancing     bool          `conf:"default:false"`
}

type DNS struct {
	Nameserver    Nameserver
	Cache         bool          `conf:"default:false"`
	FetchTimeout  time.Duration `conf:"default:1m"`
	LookupTimeout time.Duration `conf:"default:1s"`
}

type JWT struct {
	SignatureAlgorithm string `conf:"default:RS256"`
	PubCertFile        string `conf:""`
	SecretKey          string `conf:""`
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
	ExcludeList                []int `conf:"default:404,env:SHADOW_API_EXCLUDE_LIST" validate:"HttpStatusCodes"`
	UnknownParametersDetection bool  `conf:"default:true,env:SHADOW_API_UNKNOWN_PARAMETERS_DETECTION"`
}

type ModSecurity struct {
	ConfFiles []string `conf:"env:MODSEC_CONF_FILES"`
	RulesDir  string   `conf:"env:MODSEC_RULES_DIR"`
}

type GraphQL struct {
	MaxQueryComplexity      int      `conf:"required" validate:"required"`
	MaxQueryDepth           int      `conf:"required" validate:"required"`
	MaxAliasesNum           int      `conf:"required" validate:"required"`
	NodeCountLimit          int      `conf:"required" validate:"required"`
	BatchQueryLimit         int      `conf:"required" validate:"required"`
	DisableFieldDuplication bool     `conf:"default:false"`
	Playground              bool     `conf:"default:false"`
	PlaygroundPath          string   `conf:"default:/" validate:"path"`
	Introspection           bool     `conf:"required" validate:"required"`
	Schema                  string   `conf:"required" validate:"required"`
	WSCheckOrigin           bool     `conf:"default:false"`
	WSOrigin                []string `conf:"" validate:"url"`

	RequestValidation string `conf:"required" validate:"required,oneof=DISABLE BLOCK LOG_ONLY"`
}
