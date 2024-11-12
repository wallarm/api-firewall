package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"

	graphqlHandler "github.com/wallarm/api-firewall/cmd/api-firewall/internal/handlers/graphql"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/denylist"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
	"github.com/wallarm/api-firewall/internal/platform/validator"
	"github.com/wundergraph/graphql-go-tools/pkg/graphql"
)

type ServiceGraphQLTests struct {
	serverUrl       *url.URL
	shutdown        chan os.Signal
	logger          *logrus.Logger
	loggerHook      *test.Hook
	proxy           *proxy.MockPool
	client          *proxy.MockHTTPClient
	backendWSClient *proxy.MockWebSocketClient
}

var (
	validationErr = "GraphQL query validation"
	unmarshalErr  = "GraphQL request unmarshal"
)

const (
	testSchema = `
type Chatroom {
    name: String!
    messages: [Message!]!
}

type Message {
    id: ID!
    text: String!
    name: String!
    createdBy: String!
    createdAt: Time!
}

type Query {
    room(name:String!): Chatroom
}

type Mutation {
    post(text: String!, username: String!, roomName: String!): Message!
}

type Subscription {
    messageAdded(roomName: String!): Message!
}

scalar Time

directive @user(username: String!) on SUBSCRIPTION

`
)

type Errors struct {
	Message string `json:"message"`
}

type Response struct {
	Errors []Errors `json:"errors"`
}

func TestGraphQLBasic(t *testing.T) {

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	testLogger, hook := test.NewNullLogger()
	testLogger.SetLevel(logrus.ErrorLevel)

	serverUrl, err := url.ParseRequestURI("http://127.0.0.1:80/query")
	if err != nil {
		t.Fatalf("parsing API Host URL: %s", err.Error())
	}

	pool := proxy.NewMockPool(mockCtrl)
	client := proxy.NewMockHTTPClient(mockCtrl)
	backendWSClient := proxy.NewMockWebSocketClient(mockCtrl)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	apifwTests := ServiceGraphQLTests{
		serverUrl:       serverUrl,
		shutdown:        shutdown,
		logger:          testLogger,
		loggerHook:      hook,
		proxy:           pool,
		client:          client,
		backendWSClient: backendWSClient,
	}

	// basic run test
	t.Run("basicRunGraphQLService", apifwTests.testGQLRunBasic)

	// basic test
	t.Run("basicGraphQLQuerySuccess", apifwTests.testGQLSuccess)
	t.Run("basicGraphQLEndpointNotExists", apifwTests.testGQLEndpointNotExists)

	t.Run("basicGraphQLGETQuerySuccess", apifwTests.testGQLGETSuccess)
	t.Run("basicGraphQLGETQueryMutationFailed", apifwTests.testGQLGETMutationFailed)
	t.Run("basicGraphQLQueryValidationFailed", apifwTests.testGQLValidationFailed)
	t.Run("basicGraphQLInvalidQuerySyntax", apifwTests.testGQLInvalidQuerySyntax)
	t.Run("basicGraphQLQueryInvalidMaxComplexity", apifwTests.testGQLInvalidMaxComplexity)
	t.Run("basicGraphQLQueryInvalidMaxDepth", apifwTests.testGQLInvalidMaxDepth)
	t.Run("basicGraphQLQueryInvalidNodeLimit", apifwTests.testGQLInvalidNodeLimit)
	t.Run("basicGraphQLBatchQueryLimit", apifwTests.testGQLBatchQueryLimit)

	t.Run("basicGraphQLQueryDenylistBlock", apifwTests.testGQLDenylistBlock)

	// the sequence of messages in the tests: hello -> invalid gql message (<- response from APIFW) -> valid gql message -> complete -> stop
	t.Run("basicGraphQLQuerySubscription", apifwTests.testGQLSubscription)
	t.Run("basicGraphQLQuerySubscriptionLogOnly", apifwTests.testGQLSubscriptionLogOnly)

	t.Run("basicGraphQLMaxAliasesNum", apifwTests.testGQLMaxAliasesNum)
	t.Run("basicGraphQLDuplicateFields", apifwTests.testGQLDuplicateFields)
}

func (s *ServiceGraphQLTests) testGQLRunBasic(t *testing.T) {

	t.Setenv("APIFW_MODE", "graphql")
	t.Setenv("APIFW_GRAPHQL_REQUEST_VALIDATION", "BLOCK")

	t.Setenv("APIFW_GRAPHQL_INTROSPECTION", "false")
	t.Setenv("APIFW_GRAPHQL_MAX_QUERY_DEPTH", "0")
	t.Setenv("APIFW_GRAPHQL_MAX_ALIASES_NUM", "0")
	t.Setenv("APIFW_GRAPHQL_BATCH_QUERY_LIMIT", "0")
	t.Setenv("APIFW_GRAPHQL_NODE_COUNT_LIMIT", "0")
	t.Setenv("APIFW_GRAPHQL_MAX_QUERY_COMPLEXITY", "0")
	t.Setenv("APIFW_GRAPHQL_PLAYGROUND", "false")

	t.Setenv("APIFW_URL", "http://0.0.0.0:25868")
	t.Setenv("APIFW_HEALTH_HOST", "127.0.0.1:10668")
	t.Setenv("APIFW_GRAPHQL_SCHEMA", "../../../resources/test/gql/schema.graphql")

	// start GQL handler
	go func() {
		logger := logrus.New()
		logger.SetLevel(logrus.ErrorLevel)

		if err := graphqlHandler.Run(logger); err != nil {
			t.Fatal(err)
		}
	}()

	// wait for 3 secs to init the handler
	time.Sleep(3 * time.Second)
}

func (s *ServiceGraphQLTests) testGQLSuccess(t *testing.T) {

	gqlCfg := config.GraphQL{
		MaxQueryComplexity: 0,
		MaxQueryDepth:      0,
		NodeCountLimit:     0,
		Playground:         false,
		Introspection:      false,
		Schema:             "",
		RequestValidation:  "BLOCK",
	}
	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		APIFWServer: config.APIFWServer{
			APIHost: "http://localhost:8080/query",
		},
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	handler := graphqlHandler.Handlers(&cfg, schema, s.serverUrl, s.shutdown, s.logger, s.proxy, s.backendWSClient, nil, nil)

	// Construct GraphQL request payload
	query := `
		query {
    room(name: "GeneralChat") {
        name
        messages {
            id
            text
            createdBy
            createdAt
        }
    }
}
	`
	var requestBody = map[string]interface{}{
		"query": query,
	}

	responseBody := `{
    "data": {
        "room": {
            "name": "GeneralChat",
            "messages": [
                {
                    "id": "TrsXJcKa",
                    "text": "Hello, world!",
                    "createdBy": "TestUser",
                    "createdAt": "2023-01-01T00:00:00+00:00"
                }
            ]
        }
    }
}`

	jsonValue, _ := json.Marshal(requestBody)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/query")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(jsonValue), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte(responseBody))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil).Times(1)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp).Times(1)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil).Times(1)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	recvBody := strings.TrimSpace(string(reqCtx.Response.Body()))

	if recvBody != responseBody {
		t.Errorf("Incorrect response status code. Expected: %s and got %s",
			responseBody, recvBody)
	}

}

func (s *ServiceGraphQLTests) testGQLEndpointNotExists(t *testing.T) {

	gqlCfg := config.GraphQL{
		MaxQueryComplexity: 0,
		MaxQueryDepth:      0,
		NodeCountLimit:     0,
		Playground:         false,
		Introspection:      false,
		Schema:             "",
		RequestValidation:  "BLOCK",
	}
	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		APIFWServer: config.APIFWServer{
			APIHost: "http://localhost:8080/query",
		},
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	handler := graphqlHandler.Handlers(&cfg, schema, s.serverUrl, s.shutdown, s.logger, s.proxy, s.backendWSClient, nil, nil)

	// Construct GraphQL request payload
	query := `
		query {
    room(name: "GeneralChat") {
        name
        messages {
            id
            text
            createdBy
            createdAt
        }
    }
}
	`
	var requestBody = map[string]interface{}{
		"query": query,
	}

	responseBody := `{
    "data": {
        "room": {
            "name": "GeneralChat",
            "messages": [
                {
                    "id": "TrsXJcKa",
                    "text": "Hello, world!",
                    "createdBy": "TestUser",
                    "createdAt": "2023-01-01T00:00:00+00:00"
                }
            ]
        }
    }
}`

	jsonValue, _ := json.Marshal(requestBody)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/endpointNotExists")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(jsonValue), -1)
	req.Header.SetContentType("application/json")

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte(responseBody))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 403 {
		t.Errorf("Incorrect response status code. Expected: 403 and got %d",
			reqCtx.Response.StatusCode())
	}

}

func (s *ServiceGraphQLTests) testGQLGETSuccess(t *testing.T) {

	gqlCfg := config.GraphQL{
		MaxQueryComplexity: 0,
		MaxQueryDepth:      0,
		NodeCountLimit:     0,
		Playground:         false,
		Introspection:      false,
		Schema:             "",
		RequestValidation:  "BLOCK",
	}
	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		APIFWServer: config.APIFWServer{
			APIHost: "http://localhost:8080/query",
		},
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	handler := graphqlHandler.Handlers(&cfg, schema, s.serverUrl, s.shutdown, s.logger, s.proxy, s.backendWSClient, nil, nil)

	// Construct GraphQL request payload
	query := `
		query {
    room(name: "GeneralChat") {
        name
        messages {
            id
            text
            createdBy
            createdAt
        }
    }
}
	`

	responseBody := `{
    "data": {
        "room": {
            "name": "GeneralChat",
            "messages": [
                {
                    "id": "TrsXJcKa",
                    "text": "Hello, world!",
                    "createdBy": "TestUser",
                    "createdAt": "2023-01-01T00:00:00+00:00"
                }
            ]
        }
    }
}`

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/query")
	req.Header.SetMethod("GET")
	req.URI().QueryArgs().Add("query", url.QueryEscape(query))

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte(responseBody))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil).Times(1)
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp).Times(1)
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil).Times(1)

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	recvBody := strings.TrimSpace(string(reqCtx.Response.Body()))

	if recvBody != responseBody {
		t.Errorf("Incorrect response status code. Expected: %s and got %s",
			responseBody, recvBody)
	}

}

func (s *ServiceGraphQLTests) testGQLGETMutationFailed(t *testing.T) {

	gqlCfg := config.GraphQL{
		MaxQueryComplexity: 0,
		MaxQueryDepth:      0,
		NodeCountLimit:     0,
		Playground:         false,
		Introspection:      false,
		Schema:             "",
		RequestValidation:  "BLOCK",
	}
	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		APIFWServer: config.APIFWServer{
			APIHost: "http://localhost:8080/query",
		},
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	handler := graphqlHandler.Handlers(&cfg, schema, s.serverUrl, s.shutdown, s.logger, s.proxy, s.backendWSClient, nil, nil)

	// Construct GraphQL request payload
	query := `
		mutation TestMut {
    post(text: "hi", username: "test", roomName: "GeneralChat") {
        id
        text
        createdBy
        createdAt
  }
}
	`

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/query")
	req.Header.SetMethod("GET")
	req.URI().QueryArgs().Add("query", url.QueryEscape(query))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	gqlResp := new(Response)

	if err := json.Unmarshal(reqCtx.Response.Body(), gqlResp); err != nil {
		t.Error(err)
	}

	if len(gqlResp.Errors) != 1 {
		t.Errorf("Incorrect number of errors in the response. Expected: 1 and got %d",
			len(gqlResp.Errors))
	}

	expectedErrMsg := "wrong GraphQL query type in GET request"

	lastErrMsg := s.loggerHook.LastEntry().Data["error"].(error)

	if s.loggerHook.LastEntry().Message != unmarshalErr {
		t.Errorf("Got error message: %s; Expected error message: %s", s.loggerHook.LastEntry().Message, unmarshalErr)
	}

	if !strings.HasPrefix(lastErrMsg.Error(), expectedErrMsg) {
		t.Errorf("Got error message: %s; Expected error message: %s", lastErrMsg, expectedErrMsg)
	}

}

func (s *ServiceGraphQLTests) testGQLValidationFailed(t *testing.T) {

	gqlCfg := config.GraphQL{
		MaxQueryComplexity: 0,
		MaxQueryDepth:      0,
		NodeCountLimit:     0,
		Playground:         false,
		Introspection:      false,
		Schema:             "",
		RequestValidation:  "BLOCK",
	}

	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		APIFWServer: config.APIFWServer{
			APIHost: "http://localhost:8080/query",
		},
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	handler := graphqlHandler.Handlers(&cfg, schema, s.serverUrl, s.shutdown, s.logger, s.proxy, s.backendWSClient, nil, nil)

	// Construct GraphQL request payload
	query := `
		query {
    room(name: "GeneralChat") {
        name
        messages {
            id
            wrongParameter
            text
            createdBy
            createdAt
        }
    }
}
	`
	var requestBody = map[string]interface{}{
		"query": query,
	}

	jsonValue, _ := json.Marshal(requestBody)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/query")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(jsonValue), -1)
	req.Header.SetContentType("application/json")

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	gqlResp := new(Response)

	if err := json.Unmarshal(reqCtx.Response.Body(), gqlResp); err != nil {
		t.Error(err)
	}

	if len(gqlResp.Errors) != 1 {
		t.Errorf("Incorrect number of errors in the response. Expected: 1 and got %d",
			len(gqlResp.Errors))
	}

	expectedErrMsg := "field: wrongParameter not defined on type: Message"

	lastErrMsg := s.loggerHook.LastEntry().Data["error"].(error)

	if s.loggerHook.LastEntry().Message != validationErr {
		t.Errorf("Got error message: %s; Expected error message: %s", s.loggerHook.LastEntry().Message, validationErr)
	}

	if !strings.HasPrefix(lastErrMsg.Error(), expectedErrMsg) {
		t.Errorf("Got error message: %s; Expected error message: %s", lastErrMsg, expectedErrMsg)
	}

}

func (s *ServiceGraphQLTests) testGQLInvalidQuerySyntax(t *testing.T) {

	gqlCfg := config.GraphQL{
		MaxQueryComplexity: 0,
		MaxQueryDepth:      0,
		NodeCountLimit:     0,
		Playground:         false,
		Introspection:      false,
		Schema:             "",
		RequestValidation:  "BLOCK",
	}
	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		APIFWServer: config.APIFWServer{
			APIHost: "http://localhost:8080/query",
		},
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	handler := graphqlHandler.Handlers(&cfg, schema, s.serverUrl, s.shutdown, s.logger, s.proxy, s.backendWSClient, nil, nil)

	// Construct GraphQL request payload
	query := `
		query {
    room(name: "GeneralChat") {
        name
        messages {
            {
            id
            text
            createdBy
            createdAt
        }
    }
};
	`
	var requestBody = map[string]interface{}{
		"query": query,
	}

	jsonValue, _ := json.Marshal(requestBody)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/query")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(jsonValue), -1)
	req.Header.SetContentType("application/json")

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	gqlResp := new(Response)

	if err := json.Unmarshal(reqCtx.Response.Body(), gqlResp); err != nil {
		t.Error(err)
	}

	if len(gqlResp.Errors) != 1 {
		t.Errorf("Incorrect number of errors in the response. Expected: 1 and got %d",
			len(gqlResp.Errors))
	}

	expectedErrMsg := "external: unexpected token - got: LBRACE want one of: [RBRACE IDENT SPREAD]"

	lastErrMsg := s.loggerHook.LastEntry().Data["error"].(error)

	if s.loggerHook.LastEntry().Message != unmarshalErr {
		t.Errorf("Got error message: %s; Expected error message: %s", s.loggerHook.LastEntry().Message, unmarshalErr)
	}

	if !strings.HasPrefix(lastErrMsg.Error(), expectedErrMsg) {
		t.Errorf("Got error message: %s; Expected error message: %s", lastErrMsg, expectedErrMsg)
	}

}

func (s *ServiceGraphQLTests) testGQLInvalidMaxComplexity(t *testing.T) {

	gqlCfg := config.GraphQL{
		MaxQueryComplexity: 1,
		MaxQueryDepth:      0,
		NodeCountLimit:     0,
		Playground:         false,
		Introspection:      false,
		Schema:             "",
		RequestValidation:  "BLOCK",
	}
	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		APIFWServer: config.APIFWServer{
			APIHost: "http://localhost:8080/query",
		},
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	handler := graphqlHandler.Handlers(&cfg, schema, s.serverUrl, s.shutdown, s.logger, s.proxy, s.backendWSClient, nil, nil)

	// Construct GraphQL request payload
	query := `
		query {
    room(name: "GeneralChat") {
        name
        messages {
            id
            text
            createdBy
            createdAt
        }
    }
}
	`
	var requestBody = map[string]interface{}{
		"query": query,
	}

	jsonValue, _ := json.Marshal(requestBody)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/query")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(jsonValue), -1)
	req.Header.SetContentType("application/json")

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	gqlResp := new(Response)

	if err := json.Unmarshal(reqCtx.Response.Body(), &gqlResp); err != nil {
		t.Fatal(err)
	}

	if len(gqlResp.Errors) != 1 {
		t.Errorf("Incorrect amount of errors in the response. Expected: 1 and got %d",
			len(gqlResp.Errors))
	}

	expectedErrMsg := "the maximum query complexity value has been exceeded. The maximum query complexity value is 1. The current query complexity is 2"

	lastErrMsg := s.loggerHook.LastEntry().Data["error"].(error)

	if s.loggerHook.LastEntry().Message != validationErr {
		t.Errorf("Got error message: %s; Expected error message: %s", s.loggerHook.LastEntry().Message, validationErr)
	}

	if !strings.HasPrefix(lastErrMsg.Error(), expectedErrMsg) {
		t.Errorf("Got error message: %s; Expected error message: %s", lastErrMsg, expectedErrMsg)
	}

}

func (s *ServiceGraphQLTests) testGQLInvalidMaxDepth(t *testing.T) {

	gqlCfg := config.GraphQL{
		MaxQueryComplexity: 0,
		MaxQueryDepth:      1,
		NodeCountLimit:     0,
		Playground:         false,
		Introspection:      false,
		Schema:             "",
		RequestValidation:  "BLOCK",
	}
	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		APIFWServer: config.APIFWServer{
			APIHost: "http://localhost:8080/query",
		},
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	handler := graphqlHandler.Handlers(&cfg, schema, s.serverUrl, s.shutdown, s.logger, s.proxy, s.backendWSClient, nil, nil)

	// Construct GraphQL request payload
	query := `
		query {
    room(name: "GeneralChat") {
        name
        messages {
            id
            text
            createdBy
            createdAt
        }
    }
}
	`
	var requestBody = map[string]interface{}{
		"query": query,
	}

	jsonValue, _ := json.Marshal(requestBody)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/query")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(jsonValue), -1)
	req.Header.SetContentType("application/json")

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	gqlResp := new(Response)

	if err := json.Unmarshal(reqCtx.Response.Body(), &gqlResp); err != nil {
		t.Fatal(err)
	}

	if len(gqlResp.Errors) != 1 {
		t.Errorf("Incorrect amount of errors in the response. Expected: 1 and got %d",
			len(gqlResp.Errors))
	}

	expectedErrMsg := "the maximum query depth value has been exceeded. The maximum query depth value is 1. The current query depth is 3"

	lastErrMsg := s.loggerHook.LastEntry().Data["error"].(error)

	if s.loggerHook.LastEntry().Message != validationErr {
		t.Errorf("Got error message: %s; Expected error message: %s", s.loggerHook.LastEntry().Message, validationErr)
	}

	if !strings.HasPrefix(lastErrMsg.Error(), expectedErrMsg) {
		t.Errorf("Got error message: %s; Expected error message: %s", lastErrMsg, expectedErrMsg)
	}

}

func (s *ServiceGraphQLTests) testGQLInvalidNodeLimit(t *testing.T) {

	gqlCfg := config.GraphQL{
		MaxQueryComplexity: 0,
		MaxQueryDepth:      0,
		NodeCountLimit:     1,
		Playground:         false,
		Introspection:      false,
		Schema:             "",
		RequestValidation:  "BLOCK",
	}
	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		APIFWServer: config.APIFWServer{
			APIHost: "http://localhost:8080/query",
		},
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	handler := graphqlHandler.Handlers(&cfg, schema, s.serverUrl, s.shutdown, s.logger, s.proxy, s.backendWSClient, nil, nil)

	// Construct GraphQL request payload
	query := `
		query {
    room(name: "GeneralChat") {
        name
        messages {
            id
            text
            createdBy
            createdAt
        }
    }
}
	`

	var requestBody = map[string]interface{}{
		"query": query,
	}

	jsonValue, _ := json.Marshal(requestBody)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/query")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(jsonValue), -1)
	req.Header.SetContentType("application/json")

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	gqlResp := new(Response)

	if err := json.Unmarshal(reqCtx.Response.Body(), &gqlResp); err != nil {
		t.Fatal(err)
	}

	if len(gqlResp.Errors) != 1 {
		t.Errorf("Incorrect amount of errors in the response. Expected: 1 and got %d",
			len(gqlResp.Errors))
	}

	expectedErrMsg := "the query node limit has been exceeded. The query node count limit is 1. The current query node count value is 2"

	lastErrMsg := s.loggerHook.LastEntry().Data["error"].(error)

	if s.loggerHook.LastEntry().Message != validationErr {
		t.Errorf("Got error message: %s; Expected error message: %s", s.loggerHook.LastEntry().Message, validationErr)
	}

	if !strings.HasPrefix(lastErrMsg.Error(), expectedErrMsg) {
		t.Errorf("Got error message: %s; Expected error message: %s", lastErrMsg, expectedErrMsg)
	}

}

func (s *ServiceGraphQLTests) testGQLBatchQueryLimit(t *testing.T) {

	gqlCfg := config.GraphQL{
		MaxQueryComplexity: 0,
		MaxQueryDepth:      0,
		NodeCountLimit:     0,
		MaxAliasesNum:      0,
		BatchQueryLimit:    1,
		Playground:         false,
		Introspection:      false,
		Schema:             "",
		RequestValidation:  "BLOCK",
	}
	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		APIFWServer: config.APIFWServer{
			APIHost: "http://localhost:8080/query",
		},
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	handler := graphqlHandler.Handlers(&cfg, schema, s.serverUrl, s.shutdown, s.logger, s.proxy, s.backendWSClient, nil, nil)

	// Construct GraphQL request payload
	bqReq := `[
{"query":"query {\n  room(name: \"GeneralChat\") {\n name\n}\n}","variables":[]},
{"query":"query {\n  room(name: \"GeneralChat\") {\n name\n}\n}","variables":[]},
{"query":"query {\n  room(name: \"GeneralChat\") {\n name\n}\n}","variables":[]}
]
	`

	//jsonValue, _ := json.Marshal(requestBody)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/query")
	req.Header.SetMethod("POST")
	req.SetBodyStream(strings.NewReader(bqReq), -1)
	req.Header.SetContentType("application/json")

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	gqlResp := new(Response)

	if err := json.Unmarshal(reqCtx.Response.Body(), &gqlResp); err != nil {
		t.Fatal(err)
	}

	expectedErrMsg := "the batch query limit has been exceeded. The number of queries in the batch is 3. The current batch query limit is 1"

	lastErrMsg := s.loggerHook.LastEntry().Data["error"].(error)

	if s.loggerHook.LastEntry().Message != validationErr {
		t.Errorf("Got error message: %s; Expected error message: %s", s.loggerHook.LastEntry().Message, validationErr)
	}

	if !strings.EqualFold(lastErrMsg.Error(), expectedErrMsg) {
		t.Errorf("Got error message: %s; Expected error message: %s", lastErrMsg, expectedErrMsg)
	}

}

func (s *ServiceGraphQLTests) testGQLDenylistBlock(t *testing.T) {

	gqlCfg := config.GraphQL{
		MaxQueryComplexity: 0,
		MaxQueryDepth:      0,
		NodeCountLimit:     0,
		Playground:         false,
		Introspection:      false,
		Schema:             "",
		RequestValidation:  "BLOCK",
	}

	tokensCfg := config.Token{
		CookieName: testDeniedCookieName,
		HeaderName: "",
		File:       "../../../resources/test/tokens/test.db",
	}

	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		APIFWServer: config.APIFWServer{
			APIHost: "http://localhost:8080/query",
		},
		Denylist: config.Denylist{Tokens: tokensCfg},
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	deniedTokens, err := denylist.New(&cfg.Denylist, s.logger)
	if err != nil {
		t.Fatal(err)
	}

	handler := graphqlHandler.Handlers(&cfg, schema, s.serverUrl, s.shutdown, s.logger, s.proxy, s.backendWSClient, deniedTokens, nil)

	// Construct GraphQL request payload
	query := `
		query {
    room(name: "GeneralChat") {
        name
        messages {
            id
            text
            createdBy
            createdAt
        }
    }
}
	`
	var requestBody = map[string]interface{}{
		"query": query,
	}

	jsonValue, _ := json.Marshal(requestBody)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/query")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(jsonValue), -1)
	req.Header.SetContentType("application/json")

	// add denied token to the Cookie header of the successful HTTP request (200)
	req.Header.SetCookie(testDeniedCookieName, testDeniedToken)

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 401 {
		t.Errorf("Incorrect response status code. Expected: 401 and got %d",
			reqCtx.Response.StatusCode())
	}

	gqlResp := new(Response)

	if err := json.Unmarshal(reqCtx.Response.Body(), gqlResp); err != nil {
		t.Error(err)
	}

	if len(gqlResp.Errors) != 1 {
		t.Errorf("Incorrect number of errors in the response. Expected: 1 and got %d",
			len(gqlResp.Errors))
	}

	expectedErrMsg := "access denied"
	if gqlResp.Errors[0].Message != expectedErrMsg {
		t.Errorf("Incorrect error message in the response. Expected: %s and got %s",
			expectedErrMsg, gqlResp.Errors[0].Message)
	}

}

var (
	msg0cWrongMessage = []byte("wrongMessage")

	msg0c = []byte("{\"type\":\"connection_init\",\"payload\":{}}")
	msg1c = []byte("{\"id\":\"1\",\"type\":\"start\",\"payload\":{\"variables\":{},\"extensions\":{},\"operationName\":\"NewMessageInGeneralChat\",\"query\":\"subscription NewMessageInGeneralChat {\\n  messageAdded(roomName: \\\"GeneralChat\\\") {\\n    id\\n    text\\n    createdBy\\n    createdAt\\n  }\\n}\\n\"}}")
	msg2c = []byte("{\"id\":\"1\",\"type\":\"stop\"}")
	msg0s = []byte("{\"type\":\"connection_ack\"}")
	msg1s = []byte("{\"payload\":{\"data\":{\"messageAdded\":{\"id\":\"gjmnSpbt\",\"text\":\"You've joined the room\",\"createdBy\":\"system\",\"createdAt\":\"2023-00-00T00:00:00.000000+03:00\"}}},\"id\":\"1\",\"type\":\"data\"}")
	msg2s = []byte("{\"id\":\"1\",\"type\":\"complete\"}")

	msg1cInvalid     = []byte("{\"id\":\"1\",\"type\":\"start\",\"payload\":{\"variables\":{},\"extensions\":{},\"operationName\":\"NewMessageInGeneralChat\",\"query\":\"subscription NewMessageInGeneralChat {\\n  messageAdded(roomName: \\\"GeneralChat\\\"WRONGSYNTAX\\n    id\\n    text\\n    WrongParameter\\n    createdBy\\n    createdAt\\n  }\\n}\\n\"}}")
	msg1cInvalidResp = []byte("{\"id\":\"1\",\"type\":\"error\",\"payload\":[{\"message\":\"invalid graphql request\"}]}")

	msg1cWrong            = []byte("{\"id\":\"1\",\"type\":\"start\",\"payload\":{\"variables\":{},\"extensions\":{},\"operationName\":\"NewMessageInGeneralChat\",\"query\":\"subscription NewMessageInGeneralChat {\\n  messageAdded(roomName: \\\"GeneralChat\\\") {\\n    id\\n    text\\n    WrongParameter\\n    createdBy\\n    createdAt\\n  }\\n}\\n\"}}")
	msg1sWrong            = []byte("{\"id\":\"1\",\"type\":\"error\",\"payload\":[{\"message\":\"field: WrongParameter not defined on type: Message\",\"path\":[\"subscription\",\"messageAdded\",\"WrongParameter\"]}]}")
	msg1sWrongFromBackend = []byte("{\"id\":\"1\",\"type\":\"error\",\"payload\":[{\"message\":\"field: WrongParameter not defined on type: Message\"}]}")
)

func StartWSBackendServer(t testing.TB, addr string) *fasthttp.Server {
	upgrader := websocket.FastHTTPUpgrader{
		Subprotocols: []string{"graphql-ws"},
		CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
			return true
		},
	}

	gqlHandler := func(ctx *fasthttp.RequestCtx) {
		t.Logf("recv headers: %v\n", string(ctx.Request.Header.Header()))

		err := upgrader.Upgrade(ctx, func(ws *websocket.Conn) {
			defer ws.Close()
			for {
				mt, message, err := ws.ReadMessage()
				assert.Nil(t, err)
				if err != nil {
					t.Error(err)
					break
				}

				assert.Equal(t, websocket.TextMessage, mt)
				assert.Equal(t, message, msg0c)

				err = ws.WriteMessage(websocket.TextMessage, msg0s)
				assert.Nil(t, err)
				if err != nil {
					t.Error(err)
					break
				}

				// read again
				mt, message, err = ws.ReadMessage()
				assert.Nil(t, err)
				if err != nil {
					t.Error(err)
					break
				}

				if bytes.Equal(message, msg1cWrong) {
					err = ws.WriteMessage(websocket.TextMessage, msg1sWrongFromBackend)
					assert.Nil(t, err)
					if err != nil {
						t.Error(err)
						break
					}

					mt, message, err = ws.ReadMessage()
					assert.Nil(t, err)
					if err != nil {
						t.Error(err)
						break
					}
				}

				assert.Equal(t, websocket.TextMessage, mt)
				assert.Equal(t, message, msg1c)

				err = ws.WriteMessage(websocket.TextMessage, msg1s)
				assert.Nil(t, err)
				if err != nil {
					t.Error(err)
					break
				}

				mt, message, err = ws.ReadMessage()
				assert.Nil(t, err)
				if err != nil {
					t.Error(err)
					break
				}

				assert.Equal(t, websocket.TextMessage, mt)
				assert.Equal(t, message, msg2c)

				err = ws.WriteMessage(websocket.TextMessage, msg2s)
				assert.Nil(t, err)
				if err != nil {
					t.Error(err)
					break
				}

				mt, message, err = ws.ReadMessage()
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					break
				}
			}
		})

		if err != nil {
			if _, ok := err.(websocket.HandshakeError); ok {
				assert.Errorf(t, err, "websocket handshake")
			}
			return
		}
	}

	// backend server initializing
	server := fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			switch string(ctx.Path()) {
			case "/graphql":
				gqlHandler(ctx)
			}
		},
	}

	go func() {
		// backend websocket server
		if err := server.ListenAndServe(addr); err != nil {
			assert.Errorf(t, err, "websocket backend server `ListenAndServe` quit")
		}
	}()

	return &server
}

func (s *ServiceGraphQLTests) testGQLSubscription(t *testing.T) {

	gqlCfg := config.GraphQL{
		MaxQueryComplexity: 0,
		MaxQueryDepth:      0,
		NodeCountLimit:     0,
		Playground:         false,
		Introspection:      false,
		Schema:             "",
		WSCheckOrigin:      true,
		WSOrigin:           []string{"http://localhost:19091"},
		RequestValidation:  "BLOCK",
	}
	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		APIFWServer: config.APIFWServer{
			APIHost: "http://localhost:8080/test",
		},
		Server: config.ProtectedAPI{
			URL: "http://localhost:19090/graphql",
		},
	}

	// start backend
	server := StartWSBackendServer(t, "localhost:19090")
	defer server.Shutdown()

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	serverUrl, err := url.ParseRequestURI(cfg.Server.URL)
	assert.Nil(t, err)

	handler := graphqlHandler.Handlers(&cfg, schema, serverUrl, s.shutdown, s.logger, s.proxy, s.backendWSClient, nil, nil)

	// connection to the backend
	headers := http.Header{}
	headers.Set("Sec-WebSocket-Protocol", "graphql-ws")
	headers.Set("Origin", "http://localhost:19090")

	wsBackendConn, wsResp, err := websocket.DefaultDialer.Dial("ws://localhost:19090/graphql", headers)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusSwitchingProtocols, wsResp.StatusCode)
	t.Logf("got resp from backend server (ws connection): %+v", wsResp)

	newConn := proxy.FastHTTPWebSocketConn{
		Conn: wsBackendConn,
	}

	s.backendWSClient.EXPECT().GetConn(gomock.Any()).Times(1).Return(&newConn, nil)

	srv := fasthttp.Server{
		Handler: handler,
	}

	go func() {
		if err := srv.ListenAndServe("localhost:19091"); err != nil {
			t.Errorf("websocket proxy server `ListenAndServe` quit, err=%v\n", err)
		}
	}()

	defer srv.Shutdown()

	time.Sleep(1 * time.Second)

	headers = http.Header{}
	headers.Set("Sec-WebSocket-Protocol", "graphql-ws")
	headers.Set("Origin", "http://localhost:19091")

	wsClientConn, wsClientResp, err := websocket.DefaultDialer.Dial("ws://localhost:19091/test", headers)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusSwitchingProtocols, wsClientResp.StatusCode)
	t.Logf("got resp from APIFW (ws connection): %+v", wsClientResp)

	// client send wrong graphql-ws message and it will be dropped
	err = wsClientConn.WriteMessage(websocket.TextMessage, msg0cWrongMessage)
	assert.Nil(t, err)
	t.Log("sent (will be dropped):", string(msg0c))

	// client send
	err = wsClientConn.WriteMessage(websocket.TextMessage, msg0c)
	assert.Nil(t, err)
	t.Log("sent:", string(msg0c))

	messageType, p, err := wsClientConn.ReadMessage()
	assert.Nil(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.NotZero(t, p)
	assert.Equal(t, msg0s, p)

	// client send wrong request
	err = wsClientConn.WriteMessage(websocket.TextMessage, msg1cWrong)
	assert.Nil(t, err)
	t.Log("sent:", string(msg1cWrong))

	messageType, p, err = wsClientConn.ReadMessage()
	assert.Nil(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.NotZero(t, p)
	assert.Equal(t, msg1sWrong, p)

	messageType, p, err = wsClientConn.ReadMessage()
	assert.Nil(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.NotZero(t, p)
	assert.Equal(t, msg2s, p)

	// client send wrong request
	err = wsClientConn.WriteMessage(websocket.TextMessage, msg1cInvalid)
	assert.Nil(t, err)
	t.Log("sent:", string(msg1cInvalid))

	messageType, p, err = wsClientConn.ReadMessage()
	assert.Nil(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.NotZero(t, p)
	assert.Equal(t, msg1cInvalidResp, p)

	messageType, p, err = wsClientConn.ReadMessage()
	assert.Nil(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.NotZero(t, p)
	assert.Equal(t, msg2s, p)

	// client send
	err = wsClientConn.WriteMessage(websocket.TextMessage, msg1c)
	assert.Nil(t, err)
	t.Log("sent:", string(msg1c))

	messageType, p, err = wsClientConn.ReadMessage()
	assert.Nil(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.NotZero(t, p)
	assert.Equal(t, msg1s, p)

	// client send
	err = wsClientConn.WriteMessage(websocket.TextMessage, msg2c)
	assert.Nil(t, err)
	t.Log("sent:", string(msg2c))

	// client read
	messageType, p, err = wsClientConn.ReadMessage()
	assert.Nil(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.NotZero(t, p)
	assert.Equal(t, msg2s, p)

	err = wsClientConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	assert.Nil(t, err)

	wsClientConn.Close()

	s.backendWSClient.EXPECT().GetConn(gomock.Any()).Times(1).Return(&newConn, nil)

	// check ws connection with wrong origin
	headers.Set("Origin", "http://wrongOrigin.com")
	_, wsClientRespE, err := websocket.DefaultDialer.Dial("ws://localhost:19091/test", headers)
	assert.NotNil(t, err)

	assert.Equal(t, fasthttp.StatusForbidden, wsClientRespE.StatusCode)
	t.Logf("got resp from APIFW (ws connection): %+v", wsClientRespE)
}

func (s *ServiceGraphQLTests) testGQLSubscriptionLogOnly(t *testing.T) {

	gqlCfg := config.GraphQL{
		MaxQueryComplexity: 0,
		MaxQueryDepth:      0,
		NodeCountLimit:     0,
		Playground:         false,
		Introspection:      false,
		Schema:             "",
		RequestValidation:  "LOG_ONLY",
	}
	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		APIFWServer: config.APIFWServer{
			APIHost: "http://localhost:8080/graphql",
		},
		Server: config.ProtectedAPI{
			URL: "http://localhost:19092/graphql",
		},
	}

	// start backend
	server := StartWSBackendServer(t, "localhost:19092")
	defer server.Shutdown()

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	serverUrl, err := url.ParseRequestURI(cfg.Server.URL)
	assert.Nil(t, err)

	handler := graphqlHandler.Handlers(&cfg, schema, serverUrl, s.shutdown, s.logger, s.proxy, s.backendWSClient, nil, nil)

	// connection to the backend
	headers := http.Header{}
	headers.Set("Sec-WebSocket-Protocol", "graphql-ws")
	headers.Set("Origin", "http://localhost:19092")

	wsBackendConn, wsResp, err := websocket.DefaultDialer.Dial("ws://localhost:19092/graphql", headers)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusSwitchingProtocols, wsResp.StatusCode)
	t.Logf("got resp: %+v", wsResp)

	newConn := proxy.FastHTTPWebSocketConn{
		Conn: wsBackendConn,
	}

	s.backendWSClient.EXPECT().GetConn(gomock.Any()).Times(1).Return(&newConn, nil)

	srv := fasthttp.Server{
		Handler: handler,
	}

	go func() {
		if err := srv.ListenAndServe("localhost:19093"); err != nil {
			t.Errorf("websocket proxy server `ListenAndServe` quit, err=%v\n", err)
		}
	}()

	defer srv.Shutdown()

	time.Sleep(1 * time.Second)

	headers = http.Header{}
	headers.Set("Sec-WebSocket-Protocol", "graphql-ws")
	headers.Set("Origin", "http://localhost:19093")

	wsClientConn, wsClientResp, err := websocket.DefaultDialer.Dial("ws://localhost:19093/graphql", headers)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, http.StatusSwitchingProtocols, wsClientResp.StatusCode)
	t.Logf("got resp: %+v", wsClientResp)

	// client send
	err = wsClientConn.WriteMessage(websocket.TextMessage, msg0c)
	assert.Nil(t, err)
	t.Log("sent:", string(msg0c))

	messageType, p, err := wsClientConn.ReadMessage()
	assert.Nil(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.NotZero(t, p)
	assert.Equal(t, msg0s, p)

	// client send wrong request
	err = wsClientConn.WriteMessage(websocket.TextMessage, msg1cWrong)
	assert.Nil(t, err)
	t.Log("sent:", string(msg1cWrong))

	messageType, p, err = wsClientConn.ReadMessage()
	assert.Nil(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.NotZero(t, p)
	assert.Equal(t, msg1sWrongFromBackend, p)

	// client send
	err = wsClientConn.WriteMessage(websocket.TextMessage, msg1c)
	assert.Nil(t, err)
	t.Log("sent:", string(msg1c))

	messageType, p, err = wsClientConn.ReadMessage()
	assert.Nil(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.NotZero(t, p)
	assert.Equal(t, msg1s, p)

	// client send
	err = wsClientConn.WriteMessage(websocket.TextMessage, msg2c)
	assert.Nil(t, err)
	t.Log("sent:", string(msg2c))

	// client read
	messageType, p, err = wsClientConn.ReadMessage()
	assert.Nil(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.NotZero(t, p)
	assert.Equal(t, msg2s, p)

	err = wsClientConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	assert.Nil(t, err)

	wsClientConn.Close()

	time.Sleep(1 * time.Second)

}

func (s *ServiceGraphQLTests) testGQLMaxAliasesNum(t *testing.T) {

	gqlCfg := config.GraphQL{
		MaxQueryComplexity: 0,
		MaxQueryDepth:      0,
		NodeCountLimit:     0,
		MaxAliasesNum:      1,
		Playground:         false,
		Introspection:      false,
		Schema:             "",
		RequestValidation:  "BLOCK",
	}
	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		APIFWServer: config.APIFWServer{
			APIHost: "http://localhost:8080/query",
		},
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	handler := graphqlHandler.Handlers(&cfg, schema, s.serverUrl, s.shutdown, s.logger, s.proxy, s.backendWSClient, nil, nil)

	// Construct GraphQL request payload
	query := `
		query {
    a0:room(name: "GeneralChat") {
        name
        messages {
            id
            text
            createdBy
            createdAt
        }
    }
    a1:room(name: "GeneralChat") {
        name
        messages {
            id
            text
            createdBy
            createdAt
        }
    }
}
	`
	var requestBody = map[string]interface{}{
		"query": query,
	}

	jsonValue, _ := json.Marshal(requestBody)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/query")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(jsonValue), -1)
	req.Header.SetContentType("application/json")

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	gqlResp := new(Response)

	if err := json.Unmarshal(reqCtx.Response.Body(), &gqlResp); err != nil {
		t.Fatal(err)
	}

	if len(gqlResp.Errors) != 1 {
		t.Errorf("Incorrect amount of errors in the response. Expected: 1 and got %d",
			len(gqlResp.Errors))
	}

	expectedErrMsg := "the maximum number of aliases in the GraphQL document has been exceeded. The maximum number of aliases value is 1. The current number of aliases is 2, locations: [], path: []"

	lastErrMsg := s.loggerHook.LastEntry().Data["error"].(error)

	if s.loggerHook.LastEntry().Message != validationErr {
		t.Errorf("Got error message: %s; Expected error message: %s", s.loggerHook.LastEntry().Message, validationErr)
	}

	if !strings.EqualFold(lastErrMsg.Error(), expectedErrMsg) {
		t.Errorf("Got error message: %s; Expected error message: %s", lastErrMsg, expectedErrMsg)
	}

}

func (s *ServiceGraphQLTests) testGQLDuplicateFields(t *testing.T) {

	gqlCfg := config.GraphQL{
		MaxQueryComplexity:      0,
		MaxQueryDepth:           0,
		NodeCountLimit:          0,
		MaxAliasesNum:           0,
		DisableFieldDuplication: true,
		Playground:              false,
		Introspection:           false,
		Schema:                  "",
		RequestValidation:       "BLOCK",
	}
	var cfg = config.GraphQLMode{
		Graphql: gqlCfg,
		APIFWServer: config.APIFWServer{
			APIHost: "http://localhost:8080/query",
		},
	}

	// parse the GraphQL schema
	schema, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatalf("Loading GraphQL Schema error: %v", err)
	}

	handler := graphqlHandler.Handlers(&cfg, schema, s.serverUrl, s.shutdown, s.logger, s.proxy, s.backendWSClient, nil, nil)

	// Construct GraphQL request payload
	query := `
		query {
    a0:room(name: "GeneralChat") {
        name
        messages {
            id
            text
            createdBy
            createdAt
        }
    }
    a1:room(name: "GeneralChat") {
        name
        messages {
            id
            name
            text
            createdBy
            createdAt
        }
    }
}
	`
	var requestBody = map[string]interface{}{
		"query": query,
	}

	jsonValue, _ := json.Marshal(requestBody)

	req := fasthttp.AcquireRequest()
	req.SetRequestURI("/query")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(jsonValue), -1)
	req.Header.SetContentType("application/json")

	responseBody := `{
    "data": {
        "room": {
            "name": "GeneralChat",
            "messages": [
                {
                    "id": "TrsXJcKa",
                    "text": "Hello, world!",
                    "createdBy": "TestUser",
                    "createdAt": "2023-01-01T00:00:00+00:00"
                }
            ]
        }
    }
}`

	resp := fasthttp.AcquireResponse()
	resp.SetStatusCode(fasthttp.StatusOK)
	resp.Header.SetContentType("application/json")
	resp.SetBody([]byte(responseBody))

	reqCtx := fasthttp.RequestCtx{
		Request: *req,
	}

	s.proxy.EXPECT().Get().Return(s.client, resolvedIP, nil).AnyTimes()
	s.client.EXPECT().Do(gomock.Any(), gomock.Any()).SetArg(1, *resp).AnyTimes()
	s.proxy.EXPECT().Put(resolvedIP, s.client).Return(nil).AnyTimes()

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	recvBody := strings.TrimSpace(string(reqCtx.Response.Body()))

	if recvBody != responseBody {
		t.Errorf("Incorrect response status code. Expected: %s and got %s",
			responseBody, recvBody)
	}

	// query with duplication of the name field
	query = `
		query {
    a0:room(name: "GeneralChat") {
        name
        messages {
            id
            text
            createdBy
            createdAt
        }
    }
    a1:room(name: "GeneralChat") {
        name
        messages {
            id
            name
            name
            text
            createdBy
            createdAt
        }
    }
}
	`

	requestBody = map[string]interface{}{
		"query": query,
	}

	jsonValue, _ = json.Marshal(requestBody)

	req = fasthttp.AcquireRequest()
	req.SetRequestURI("/query")
	req.Header.SetMethod("POST")
	req.SetBodyStream(bytes.NewReader(jsonValue), -1)
	req.Header.SetContentType("application/json")

	reqCtx = fasthttp.RequestCtx{
		Request: *req,
	}

	handler(&reqCtx)

	if reqCtx.Response.StatusCode() != 200 {
		t.Errorf("Incorrect response status code. Expected: 200 and got %d",
			reqCtx.Response.StatusCode())
	}

	gqlResp := new(Response)

	if err := json.Unmarshal(reqCtx.Response.Body(), &gqlResp); err != nil {
		t.Fatal(err)
	}

	if len(gqlResp.Errors) != 1 {
		t.Errorf("Incorrect amount of errors in the response. Expected: 1 and got %d",
			len(gqlResp.Errors))
	}

	lastErrMsg := s.loggerHook.LastEntry().Data["error"].(error)

	if s.loggerHook.LastEntry().Message != validationErr {
		t.Errorf("Got error message: %s; Expected error message: %s", s.loggerHook.LastEntry().Message, validationErr)
	}

	if !strings.HasPrefix(lastErrMsg.Error(), validator.ErrFieldDuplicationFound.Error()) {
		t.Errorf("Got error message: %s; Expected error message: %s", lastErrMsg, validator.ErrFieldDuplicationFound.Error())
	}
}
