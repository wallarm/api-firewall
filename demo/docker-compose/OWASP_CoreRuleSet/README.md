# Wallarm API Firewall demo with OWASP CoreRuleSet v4.1.0

This demo deploys the application [**httpbin**](https://httpbin.org/) and Wallarm API Firewall as a proxy protecting **httpbin** API from general attacks using the OWASP CoreRuleSet v4.1.0. Both applications are running in the Docker containers connected using Docker Compose.

## System requirements

Before running this demo, please ensure your system meets the following requirements:

* Docker Engine 20.x or later installed for [Mac](https://docs.docker.com/docker-for-mac/install/), [Windows](https://docs.docker.com/docker-for-windows/install/), or [Linix](https://docs.docker.com/engine/install/#server)
* [Docker Compose](https://docs.docker.com/compose/install/) installed
* **make** installed for [Mac](https://formulae.brew.sh/formula/make), [Windows](https://sourceforge.net/projects/ezwinports/files/make-4.3-without-guile-w32-bin.zip/download), or Linux (using suitable package-management utilities)

## Used resources

The following resources are used in this demo:

* [**httpbin** Docker image](https://hub.docker.com/r/kennethreitz/httpbin/)
* [API Firewall Docker image](https://hub.docker.com/r/wallarm/api-firewall)
* [OWASP CoreRuleSet](https://github.com/coreruleset/coreruleset)

## Demo code description

In this demo scenario the OWASP CoreRuleSet collection is used in the API-Firewall to protect the API. 
There are 3 basic steps in the scenario:
1. Download the OWASP CRS (v4.x.x) from the https://github.com/coreruleset/coreruleset/releases repo. 

Note: the Makefile (in the root of this demo app) automates this step

2. Configure the ModSecurity engine. The `resources/coraza.conf-recommended` is the configuration which is recommended to use. 

Please note that by default the `SecRuleEngine` directive in the configuration is set to `DetectionOnly`. If the malicious requests should be blocked (not log only) then the value should be `On` and the Request/Response blocking mode of the API-Firewall should be also set to `BLOCK`.  
The directives description could be found on the https://coraza.io/docs/seclang/directives/. 
3. Unpack the OWASP CRS collection and mount it to the API-Firewall docker container.
The OWASP CRS contain the `crs/crs-setup.conf.example` which could be used to configure the collection and could be loaded together with the recommended configuration.
To load both configurations the absolute paths for each of them should be provided to the `APIFW_MODSEC_CONF_FILES` env var using the `;` delimiter.    
4. Run the API-Firewall. After the successful configuration and rules loading the log message `The ModSecurity configuration has been loaded successfully` should appear.

The [demo code](https://github.com/wallarm/api-firewall/tree/main/demo/docker-compose/OWASP_CoreRuleSet) contains the following configuration files:

* The demo uses the following files: 
    * `httpbin.json` is the [**httpbin** OpenAPI 2.0 specification](https://httpbin.org/spec.json) converted to the OpenAPI 3.0 specification format.
  * ModSecurity related configuration files:
    * `coraza.conf` is the configuration file that contains recommended Coraza ModSecurity rules and parameters. 
    The only difference with the `resources/coraza.conf-recommended` file is in the `SecRuleEngine` directive which is set to `On` in this file.
    * `crs/rules/` is the directory with the OWASP CRS rules files (`*.conf`)
    * `crs/crs-setup.conf.example` is the OWASP CRS configuration file example

  
  **NOTE**: the `crs/rules` and `crs/crs-setup.conf.example` files will be downloaded automatically after starting the demo 

  Both these files will be used to test the demo deployment.
  * `Makefile` is the configuration file defining Docker routines.
  * `docker-compose.yml` is the file defining the API Firewall demo configuration.

To run the demo follow the steps below.

## Step 1: Running the demo code

To run the demo code:

1. Clone the GitHub repository containing the demo code:

    ```bash
    git clone https://github.com/wallarm/api-firewall.git
    ```
2. Change to the `demo/docker-compose/OWASP_CoreRuleSet` directory of the cloned repository:

    ```bash
    cd api-firewall/demo/docker-compose/OWASP_CoreRuleSet
    ```
3. Run the demo code by using the following command:

    ```bash
    make start
    ```

    * The application **httpbin** protected by API Firewall will be available at http://localhost:8080.
4. Proceed to the demo testing.

## Step 2: Testing the OWASP ModSecurity rules

By default, this demo is running with the **httpbin** OpenAPI 3.0 specification and OWASP CoreRuleSet v4.1.0. To test this demo option, you can use the following requests:

* Check that API Firewall pass the requests that contain:

    ```bash
    curl -sD - 'http://localhost:8080/anything?id=test'
    ```

    Expected response:

    ```bash
    HTTP/1.1 200 OK
    Server: gunicorn/19.9.0
    Date: Mon, 15 Apr 2024 18:28:02 GMT
    Content-Type: application/json
    Content-Length: 361
    Access-Control-Allow-Origin: *
    Access-Control-Allow-Credentials: true

    {
      "args": {
        "id": "test"
      },
      "data": "",
      "files": {},
      "form": {},
      "headers": {
        "Accept": "*/*",
        "Apifw-Request-Id": "c8feabc1-0ae5-4506-ac14-6dd1b9c4fe86",
        "Host": "backend:80",
        "User-Agent": "curl/8.1.2"
      },
      "json": null,
      "method": "GET",
      "origin": "172.30.0.1",
      "url": "http://backend:80/anything?id=test"
    }

    ```
* Check that API Firewall blocks requests with the malicious XSS payload:

    ```bash
    curl -sD - 'http://localhost:8080/anything?id=<svg%20onload%3Dalert%281%29<%21--&context=html'
    ```

    Expected response:

    ```bash
    HTTP/1.1 403 Forbidden
    Date: Mon, 15 Apr 2024 18:35:08 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    ```

* Check that API Firewall blocks requests with the malicious SQLi payload:

    ```bash
    curl -sD - 'http://localhost:8080/anything?id="+select+1'
    ```

  Expected response:

    ```bash
    HTTP/1.1 403 Forbidden
    Date: Mon, 15 Apr 2024 18:41:49 GMT
    Content-Type: text/plain; charset=utf-8
    Content-Length: 0
    ```

    These cases demonstrate that API Firewall protects the application from general types of attacks using the OWASP CoreRuleSet.

## Step 3: Stopping the demo

To stop the demo deployment and clear your environment, use the commands:

```bash
make stop

make clean
```
