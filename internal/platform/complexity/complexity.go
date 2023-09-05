package complexity

import (
	"fmt"

	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wundergraph/graphql-go-tools/pkg/graphql"
)

// ValidateQuery performs the query complexity checks
func ValidateQuery(cfg *config.GraphQL, s *graphql.Schema, r *graphql.Request) graphql.RequestErrors {
	result, err := r.CalculateComplexity(graphql.DefaultComplexityCalculator, s)
	if err != nil {
		return graphql.RequestErrorsFromError(err)
	}

	var requestErrors graphql.RequestErrors

	if cfg.MaxQueryComplexity > 0 && result.Complexity > cfg.MaxQueryComplexity {
		requestErrors = append(requestErrors,
			graphql.RequestError{Message: fmt.Sprintf("the maximum query complexity value has been exceeded. The maximum query complexity value is %d. The current query complexity is %d", cfg.MaxQueryComplexity, result.Complexity)})
	}

	if cfg.MaxQueryDepth > 0 && result.Depth > cfg.MaxQueryDepth {
		requestErrors = append(requestErrors,
			graphql.RequestError{Message: fmt.Sprintf("the maximum query depth value has been exceeded. The maximum query depth value is %d. The current query depth is %d", cfg.MaxQueryDepth, result.Depth)})
	}

	if cfg.NodeCountLimit > 0 && result.NodeCount > cfg.NodeCountLimit {
		requestErrors = append(requestErrors,
			graphql.RequestError{Message: fmt.Sprintf("the query node limit has been exceeded. The query node count limit is %d. The current query node count value is %d", cfg.NodeCountLimit, result.NodeCount)})
	}

	return requestErrors
}
