# Allowlisting IPs

The Wallarm API Firewall enables secure access to your backend by allowing requests exclusively from predefined IP addresses. This document provides a step-by-step guide on how to implement IP allowlisting, applicable for the [REST API in the `PROXY` mode](../installation-guides/docker-container.md) or [GraphQL API](../installation-guides/graphql/docker-container.md).

This feature ensures that only requests from allowlisted IP addresses are validated against the OpenAPI specification 3.0. Requests from non-allowlisted IPs are outright rejected, returning a 403 error code, regardless of their compliance with the OpenAPI specification.

To allowlist IP addresses:

1. Prepare a file listing the IP addresses you wish to allowlist. The file format is flexible (e.g., `.txt` or `.db`), with each IP address on a separate line. For instance:

    ```
    1.1.1.1
    2.2.2.2
    ```

    !!! info "Allowlist validation and supported data formats"
        The API Firewall does not perform content validation on the allowlist file during download.

        It supports both IPv4 and IPv6 addresses, but does not support subnets.
1. Mount the allowlist file to the API Firewall Docker container using the `-v` Docker option.
1. Run the API Firewall container with the `APIFW_ALLOW_IP_FILE` environment variable indicating the path to the mounted allowlist file inside the container.
1. (Optional) Pass to the container the `APIFW_ALLOW_IP_HEADER_NAME` environment variable with the name of the request header that carries the origin IP address, if necessary. By default, `connection.remoteAddress` is used (the variable value is empty).

Example `docker run` command:

```
docker run --rm -it --network api-firewall-network --network-alias api-firewall \
    -v <HOST_PATH_TO_SPEC>:<CONTAINER_PATH_TO_SPEC> -e APIFW_API_SPECS=<PATH_TO_MOUNTED_SPEC> \
    -v ./ip-allowlist.txt:/opt/ip-allowlist.txt \
    -e APIFW_URL=<API_FIREWALL_URL> -e APIFW_SERVER_URL=<PROTECTED_APP_URL> \
    -e APIFW_REQUEST_VALIDATION=<REQUEST_VALIDATION_MODE> -e APIFW_RESPONSE_VALIDATION=<RESPONSE_VALIDATION_MODE> \
    -e APIFW_ALLOW_IP_FILE=/opt/ip-allowlist.txt -e APIFW_ALLOW_IP_HEADER_NAME="X-Real-IP" \
    -p 8088:8088 wallarm/api-firewall:v0.6.16
```

| Environment variable | Description |
| -------------------- | ----------- |
| `APIFW_ALLOW_IP_FILE` | Specifies the container path to the mounted file with allowlisted IP addresses (e.g., `/opt/ip-allowlist.txt`). |
| `APIFW_ALLOW_IP_HEADER_NAME` | Defines the request header name that contains the origin IP address. The defauls value is `""` that points to using `connection.remoteAddress`. |
