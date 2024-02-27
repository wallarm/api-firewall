# API Firewall Changelog

This page describes new releases of Wallarm API Firewall.

## v0.6.16 (2023-01-29)

* Fixed the processing issues of the HEAD request type in the [`api` mode](installation-guides/api-mode.md)
* Improved log messages by adding `host` and `path` parameters, providing immediate insight into request destinations. [Issue #78](https://github.com/wallarm/api-firewall/issues/78)
* Adjusted TEXT log formatting to remove multi-line outputs. All log messages in TEXT logging mode are now formatted in a single line, enhancing readability for log parsers. For example, previously, multi-line outputs were consolidated into a single line, replacing occurrences of `\r\n` with spaces. [Issue #79](https://github.com/wallarm/api-firewall/issues/79)
* Implemented a solution to generate unique `request_id` values, resolving conflicts caused by the incremental nature of `request_id`. [Issue #80](https://github.com/wallarm/api-firewall/issues/80)
* Add tests
* Dependency upgrade

## v0.6.15 (2023-12-21)

* Dependency upgrade
* Bug fixes
* Add tests
* When operating in the [`api` mode](installation-guides/api-mode.md), the API Firewall now returns error messages in responses for requests containing parameter values that exceed the minimum and maximum limits defined in the OpenAPI specification

## v0.6.14 (2023-11-23)

* Introduced new environment variables to limit GraphQL queries: `APIFW_GRAPHQL_MAX_ALIASES_NUM` and `APIFW_GRAPHQL_FIELD_DUPLICATION`.
* Implemented more [detailed responses](installation-guides/api-mode.md#understanding-api-firewall-responses) for requests that do not match mounted specifications in the **API non-proxy mode**.

## v0.6.13 (2023-09-08)

* [Support for GraphQL API requests validation](installation-guides/graphql/docker-container.md)

## v0.6.12 (2023-08-04)

* Ability to set the general API Firewall mode using the `APIFW_MODE` environment variable. The default value is `PROXY`. When set to API, you can [validate individual API requests based on a provided OpenAPI specification without further proxying](installation-guides/api-mode.md).
* Introduced the ability to allow `OPTIONS` requests for endpoints specified in the OpenAPI, even if the `OPTIONS` method is not explicitly defined. This can be achieved using the `APIFW_PASS_OPTIONS` variable. The default value is `false`.
* Introduced a feature that allows control over whether requests should be identified as non-matching the specification if their parameters do not align with those outlined in the OpenAPI specification. It is set to `true` by default.

    This can be controlled through the `APIFW_SHADOW_API_UNKNOWN_PARAMETERS_DETECTION` variable in `PROXY` mode and via the `APIFW_API_MODE_UNKNOWN_PARAMETERS_DETECTION` variable in `API` mode.
* The new logging level mode `TRACE` to log incoming requests and API Firewall responses, including their content. This level can be set using the `APIFW_LOG_LEVEL` environment variable.
* Dependency updates
* Bug fixes

## v0.6.11 (2023-02-10)

* Add the `APIFW_SERVER_DELETE_ACCEPT_ENCODING` environment variable. If it is set to `true`, the `Accept-Encoding` header is deleted from proxied requests. The default value is `false`.
* https://github.com/wallarm/api-firewall/issues/56
* https://github.com/wallarm/api-firewall/issues/57
* Add decompression for the request body and response body

## v0.6.10 (2022-12-15)

* https://github.com/wallarm/api-firewall/issues/54
* Update dependencies

## v0.6.9 (2022-09-12)

* Upgrade Go to 1.19
* Upgrade other dependencies
* Fix bugs of Shadow API detection and denylist processing
* Delete the `Apifw-Request-Id` header from responses returned by API Firewall
* Add compatibility of the Ingress object with Kubernetes 1.22
* Terminate logging of incoming requests matching API specification at the INFO log level

## v0.6.8 (2022-04-11)

### New features

* Ability to specify the URL address of the OpenAPI 3.0 specification instead of mounting the specification file into the Docker container (via the environment variable [`APIFW_API_SPECS`](installation-guides/docker-container.md#apifw-api-specs)).
* Ability to use the custom `Content-Type` header when sending requests to the token introspection service (via the environment variable [`APIFW_SERVER_OAUTH_INTROSPECTION_CONTENT_TYPE`](configuration-guides/validate-tokens.md)).
* [Support for the authentication token denylists](configuration-guides/denylist-leaked-tokens.md).

## v0.6.7 (2022-01-25)

Wallarm API Firewall is now open source. There are the following related changes in [this release](https://github.com/wallarm/api-firewall/releases/tag/v0.6.7):

* API Firewall source code and related open source license are published
* GitHub workflow for binary, Helm chart and Docker image building is implemented

## v0.6.6 (2021-12-09)

### New features

* Support for [OAuth 2.0 token validation](configuration-guides/validate-tokens.md).
* [Connection](configuration-guides/ssl-tls.md) to the servers signed with the custom CA certificates and support for insecure connection flag.

### Bug fixes

* https://github.com/wallarm/api-firewall/issues/27

## v0.6.5 (2021-10-12)

### New features

* Configuration of the maximum number of the fasthttp clients (via the environment variable `APIFW_SERVER_CLIENT_POOL_CAPACITY`).
* Health checks on the 9667 port of the API Firewall container (the port can be changed via the environment variable `APIFW_HEALTH_HOST`).

[Instructions on running the API Firewall with new environment variables](installation-guides/docker-container.md)

### Bug fixes

* https://github.com/wallarm/api-firewall/issues/15
* Some other bugs

## v0.6.4 (2021-08-18)

### New features

* Added monitoring for Shadow API endpoints. API Firewall operating in the `LOG_ONLY` mode for both the requests and responses marks all endpoints that are not included in the specification and are returning the code different from `404` as the shadow ones. You can exclude response codes indicating shadow endpoints using the environment variable `APIFW_SHADOW_API_EXCLUDE_LIST`.
* Configuration of the HTTP response status code returned by API Firewall to blocked requests (via the environment variable `APIFW_CUSTOM_BLOCK_STATUS_CODE`). 
* Ability to return the header containing the reason for the request blocking (via the environment variable `APIFW_ADD_VALIDATION_STATUS_HEADER`). This feature is **experimental**.
* Configuration of the API Firewall log format (via the environment variable `APIFW_LOG_FORMAT`).

[Instructions on running the API Firewall with new environment variables](installation-guides/docker-container.md)

### Optimizations

* Optimized validation of the OpenAPI 3.0 specification due to added `fastjson` parser.
* Added support for fasthttp.

## v0.6.2 (2021-06-22)

* The first release!
