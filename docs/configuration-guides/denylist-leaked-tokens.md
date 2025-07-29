# Blocking Requests with Compromised Tokens

The Wallarm API Firewall provides a feature to prevent the use of leaked authentication tokens. This guide outlines how to enable this feature using the API Firewall Docker container for either [REST API](../installation-guides/docker-container.md) or [GraphQL API](../installation-guides/graphql/docker-container.md).

This capability relies on your supplied data regarding compromised tokens. To activate it, mount a .txt file containing these tokens to the firewall Docker container, then set the corresponding environment variable. For an in-depth look into this feature, read our [blog post](https://lab.wallarm.com/oss-api-firewall-unveils-new-feature-blacklist-for-compromised-api-tokens-and-cookies/).

For REST API, should any of the flagged tokens surface in a request, the API Firewall will respond using the status code specified in the [`APIFW_CUSTOM_BLOCK_STATUS_CODE`](../installation-guides/docker-container.md#apifw-custom-block-status-code) environment variable. For GraphQL API, any request containing a flagged token will be blocked, even if it aligns with the mounted schema. 

To enable the denylist feature:

1. Draft a `.txt` file with the compromised tokens. Each token should be on a new line. Here is an example:

    ```txt
    eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzb21lIjoicGF5bG9hZDk5OTk5ODIifQ.CUq8iJ_LUzQMfDTvArpz6jUyK0Qyn7jZ9WCqE0xKTCA
    eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzb21lIjoicGF5bG9hZDk5OTk5ODMifQ.BinZ4AcJp_SQz-iFfgKOKPz_jWjEgiVTb9cS8PP4BI0
    eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzb21lIjoicGF5bG9hZDk5OTk5ODQifQ.j5Iea7KGm7GqjMGBuEZc2akTIoByUaQc5SSX7w_qjY8
    ```
1. Mount the denylist file to the firewall Docker container. For example, in your `docker-compose.yaml`, make the following modification:

    ```diff
    ...
        volumes:
          - <HOST_PATH_TO_SPEC>:<CONTAINER_PATH_TO_SPEC>
    +     - <HOST_PATH_TO_LEAKED_TOKEN_FILE>:<CONTAINER_PATH_TO_LEAKED_TOKEN_FILE>
    ...
    ```
1. Input the following environment variables when initiating the Docker container:

| Environment variable | Description |
| -------------------- | ----------- |
| `APIFW_DENYLIST_TOKENS_FILE` | Path in the container to the mounted denylist file. Example: `/auth-data/tokens-denylist.txt`. |
| `APIFW_DENYLIST_TOKENS_COOKIE_NAME` | Name of the Cookie that carries the authentication token. |
| `APIFW_DENYLIST_TOKENS_HEADER_NAME` | Name of the Header transmitting the authentication token. If both the `APIFW_DENYLIST_TOKENS_COOKIE_NAME` and `APIFW_DENYLIST_TOKENS_HEADER_NAME` are specified, the API Firewall checks both in sequence. |
| `APIFW_DENYLIST_TOKENS_TRIM_BEARER_PREFIX` | Indicates if the `Bearer` prefix should be removed from the authentication header during comparison with the denylist. If tokens in the denylist do not have this prefix, but the authentication header does, the tokens might not be matched correctly. Accepts `true` or `false` (default). |
