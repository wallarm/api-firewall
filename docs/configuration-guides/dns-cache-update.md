# DNS Cache Update

The DNS cache update feature allows you to make asynchronous DNS requests and cache results for a configured period of time. This feature could be useful when DNS load balancing is used. 

!!! info "Feature availability"
    This feature and corresponding variables are supported only in the [`PROXY`](installation-guides/docker-container.md) API Firewall mode.

To configure the DNS cache update, use the following environment variables:

| Environment variable | Type | Description |
| -------------------- | ----------- | ----------- |
| `APIFW_DNS_CACHE` | `bool` | Turns on using async DNS resolving and caching feature. <br> The default value is `false`. |
| `APIFW_DNS_FETCH_TIMEOUT` | `time.Duration` | TTL of the cache. <br> The default value is `1 minute`. |
| `APIFW_DNS_LOOKUP_TIMEOUT` | `time.Duration` | Lookup timeout. <br> The default value is `1 second`. |
| `APIFW_DNS_NAMESERVER_HOST` | `string` | Host of the custom nameserver. <br> By default the value is `“”`. In this case the configured in the system DNS server will be used. |
| `APIFW_DNS_NAMESERVER_PORT` | `string` | Port of the custom nameserver. <br> The default value is `53`. |
| `APIFW_DNS_NAMESERVER_PROTO` | `string` | Protocol to use. <br> Possible values are case `tcp`, `tcp4`, `tcp6`, `udp`, `udp4`, `udp6` - `4` and `6` are IPv4 and IPv6. <br><br> The default value is `udp`. |

When the asynchronous DNS resolving and caching feature is turned on, a dedicated goroutine is started and the DNS cache is updated every fetch timeout period. If a custom nameserver is configured then it will be used by the APIFW for all requests and DNS caching system. If a host contains multiple IPs for one entry then the first entry will be used. Also, the IPv4 has higher priority than the IPv6 IPs.
