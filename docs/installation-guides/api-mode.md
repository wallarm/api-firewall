# Validating Individual Requests Without Proxying

If you need to validate individual API requests based on a given OpenAPI specification without further proxying, you can utilize Wallarm API Firewall in a non-proxy mode. In this case, the solution does not validate responses.

!!! info "Feature availability"
    This feature is available for the API Firewall versions 0.6.12 and later, and it is tailored for REST API.

To do so:

1. Instead of [mounting the OpenAPI specification](../installation-guides/docker-container.md) file to the container, mount the [SQLite database](https://www.sqlite.org/index.html) containing one or more OpenAPI 3.0 specifications to `/var/lib/wallarm-api/1/wallarm_api.db`. The database should adhere to the following schema:

    * `schema_id`, integer (auto-increment) - ID of the specification.
    * `schema_version`, string - Specification version. You can assign any preferred version. When this field changes, API Firewall assumes the specification itself has changed and updates it accordingly.
    * `schema_format`, string - The specification format, can be `json` or `yaml`.
    * `schema_content`, string - The specification content.
1. Run the container with the environment variable `APIFW_MODE=API` and if needed, with other variables that specifically designed for this mode:

    | Environment variable | Description |
    | -------------------- | ----------- |
    | `APIFW_MODE` | Sets the general API Firewall mode. Possible values are [`PROXY`](docker-container.md) (default), [`graphql`](graphql/docker-container.md), and `API`. |
    | `APIFW_SPECIFICATION_UPDATE_PERIOD` | Determines the frequency of specification updates. If set to `0`, the specification update is disabled. The default value is `1m` (1 minute). |
    | `APIFW_API_MODE_UNKNOWN_PARAMETERS_DETECTION` | Specifies whether to return an error code if the request parameters do not match those defined in the the specification. The default value is `true`. |
    | `APIFW_PASS_OPTIONS` | When set to `true`, the API Firewall allows `OPTIONS` requests to endpoints in the specification, even if the `OPTIONS` method is not described. The default value is `false`. |

1. When evaluating whether requests align with the mounted specifications, include the header `X-Wallarm-Schema-ID: <schema_id>` to indicate to API Firewall which specification should be used for validation.

API Firewall validates requests as follows:

* If a request matches the specification, an empty response with a 200 status code is returned.
* If a request does not match the specification, the response will provide a 403 status code and a JSON document explaining the reasons for the mismatch.
* If it is unable to handle or validate a request, an empty response with a 500 status code is returned.
