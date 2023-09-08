package complexity

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wundergraph/graphql-go-tools/pkg/graphql"
)

const (
	testQuery = `
query {
    room(name: "TestChat") {
        name
        messages {
            id
            text
            createdBy
            createdAt
        }
    }
}`
	testSchema = `
type Chatroom {
    name: String!
    messages: [Message!]!
}

type Message {
    id: ID!
    text: String!
    createdBy: String!
    createdAt: Time!
}

type Query {
    room(name:String!): Chatroom
}
`
)

func TestComplexity(t *testing.T) {
	testCases := map[string]struct {
		cfgGraphQL         *config.GraphQL
		expectedErrorCount int
	}{
		"disabled_all": {
			cfgGraphQL: &config.GraphQL{},
		},
		"invalid_query": {
			cfgGraphQL: &config.GraphQL{
				NodeCountLimit:     1,
				MaxQueryDepth:      1,
				MaxQueryComplexity: 1,
			},
			expectedErrorCount: 3,
		},
		"invalid_query_node_count_limit": {
			cfgGraphQL: &config.GraphQL{
				NodeCountLimit: 1,
			},
			expectedErrorCount: 1,
		},
		"invalid_query_max_depth": {
			cfgGraphQL: &config.GraphQL{
				MaxQueryDepth: 1,
			},
			expectedErrorCount: 1,
		},
		"invalid_query_max_complexity": {
			cfgGraphQL: &config.GraphQL{
				MaxQueryComplexity: 1,
			},
			expectedErrorCount: 1,
		},
		"valid_complexity": {
			cfgGraphQL: &config.GraphQL{
				MaxQueryComplexity: 2,
			},
			expectedErrorCount: 0,
		},
		"valid_max_depth": {
			cfgGraphQL: &config.GraphQL{
				MaxQueryDepth: 3,
			},
			expectedErrorCount: 0,
		},
		"valid_node_count_limit": {
			cfgGraphQL: &config.GraphQL{
				NodeCountLimit: 2,
			},
			expectedErrorCount: 0,
		},
		"valid_query_limits": {
			cfgGraphQL: &config.GraphQL{
				MaxQueryComplexity: 2,
				MaxQueryDepth:      3,
				NodeCountLimit:     2,
			},
			expectedErrorCount: 0,
		},
	}

	s, err := graphql.NewSchemaFromString(testSchema)
	if err != nil {
		t.Fatal(err)
	}

	gqlRequest := &graphql.Request{
		Query: testQuery,
	}

	if _, err := s.Normalize(); err != nil {
		t.Fatal(err)
	}

	if _, err := gqlRequest.Normalize(s); err != nil {
		t.Fatal(err)
	}

	for name, testCase := range testCases {
		requestErrors := ValidateQuery(testCase.cfgGraphQL, s, gqlRequest)
		require.Equalf(t, testCase.expectedErrorCount, requestErrors.Count(), "case %s: unexpected error count", name)
	}
}
