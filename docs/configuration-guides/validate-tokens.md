# Validating Request Authentication Tokens

When leveraging OAuth 2.0 for authentication, the API Firewall can be set up to validate access tokens before directing requests to your application server. The Firewall expects the access token in the `Authorization: Bearer` request header.

API Firewall considers the token to be valid if the scopes defined in the [specification](https://swagger.io/docs/specification/authentication/oauth2/) and in the token meta information are the same. If the value of `APIFW_REQUEST_VALIDATION` is `BLOCK`, API Firewall blocks requests with invalid tokens. In the `LOG_ONLY` mode, requests with invalid tokens are only logged.

!!! info "Feature availability"
    This feature is available only when running API Firewall for [REST API](../installation-guides/docker-container.md) request filtering.

To configure the OAuth 2.0 token validation flow, use the following environment variables:

| Environment variable | Description |
| -------------------- | ----------- |
| `APIFW_SERVER_OAUTH_VALIDATION_TYPE` | The type of authentication token validation:<ul><li>`JWT` if using JWT for request authentication. Perform further configuration via the `APIFW_SERVER_OAUTH_JWT_*` variables.</li><li>`INTROSPECTION` if using other token types that can be validated by the particular token introspection service. Perform further configuration via the `APIFW_SERVER_OAUTH_INTROSPECTION_*` variables.</li></ul> |
| `APIFW_SERVER_OAUTH_JWT_SIGNATURE_ALGORITHM` | The algorithm being used to sign JWTs: `RS256`, `RS384`, `RS512`, `HS256`, `HS384` or `HS512`.<br><br>JWTs signed using the `ECDSA` algorithm cannot be validated by API Firewall. |
| `APIFW_SERVER_OAUTH_JWT_PUB_CERT_FILE` | If JWTs are signed using the RS256, RS384 or RS512 algorithm, the path to the file with the RSA public key (`*.pem`). This file must be mounted to the API Firewall Docker container. |
| `APIFW_SERVER_OAUTH_JWT_SECRET_KEY` | If JWTs are signed using the HS256, HS384 or HS512 algorithm, the secret key value being used to sign JWTs. |
| `APIFW_SERVER_OAUTH_INTROSPECTION_ENDPOINT` | [Token introspection endpoint](https://www.oauth.com/oauth2-servers/token-introspection-endpoint/). Endpoint examples:<ul><li>`https://www.googleapis.com/oauth2/v1/tokeninfo` if using Google OAuth</li><li>`http://sample.com/restv1/introspection` for Gluu OAuth 2.0 tokens</li></ul> |
| `APIFW_SERVER_OAUTH_INTROSPECTION_ENDPOINT_METHOD` | The method of the requests to the token introspection endpoint. Can be `GET` or `POST`.<br><br>The default value is `GET`. |
| `APIFW_SERVER_OAUTH_INTROSPECTION_TOKEN_PARAM_NAME` | The name of the parameter with the token value in the requests to the introspection endpoint. Depending on the `APIFW_SERVER_OAUTH_INTROSPECTION_ENDPOINT_METHOD` value, API Firewall automatically considers the parameter to be either the query or body parameter. |
| `APIFW_SERVER_OAUTH_INTROSPECTION_CLIENT_AUTH_BEARER_TOKEN` | The Bearer token value to authenticate the requests to the introspection endpoint. |
| <a name="apifw-server-oauth-introspection-content-type"></a>`APIFW_SERVER_OAUTH_INTROSPECTION_CONTENT_TYPE` | The value of the `Content-Type` header indicating the media type of the token introspection service. The default value is `application/octet-stream`. |
| `APIFW_SERVER_OAUTH_INTROSPECTION_REFRESH_INTERVAL` | Time-to-live of cached token metadata. API Firewall caches token metadata and if getting requests with the same tokens, gets its metadata from the cache.<br><br>The interval can be set in hours (`h`), minutes (`m`), seconds (`s`) or in the combined format (e.g. `1h10m50s`).<br><br>The default value is `10m` (10 minutes).  |
