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

[Check the demo on running API Firewall with OWASP CoreRuleSet v4.1.0](../demos/owasp-coreruleset.md)
