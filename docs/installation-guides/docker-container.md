# Running API Firewall on Docker for REST API

This guide walks through downloading, installing, and starting [Wallarm API Firewall](../index.md) on Docker for REST API request validation.

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
    image: wallarm/api-firewall:v0.9.2
    restart: on-failure
    volumes:
      - <HOST_PATH_TO_SPEC>:<CONTAINER_PATH_TO_SPEC>
    environment:
      APIFW_API_SPECS: <PATH_TO_MOUNTED_SPEC>
      APIFW_URL: http://0.0.0.0:8088/
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
      - 80:80
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

The provided **docker-compose.yml** instructs Docker to start the [kennethreitz/httpbin](https://hub.docker.com/r/kennethreitz/httpbin/) Docker container connected to the `api-firewall-network` and assigned with the `backend` [network alias](https://docs.docker.com/config/containers/container-networking/#ip-address-and-hostname). The container port is 80.

If configuring your own application, define only settings required for the correct application container start. No specific configuration for API Firewall is required.

## Step 4. Configure API Firewall

Configure API Firewall as follows:

1. With `services.api-firewall.volumes`, mount the [OpenAPI 3.0 specification](https://swagger.io/specification/) to the API Firewall container directory:
    
    * `<HOST_PATH_TO_SPEC>`: the path to the OpenAPI 3.0 specification for your application REST API located on the host machine. The accepted file formats are YAML and JSON (`.yaml`, `.yml`, `.json` file extensions). For example: `/opt/my-api/openapi3/swagger.json`.
    * `<CONTAINER_PATH_TO_SPEC>`: the path to the container directory to mount the OpenAPI 3.0 specification to. For example: `/api-firewall/resources/swagger.json`.

1. Set the general API Firewall configuration using one of the approaches:

    * With `services.api-firewall.environment`, pass environment variables to **docker-compose.yml** → `services.api-firewall`.
    * With `services.api-firewall.volumes`, mount the [`apifw.yaml`](#apifw-yaml-example) configuation file to the API Firewall container directory.

    !!! info "Priority"
        If both specified, values in `apifw.yaml` have priority over environment variables.

| Environment variable | YAML parameter             | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       | Required? |
|-----------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------|-----------|
| <a name="apifw-api-specs"></a>`APIFW_API_SPECS`                 | `APISpecs` | Path to the OpenAPI 3.0 specification. There are the following ways to specify the path:<ul><li>Path to the specification file mounted to the container, for example: `/api-firewall/resources/swagger.json`. When running the container, mount this file with the `-v <HOST_PATH_TO_SPEC>:<CONTAINER_PATH_TO_SPEC>` option.</li><li>URL address of the specification file, for example: `https://example.com/swagger.json`. When running the container, omit the `-v <HOST_PATH_TO_SPEC>:<CONTAINER_PATH_TO_SPEC>` option.</li></ul>                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | Yes       |
| `APIFW_URL`                       | Server → `APIHost` | URL for API Firewall. For example: `http://0.0.0.0:8088/`. The port value should correspond to the container port published to the host.<br><br>If API Firewall listens to the HTTPS protocol, please mount the generated SSL/TLS certificate and private key to the container, and pass to the container the [API Firewall SSL/TLS settings](../configuration-guides/ssl-tls.md).<br><br>The default value is `http://0.0.0.0:8282/`.                                                                                                                                                                                                                                                   | Yes       |
| `APIFW_SERVER_URL`                | Backend → ProtectedAPI → URL | URL of the application described in the mounted OpenAPI specification that should be protected with API Firewall. For example: `http://backend:80`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 | Yes       |
| <a name="apifw-req-val"></a>`APIFW_REQUEST_VALIDATION`        | `RequestValidation` | API Firewall mode when validating requests sent to the application URL:<ul><li>`BLOCK` to block and log the requests that do not match the schema provided in the mounted OpenAPI 3.0 specification (the `403 Forbidden` response will be returned to the blocked requests). Logs are sent to the [`STDOUT` and `STDERR` Docker services](https://docs.docker.com/config/containers/logging/).</li><li>`LOG_ONLY` to log but not block the requests that do not match the schema provided in the mounted OpenAPI 3.0 specification. Logs are sent to the [`STDOUT` and `STDERR` Docker services](https://docs.docker.com/config/containers/logging/).</li><li>`DISABLE` to disable request validation.<br><br>**Note** that you can [set validation mode for specific endpoints](../configuration-guides/endpoint-related-response.md).</li></ul>                                                                                                                           | Yes       |
| `APIFW_RESPONSE_VALIDATION`       | `ResponseValidation` | API Firewall mode when validating application responses to incoming requests:<ul><li>`BLOCK` to block and log the request if the application response to this request does not match the schema provided in the mounted OpenAPI 3.0 specification. This request will be proxied to the application URL but the client will receive the `403 Forbidden` response. Logs are sent to the [`STDOUT` and `STDERR` Docker services](https://docs.docker.com/config/containers/logging/).</li><li>`LOG_ONLY` to log but not block the request if the application response to this request does not match the schema provided in the mounted OpenAPI 3.0 specification. Logs are sent to the [`STDOUT` and `STDERR` Docker services](https://docs.docker.com/config/containers/logging/).</li><li>`DISABLE` to disable request validation.<br><br>**Note** that you can [set validation mode for specific endpoints](../configuration-guides/endpoint-related-response.md).</li></ul> | Yes       |
| `APIFW_LOG_LEVEL`                 | - | API Firewall logging level. Possible values:<ul><li>`DEBUG` to log events of any type (INFO, ERROR, WARNING, and DEBUG).</li><li>`INFO` to log events of the INFO, WARNING, and ERROR types.</li><li>`WARNING` to log events of the WARNING and ERROR types.</li><li>`ERROR` to log events of only the ERROR type.</li><li>`TRACE` to log incoming requests and API Firewall responses, including their content.</li></ul> The default value is `DEBUG`. Logs on requests and responses that do not match the provided schema have the ERROR type.                                                                                                                                                                                                                                       | No        |
| <a name="apifw-custom-block-status-code"></a>`APIFW_CUSTOM_BLOCK_STATUS_CODE` | `CustomBlockStatusCode` | [HTTP response status code](https://en.wikipedia.org/wiki/List_of_HTTP_status_codes) returned by API Firewall operating in the `BLOCK` mode if the request or response does not match the schema provided in the mounted OpenAPI 3.0 specification. The default value is `403`. | No 
| `APIFW_ADD_VALIDATION_STATUS_HEADER`<br>(EXPERIMENTAL) | `AddValidationStatusHeader` | Whether to return the header `Apifw-Validation-Status` containing the reason for the request blocking in the response to this request. The value can be `true` or `false`. The default value is `false`.| No
| `APIFW_SERVER_DELETE_ACCEPT_ENCODING` | `DeleteAcceptEncoding` | If it is set to `true`, the `Accept-Encoding` header is deleted from proxied requests. The default value is `false`. | No |
| `APIFW_LOG_FORMAT` | - | The format of API Firewall logs. The value can be `TEXT` or `JSON`. The default value is `TEXT`. | No |
| `APIFW_SHADOW_API_EXCLUDE_LIST`<br>(only if API Firewall is operating in the `LOG_ONLY` mode for both the requests and responses) | ShadowAPI → `ExcludeList` | [HTTP response status codes](https://en.wikipedia.org/wiki/List_of_HTTP_status_codes) indicating that the requested API endpoint that is not included in the specification is NOT a shadow one. You can specify several status codes separated by a semicolon (e.g. `404;401`). The default value is `404`.<br><br>By default, API Firewall operating in the `LOG_ONLY` mode for both the requests and responses marks all endpoints that are not included in the specification and are returning the code different from `404` as the shadow ones. | No
| `APIFW_MODE` | `mode` | Sets the general API Firewall mode. Possible values are `PROXY` (default), [`graphql`](graphql/docker-container.md) and [`API`](api-mode.md). | No |
| `APIFW_PASS_OPTIONS` | `PassOptionsRequests` | When set to `true`, the API Firewall allows `OPTIONS` requests to endpoints in the specification, even if the `OPTIONS` method is not described. The default value is `false`. | No |
| `APIFW_SHADOW_API_UNKNOWN_PARAMETERS_DETECTION` | ShadowAPI → `UnknownParametersDetection` | This specifies whether requests are identified as non-matching the specification if their parameters do not align with those defined in the OpenAPI specification. The default value is `true`.<br><br>If running API Firewall in the [`API` mode](api-mode.md), this variable takes on a different name `APIFW_API_MODE_UNKNOWN_PARAMETERS_DETECTION`. | No |
| `APIFW_API_SPECS_CUSTOM_HEADER_NAME` | APISpecsCustomHeader → `Name` | Specifies the custom header name to be added to requests for your OpenAPI specification URL (defined in `APIFW_API_SPECS`). For example, you can specify a header name for authentication data required to access the URL. | No |
| `APIFW_API_SPECS_CUSTOM_HEADER_VALUE` | APISpecsCustomHeader → `Value` | Specifies the custom header value to be added to requests for your OpenAPI specification URL. For example, you can specify authentication data for the custom header defined in `APIFW_API_SPECS_CUSTOM_HEADER_NAME` to access the URL. | No |
| `APIFW_SPECIFICATION_UPDATE_PERIOD` | `SpecificationUpdatePeriod` | Specifies the interval for updating the OpenAPI specification from the hosted URL (defined in `APIFW_API_SPECS`). The default value is `0`, which disables updates and uses the initially downloaded specification. The value format is: `5s`, `1h`, etc. | No |
| `APIFW_MODSEC_CONF_FILES` | ModSecurity → `ConfFiles` | Allows to set the list of [ModSecurity](../migrating/modseс-to-apif.md) configuration files. The delimiter is ;. The default value is [] (empty). Example: `APIFW_MODSEC_CONF_FILES=modsec.conf;crs-setup.conf.example`. | No |
| `APIFW_MODSEC_RULES_DIR` | ModSecurity → `RulesDir` | Allows to set the [ModSecurity](../migrating/modseс-to-apif.md) directory with the rules that should be loaded. The files with the `*.conf` wildcard will be loaded from the directory. The default value is `""`. | No |
| `APIFW_SERVER_REQUEST_HOST_HEADER` | `RequestHostHeader` | Sets a custom `Host` header for requests forwarded to your backend after API Firewall validation. | No |
| `APIFW_MODSEC_REQUEST_VALIDATION` | ModSecurity → `RequestValidation` | Defines how requests to the application URL are validated against the [ModSecurity](../migrating/modseс-to-apif.md) Rule Set.<ul><li>`BLOCK` to block and log the requests that violate the ModSecurity Rule Set (the `403 Forbidden` response will be returned to the blocked requests). Logs are sent to the [`STDOUT` and `STDERR` Docker services](https://docs.docker.com/config/containers/logging/).</li><li>`LOG_ONLY` to log but not block the requests that violate the rule set. Logs are sent to the [`STDOUT` and `STDERR` Docker services](https://docs.docker.com/config/containers/logging/).</li><li>`DISABLE` (default) to disable request validation against the ModSecurity Rule Set.</li></ul>This setting takes priority if used together with `APIFW_REQUEST_VALIDATION`. | No |
| `APIFW_MODSEC_RESPONSE_VALIDATION` | ModSecurity → `ResponseValidation` | Defines how application responses are validated against the [ModSecurity](../migrating/modseс-to-apif.md) Rule Set.<ul><li>`BLOCK` to block and log the corresponding requests (the `403 Forbidden` response will be returned to the blocked requests). Logs are sent to the [`STDOUT` and `STDERR` Docker services](https://docs.docker.com/config/containers/logging/).</li><li>`LOG_ONLY` to log but not block the corresponding requests. Logs are sent to the [`STDOUT` and `STDERR` Docker services](https://docs.docker.com/config/containers/logging/).</li><li>`DISABLE` (default) to disable response validation against the ModSecurity Rule Set.</li></ul>This setting takes priority if used together with `APIFW_RESPONSE_VALIDATION`. | No |

<a name="apifw-yaml-example"></a>
??? info "Example of `apifw.yaml`"
    --8<-- "include/apifw-yaml-example.md"

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
        --network-alias backend -p 80:80 kennethreitz/httpbin
    ```
3. [To start API Firewall](#step-4-configure-api-firewall):

    ```bash
    docker run --rm -it --network api-firewall-network --network-alias api-firewall \
        -v <HOST_PATH_TO_SPEC>:<CONTAINER_PATH_TO_SPEC> -e APIFW_API_SPECS=<PATH_TO_MOUNTED_SPEC> \
        -e APIFW_URL=<API_FIREWALL_URL> -e APIFW_SERVER_URL=<PROTECTED_APP_URL> \
        -e APIFW_REQUEST_VALIDATION=<REQUEST_VALIDATION_MODE> -e APIFW_RESPONSE_VALIDATION=<RESPONSE_VALIDATION_MODE> \
        -p 8088:8088 wallarm/api-firewall:v0.9.2
    ```
4. When the environment is started, test it and enable traffic on API Firewall following steps 6 and 7.
