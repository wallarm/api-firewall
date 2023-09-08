# WebSocket Origin Validation

When a browser initiates a WebSocket connection, it automatically includes an `Origin` header that denotes the domain from which the request originates. With Wallarm API Firewall, you can ensure that the value of the `Origin` header matches your predefined list during the upgrade phase of the WebSocket connection. This article outlines the steps to enable `Origin` validation for [GraphQL queries](docker-container.md).

By default, the WebSocket Origin validation feature is disabled. To activate it, configure the following environment variables:
 
| Environment variable | Description |
| -------------------- | ----------- |
| `APIFW_GRAPHQL_WS_CHECK_ORIGIN` | Enables the validation of the `Origin` header during the WebSocket upgrade phase. Default: `false`. |
| `APIFW_GRAPHQL_WS_ORIGIN` (required if `APIFW_GRAPHQL_WS_CHECK_ORIGIN` is `true`) | The list of allowed origins for WebSocket connections. Origins are separated by `;`. Default value is `""`. |

The `APIFW_GRAPHQL_WS_CHECK_ORIGIN` operates independently of [`APIFW_GRAPHQL_REQUEST_VALIDATION`](docker-container.md#apifw-graphql-request-validation). WebSocket requests with incorrect `Origin` headers will be blocked regardless of the request validation mode.
