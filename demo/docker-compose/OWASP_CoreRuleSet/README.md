# API Firewall demo with OWASP Core Rule Set v4.x.x

This demo deploys [**httpbin**](https://httpbin.org/) and Wallarm API Firewall as a proxy protecting **httpbin** API from general attacks using the OWASP ModSecurity Core Rule Set (CRS) v4.x.x. Both applications are running in the Docker containers connected using Docker Compose.

## System requirements

Before running this demo, please ensure your system meets the following requirements:

* Docker Engine 20.x or later installed for [Mac](https://docs.docker.com/docker-for-mac/install/), [Windows](https://docs.docker.com/docker-for-windows/install/), or [Linix](https://docs.docker.com/engine/install/#server)
* [Docker Compose](https://docs.docker.com/compose/install/) installed
* **make** installed for [Mac](https://formulae.brew.sh/formula/make), [Windows](https://sourceforge.net/projects/ezwinports/files/make-4.3-without-guile-w32-bin.zip/download), or Linux (using suitable package-management utilities)
* [**wget**](https://www.gnu.org/software/wget/) installed

## Used resources

The following resources are used in this demo:

* [**httpbin** Docker image](https://hub.docker.com/r/kennethreitz/httpbin/)
* [API Firewall Docker image](https://hub.docker.com/r/wallarm/api-firewall)
* [OWASP ModSecurity Core Rule Set](https://github.com/coreruleset/coreruleset)

## Demo code description

The [demo code](https://github.com/wallarm/api-firewall/tree/main/demo/docker-compose/OWASP_CoreRuleSet) contains the following configuration files:

* `httpbin.json` is the [**httpbin** OpenAPI 2.0 specification](https://httpbin.org/spec.json) converted to the OpenAPI 3.0 specification format.
* ModSecurity-related configuration files:

    * `coraza.conf` is the configuration file that contains recommended Coraza ModSecurity rules and parameters. It is created based on `resources/coraza.conf-recommended`. The only difference lies in the `SecRuleEngine` directive, which is set to `On` in `coraza.conf`.
    * `crs/rules/` is the directory with the OWASP CRS rule files (`*.conf`),
    * `crs/crs-setup.conf.example` is the OWASP CRS configuration file example.

    !!! info "Automatically downloading files"
        The `crs/rules` and `crs/crs-setup.conf.example` files will be downloaded automatically after starting the demo. Both these files will be used to test the demo deployment.
* `Makefile` is the configuration file defining Docker routines.
* `docker-compose.yml` is the file defining the API Firewall demo configuration.

When executed, the demo code performs the following operations automatically:

1. Fetches the latest OWASP CRS (v4.x.x) directly from the [CoreRuleSet GitHub repository](https://github.com/coreruleset/coreruleset).
1. Configures the ModSecurity engine based on the `coraza.conf` file from the API Firewall repository, which is adapted from the recommended `resources/coraza.conf-recommended`.

    By default, [`SecRuleEngine`](https://coraza.io/docs/seclang/directives/#secruleengine) is set to `DetectionOnly`. To block malicious requests, it should be set to `On`, and the request/response blocking mode of the API Firewall (`APIFW_REQUEST_VALIDATION`) should be set to `BLOCK` as demonstrated in this demo code. This is the sole difference from the original configuration.
1. Unpacks and mounts the OWASP CRS to the API Firewall Docker container at the `/opt/resources` directory. This setup is influenced by the specific environment variables:

    * `APIFW_MODSEC_CONF_FILES`: loads both the recommended configuration and the example configuration (`crs/crs-setup.conf.example`). The absolute paths for each are specified using the `;` delimiter.
    * `APIFW_MODSEC_RULES_DIR`: directs the API Firewall to apply specific rules from the `/opt/resources/crs/rules/` directory during request evaluation.

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

    After the successful configuration and rules loading:
    
    * The log message `The ModSecurity configuration has been loaded successfully` should appear.
    * The application **httpbin** protected by API Firewall will be available at http://localhost:8080.
4. Proceed to the demo testing.

## Step 2: Testing the OWASP ModSecurity rules

By default, this demo is running with the **httpbin** OpenAPI 3.0 specification and OWASP Core Rule Set v4.x.x. To test this demo option, you can use the following requests:

* Check that API Firewall allows the requests that contain:

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

These cases demonstrate that API Firewall protects the application from general types of attacks using the OWASP Core Rule Set.

## Step 3: Stopping the demo

To stop the demo deployment and clear your environment, use the commands:

```bash
make stop
make clean
```
