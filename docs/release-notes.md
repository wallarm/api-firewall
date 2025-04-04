# API Firewall Changelog

This page describes new releases of Wallarm API Firewall.

## v0.9.0 (2025-04-04)

* Added support of the [YAML configuration file](installation-guides/docker-container.md#step-4-configure-api-firewall)
* Added support of the [endpoint-related response actions](configuration-guides/endpoint-related-response.md)
* Replaced the Logrus logging library with ZeroLog

## v0.8.9 (2025-03-28)

* Dependency upgrade
* Update the Go version up to v1.23.7

## v0.8.8 (2025-02-27)

* Dependency upgrade
* Fix schema update bug in API mode 
* Update the Go version up to v1.23.6

## v0.8.7 (2025-02-21)

* Fix the high CPU load issue
* Update the Go version up to v1.22.12

## v0.8.6 (2024-12-20)

* Dependency upgrade
* Make the release binaries version detectable by Syft

## v0.8.5 (2024-12-13)

* Dependency upgrade
* Bump Go version to 1.22.10

## v0.8.4 (2024-11-12)

* Fixed the DNS resolver issue in the GraphQL mode
* Updated the Helm chart
* Bump Go version to 1.22.9

## v0.8.3 (2024-10-22)

* Add additional API-Firewall server [configuration parameters](configuration-guides/system-settings.md)
* Bump Go version to 1.22.8

## v0.8.2 (2024-09-24)

* Fixed DNS resolver cache issue

## v0.8.1 (2024-09-13)

* Fixed incorrect request to get API specification structure issue
* Dependency upgrade
* Bump Go version to 1.22.7

## v0.8.0 (2024-08-19)

* Added [DNS cache update](configuration-guides/dns-cache-update.md) feature

    Allows making asynchronous DNS requests and cache results for a configured period of time. This could be useful when DNS load balancing is used.

* Fixed GQL proxying configuration issue
* Dependency upgrade

## v0.7.4 (2024-07-12)

* Added `APIFW_API_SPECS_CUSTOM_HEADER_NAME` and `APIFW_API_SPECS_CUSTOM_HEADER_VALUE` environment variables. These allow adding a custom header to requests for your OpenAPI specification URL (defined in `APIFW_API_SPECS`).

    For example, this can be used to specify the authentication data for API Firewall to reach the specification URL.
* Added the `APIFW_SPECIFICATION_UPDATE_PERIOD` environment variable to specify the interval for updating the OpenAPI specification from the hosted URL (defined in `APIFW_API_SPECS`).
* Bump Alpine version to 3.20
* Bump Go version to 1.21.12

## v0.7.3 (2024-06-06)

* Dependency upgrade
* Supported new interface for the `api` mode usage, only for internal use
* Added the `APIFW_SERVER_REQUEST_HOST_HEADER` environment variable to set a custom `Host` header for requests forwarded to your backend after API Firewall validation

    This variable is supported in the [`PROXY`](installation-guides/docker-container.md) and [`graphql`](installation-guides/graphql/docker-container.md) API Firewall modes.

## v0.7.2 (2024-04-16)

* Added the [demo for running the API Firewall with OWASP CoreRuleSet v4.1.0](demos/owasp-coreruleset.md).
* Fixed multiple entries in `related_fields` in the `api` mode.
* Moved logging of errors caused by requests not matching the uploaded specification from the `ERROR` level to the `DEBUG` level. Now, `ERROR` level logs only include issues directly related to API Firewall operations. This change applies exclusively to `api` mode.

## v0.7.1 (2024-04-15)

* Bug fixes in the `api` mode
* Updated router
* Supported parsing of `Content-Type` headers with the `+json`, `+xml`, `+yaml`, `+csv` structured syntax suffixes

## v0.7.0 (2024-04-03)

* Added [ModSecurity rules support](migrating/modseс-to-apif.md) (based on the [Coraza](https://github.com/corazawaf/coraza) project)
* Fixed processing issues for the requests with the OPTIONS method
* Added additional info to the log message of the Shadow API module

## v0.6.17 (2024-03-28)

* Added [IP allowlisting](configuration-guides/allowlist.md) support in the `API` mode
* ​​Added support for subnets in allowlisted IP file and IP address validation during the file upload
* Added support for a new SQLite database structure (V2) in the [`API`](installation-guides/api-mode.md) mode of the API Firewall. This version adds a `status` field to track specifications as `new` (unprocessed by the firewall) or `applied` (processed).
    
    For backward compatibility, the `APIFW_API_MODE_DB_VERSION` environment variable has been added - it defaults to attempting to parse the database as V2; if unsuccessful, it falls back to previous format  (V1).
* Added the following default response from the API Firewall to GraphQL requests that do not match a provided API schema:


    ```json
    {
      "errors": [
        {
          "message":"invalid query"
        }
      ]
    }
    ```
* Introduced the new environment variable to limit the number of queries that can be batched together in a single GraphQL request, `APIFW_GRAPHQL_BATCH_QUERY_LIMIT`
* Upgraded Go up to 1.21 and some other dependencies

## v0.6.16 (2024-02-27)

* Added IP allowlisting, enabling secure access to backends by allowing only requests from predefined IP addresses for both REST and GraphQL APIs. This update ensures requests from allowlisted IPs are validated against the OpenAPI specification 3.0, with non-allowlisted IP requests being rejected with a 403 error code. Thanks for [PR #76 contributors](https://github.com/wallarm/api-firewall/pull/76). [Read more](configuration-guides/allowlist.md)
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
