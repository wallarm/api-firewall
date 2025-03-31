package config

type GraphQLMode struct {
	APIFWInit
	APIFWServer
	Graphql  GraphQL
	TLS      TLS
	Server   ProtectedAPI
	Denylist Denylist
	AllowIP  AllowIP
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
