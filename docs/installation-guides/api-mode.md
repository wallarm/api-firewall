# Validating Individual Requests Without Proxying

If you need to validate individual API requests based on a given OpenAPI specification without further proxying, you can utilize Wallarm API Firewall in a non-proxy mode. In this case, the solution does not validate responses.

!!! info "Feature availability"
    This feature is available for the API Firewall versions 0.6.12 and later, and it is tailored for REST API.

## Requirements

* [Installed and configured Docker](https://docs.docker.com/get-docker/)
* [SQLite database](https://www.sqlite.org/index.html) with the table containing one or more [OpenAPI 3.0 specifications](https://swagger.io/specification/). The database can be of one of the following formats:

    === "SQLite database V1"
        * Table name is `openapi_schemas`.
        * `schema_id`, integer (auto-increment) - ID of the specification.
        * `schema_version`, string - Specification version. You can assign any preferred version. When this field changes, API Firewall assumes the specification itself has changed and updates it accordingly.
        * `schema_format`, string - The specification format, can be `json` or `yaml`.
        * `schema_content`, string - The specification content.
    === "SQLite database V2"
        Use this format if you need to control whether a specification from the database has been handled by the API Firewall or not.

        * Table name is `openapi_schemas`.
        * `schema_id`, integer (auto-increment) - ID of the specification.
        * `schema_version`, string - Specification version. You can assign any preferred version. When this field changes, API Firewall assumes the specification itself has changed and updates it accordingly.
        * `schema_format`, string - The specification format, can be `json` or `yaml`.
        * `schema_content`, string - The specification content.
        * `status`, string - Specifies whether a specification is `new` (not yet processed) or `applied` (already processed). It is expected to be set to `new` by default.
        
            At startup, the API Firewall automatically updates processed specification status from `new` to `applied`.
            
            During the `APIFW_SPECIFICATION_UPDATE_PERIOD`, only specifications marked as `new` receive updates.

## Running the API Firewall container

To use the API Firewall for request validation without further proxying, you need to mount the [SQLite database containing OpenAPI 3.0 specifications](#requirements) to `/var/lib/wallarm-api/1/wallarm_api.db` inside the API Firewall Docker container. The path can be changed using the `APIFW_API_MODE_DEBUG_PATH_DB` variable.

Use the following command to run the API Firewall container:

```
docker run --rm -it -v <PATH_TO_SQLITE_DATABASE>:/var/lib/wallarm-api/1/wallarm_api.db \
    -e APIFW_MODE=API -p 8282:8282 wallarm/api-firewall:v0.8.0
```

You can pass to the container the following variables:

| Environment variable | Description | Required? |
| -------------------- | ----------- | --------- |
| `APIFW_MODE` | Sets the general API Firewall mode. Possible values are [`PROXY`](docker-container.md) (default), [`graphql`](graphql/docker-container.md), and `API`.<br><br>The appropriate value for this case is `API`. | Yes |
| `APIFW_URL` | URL for API Firewall. For example: `http://0.0.0.0:8088/`. The port value should correspond to the container port published to the host.<br><br>If API Firewall listens to the HTTPS protocol, please mount the generated SSL/TLS certificate and private key to the container, and pass to the container the [API Firewall SSL/TLS settings](../configuration-guides/ssl-tls.md).<br><br>The default value is `http://0.0.0.0:8282/`. | No |
| `APIFW_API_MODE_DEBUG_PATH_DB` | Sets a path to a specification database inside the Docker container.<br><br>The default value is `/var/lib/wallarm-api/1/wallarm_api.db`. | No |
| `APIFW_SPECIFICATION_UPDATE_PERIOD` | Determines the frequency of fetching updates from the mounted database. If set to `0`, the update is disabled. The default value is `1m` (1 minute). | No |
| `APIFW_API_MODE_UNKNOWN_PARAMETERS_DETECTION` | Determines if requests with undefined parameters, as per the specification, are blocked.<br><br>When set to `true`, requests with any non-required, undefined parameters are rejected (e.g., `GET test?a=123&b=123` is blocked if `b` is undefined in the `/test` endpoint specification). If set to `false`, such requests are allowed, provided they contain all required parameters.<br><br>The default vaue is `true`. | No |
| `APIFW_PASS_OPTIONS` | When set to `true`, the API Firewall allows `OPTIONS` requests to endpoints in the specification, even if the `OPTIONS` method is not described. The default value is `false`. | No |
| `APIFW_READ_TIMEOUT` | The timeout for API Firewall to read the full request (including the body). The default value is `5s`. | No |
| `APIFW_WRITE_TIMEOUT` | The timeout for API Firewall to return the response to the request. The default value is `5s`. | No |
| `APIFW_HEALTH_HOST` | The host of the health check service. The default value is `0.0.0.0:9667`. The liveness probe service path is `/v1/liveness` and the readiness service path is `/v1/readiness`. | No |
| `APIFW_API_MODE_DB_VERSION` | Determines the SQLite database version that the API Firewall is configured to use. Available options are:<ul><li>`0` (default) - tries to load V2 (with the `status` field) first; if unsuccessful, attempts V1. On both failures, the firewall fails to start.</li><li>`1` - recognize and process the database as V1 only.</li><li>`2` - recognize and process the database as V2 only.</li></ul> | No |

## Evaluating requests against the specification

When evaluating requests against the mounted specification, include the header `X-Wallarm-Schema-ID: <schema_id>` to indicate to API Firewall which specification should be used for validation:

=== "Single specification"
    ```
    curl http://0.0.0.0:8282/path -H "X-Wallarm-Schema-ID: <SCHEMA_ID>"
    ```
=== "Multiple specifications"
    You can evaluate requests against multiple specifications simultaneously. To do this, include the relevant list of specification IDs in the `X-Wallarm-Schema-ID` header, separated by commas. For instance, to assess a request against specifications with IDs 1 and 2, use the following format:

    
    ```
    curl http://0.0.0.0:8282/path -H "X-Wallarm-Schema-ID: 1, 2"
    ```

## Understanding API Firewall responses

API Firewall responds with the `200` HTTP code and JSON with details on request validation:

=== "Request matches the specification"
    ```json
    {
        "summary": [
            {
                "schema_id": 1,
                "status_code": 200
            }
        ]
    }
    ```
=== "Request does not match the specification"
    ```json
    {
        "summary": [
            {
                "schema_id":1,
                "status_code":403
            }
        ],
        "errors": [
            {
                "message":"method and path are not found",
                "code":"method_and_path_not_found",
                "schema_id":1
            }
        ]
    }
    ```
=== "Unable to validate a request"
    ```json
    {
        "summary": [
            {
                "schema_id": 0,
                "status_code": 500
            }
        ]
    }
    ```

| JSON key | Description |
| -------- | ----------- |
| `summary` | Array with a request validation summary. |
| `summary.schema_id` | The ID of the specification against which the API Firewall performed the request validation. |
| `summary.status_code` | Request validation status code. Possible values:<ul><li>`200` if a request matches the specification.</li><li>`403` if a request does not match the specification.</li><li>`500` if it is unable to handle or validate a request.</li></ul> |
| `errors` | Array containing details about the reasons why a request does not match the specification. |
| `errors.message` | Explanation for the request's dismatch with the specification. |
| `errors.code` | Code indicating the reason for a request's mismatch with the specification. [Possible values](https://github.com/wallarm/api-firewall/blob/50451e6ae99daf958fa75e592d724c8416a098dd/cmd/api-firewall/internal/handlers/api/errors.go#L14). |
| `errors.schema_version` | The version of the specification against which the API Firewall performed the request validation. |
| `errors.related_fields` | An array of parameters that violated the specification. |
| `errors.related_fields_details` | Details on parameters that violated the specification. |
| `errors.related_fields_details.name` | Parameter name. |
| `errors.related_fields_details.expected_type` | Expected parameter type (if the type is wrong). |
| `errors.related_fields_details.current_value` | Parameter value passed in a request. |
| `errors.related_fields_details.pattern` | Parameter value pattern specified in the specification. |

## Database issues

### Handling invalidity in an already mounted SQLite database

The API Firewall automatically retrieves specification updates from the mounted database at intervals defined by the `APIFW_SPECIFICATION_UPDATE_PERIOD` variable. If the database structure or specifications become invalid, or if the database file disappears post-update, the Firewall maintains the last valid specification file and pauses further updates. This method guarantees continuous operation with the most recent valid specifications until a correct database file is reestablished in the API Firewall.

In cases where the database file is valid but contains an invalid specification, the API Firewall will disregard the faulty specification and proceed to load all valid specifications.

**Example**

Suppose the API Firewall has loaded two specifications, labeled 1 and 2. If specification 1 is modified and becomes invalid (due to syntax errors or parsing issues), the API Firewall will then only load and use specification 2. It will log an error message indicating the issue and will operate with only specification 2.

### Mounting empty SQLite database

If the API Firewall is initiated with an empty, invalid, or non-existent database file, it will start and log errors if updates fail. In this state, the API Firewall will not have any specification, thus unable to validate requests, and will respond with a 500 status code. Note that the readiness probe will fail until a valid database is loaded.
