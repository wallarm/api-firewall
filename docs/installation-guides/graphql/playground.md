# GraphQL Playground in API Firewall

Wallarm API Firewall equips developers with the [GraphQL Playground](https://github.com/graphql/graphql-playground). This guide explains how to run the playground.

GraphQL Playground is an in-browser Integrated Development Environment (IDE) specifically for GraphQL. It is designed as a visual platform where developers can effortlessly write, examine, and delve into the myriad possibilities of GraphQL queries, mutations, and subscriptions.

The playground automatically fetches the schema from the URL set in `APIFW_SERVER_URL`. This action is an introspection query that discloses the GraphQL schema. Therefore, it is required to ensure the `APIFW_GRAPHQL_INTROSPECTION` variable is set to `true`. Doing so permits this process, averting potential errors in the API Firewall logs.

To activate the Playground within the API Firewall, you need to use the following environment variables:

| Environment variable | Description |
| -------------------- | ----------- |
| `APIFW_GRAPHQL_INTROSPECTION` | Allows introspection queries, which disclose the layout of your GraphQL schema. Ensure this variable is set to `true`. |
| `APIFW_GRAPHQL_PLAYGROUND` | Toggles the playground feature. By default, it is set to `false`. To enable, change to `true`. |
| `APIFW_GRAPHQL_PLAYGROUND_PATH` | Designates the path where the playground will be accessible. By default, it is the root path `/`. |

Once set up, you can access the playground interface from the designated path in your browser:

![Playground](https://github.com/wallarm/api-firewall/blob/main/images/graphql-playground.png?raw=true)
