# Wallarm API Firewall demo with Kubernetes

This demo deploys the application [**httpbin**](https://httpbin.org/) and Wallarm API Firewall as a proxy protecting **httpbin** API. Both applications are running in the Docker containers in Kubernetes.

## System requirements

Before running this demo, please ensure your system meets the following requirements:

* Docker Engine 20.x or later installed for [Mac](https://docs.docker.com/docker-for-mac/install/), [Windows](https://docs.docker.com/docker-for-windows/install/), or [Linux](https://docs.docker.com/engine/install/#server)
* [Docker Compose](https://docs.docker.com/compose/install/) installed
* **make** installed for [Mac](https://formulae.brew.sh/formula/make), [Windows](https://sourceforge.net/projects/ezwinports/files/make-4.3-without-guile-w32-bin.zip/download), or Linux (using suitable package-management utilities)

Running this demo environment can be resource-intensive. Please ensure you have the following resources available:

* At least 2 CPU cores
* At least 6GB volatile memory

## Used resources

The following resources are used in this demo:

* [**httpbin** Docker image](https://hub.docker.com/r/kennethreitz/httpbin/)
* [API Firewall Docker image](https://hub.docker.com/r/wallarm/api-firewall)

## Demo code description

The [demo code](https://github.com/wallarm/api-firewall/tree/main/demo/kubernetes) runs the Kubernetes cluster with deployed **httpbin** and API Firewall.

To run the Kubernetes cluster, this demo uses the tool [**kind**](https://kind.sigs.k8s.io/) which allows running the K8s cluster in minutes using Docker containers as nodes. By using several abstraction layers, **kind** and its dependencies are packed into the Docker image which starts the Kubernetes cluster.

The demo deployment is configured via the following directories/files:

* The OpenAPI 3.0 specification for **httpbin** API is located in the file `volumes/helm/api-firewall.yaml` under the `manifest.body` path. Using this specification, API Firewall will validate whether requests and responses sent to the application address match the application API schema.

    This specification does not define the [original API schema of **httpbin**](https://httpbin.org/spec.json). To demonstrate more transparently the API Firewall features, we have explicitly converted and complicated the original OpenAPI 2.0 schema and saved the changed specification to `volumes/helm/api-firewall.yaml` > `manifest.body`.
* `Makefile` is the configuration file defining Docker routines.
* `docker-compose.yml` is the file defining the following configuration for running the temporary Kubernetes cluster:

    * The [**kind**](https://kind.sigs.k8s.io/) node building based on [`docker/Dockerfile`](https://github.com/wallarm/api-firewall/blob/main/demo/kubernetes/docker/Dockerfile).
    * Deployment of the DNS server providing simultaneous Kubernetes and Docker service discovery.
    * Local Docker registry and the `dind` service deployment.
    * **httpbin** and [API Firewall Docker](https://docs.wallarm.com/api-firewall/installation-guides/docker-container/) images configuration.

## Step 1: Running the demo code

To run the demo code:

1. Clone the GitHub repository containing the demo code:

    ```bash
    git clone https://github.com/wallarm/api-firewall.git
    ```
2. Change to the `demo/kubernetes` directory of the cloned repository:

    ```bash
    cd api-firewall/demo/kubernetes
    ```
3. Run the demo code by using the command below. Please note that running this demo can be resource-intensive. It takes up to 3 minutes to start the demo environment.

    ```bash
    make start
    ```

    * The application **httpbin** protected by API Firewall will be available at http://localhost:8080.
    * The application **httpbin** unprotected by API Firewall will be available at http://localhost:8090. When testing the demo deployment, you can send requests to the unprotected application to know the difference.
4. Proceed to the demo testing.

## Step 2: Testing the demo

Using the following request, you can test deployed API Firewall:

* Check that API Firewall blocks requests sent to the unexposed path:

    ```bash
    curl -sD - http://localhost:8080/unexposed/path
    ```

    Expected response:

    ```bash
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 06:58:29 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    Apifw-Request-Id: 0000000200000001
    ```
* Check that API Firewall blocks requests with string value passed in the parameter that requires integer data type:

    ```bash
    curl -sD - http://localhost:8080/cache/arewfser
    ```

    Expected response:

    ```bash
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 06:58:29 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    Apifw-Request-Id: 0000000200000001
    ```

    This case demonstrates that API Firewall protects the application from Cache-Poisoned DoS attacks.
* Check that API Firewall blocks requests with the required query parameter `int` that does not match the following definition:

    ```json
    ...
    {
      "in": "query",
      "name": "int",
      "schema": {
        "type": "integer",
        "minimum": 10,
        "maximum": 100
      },
      "required": true
    },
    ...
    ```

    Test the definition by using the following requests:

    ```bash
    # Request with missed required query parameter
    curl -sD - http://localhost:8080/get

    # Expected response
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:09:08 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    Apifw-Request-Id: 0000000100000001

    
    # Request with the int parameter value which is in a valid range
    curl -sD - http://localhost:8080/get?int=15

    # Expected response
    HTTP/1.1 200 OK
    Server: gunicorn/19.9.0
    Date: Mon, 31 May 2021 07:09:38 GMT
    Content-Type: application/json
    Content-Length: 280
    Access-Control-Allow-Origin: *
    Access-Control-Allow-Credentials: true
    Apifw-Request-Id: 0000000300000001
    ...


    # Request with the int parameter value which is out of range
    curl -sD - http://localhost:8080/get?int=5

    # Expected response
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:09:27 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    Apifw-Request-Id: 0000000200000001


    # Request with the int parameter value which is out of range
    curl -sD - http://localhost:8080/get?int=1000

    # Expected response
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:09:53 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    Apifw-Request-Id: 0000000400000001


    # Request with the int parameter value which is out of range
    # POTENTIAL EVIL: 8-byte integer overflow can respond with stack drop
    curl -sD - http://localhost:8080/get?int=18446744073710000001

    # Expected response
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:10:04 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    Apifw-Request-Id: 0000000500000001
    ```
* Check that API Firewall blocks requests with the query parameter `str` that does not match the following definition:

    ```json
    ...
    {
      "in": "query",
      "name": "str",
      "schema": {
        "type": "string",
        "pattern": "^.{1,10}-\\d{1,10}$"
      }
    },
    ...
    ```

    Test the definition by using the following requests (the `int` parameter is still required):

    ```bash
    # Request with the str parameter value that does not match the defined regular expression
    curl -sD - "http://localhost:8080/get?int=15&str=fasxxx.xxxawe-6354"

    # Expected response
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:10:42 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    Apifw-Request-Id: 0000000700000001


    # Request with the str parameter value that does not match the defined regular expression
    curl -sD - "http://localhost:8080/get?int=15&str=faswerffa-63sss54"
    
    # Expected response
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:10:42 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    Apifw-Request-Id: 0000000700000001


    # Request with the str parameter value that matches the defined regular expression
    curl -sD - http://localhost:8080/get?int=15&str=ri0.2-3ur0-6354

    # Expected response
    HTTP/1.1 200 OK
    Server: gunicorn/19.9.0
    Date: Mon, 31 May 2021 07:11:03 GMT
    Content-Type: application/json
    Content-Length: 331
    Access-Control-Allow-Origin: *
    Access-Control-Allow-Credentials: true
    Apifw-Request-Id: 0000000800000001
    ...


    # Request with the str parameter value that does not match the defined regular expression
    # POTENTIAL EVIL: SQL Injection
    curl -sD - 'http://localhost:8080/get?int=15&str=";SELECT%20*%20FROM%20users.credentials;"'

    # Expected response
    HTTP/1.1 403 Forbidden
    Date: Mon, 31 May 2021 07:12:04 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    Apifw-Request-Id: 0000000B00000001
    ```

## Step 4: Stopping the demo code

To stop the demo deployment and clear your environment, use the command:

```bash
make stop
```
