# Endpoint-Related Response Actions

You can configure [validation modes](../installation-guides/docker-container.md#apifw-req-val) (`RequestValidation`, `ResponseValidation`) for each endpoint separately. If not set for the endpoint specifically, global value is used.

!!! info "Example of `apifw.yaml`"
    ```yaml
    mode: "PROXY"
    RequestValidation: "BLOCK"
    ResponseValidation: "BLOCK"
    ...
    Endpoints:
    - Path: "/test/endpoint1"
        RequestValidation: "LOG_ONLY"
        ResponseValidation: "LOG_ONLY"
    - Path: "/test/endpoint1/{internal_id}"
        Method: "get"
        RequestValidation: "LOG_ONLY"
        ResponseValidation: "DISABLE"
    ```

The `Method` value is optional. If the `Method` is not set then the validation modes will be applied to all methods of the endpoint.

Example of the same configuration via environment variables:

```
APIFW_ENDPOINTS=/test/endpoint1|LOG_ONLY|LOG_ONLY,GET:/test/endpoint1/{internal_id}|LOG_ONLY|DISABLE
```

The format of the `APIFW_ENDPOINTS` environment variable: 

```
[METHOD:]PATH|REQUEST_VALIDATION|RESPONSE_VALIDATION
```