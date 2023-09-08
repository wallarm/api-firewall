# GraphQL Playground in API Firewall

Wallarm API Firewall equips developers with the [GraphQL Playground](https://github.com/graphql/graphql-playground). This guide explains how to run the playground.

GraphQL Playground is an in-browser Integrated Development Environment (IDE) specifically for GraphQL. It is designed as a visual platform where developers can effortlessly write, examine, and delve into the myriad possibilities of GraphQL queries, mutations, and subscriptions.

It automatically gets the schema from the URL specified in `APIFW_SERVER_URL`. You can then easily construct queries using this intuitive tool. The GraphQL Playground UI facilitates a semi-automatic query-building process.

To activate the Playground within the API Firewall, you need to use the following environment variables:

| Environment variable | Description |
| -------------------- | ----------- |
| `APIFW_GRAPHQL_PLAYGROUND` | Toggles the playground feature. By default, it is set to `false`. To enable, change to `true`. |
| `APIFW_GRAPHQL_PLAYGROUND_PATH` | Designates the path where the playground will be accessible. By default, it is the root path `/`. |

Once set up, you can access the playground interface from the designated path in your browser:

![Playground](https://github.com/wallarm/api-firewall/blob/main/images/graphql-playground.png?raw=true)
