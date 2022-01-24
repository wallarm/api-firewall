package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/karlseguin/ccache/v2"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"

	"github.com/wallarm/api-firewall/internal/config"
)

type Introspection struct {
	Cfg    *config.Oauth
	Logger *logrus.Logger
	Cache  *ccache.Cache
}

func (i *Introspection) Validate(ctx context.Context, tokenWithBearer string, scopes []string) error {

	// openapi doesn't contain scopes in endpoint configuration
	if len(scopes) == 0 {
		return nil
	}

	tokenString := strings.TrimPrefix(tokenWithBearer, "Bearer ")

	if tokenString == "" {
		return errors.New("oauth token not found")
	}

	var meta map[string]interface{}
	var err error

	metaCached := i.Cache.Get(tokenString)
	switch metaCached {
	case nil:
		meta, err = i.getTokenMetaInfo(tokenString)
		if err != nil {
			return err
		}
	default:
		meta = metaCached.Value().(map[string]interface{})
	}

	scopeString, ok := meta["scope"].(string)
	if !ok && len(scopes) > 0 {
		return errors.New("scope field not found in OAuth provider response")
	}

	scopesInToken := strings.Split(scopeString, " ")

	i.Cache.Set(tokenString, meta, i.Cfg.Introspection.RefreshInterval)

	for _, scope := range scopes {
		scopeFound := false
		for _, scopeInToken := range scopesInToken {
			if scope == scopeInToken {
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

func (i *Introspection) getTokenMetaInfo(token string) (map[string]interface{}, error) {

	req := fasthttp.AcquireRequest()
	req.Header.SetMethod(i.Cfg.Introspection.EndpointMethod)

	parsedEndpointUrl, err := url.Parse(i.Cfg.Introspection.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse introspection endpoint url: %v", err)
	}
	switch strings.ToLower(i.Cfg.Introspection.EndpointMethod) {
	case "post":
		if i.Cfg.Introspection.TokenParamName != "" {
			req.SetBodyString(fmt.Sprintf("%s=%s", i.Cfg.Introspection.TokenParamName, token))
			if i.Cfg.Introspection.EndpointParams != "" {
				req.AppendBodyString(fmt.Sprintf("&%s", i.Cfg.Introspection.EndpointParams))
			}
		} else {
			if i.Cfg.Introspection.EndpointParams != "" {
				req.SetBodyString(i.Cfg.Introspection.EndpointParams)
			}
		}
	case "get":
		if i.Cfg.Introspection.EndpointParams != "" {
			parsedEndpointUrl.RawQuery = i.Cfg.Introspection.EndpointParams
		}

		if i.Cfg.Introspection.TokenParamName != "" {
			reqQuery := parsedEndpointUrl.Query()
			reqQuery.Add(i.Cfg.Introspection.TokenParamName, token)
			parsedEndpointUrl.RawQuery = reqQuery.Encode()
		}

	}

	t := parsedEndpointUrl.String()
	req.SetRequestURI(t)

	if i.Cfg.Introspection.ClientAuthBearerToken == "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else {
		req.Header.Set("Authorization", "Bearer "+i.Cfg.Introspection.ClientAuthBearerToken)
	}

	res := fasthttp.AcquireResponse()
	if err := fasthttp.Do(req, res); err != nil {
		return nil, fmt.Errorf("failed to send introspection request: %v", err)
	}
	fasthttp.ReleaseRequest(req)

	body := res.Body()

	var tokenStatus map[string]interface{}
	if err := json.Unmarshal(body, &tokenStatus); err != nil {
		return nil, fmt.Errorf("failed to unmarshal extension properties: %v (%s)", err, body)
	}

	fasthttp.ReleaseResponse(res)

	return tokenStatus, nil
}
