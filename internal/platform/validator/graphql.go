package validator

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"sync"

	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/complexity"
	"github.com/wundergraph/graphql-go-tools/pkg/astparser"
	"github.com/wundergraph/graphql-go-tools/pkg/graphql"
	"github.com/wundergraph/graphql-go-tools/pkg/operationreport"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

var (
	ErrNotAllowIntrospectionQuery        = errors.New("introspection query is not allowed")
	ErrGraphQLQueryNotFound              = errors.New("GraphQL query not found in the request")
	ErrWrongGraphQLQueryTypeInGETRequest = errors.New("wrong GraphQL query type in GET request")
)

// ValidateGraphQLRequest validates the GraphQL request
func ValidateGraphQLRequest(cfg *config.GraphQL, schema *graphql.Schema, r *graphql.Request) (*graphql.ValidationResult, error) {

	// introspection request check
	if !cfg.Introspection {
		isIntrospectQuery, err := r.IsIntrospectionQuery()
		if err != nil {
			return &graphql.ValidationResult{Valid: false, Errors: nil}, err
		}

		if isIntrospectQuery {
			return &graphql.ValidationResult{Valid: false, Errors: graphql.RequestErrorsFromError(ErrNotAllowIntrospectionQuery)}, nil
		}

	}

	// validate operation name value
	if err := validateOperationName(r); err != nil {
		return &graphql.ValidationResult{Valid: false, Errors: graphql.RequestErrorsFromError(err)}, nil
	}

	// skip query complexity check if it is not configured
	if cfg.NodeCountLimit > 0 || cfg.MaxQueryDepth > 0 || cfg.MaxQueryComplexity > 0 {

		// check query complexity
		requestErrors := complexity.ValidateQuery(cfg, schema, r)
		if requestErrors.Count() > 0 {
			return &graphql.ValidationResult{Valid: false, Errors: requestErrors}, nil
		}
	}

	// normalize query
	normResult, err := r.Normalize(schema)
	if err != nil {
		return &graphql.ValidationResult{Valid: false, Errors: nil}, err
	}

	if !normResult.Successful {
		return &graphql.ValidationResult{Valid: false, Errors: normResult.Errors}, nil
	}

	// validate query
	result, err := r.ValidateForSchema(schema)
	if err != nil {
		return &graphql.ValidationResult{Valid: false, Errors: nil}, err
	}

	if !result.Valid {
		return &result, nil
	}

	return &graphql.ValidationResult{Valid: true, Errors: nil}, nil
}

// ParseGraphQLRequest function parses the GraphQL request
func ParseGraphQLRequest(ctx *fasthttp.RequestCtx, schema *graphql.Schema) (*graphql.Request, error) {

	gqlRequest := new(graphql.Request)

	query := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		query.Reset()
		bufferPool.Put(query)
	}()

	httpMethod := string(ctx.Method())

	switch httpMethod {
	case fasthttp.MethodGet:

		// build json query
		query.Write([]byte("{\"query\":"))
		// unescape the query string and encode JSON special chars
		queryArgQuery, err := url.QueryUnescape(strconv.B2S(ctx.Request.URI().QueryArgs().Peek("query")))
		if err != nil {
			return nil, err
		}
		err = json.NewEncoder(query).Encode(&queryArgQuery)
		if err != nil {
			return nil, err
		}
		query.Write([]byte(",\"operationName\":\""))
		query.Write(ctx.Request.URI().QueryArgs().Peek("operationName"))
		query.Write([]byte("\"}"))

	case fasthttp.MethodPost:
		query = bytes.NewBuffer(ctx.Request.Body())
	}

	if query.Len() > 0 {

		if err := graphql.UnmarshalRequest(io.NopCloser(query), gqlRequest); err != nil {
			return nil, err
		}

		operationType, err := gqlRequest.OperationType()
		if err != nil {
			return nil, err
		}

		if httpMethod == fasthttp.MethodGet && operationType != graphql.OperationTypeQuery {
			return nil, ErrWrongGraphQLQueryTypeInGETRequest
		}

		return gqlRequest, nil
	}

	return nil, ErrGraphQLQueryNotFound
}

func validateOperationName(gqlRequest *graphql.Request) error {

	operation, _ := astparser.ParseGraphqlDocumentString(gqlRequest.Query)
	numOfOperations := operation.NumOfOperationDefinitions()
	operationName := strings.TrimSpace(gqlRequest.OperationName)
	report := &operationreport.Report{}

	// operationName is not present in the request but the number of operations is more than 1
	if operationName == "" && numOfOperations > 1 {
		report.AddExternalError(operationreport.ErrRequiredOperationNameIsMissing())

		return report
	}

	// operationName is not found in the request
	if !operation.OperationNameExists(operationName) {
		report.AddExternalError(operationreport.ErrOperationWithProvidedOperationNameNotFound(operationName))

		return report
	}

	return nil
}
