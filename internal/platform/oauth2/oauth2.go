package oauth2

import (
	"context"
)

type OAuth2 interface {
	Validate(ctx context.Context, tokenWithBearer string, scopes []string) error
}
