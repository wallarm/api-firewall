# SSL/TLS Configuration

This guide explains how to set environment variables for configuring SSL/TLS connections between the API Firewall and the protected application, as well as for the API Firewall server itself. Provide these variables when launching the API Firewall Docker container for [REST API](../installation-guides/docker-container.md) or [GraphQL API](../installation-guides/graphql/docker-container.md).

## Secure SSL/TLS connection between API Firewall and the application

To establish a secure connection between the API Firewall and the protected application's server that utilizes custom CA certificates, utilize the following environment variables:

1. Mount the custom CA certificate to the API Firewall container. For example, in your `docker-compose.yaml`, make the following modification:

    ```diff
    ...
        volumes:
          - <HOST_PATH_TO_SPEC>:<CONTAINER_PATH_TO_SPEC>
    +     - <HOST_PATH_TO_CA>:<CONTAINER_PATH_TO_CA>
    ...
    ```
1. Provide the mounted file path using the following environment variables:

| Environment variable | Description |
| -------------------- | ----------- |
| `APIFW_SERVER_ROOT_CA`<br>(only if the `APIFW_SERVER_INSECURE_CONNECTION` value is `false`) | Path inside the Docker container to the protected application server's CA certificate. |

## Insecure connection between API Firewall and the application

To set up an insecure connection (i.e., bypassing SSL/TLS verification) between the API Firewall and the protected application's server, use this environment variable:

| Environment variable | Description |
| -------------------- | ----------- |
| `APIFW_SERVER_INSECURE_CONNECTION` | Determines whether the SSL/TLS certificate validation of the protected application server should be disabled. The server address is denoted in the `APIFW_SERVER_URL` variable. By default (`false`), the system attempts a secure connection using either the default CA certificate or the one specified in `APIFW_SERVER_ROOT_CA`. |

## SSL/TLS for the API Firewall server

To ensure the server running the API Firewall accepts HTTPS connections, follow the steps below:

1. Mount the certificate and private key directory to the API Firewall container. For example, in your `docker-compose.yaml`, make the following modification:

    ```diff
    ...
        volumes:
          - <HOST_PATH_TO_SPEC>:<CONTAINER_PATH_TO_SPEC>
    +     - <HOST_PATH_TO_CERT_DIR>:<CONTAINER_PATH_TO_CERT_DIR>
    ...
    ```
1. Provide mounted file paths using the following environment variables:

| Environment variable | Description |
| -------------------- | ----------- |
| `APIFW_TLS_CERTS_PATH`            | Path in the container to the directory where the certificate and private key for the API Firewall are mounted. |
| `APIFW_TLS_CERT_FILE`             | Filename of the SSL/TLS certificate for the API Firewall, located within the `APIFW_TLS_CERTS_PATH` directory. |
| `APIFW_TLS_CERT_KEY`              | Filename of the SSL/TLS private key for the API Firewall, found in the `APIFW_TLS_CERTS_PATH` directory. |
