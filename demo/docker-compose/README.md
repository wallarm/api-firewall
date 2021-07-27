# Wallarm API Firewall demo with Docker Compose

This demo deploys the application [**httpbin**](https://httpbin.org/) and Wallarm API Firewall as a proxy protecting **httpbin** API. Both applications are running in the Docker containers connected using Docker Compose.

## System requirements

Before running this demo, please ensure your system meets the following requirements:

* Docker Engine 20.x or later installed for [Mac](https://docs.docker.com/docker-for-mac/install/), [Windows](https://docs.docker.com/docker-for-windows/install/), or [Linix](https://docs.docker.com/engine/install/#server)
* [Docker Compose](https://docs.docker.com/compose/install/) installed
* **make** installed for [Mac](https://formulae.brew.sh/formula/make), [Windows](https://sourceforge.net/projects/ezwinports/files/make-4.3-without-guile-w32-bin.zip/download), or Linux (using suitable package-management utilities)

## Used resources

The following resources are used in this demo:

* [**httpbin** Docker image](https://hub.docker.com/r/kennethreitz/httpbin/)
* [API Firewall Docker image](https://hub.docker.com/r/wallarm/api-firewall)

## Demo code description

The [demo code](https://github.com/wallarm/api-firewall/tree/main/demo/docker-compose) contains the following configuration files:

* The following OpenAPI 3.0 specifications located in the `volumes` directory:
    * `httpbin.json` is the [**httpbin** OpenAPI 2.0 specification](https://httpbin.org/spec.json) converted to the OpenAPI 3.0 specification format.
    * `httpbin-with-constraints.json` is the **httpbin** OpenAPI 3.0 specification with additional API restrictions added explicitly.

    Both these files will be used to test the demo deployment.
* `Makefile` is the configuration file defining Docker routines.
* `docker-compose.yml` is the file defining the **httpbin** and [API Firewall Docker](https://docs.wallarm.com/api-firewall/installation-guides/docker-container/) images configuration.

## Step 1: Running the demo code

To run the demo code:

1. Clone the GitHub repository containing the demo code:

    ```bash
    git clone https://github.com/wallarm/api-firewall.git
    ```
2. Change to the `demo/docker-compose` directory of the cloned repository:

    ```bash
    cd api-firewall/demo/docker-compose
    ```
3. Run the demo code by using the following command:

    ```bash
    make start
    ```

    * The application **httpbin** protected by API Firewall will be available at http://localhost:8080.
    * The application **httpbin** unprotected by API Firewall will be available at http://localhost:8090. When testing the demo deployment, you can send requests to the unprotected application to know the difference.
4. Proceed to the demo testing.

## Step 2: Testing the demo based on the original OpenAPI 3.0 specification

By default, this demo is running with the original **httpbin** OpenAPI 3.0 specification. To test this demo option, you can use the following requests:

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

## Step 3: Testing the demo based on the stricter OpenAPI 3.0 specification

Firstly, please update the path to the OpenAPI 3.0 specification used in the demo:

1. In the `docker-compose.yml` file, replace the `APIFW_API_SPECS` environment variable value with the path to the stricter OpenAPI 3.0 specification (`/opt/resources/httpbin-with-constraints.json`).
2. Restart the demo by using the commands:

    ```bash
    make stop
    make start
    ```

Then, to test this demo option, you can use the following methods:

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
