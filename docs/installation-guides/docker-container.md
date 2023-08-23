# Running API Firewall on Docker

This guide walks through downloading, installing, and starting [Wallarm API Firewall](../index.md) on Docker.

## Requirements

* [Installed and configured Docker](https://docs.docker.com/get-docker/)
* [OpenAPI 3.0 specification](https://swagger.io/specification/) developed for the REST API of the application that should be protected with Wallarm API Firewall

## Methods to run API Firewall on Docker

The fastest method to deploy API Firewall on Docker is [Docker Compose](https://docs.docker.com/compose/). The steps below rely on using this method.

If required, you can also use `docker run`. We have provided proper `docker run` commands to deploy the same environment in [this section](#using-docker-run-to-start-api-firewall).

## Step 1. Create the `docker-compose.yml` file

To deploy API Firewall and proper environment using Docker Compose, create the **docker-compose.yml** with the following content first:

```yml
version: '3.8'

networks:
  api-firewall-network:
    name: api-firewall-network

services:
  api-firewall:
    container_name: api-firewall
    image: wallarm/api-firewall:v0.6.12
    restart: on-failure
    volumes:
      - <HOST_PATH_TO_SPEC>:<CONTAINER_PATH_TO_SPEC>
    environment:
      APIFW_API_SPECS: <PATH_TO_MOUNTED_SPEC>
      APIFW_URL: <API_FIREWALL_URL>
      APIFW_SERVER_URL: <PROTECTED_APP_URL>
      APIFW_REQUEST_VALIDATION: <REQUEST_VALIDATION_MODE>
      APIFW_RESPONSE_VALIDATION: <RESPONSE_VALIDATION_MODE>
    ports:
      - "8088:8088"
    stop_grace_period: 1s
    networks:
      - api-firewall-network
  backend:
    container_name: api-firewall-backend
    image: kennethreitz/httpbin
    restart: on-failure
    ports:
      - 8090:8090
    stop_grace_period: 1s
    networks:
      - api-firewall-network
```

## Step 2. Configure the Docker network

If required, change the [Docker network](https://docs.docker.com/network/) configuration defined in **docker-compose.yml** → `networks`.

The provided **docker-compose.yml** instructs Docker to create the network `api-firewall-network` and link the application and API Firewall containers to it.

It is recommended to use a separate Docker network to allow the containerized application and API Firewall communication without manual linking.

## Step 3. Configure the application to be protected with API Firewall

Change the configuration of the containerized application to be protected with API Firewall. This configuration is defined in **docker-compose.yml** → `services.backend`.

The provided **docker-compose.yml** instructs Docker to start the [kennethreitz/httpbin](https://hub.docker.com/r/kennethreitz/httpbin/) Docker container connected to the `api-firewall-network` and assigned with the `backend` [network alias](https://docs.docker.com/config/containers/container-networking/#ip-address-and-hostname). The container port is 8090.

If configuring your own application, define only settings required for the correct application container start. No specific configuration for API Firewall is required.

## Step 4. Configure API Firewall

Pass API Firewall configuration in **docker-compose.yml** → `services.api-firewall` as follows:

**With `services.api-firewall.volumes`**, please mount the [OpenAPI 3.0 specification](https://swagger.io/specification/) to the API Firewall container directory:
    
* `<HOST_PATH_TO_SPEC>`: the path to the OpenAPI 3.0 specification for your application REST API located on the host machine. The accepted file formats are YAML and JSON (`.yaml`, `.yml`, `.json` file extensions). For example: `/opt/my-api/openapi3/swagger.json`.
* `<CONTAINER_PATH_TO_SPEC>`: the path to the container directory to mount the OpenAPI 3.0 specification to. For example: `/api-firewall/resources/swagger.json`.

**With `services.api-firewall.environment`**, please set the general API Firewall configuration through the following environment variables:

| Environment variable              | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | Required? |
|-----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------|
| <a name="apifw-api-specs"></a>`APIFW_API_SPECS`                 | Path to the OpenAPI 3.0 specification. There are the following ways to specify the path:<ul><li>Path to the specification file mounted to the container, for example: `/api-firewall/resources/swagger.json`. When running the container, mount this file with the `-v <HOST_PATH_TO_SPEC>:<CONTAINER_PATH_TO_SPEC>` option.</li><li>URL address of the specification file, for example: `https://example.com/swagger.json`. When running the container, omit the `-v <HOST_PATH_TO_SPEC>:<CONTAINER_PATH_TO_SPEC>` option.</li></ul>                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | Yes       |
| `APIFW_URL`                       | URL for API Firewall. For example: `http://0.0.0.0:8088/`. The port value should correspond to the container port published to the host.<br><br>If API Firewall listens to the HTTPS protocol, please mount the generated SSL/TLS certificate and private key to the container, and pass to the container the **API Firewall SSL/TLS settings** described below.                                                                                                                                                                                                                                                   | Yes       |
| `APIFW_SERVER_URL`                | URL of the application described in the mounted OpenAPI specification that should be protected with API Firewall. For example: `http://backend:80`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Yes       |
| `APIFW_REQUEST_VALIDATION`        | API Firewall mode when validating requests sent to the application URL:<ul><li>`BLOCK` to block and log the requests that do not match the schema provided in the mounted OpenAPI 3.0 specification (the `403 Forbidden` response will be returned to the blocked requests). Logs are sent to the [`STDOUT` and `STDERR` Docker services](https://docs.docker.com/config/containers/logging/).</li><li>`LOG_ONLY` to log but not block the requests that do not match the schema provided in the mounted OpenAPI 3.0 specification. Logs are sent to the [`STDOUT` and `STDERR` Docker services](https://docs.docker.com/config/containers/logging/).</li><li>`DISABLE` to disable request validation.</li></ul>                                                                                                                           | Yes       |
| `APIFW_RESPONSE_VALIDATION`       | API Firewall mode when validating application responses to incoming requests:<ul><li>`BLOCK` to block and log the request if the application response to this request does not match the schema provided in the mounted OpenAPI 3.0 specification. This request will be proxied to the application URL but the client will receive the `403 Forbidden` response. Logs are sent to the [`STDOUT` and `STDERR` Docker services](https://docs.docker.com/config/containers/logging/).</li><li>`LOG_ONLY` to log but not block the request if the application response to this request does not match the schema provided in the mounted OpenAPI 3.0 specification. Logs are sent to the [`STDOUT` and `STDERR` Docker services](https://docs.docker.com/config/containers/logging/).</li><li>`DISABLE` to disable request validation.</li></ul> | Yes       |
| `APIFW_LOG_LEVEL`                 | API Firewall logging level. Possible values:<ul><li>`DEBUG` to log events of any type (INFO, ERROR, WARNING, and DEBUG).</li><li>`INFO` to log events of the INFO, WARNING, and ERROR types.</li><li>`WARNING` to log events of the WARNING and ERROR types.</li><li>`ERROR` to log events of only the ERROR type.</li><li>`TRACE` to log incoming requests and API Firewall responses, including their content.</li></ul> The default value is `DEBUG`. Logs on requests and responses that do not match the provided schema have the ERROR type.                                                                                                                                                                                                                                       | No        |
| <a name="apifw-custom-block-status-code"></a>`APIFW_CUSTOM_BLOCK_STATUS_CODE` | [HTTP response status code](https://en.wikipedia.org/wiki/List_of_HTTP_status_codes) returned by API Firewall operating in the `BLOCK` mode if the request or response does not match the schema provided in the mounted OpenAPI 3.0 specification. The default value is `403`. | No 
| `APIFW_ADD_VALIDATION_STATUS_HEADER`<br>(EXPERIMENTAL) | Whether to return the header `Apifw-Validation-Status` containing the reason for the request blocking in the response to this request. The value can be `true` or `false`. The default value is `false`.| No
| `APIFW_SERVER_DELETE_ACCEPT_ENCODING` | If it is set to `true`, the `Accept-Encoding` header is deleted from proxied requests. The default value is `false`. | No |
| `APIFW_LOG_FORMAT` | The format of API Firewall logs. The value can be `TEXT` or `JSON`. The default value is `TEXT`. | No |
| `APIFW_SHADOW_API_EXCLUDE_LIST`<br>(only if API Firewall is operating in the `LOG_ONLY` mode for both the requests and responses) | [HTTP response status codes](https://en.wikipedia.org/wiki/List_of_HTTP_status_codes) indicating that the requested API endpoint that is not included in the specification is NOT a shadow one. You can specify several status codes separated by a semicolon (e.g. `404;401`). The default value is `404`.<br><br>By default, API Firewall operating in the `LOG_ONLY` mode for both the requests and responses marks all endpoints that are not included in the specification and are returning the code different from `404` as the shadow ones. | No
| `APIFW_MODE` | Sets the general API Firewall mode. Possible values are `PROXY` (default) and [`API`](#validating-individual-requests-without-proxying-for-v0612-and-above). | No |
| `APIFW_PASS_OPTIONS` | When set to `true`, the API Firewall allows `OPTIONS` requests to endpoints in the specification, even if the `OPTIONS` method is not described. The default value is `false`. | No |
| `APIFW_SHADOW_API_UNKNOWN_PARAMETERS_DETECTION` | This specifies whether requests are identified as non-matching the specification if their parameters do not align with those defined in the OpenAPI specification. The default value is `true`.<br><br>If running API Firewall in the [`API` mode](#validating-individual-requests-without-proxying-for-v0612-and-above), this variable takes on a different name `APIFW_API_MODE_UNKNOWN_PARAMETERS_DETECTION`. | No |

More API Firewall configuration options are described within the [link](#api-firewall-fine-tuning-options).

**With `services.api-firewall.ports` and `services.api-firewall.networks`**, set the API Firewall container port and connect the container to the created network. The provided **docker-compose.yml** instructs Docker to start API Firewall connected to the `api-firewall-network` [network](https://docs.docker.com/network/) on the port 8088.

## Step 5. Deploy the configured environment

To build and start the configured environment, run the following command:

```bash
docker-compose up -d --force-recreate
```

To check the log output:

```bash
docker-compose logs -f
```

## Step 6. Test API Firewall operation

To test API Firewall operation, send the request that does not match the mounted Open API 3.0 specification to the API Firewall Docker container address. For example, you can pass the string value in the parameter that requires the integer value.

If the request does not match the provided API schema, the appropriate ERROR message will be added to the API Firewall Docker container logs.

## Step 7. Enable traffic on API Firewall

To finalize the API Firewall configuration, please enable incoming traffic on API Firewall by updating your application deployment scheme configuration. For example, this would require updating the Ingress, NGINX, or load balancer settings.

## API Firewall fine-tuning options

To address more business issues by API Firewall, you can fine-tune the tool operation. Supported fine-tuning options are listed below. Please pass them as environment variables when [configuring the API Firewall Docker container](#step-4-configure-api-firewall).

### Validation of request authentication tokens

If using OAuth 2.0 protocol-based authentication, you can configure API Firewall to validate the access tokens before proxying requests to the application's server. API Firewall expects the access token to be passed in the `Authorization: Bearer` request header.

API Firewall considers the token to be valid if the scopes defined in the [specification](https://swagger.io/docs/specification/authentication/oauth2/) and in the token meta information are the same. If the value of `APIFW_REQUEST_VALIDATION` is `BLOCK`, API Firewall blocks requests with invalid tokens. In the `LOG_ONLY` mode, requests with invalid tokens are only logged.

To configure the OAuth 2.0 token validation flow, use the following optional environment variables:

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

### Blocking requests with compromised authentication tokens

If an API leak is detected, the Wallarm API Firewall is able to [stop the compromised authentication tokens from being used](https://lab.wallarm.com/oss-api-firewall-unveils-new-feature-blacklist-for-compromised-api-tokens-and-cookies/). If the request contains compromised tokens, API Firewall responses to this request with the code configured via [`APIFW_CUSTOM_BLOCK_STATUS_CODE`](#apifw-custom-block-status-code).

To enable the denylist feature:

1. Mount the denylist file with compromised tokens into the Docker container. The denylist text file may look as follows:

    ```txt
    eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzb21lIjoicGF5bG9hZDk5OTk5ODIifQ.CUq8iJ_LUzQMfDTvArpz6jUyK0Qyn7jZ9WCqE0xKTCA
    eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzb21lIjoicGF5bG9hZDk5OTk5ODMifQ.BinZ4AcJp_SQz-iFfgKOKPz_jWjEgiVTb9cS8PP4BI0
    eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzb21lIjoicGF5bG9hZDk5OTk5ODQifQ.j5Iea7KGm7GqjMGBuEZc2akTIoByUaQc5SSX7w_qjY8
    ```
2. Configure the denylist feature passing the following variables to the Docker container:

    | Environment variable | Description |
    | -------------------- | ----------- |
    | `APIFW_DENYLIST_TOKENS_FILE` | The path to the text denylist file mounted to the container. The tokens in the file must be separated by newlines. Example value: `/api-firewall/resources/tokens-denylist.txt`. |
    | `APIFW_DENYLIST_TOKENS_COOKIE_NAME` | The name of the Cookie used to pass the authentication token. |
    | `APIFW_DENYLIST_TOKENS_HEADER_NAME` | The name of the Header used to pass the authentication token. If both the `APIFW_DENYLIST_TOKENS_COOKIE_NAME` and `APIFW_DENYLIST_TOKENS_HEADER_NAME` variables are specified, API Firewall sequentially checks its values. |
    | `APIFW_DENYLIST_TOKENS_TRIM_BEARER_PREFIX` | Whether to trim the `Bearer` prefix from the authentication header when comparing its value with the denylist contents. If the `Bearer` prefix is passed in the authentication header and tokens in the denylist do not contain this prefix, tokens will not be validated reliably.<br>The value can be `true` or `false`. The default value is `false`. |

### Protected application SSL/TLS settings

To facilitate the connection between API Firewall and the protected application's server signed with the custom CA certificates or insecure connection, use the following optional environment variables:

| Environment variable | Description |
| -------------------- | ----------- |
| `APIFW_SERVER_INSECURE_CONNECTION` | Whether to disable validation of the SSL/TLS certificate of the protected application server. The server address is specified in the variable `APIFW_SERVER_URL`.<br><br>The default value is `false` - all connections to the application are attempted to be made secure by using the CA certificate installed by default or the one specified in `APIFW_SERVER_ROOT_CA`. |
| `APIFW_SERVER_ROOT_CA`<br>(only if the `APIFW_SERVER_INSECURE_CONNECTION` value is `false`) | The path to the protected application server's CA certificate in the Docker container. The CA certificate must be mounted to the API Firewall Docker container first. |

### API Firewall SSL/TLS settings

To set up SSL/TLS for the server with the running API Firewall, use the following optional environment variables:

| Environment variable | Description |
| -------------------- | ----------- |
| `APIFW_TLS_CERTS_PATH`            | The path to the container directory with the mounted certificate and private key generated for API Firewall.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
| `APIFW_TLS_CERT_FILE`             | The name of the file with the SSL/TLS certificate generated for API Firewall and located in the directory specified in `APIFW_TLS_CERTS_PATH`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| `APIFW_TLS_CERT_KEY`              | The name of the file with the SSL/TLS private key generated for API Firewall and located in the directory specified in `APIFW_TLS_CERTS_PATH`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |

### Validating individual requests without proxying (for v0.6.12 and above)

If you need to validate individual API requests based on a given OpenAPI specification without further proxying, you can utilize Wallarm API Firewall in a non-proxy mode. In this case, the solution does not validate responses.

To do so:

1. Instead of mounting the specification file to the container, mount the [SQLite database](https://www.sqlite.org/index.html) containing one or more OpenAPI 3.0 specifications to `/var/lib/wallarm-api/1/wallarm_api.db`. The database should adhere to the following schema:

    * `schema_id`, integer (auto-increment) - ID of the specification.
    * `schema_version`, string - Specification version. You can assign any preferred version. When this field changes, API Firewall assumes the specification itself has changed and updates it accordingly.
    * `schema_format`, string - The specification format, can be `json` or `yaml`.
    * `schema_content`, string - The specification content.
1. Run the container with the environment variable `APIFW_MODE=API` and if needed, with other variables that specifically designed for this mode:

    | Environment variable | Description |
    | -------------------- | ----------- |
    | `APIFW_MODE` | Sets the general API Firewall mode. Possible values are `PROXY` (default) and `API`. |
    | `APIFW_SPECIFICATION_UPDATE_PERIOD` | Determines the frequency of specification updates. If set to `0`, the specification update is disabled. The default value is `1m` (1 minute). |
    | `APIFW_API_MODE_UNKNOWN_PARAMETERS_DETECTION` | Specifies whether to return an error code if the request parameters do not match those defined in the the specification. The default value is `true`. |
    | `APIFW_PASS_OPTIONS` | When set to `true`, the API Firewall allows `OPTIONS` requests to endpoints in the specification, even if the `OPTIONS` method is not described. The default value is `false`. |

1. When evaluating whether requests align with the mounted specifications, include the header `X-Wallarm-Schema-ID: <schema_id>` to indicate to API Firewall which specification should be used for validation.

API Firewall validates requests as follows:

* If a request matches the specification, an empty response with a 200 status code is returned.
* If a request does not match the specification, the response will provide a 403 status code and a JSON document explaining the reasons for the mismatch.
* If it is unable to handle or validate a request, an empty response with a 500 status code is returned.

### System settings

To fine-tune system API Firewall settings, use the following optional environment variables:

| Environment variable | Description |
| -------------------- | ----------- |
| `APIFW_READ_TIMEOUT`              | The timeout for API Firewall to read the full request (including the body) sent to the application URL. The default value is `5s`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| `APIFW_WRITE_TIMEOUT`             | The timeout for API Firewall to return the response to the request sent to the application URL. The default value is `5s`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `APIFW_SERVER_MAX_CONNS_PER_HOST` | The maximum number of connections that API Firewall can handle simultaneously. The default value is `512`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `APIFW_SERVER_READ_TIMEOUT`       | The timeout for API Firewall to read the full response (including the body) returned to the request by the application. The default value is `5s`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| `APIFW_SERVER_WRITE_TIMEOUT`      | The timeout for API Firewall to write the full request (including the body) to the application. The default value is `5s`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| `APIFW_SERVER_DIAL_TIMEOUT`       | The timeout for API Firewall to connect to the application. The default value is `200ms`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `APIFW_SERVER_CLIENT_POOL_CAPACITY`       | Maximum number of the fasthttp clients. The default value is `1000`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `APIFW_HEALTH_HOST`       | The host of the health check service. The default value is `0.0.0.0:9667`. The liveness probe service path is `/v1/liveness` and the readiness service path is `/v1/readiness`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |

## Stopping the deployed environment

To stop the environment deployed using Docker Compose, run the following command:

```bash
docker-compose down
```

## Using `docker run` to start API Firewall

To start API Firewall on Docker, you can also use regular Docker commands as in the examples below:

1. [To create a separate Docker network](#step-2-configure-the-docker-network) to allow the containerized application and API Firewall communication without manual linking:

    ```bash
    docker network create api-firewall-network
    ```
2. [To start the containerized application](#step-3-configure-the-application-to-be-protected-with-api-firewall) to be protected with API Firewall:

    ```bash
    docker run --rm -it --network api-firewall-network \
        --network-alias backend -p 8090:8090 kennethreitz/httpbin
    ```
3. [To start API Firewall](#step-4-configure-api-firewall):

    ```bash
    docker run --rm -it --network api-firewall-network --network-alias api-firewall \
        -v <HOST_PATH_TO_SPEC>:<CONTAINER_PATH_TO_SPEC> -e APIFW_API_SPECS=<PATH_TO_MOUNTED_SPEC> \
        -e APIFW_URL=<API_FIREWALL_URL> -e APIFW_SERVER_URL=<PROTECTED_APP_URL> \
        -e APIFW_REQUEST_VALIDATION=<REQUEST_VALIDATION_MODE> -e APIFW_RESPONSE_VALIDATION=<RESPONSE_VALIDATION_MODE> \
        -p 8088:8088 wallarm/api-firewall:v0.6.12
    ```
4. When the environment is started, test it and enable traffic on API Firewall following steps 6 and 7.
