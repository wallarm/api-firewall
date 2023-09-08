# GraphQL Limits Compliance

You can configure the API Firewall to validate incoming GraphQL queries against predefined query constraints. By adhering to these limits, you can shield your GraphQL API from malicious queries, including potential DoS attacks. This guide explains how the firewall calculates query attributes like node requests, query depth, and complexity before aligning them with your set parameters.

When [running](docker-container.md) the API Firewall Docker container for a GraphQL API, you set limits using the following environment variables:

| Environment variable | Description |
| -------------------- | ----------- |
| `APIFW_GRAPHQL_MAX_QUERY_COMPLEXITY` | Defines the maximum number of Node requests that might be needed to execute the query. Setting it to `0` disables the complexity check. |
| `APIFW_GRAPHQL_MAX_QUERY_DEPTH` | Specifies the maximum permitted depth of a GraphQL query. A value of `0` means the query depth check is skipped. |
| `APIFW_GRAPHQL_NODE_COUNT_LIMIT` | Sets the upper limit for the node count in a query. When set to `0`, the node count limit check is skipped. | 

## How limit calculation works

API Firewall leverages the [wundergraph/graphql-go-tools](https://github.com/wundergraph/graphql-go-tools) library, which adopts algorithms similar to those used by GitHub for calculating GraphQL query complexity. Central to this is the `OperationComplexityEstimator` function, which processes a schema definition and a query, iteratively examining the query to get both its complexity and depth.

You can fine-tune this calculation by integrating integer arguments on fields that signify the number of Nodes a field returns:

* `directive @nodeCountMultiply on ARGUMENT_DEFINITION`

    Indicates that the Int value the directive is applied on should be used as a Node multiplier.
* `directive @nodeCountSkip on FIELD`
    Indicates that the algorithm should skip this Node. This is useful to whitelist certain query paths, e.g. for introspection.

For documents with multiple queries, the calculated complexity, depth, and node count apply to the whole document, not just the single query being run.

## Calculation examples

Below there are a few examples which will grant a clearer perspective on the calculations. They are based on the following GraphQL schema:

```
type User {
    name: String!
    messages(first: Int! @nodeCountMultiply): [Message]
}

type Message {
    id: ID!
    text: String!
    createdBy: String!
    createdAt: Time!
}

type Query {
        __schema: __Schema! @nodeCountSkip
    users(first: Int! @nodeCountMultiply): [User]
    messages(first: Int! @nodeCountMultiply): [Message]
}

type Mutation {
    post(text: String!, username: String!, roomName: String!): Message!
}

type Subscription {
    messageAdded(roomName: String!): Message!
}

scalar Time

directive @nodeCountMultiply on ARGUMENT_DEFINITION
directive @nodeCountSkip on FIELD
```

The depth always represents the nesting levels of fields. For instance, the query below has a depth of 3:

```
{
    a {
        b {
            c
        }
    }
}
```

### Example 1

```
query {
  users(first: 10) {
    name
    messages(first:100) {
      id
      text
    }
  }
}
```

* NodeCount = {int} 1010

    ```
    Node count = 10 [users(first: 10)] + 10*100 [messages(first:100)] = 1010
    ```

* Complexity = {int} 11
    
    ```
    Complexity = 1 [users(first: 10)] + 10 [messages(first:100)] = 11
    ```

* Depth = {int} 3

### Example 2

```
query {
  users(first: 10) {
    name
  }
}
```

* NodeCount = {int} 10

    ```
    Node count = 10 [users(first: 10)] = 10
    ```

* Complexity = {int} 1

    ```
    Complexity = 1 [users(first: 10)] = 1
    ```
* Depth = {int} 2

### Example 3

```
query {
  message(id:1) {
    id
    text
  }
}
```

* NodeCount = {int} 1

    ```
    Node count = 1 [message(fid:1)] = 1
    ```

* Complexity = {int} 1

    ```
    Complexity = 1 [messages(first:1)] = 1
    ```

* Depth = {int} 2

### Example 4

```
query {
  users(first: 10) {
    name
    messages(first:1) {
      id
      text
    }
  }
}
```

* NodeCount = {int} 20

    ```
    Node count = 10 [users(first: 10)] + 10*1 [messages(first:1)] = 20
    ```

* Complexity = {int} 11

    ```
    Complexity = 1 [users(first: 10)] + 10 [messages(first:1)] = 11
    ```

* Depth = {int} 3

### Example 5 (introspection query)

```
query IntrospectionQuery {
  __schema {
    queryType {
      name
    }
    mutationType {
      name
    }
    subscriptionType {
      name
    }
    types {
      ...FullType
    }
    directives {
      name
      description
      locations
      args {
        ...InputValue
      }
    }
  }
}

fragment FullType on __Type {
  kind
  name
  description
  fields(includeDeprecated: true) {
    name
    description
    args {
      ...InputValue
    }
    type {
      ...TypeRef
    }
    isDeprecated
    deprecationReason
  }
  inputFields {
    ...InputValue
  }
  interfaces {
    ...TypeRef
  }
  enumValues(includeDeprecated: true) {
    name
    description
    isDeprecated
    deprecationReason
  }
  possibleTypes {
    ...TypeRef
  }
}

fragment InputValue on __InputValue {
  name
  description
  type {
    ...TypeRef
  }
  defaultValue
}

fragment TypeRef on __Type {
  kind
  name
  ofType {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
        ofType {
          kind
          name
          ofType {
            kind
            name
            ofType {
              kind
              name
              ofType {
                kind
                name
              }
            }
          }
        }
      }
    }
  }
}
```

* NodeCount = {int} 0
* Complexity = {int} 0
* Depth = {int} 0

Since the `__schema: __Schema! @nodeCountSkip` directive is present in the schema, the calculated NodeCount, Complexity, and Depth are all 0.
