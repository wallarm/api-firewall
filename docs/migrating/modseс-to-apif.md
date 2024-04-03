# Migrating to API Firewall from ModSecurity

This guide walks through migrating from [ModSecurity](https://github.com/owasp-modsecurity/ModSecurity) to Wallarm's API Firewall by explaining how to import the ModSecurity rules to API Firewall and set API Firewall to perform protection in accordance with these rules.

## Problem and solution

In August 2021, Trustwave [announced](https://www.trustwave.com/en-us/resources/security-resources/software-updates/end-of-sale-and-trustwave-support-for-modsecurity-web-application-firewall/) the end-of-sale for ModSecurity support, and the subsequent end-of-life date for their support of ModSecurity of July 2024. Trustwave has been providing regular updates to the standard rules for ModSecurity, supporting what was effectively an open source community tool with commercial quality detection rules. Reaching the end-of-life date and support ending may quickly put any organizations using ModSecurity rules at risk by quickly becoming out-of-date with their attack detection.

Wallarm supports easy transitioning from ModSecurity to Wallarm's API Firewall: ModSecurity rules can be effortlessly connected to API Firewall and continued to be used without additional configuration.

## ModSecurity rules support

API Firewall's ModSecurity Rules Support module allows parsing and applying ModSecurity rules (secLang) to the traffic. The module is implemented using the [Coraza](https://github.com/corazawaf/coraza) project.

The module works for REST API both in the [API](../installation-guides/api-mode.md) and [PROXY](../installation-guides/docker-container.md) modes. In the API mode, only requests are checked.

Supported response actions: 

* `drop`, `deny` - respond to the client by error message with APIFW_CUSTOM_BLOCK_STATUS_CODE code or status value (if configured in the rule).
* `redirect` - responds by status code and target which were specified in the rule.

GraphQL API is currently not supported.

## Running API Firewall on ModSecurity rules

To run API Firewall on ModSecurity rules:

1. Prepare ModSecurity configuration and rule files.
1. Run API Firewall for REST API as described [here](../installation-guides/docker-container.md) using the [ModSecurity configuration parameters](#modsecurity-configuration-parameters) to connect the prepared configuration and rule files.

### ModSecurity configuration parameters

To start API Firewall on ModSecurity rules, you will need the set of configuration parameters that allow connecting and using ModSecurity rules:

* `APIFW_MODSEC_CONF_FILES`: allows to set the list of ModSecurity configuration files. The delimiter is ;. The default value is [] (empty). Example: `APIFW_MODSEC_CONF_FILES=modsec.conf;crs-setup.conf`
* `APIFW_MODSEC_RULES_DIR`: allows to set the ModSecurity directory with the rules that should be loaded. The files with the following wildcard *.conf will be loaded from the dir. The default value is “”.

### Example: Starting API Firewall on OWASP CRS with Coraza recommended configuration

You can start API Firewall on [OWASP ModSecurity Core Rule Set (CRS)](https://owasp.org/www-project-modsecurity-core-rule-set/) with Coraza [recommended configuration](https://github.com/corazawaf/coraza/blob/main/coraza.conf-recommended) (copy in included into API Firewall's `./resources/` folder):

1. Clone the repo with the OWASP CRS:

    ```
    git clone https://github.com/coreruleset/coreruleset.git
    ```

1. Start the APIFW v0.7.0 with the provided API specification and OWASP CRS:

    ```
    docker docker run --rm -it --network api-firewall-network --network-alias api-firewall \
        -v <HOST_PATH_TO_SPEC>:<CONTAINER_PATH_TO_SPEC> \
        -v ./resources/coraza.conf-recommended:/opt/coraza.conf \
        -v ./coreruleset/:/opt/coreruleset/ \
        -e APIFW_API_SPECS=<CONTAINER_PATH_TO_SPEC> \
        -e APIFW_URL=<API_FIREWALL_URL> \
        -e APIFW_SERVER_URL=<PROTECTED_APP_URL> \
        -e APIFW_REQUEST_VALIDATION=BLOCK \
        -e APIFW_RESPONSE_VALIDATION=BLOCK \
        -e APIFW_MODSEC_CONF_FILES=/opt/coraza.conf;/opt/coreruleset/crs-setup.conf.example \
        -e APIFW_MODSEC_RULES_DIR=/opt/coreruleset/rules/ \
        -p 8088:8088 wallarm/api-firewall:v0.7.0
    ```