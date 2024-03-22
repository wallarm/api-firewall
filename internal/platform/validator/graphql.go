package validator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"

	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fastjson"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/complexity"
	"github.com/wundergraph/graphql-go-tools/pkg/ast"
	"github.com/wundergraph/graphql-go-tools/pkg/astparser"
	"github.com/wundergraph/graphql-go-tools/pkg/graphql"
	"github.com/wundergraph/graphql-go-tools/pkg/operationreport"
)

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

var (
	ErrBatchQueryLimitExceeded           = errors.New("batch query limit exceeded")
	ErrNotAllowIntrospectionQuery        = errors.New("introspection queries are not allowed")
	ErrGraphQLQueryNotFound              = errors.New("GraphQL query not found in the request")
	ErrWrongGraphQLQueryTypeInGETRequest = errors.New("wrong GraphQL query type in GET request")
	ErrFieldDuplicationFound             = errors.New("duplicate fields were found in the GraphQL document")
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

	// parse the GraphQL document
	document, _ := astparser.ParseGraphqlDocumentString(r.Query)

	// validate that there are no duplication fields
	if err := validateOperationName(&document, r); err != nil {
		return &graphql.ValidationResult{Valid: false, Errors: graphql.RequestErrorsFromError(err)}, nil
	}

	if cfg.MaxAliasesNum > 0 {
		// validate max aliases in the GraphQL document
		if err := validateAliasesNum(&document, cfg.MaxAliasesNum); err != nil {
			return &graphql.ValidationResult{Valid: false, Errors: graphql.RequestErrorsFromError(err)}, nil
		}
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

	if cfg.DisableFieldDuplication {
		// validate operation name value
		if err := validateDuplicateFields(&document); err != nil {
			return &graphql.ValidationResult{Valid: false, Errors: graphql.RequestErrorsFromError(err)}, nil
		}
	}

	return &graphql.ValidationResult{Valid: true, Errors: nil}, nil
}

// UnmarshalGraphQLRequest function parse the JSON document and build graphql.Request
func UnmarshalGraphQLRequest(reader io.Reader, jsonParserPool *fastjson.ParserPool) ([]graphql.Request, error) {

	var queries []graphql.Request

	// Get fastjson parser
	jsonParser := jsonParserPool.Get()
	defer jsonParserPool.Put(jsonParser)

	requestBytes, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	if len(requestBytes) == 0 {
		return nil, graphql.ErrEmptyRequest
	}

	parsedRequest, err := jsonParser.ParseBytes(requestBytes)
	if err != nil {
		return nil, err
	}

	switch parsedRequest.Type() {
	case fastjson.TypeObject:
		var gqlReq graphql.Request
		var varValue, queryValue, opNameValue []byte

		if variables := parsedRequest.Get("variables"); variables != nil {
			gqlReq.Variables = variables.MarshalTo(varValue)
		}

		if query := parsedRequest.Get("query"); query != nil {
			if queryValue, err = query.StringBytes(); err != nil {
				return nil, err
			}
			gqlReq.Query = strconv.B2S(queryValue)
		}

		if opName := parsedRequest.Get("operationName"); opName != nil {
			if opNameValue, err = opName.StringBytes(); err != nil {
				return nil, err
			}
			gqlReq.OperationName = strconv.B2S(opNameValue)
		}

		queries = append(queries, gqlReq)
	case fastjson.TypeArray:
		batchQueries, err := parsedRequest.Array()
		if err != nil {
			return nil, err
		}
		for _, query := range batchQueries {
			var gqlReq graphql.Request
			var varValue, queryValue, opNameValue []byte

			if variables := query.Get("variables"); variables != nil {
				gqlReq.Variables = variables.MarshalTo(varValue)
			}

			if query := query.Get("query"); query != nil {
				if queryValue, err = query.StringBytes(); err != nil {
					return nil, err
				}
				gqlReq.Query = strconv.B2S(queryValue)
			}

			if opName := query.Get("operationName"); opName != nil {
				if opNameValue, err = opName.StringBytes(); err != nil {
					return nil, err
				}
				gqlReq.OperationName = strconv.B2S(opNameValue)
			}
			queries = append(queries, gqlReq)
		}
	default:
		return nil, errors.New("JSON parsing error: invalid query")
	}

	return queries, nil
}

// ParseGraphQLRequest function parses the GraphQL request
func ParseGraphQLRequest(ctx *fasthttp.RequestCtx, jsonParserPool *fastjson.ParserPool) ([]graphql.Request, error) {

	//var gqlRequest graphql.Request

	query := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		query.Reset()
		bufferPool.Put(query)
	}()

	httpMethod := string(ctx.Method())

	switch httpMethod {
	case fasthttp.MethodGet:

		// build json query
		query.WriteString("{\"query\":")
		// unescape the query string and encode JSON special chars
		queryArgQuery, err := url.QueryUnescape(strconv.B2S(ctx.Request.URI().QueryArgs().Peek("query")))
		if err != nil {
			return nil, err
		}
		err = json.NewEncoder(query).Encode(&queryArgQuery)
		if err != nil {
			return nil, err
		}
		query.WriteString(",\"operationName\":\"")
		query.Write(ctx.Request.URI().QueryArgs().Peek("operationName"))
		query.WriteString("\"}")

	case fasthttp.MethodPost:
		query = bytes.NewBuffer(ctx.Request.Body())
	}

	if query.Len() > 0 {

		gqlRequest, err := UnmarshalGraphQLRequest(io.NopCloser(query), jsonParserPool)
		if err != nil {
			return nil, err
		}

		for _, req := range gqlRequest {
			operationType, err := req.OperationType()
			if err != nil {
				return nil, err
			}

			if httpMethod == fasthttp.MethodGet && operationType != graphql.OperationTypeQuery {
				return nil, ErrWrongGraphQLQueryTypeInGETRequest
			}

		}

		return gqlRequest, nil
	}

	return nil, ErrGraphQLQueryNotFound
}

// validateAliasesNum validates that the total amount of aliases in the GraphQL document does not exceed the configured max value
func validateAliasesNum(document *ast.Document, MaxAliasesNum int) error {

	numOfAliases := getNumOfAliases(document)
	if numOfAliases > MaxAliasesNum {
		return fmt.Errorf("the maximum number of aliases in the GraphQL document has been exceeded. The maximum number of aliases value is %d. The current number of aliases is %d", MaxAliasesNum, numOfAliases)
	}

	return nil
}

// getNumOfAliases returns amount of aliases in the GraphQL documents
func getNumOfAliases(document *ast.Document) int {
	numOfAliases := 0

	for _, f := range document.Fields {
		if f.Alias.IsDefined {
			numOfAliases += 1
		}
	}

	return numOfAliases
}

// validateDuplicateFields checks that there are now duplicates fields in the document
func validateDuplicateFields(document *ast.Document) error {

	for _, ss := range document.SelectionSets {
		fieldsRepeatMap := make(map[string]int)
		for _, cs := range ss.SelectionRefs {
			field := document.Fields[document.Selections[cs].Ref]
			fieldName := document.FieldNameString(document.Selections[cs].Ref)

			// skip objects
			if field.HasSelections {
				continue
			}

			currentNum, ok := fieldsRepeatMap[fieldName]
			if !ok {
				fieldsRepeatMap[fieldName] = 1
				continue
			}

			currentNum += 1
			fieldsRepeatMap[fieldName] = currentNum
		}

		for f := range fieldsRepeatMap {
			if fieldsRepeatMap[f] > 1 {
				return ErrFieldDuplicationFound
			}
		}
	}

	return nil
}

func validateOperationName(operation *ast.Document, gqlRequest *graphql.Request) error {

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
