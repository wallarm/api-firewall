# Open Source API Firewall by Wallarm [![Black Hat Arsenal USA 2022](https://github.com/wallarm/api-firewall/blob/main/images/BHA2022.svg?raw=true)](https://www.blackhat.com/us-22/arsenal/schedule/index.html#open-source-api-firewall-new-features--functionalities-28038)

API Firewall is a high-performance proxy with API request and response validation based on OpenAPI/Swagger schema. It is designed to protect REST API endpoints in cloud-native environments. API Firewall provides API hardening with the use of a positive security model allowing calls that match a predefined API specification for requests and responses, while rejecting everything else.

The **key features** of API Firewall are:

* Secure REST API endpoints by blocking malicious requests
* Stop API data breaches by blocking malformed API responses
* Discover Shadow API endpoints
* Validate JWT access tokens for OAuth 2.0 protocol-based authentication
* (NEW) Denylist compromised API tokens, keys, and Cookies

The product is **open source**, available at DockerHub and already got 1 billion (!!!) pulls. To support this project, you can star the [repository](https://hub.docker.com/r/wallarm/api-firewall).

## Use cases

### Running in blocking mode
* Block malicious requests that do not match the OpenAPI 3.0 specification
* Block malformed API responses to stop data breaches and sensitive information exposure

### Running in monitoring mode
* Discover Shadow APIs and undocumented API endpoints
* Log malformed requests and responses that do not match the OpenAPI 3.0 specification

## API schema validation and positive security model

When starting API Firewall, you should provide the [OpenAPI 3.0 specification](https://swagger.io/specification/) of the application to be protected with API Firewall. The started API Firewall will operate as a reverse proxy and validate whether requests and responses match the schema defined in the specification.

The traffic that does not match the schema will be logged using the [`STDOUT` and `STDERR` Docker services](https://docs.docker.com/config/containers/logging/) or blocked (depending on the configured API Firewall operation mode). When operating in the logging mode, API Firewall also logs so-called shadow API endpoints, those that are not covered in API specification but respond to requests (except for endpoints returning the code `404`).

![API Firewall scheme](https://github.com/wallarm/api-firewall/blob/main/images/Firewall%20opensource%20-%20vertical.gif?raw=true)

[OpenAPI 3.0 specification](https://swagger.io/specification/) is supported and should be provided as a YAML or JSON file (`.yaml`, `.yml`, `.json` file extensions).

By allowing you to set the traffic requirements with the OpenAPI 3.0 specification, API Firewall relies on a positive security model.

## Technical data

API Firewall [works](https://www.wallarm.com/what/the-concept-of-a-firewall) as a reverse proxy with a built-in OpenAPI 3.0 request and response validator. It's written in Golang and using fasthttp proxy. The project is optimized for extreme performance and near-zero added latency.

## Starting API Firewall

To download, install, and start API Firewall on Docker, see the [instructions](https://docs.wallarm.com/api-firewall/installation-guides/docker-container/).

## Demos

You can try API Firewall by running the demo environment that deploys an example application protected with API Firewall. There are two available demo environments:

* [API Firewall demo with Docker Compose](https://github.com/wallarm/api-firewall/tree/main/demo/docker-compose)
* [API Firewall demo with Kubernetes](https://github.com/wallarm/api-firewall/tree/main/demo/kubernetes)

## Wallarm's blog articles related to API Firewall

* [Discovering Shadow APIs with API Firewall](https://lab.wallarm.com/discovering-shadow-apis-with-a-api-firewall/)
* [Wallarm API Firewall outperforms NGINX in a production environment](https://lab.wallarm.com/wallarm-api-firewall-outperforms-nginx-in-a-production-environment/)
* [Securing REST APIs for free with OSS APIFW](https://lab.wallarm.com/securing-rest-with-free-api-firewall-how-to-guide/)

## Performance

When creating API Firewall, we prioritized speed and efficiency to ensure that our customers would have the fastest APIs possible. Our latest tests demonstrate that the average time required for API Firewall to process one request is 1.339 ms which is 66% faster than Nginx:

```
API Firewall 0.6.2 with JSON validation

$ ab -c 200 -n 10000 -p ./large.json -T application/json http://127.0.0.1:8282/test/signup

Requests per second:    13005.81 [#/sec] (mean)
Time per request:       15.378 [ms] (mean)
Time per request:       0.077 [ms] (mean, across all concurrent requests)

NGINX 1.18.0 without JSON validation

$ ab -c 200 -n 10000 -p ./large.json -T application/json http://127.0.0.1/test/signup

Requests per second:    7887.76 [#/sec] (mean)
Time per request:       25.356 [ms] (mean)
Time per request:       0.127 [ms] (mean, across all concurrent requests)
```

These performance results are not the only ones we have got during API Firewall testing. Other results along with the methods used to improve API Firewall performance are described in this [Wallarm's blog article](https://lab.wallarm.com/wallarm-api-firewall-outperforms-nginx-in-a-production-environment/).

