# Running API Firewall on Docker for GraphQL API

This guide walks through downloading, installing, and starting [Wallarm API Firewall](../../index.md) on Docker for GraphQL API request validation. In GraphQL mode, the API Firewall acts as a proxy, forwarding GraphQL requests from users to the backend server using either HTTP or the WebSocket (`graphql-ws`) protocols. Before the backend execution, the firewall checks the query complexity, depth, and node count of the GraphQL query.

The API Firewall does not validate GraphQL query responses.

## Requirements

* [Installed and configured Docker](https://docs.docker.com/get-docker/)
* [GraphQL specification](http://spec.graphql.org/October2021/) developed for the GraphQL API of the application that should be protected with Wallarm API Firewall

## Methods to run API Firewall on Docker

The fastest method to deploy API Firewall on Docker is [Docker Compose](https://docs.docker.com/compose/). The steps below rely on using this method.

If required, you can also use `docker run`. We have provided proper `docker run` commands to deploy the same environment in [this section](#using-docker-run-to-start-api-firewall).

## Step 1. Create the `docker-compose.yml` file

To deploy API Firewall and proper environment using Docker Compose, create the **docker-compose.yml** with the following content first. In the further steps, you will change this template.

```yml
version: '3.8'

networks:
  api-firewall-network:
    name: api-firewall-network

services:
  api-firewall:
    container_name: api-firewall
    image: wallarm/api-firewall:v0.8.0
    restart: on-failure
    volumes:
      - <HOST_PATH_TO_SPEC>:<CONTAINER_PATH_TO_SPEC>
    environment:
      APIFW_MODE: graphql
      APIFW_GRAPHQL_SCHEMA: <PATH_TO_MOUNTED_SPEC>
      APIFW_URL: <API_FIREWALL_URL>
      APIFW_SERVER_URL: <PROTECTED_APP_URL>
      APIFW_GRAPHQL_REQUEST_VALIDATION: <REQUEST_VALIDATION_MODE>
      APIFW_GRAPHQL_MAX_QUERY_COMPLEXITY: <MAX_QUERY_COMPLEXITY>
      APIFW_GRAPHQL_MAX_QUERY_DEPTH: <MAX_QUERY_DEPTH>
      APIFW_GRAPHQL_NODE_COUNT_LIMIT: <NODE_COUNT_LIMIT>
      APIFW_GRAPHQL_INTROSPECTION: <ALLOW_INTROSPECTION_OR_NOT>
    ports:
      - "8088:8088"
    stop_grace_period: 1s
    networks:
      - api-firewall-network
  backend:
    container_name: api-firewall-backend
    image: <IMAGE_WITH_GRAPHQL_APP>
    restart: on-failure
    ports:
      - <HOST_PORT>:<CONTAINER_PORT>
    stop_grace_period: 1s
    networks:
      - api-firewall-network
```

## Step 2. Configure the Docker network

If required, change the [Docker network](https://docs.docker.com/network/) configuration defined in **docker-compose.yml** → `networks`.

The provided **docker-compose.yml** instructs Docker to create the network `api-firewall-network` and link the application and API Firewall containers to it.

It is recommended to use a separate Docker network for protected contanerized application and API Firewall to allow their communication without manual linking.

## Step 3. Configure the application to be protected with API Firewall

Change the configuration of the containerized application to be protected with API Firewall. This configuration is defined in **docker-compose.yml** → `services.backend`.

The template instructs Docker to boot the specified application Docker container, connecting it to the `api-firewall-network` and designating the `backend` [network alias](https://docs.docker.com/config/containers/container-networking/#ip-address-and-hostname). You can define the port as per your requirements.

When setting up your application, include only the necessary settings for a successful container launch. No special API Firewall configuration is required.

## Step 4. Configure API Firewall

Pass API Firewall configuration in **docker-compose.yml** → `services.api-firewall` as follows:

**With `services.api-firewall.volumes`**, mount the [GraphQL specification](http://spec.graphql.org/October2021/) to the API Firewall container directory:
    
* `<HOST_PATH_TO_SPEC>`: the path to the GraphQL specification for your API located on the host machine. The file format does not matter but usually it is `.graphql` or `gql`. For example: `/opt/my-api/graphql/schema.graphql`.
* `<CONTAINER_PATH_TO_SPEC>`: the path to the container directory to mount the GraphQL specification to. For example: `/api-firewall/resources/schema.graphql`.

**With `services.api-firewall.environment`**, please set the general API Firewall configuration through the following environment variables:

| Environment variable | Description | Required? |
| -------------------- | ----------- | --------- |
| `APIFW_MODE` | Sets the general API Firewall mode. Possible values are [`PROXY`](../docker-container.md) (default), `graphql` and [`API`](../api-mode.md). | No |
| <a name="apifw-api-specs"></a>`APIFW_GRAPHQL_SCHEMA` | Path to the GraphQL specification file mounted to the container, for example: `/api-firewall/resources/schema.graphql`. | Yes |
| `APIFW_URL` | URL for API Firewall. For example: `http://0.0.0.0:8088/`. The port value should correspond to the container port published to the host.<br><br>If API Firewall listens to the HTTPS protocol, please mount the generated SSL/TLS certificate and private key to the container, and pass to the container the **API Firewall SSL/TLS settings** described below. | Yes |
| `APIFW_SERVER_URL` | URL of the application described in the mounted specification that should be protected with API Firewall. For example: `http://backend:80`. | Yes |
| <a name="apifw-graphql-request-validation"></a>`APIFW_GRAPHQL_REQUEST_VALIDATION` | API Firewall mode when validating requests sent to the application URL:<ul><li>`BLOCK` blocks and logs requests not matching the mounted GraphQL schema, returning a `403 Forbidden`. Logs are sent to the [`STDOUT` and `STDERR` Docker services](https://docs.docker.com/config/containers/logging/).</li><li>`LOG_ONLY` logs (but does not block) mismatched requests.</li><li>`DISABLE` turns off request validation.</li></ul>This variable impacts all other parameters, except [`APIFW_GRAPHQL_WS_CHECK_ORIGIN`](websocket-origin-check.md). For instance, if `APIFW_GRAPHQL_INTROSPECTION` is `false` and the mode is `LOG_ONLY`, introspection requests reach the backend server, but API Firewall generates a corresponding error log. | Yes |
| `APIFW_GRAPHQL_MAX_QUERY_COMPLEXITY` | [Defines](limit-compliance.md) the maximum number of Node requests that might be needed to execute the query. Setting it to `0` disables the complexity check. The default value is `0`. | Yes |
| `APIFW_GRAPHQL_MAX_QUERY_DEPTH` | [Specifies](limit-compliance.md) the maximum permitted depth of a GraphQL query. A value of `0` means the query depth check is skipped. | Yes |
| `APIFW_GRAPHQL_NODE_COUNT_LIMIT` | [Sets](limit-compliance.md) the upper limit for the node count in a query. When set to `0`, the node count limit check is skipped. | Yes |
| `APIFW_GRAPHQL_MAX_ALIASES_NUM` | Sets a limit on the number of aliases that can be used in a GraphQL document. If this variable is set to `0`, it implies that there is no limit on the number of aliases that can be used. | Yes |
| <a name="apifw-graphql-introspection"></a>`APIFW_GRAPHQL_INTROSPECTION` | Allows introspection queries, which disclose the layout of your GraphQL schema. When set to `true`, these queries are permitted. | Yes |
| `APIFW_GRAPHQL_FIELD_DUPLICATION` | Defines whether to allow or prevent the duplication of fields in a GraphQL document. The default value is `false` (prevent). | No |
| `APIFW_GRAPHQL_BATCH_QUERY_LIMIT` | Sets a limit on the number of queries that can be batched together in a single GraphQL request. If this variable is set to `0`, it implies that there is no limit on the number of batched queries. | No |
| `APIFW_LOG_LEVEL` | API Firewall logging level. Possible values:<ul><li>`DEBUG` to log events of any type (INFO, ERROR, WARNING, and DEBUG).</li><li>`INFO` to log events of the INFO, WARNING, and ERROR types.</li><li>`WARNING` to log events of the WARNING and ERROR types.</li><li>`ERROR` to log events of only the ERROR type.</li><li>`TRACE` to log incoming requests and API Firewall responses, including their content.</li></ul> The default value is `DEBUG`. Logs on requests and responses that do not match the provided schema have the ERROR type. | No |
| `APIFW_SERVER_DELETE_ACCEPT_ENCODING` | If it is set to `true`, the `Accept-Encoding` header is deleted from proxied requests. The default value is `false`. | No |
| `APIFW_LOG_FORMAT` | The format of API Firewall logs. The value can be `TEXT` or `JSON`. The default value is `TEXT`. | No |
| `APIFW_SERVER_REQUEST_HOST_HEADER` | Sets a custom `Host` header for requests forwarded to your backend after API Firewall validation. | No |

**With `services.api-firewall.ports` and `services.api-firewall.networks`**, set the API Firewall container port and connect the container to the created network.

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

To test API Firewall operation, send the request that does not match the mounted GraphQL specification to the API Firewall Docker container address.

With `APIFW_GRAPHQL_REQUEST_VALIDATION` set to `BLOCK`, the firewall works as follows:

* If the API Firewall allows the request, it proxies the request to the backend server. 
* If the API Firewall cannot parse the request, it responds with the GraphQL error with a 500 status code.
* If the validation fails by the API Firewall, it does not proxy the request to the backend server but responds to the client with 200 status code and GraphQL error in response. 

If the request does not match the provided API schema:

* The API Firewall returns the following response:

    ```json
    {
      "errors": [
        {
          "message":"invalid query"
        }
      ]
    }
    ```

* The appropriate ERROR message is added to the API Firewall Docker container logs, e.g. in the JSON format:

    ```json
    {
      "errors": [
        {
          "message": "field: name not defined on type: Query",
          "path": [
            "query",
            "name"
          ]
        }
      ]
    }
    ```

In scenarios where multiple fields in the request are invalid, only a singular error message will be generated.

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
2. [Start the containerized application](#step-3-configure-the-application-to-be-protected-with-api-firewall) to be protected with API Firewall:

    ```bash
    docker run --rm -it --network api-firewall-network \
        --network-alias backend -p <HOST_PORT>:<CONTAINER_PORT> <IMAGE_WITH_GRAPHQL_APP>
    ```
3. [To start API Firewall](#step-4-configure-api-firewall):

    ```bash
    docker run --rm -it --network api-firewall-network --network-alias api-firewall \
        -v <HOST_PATH_TO_SPEC>:<CONTAINER_PATH_TO_SPEC> -e APIFW_MODE=graphql \
        -e APIFW_GRAPHQL_SCHEMA=<PATH_TO_MOUNTED_SPEC> -e APIFW_URL=<API_FIREWALL_URL> \
        -e APIFW_SERVER_URL=<PROTECTED_APP_URL> -e APIFW_GRAPHQL_REQUEST_VALIDATION=<REQUEST_VALIDATION_MODE> \
        -e APIFW_GRAPHQL_MAX_QUERY_COMPLEXITY=<MAX_QUERY_COMPLEXITY> \
        -e APIFW_GRAPHQL_MAX_QUERY_DEPTH=<MAX_QUERY_DEPTH> -e APIFW_GRAPHQL_NODE_COUNT_LIMIT=<NODE_COUNT_LIMIT> \
        -e APIFW_GRAPHQL_INTROSPECTION=<ALLOW_INTROSPECTION_OR_NOT> \
        -p 8088:8088 wallarm/api-firewall:v0.8.0
    ```
4. When the environment is started, test it and enable traffic on API Firewall following steps 6 and 7.
