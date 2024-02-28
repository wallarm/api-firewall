package config

import (
	"time"

	"github.com/ardanlabs/conf"
)

type APIFWMode struct {
	Mode string `conf:"default:PROXY" validate:"oneof=PROXY API GRAPHQL"`
}

type ProxyMode struct {
	conf.Version
	APIFWMode
	ModSecurity
	TLS       TLS
	ShadowAPI ShadowAPI
	Denylist  Denylist
	Server    Server
	AllowIP   AllowIP

	APIHost       string        `conf:"default:http://0.0.0.0:8282,env:URL" validate:"required,url"`
	HealthAPIHost string        `conf:"default:0.0.0.0:9667,env:HEALTH_HOST" validate:"required"`
	ReadTimeout   time.Duration `conf:"default:5s"`
	WriteTimeout  time.Duration `conf:"default:5s"`
	LogLevel      string        `conf:"default:INFO" validate:"oneof=TRACE DEBUG INFO ERROR WARNING"`
	LogFormat     string        `conf:"default:TEXT" validate:"oneof=TEXT JSON"`

	RequestValidation         string `conf:"required" validate:"required,oneof=DISABLE BLOCK LOG_ONLY"`
	ResponseValidation        string `conf:"required" validate:"required,oneof=DISABLE BLOCK LOG_ONLY"`
	CustomBlockStatusCode     int    `conf:"default:403" validate:"HttpStatusCodes"`
	AddValidationStatusHeader bool   `conf:"default:false"`
	APISpecs                  string `conf:"required,env:API_SPECS" validate:"required"`
	PassOptionsRequests       bool   `conf:"default:false,env:PASS_OPTIONS"`
}

type APIMode struct {
	conf.Version
	APIFWMode
	ModSecurity
	TLS TLS

	SpecificationUpdatePeriod  time.Duration `conf:"default:1m,env:API_MODE_SPECIFICATION_UPDATE_PERIOD"`
	PathToSpecDB               string        `conf:"env:API_MODE_DEBUG_PATH_DB"`
	UnknownParametersDetection bool          `conf:"default:true,env:API_MODE_UNKNOWN_PARAMETERS_DETECTION"`

	APIHost             string        `conf:"default:http://0.0.0.0:8282,env:URL" validate:"required,url"`
	HealthAPIHost       string        `conf:"default:0.0.0.0:9667,env:HEALTH_HOST" validate:"required"`
	ReadTimeout         time.Duration `conf:"default:5s"`
	WriteTimeout        time.Duration `conf:"default:5s"`
	LogLevel            string        `conf:"default:INFO" validate:"oneof=TRACE DEBUG INFO ERROR WARNING"`
	LogFormat           string        `conf:"default:TEXT" validate:"oneof=TEXT JSON"`
	PassOptionsRequests bool          `conf:"default:false,env:PASS_OPTIONS"`
}

type GraphQLMode struct {
	conf.Version
	APIFWMode
	Graphql  GraphQL
	TLS      TLS
	Server   Backend
	Denylist Denylist
	AllowIP  AllowIP

	APIHost       string        `conf:"default:http://0.0.0.0:8282,env:URL" validate:"required,url"`
	HealthAPIHost string        `conf:"default:0.0.0.0:9667,env:HEALTH_HOST" validate:"required"`
	ReadTimeout   time.Duration `conf:"default:5s"`
	WriteTimeout  time.Duration `conf:"default:5s"`
	LogLevel      string        `conf:"default:INFO" validate:"oneof=TRACE DEBUG INFO ERROR WARNING"`
	LogFormat     string        `conf:"default:TEXT" validate:"oneof=TEXT JSON"`
}

type TLS struct {
	CertsPath string `conf:"default:certs"`
	CertFile  string `conf:"default:localhost.crt"`
	CertKey   string `conf:"default:localhost.key"`
}

type Server struct {
	Backend
	Oauth Oauth
}

type Backend struct {
	URL                  string        `conf:"default:http://localhost:3000/v1/" validate:"required,url"`
	ClientPoolCapacity   int           `conf:"default:1000" validate:"gt=0"`
	InsecureConnection   bool          `conf:"default:false"`
	RootCA               string        `conf:""`
	MaxConnsPerHost      int           `conf:"default:512"`
	ReadTimeout          time.Duration `conf:"default:5s"`
	WriteTimeout         time.Duration `conf:"default:5s"`
	DialTimeout          time.Duration `conf:"default:200ms"`
	DeleteAcceptEncoding bool          `conf:"default:false"`
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
	Enabled  bool   `conf:"default:true"`
	ConfFile string `conf:"default:./coreruleset/crs-setup.conf,env:MODSEC_CONF_FILE" validate:"file"`
	RulesDir string `conf:"default:./coreruleset/rules,env:MODSEC_RULES_DIR" validate:"dir"`
}

type GraphQL struct {
	MaxQueryComplexity int      `conf:"required" validate:"required"`
	MaxQueryDepth      int      `conf:"required" validate:"required"`
	MaxAliasesNum      int      `conf:"required" validate:"required"`
	NodeCountLimit     int      `conf:"required" validate:"required"`
	FieldDuplication   bool     `conf:"default:false"`
	Playground         bool     `conf:"default:false"`
	PlaygroundPath     string   `conf:"default:/" validate:"path"`
	Introspection      bool     `conf:"required" validate:"required"`
	Schema             string   `conf:"required" validate:"required"`
	WSCheckOrigin      bool     `conf:"default:false"`
	WSOrigin           []string `conf:"" validate:"url"`

	RequestValidation string `conf:"required" validate:"required,oneof=DISABLE BLOCK LOG_ONLY"`
}
