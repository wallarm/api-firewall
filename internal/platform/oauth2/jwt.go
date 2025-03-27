package oauth2

import (
	"context"
	"crypto/rsa"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/wallarm/api-firewall/internal/config"
)

type JWT struct {
	Cfg       *config.Oauth
	Logger    *logrus.Logger
	PubKey    *rsa.PublicKey
	SecretKey []byte
}

func (j *JWT) Validate(ctx context.Context, tokenWithBearer string, scopes []string) error {

	tokenString := strings.TrimPrefix(tokenWithBearer, "Bearer ")

	type MyCustomClaims struct {
		Scope string `json:"scope"`
		jwt.RegisteredClaims
	}

	token, err := jwt.ParseWithClaims(tokenString, &MyCustomClaims{}, func(token *jwt.Token) (any, error) {

		switch j.Cfg.JWT.SignatureAlgorithm {
		case "RS256", "RS384", "RS512":
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, errors.New("unknown signing method")
			}
			return j.PubKey, nil
		case "HS256", "HS384", "HS512":
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unknown signing method")
			}
			return j.SecretKey, nil
		}

		return nil, errors.New("unknown signing method")
	})

	if err != nil {
		return fmt.Errorf("oauth2 token invalid: %s", err)
	}

	claims, ok := token.Claims.(*MyCustomClaims)
	if ok && token.Valid {
		j.Logger.Debugf("%v %v", claims.Scope, claims.RegisteredClaims.ExpiresAt)
	} else {
		return errors.New("oauth2 token invalid")
	}

	scopesInToken := strings.Split(strings.ToLower(claims.Scope), " ")

	for _, scope := range scopes {
		scopeFound := false
		for _, scopeInToken := range scopesInToken {
			if strings.EqualFold(scope, scopeInToken) {
				scopeFound = true
				break
			}
		}
		if !scopeFound {
			return errors.New("token doesn't contain a necessary scope")
		}
	}

	return nil
}
