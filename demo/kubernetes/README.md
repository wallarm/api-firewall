# Demo: Protecting Kubernetes Application

This Demo environment shows you a common and simple api-firewall configuration running in Kubernetes. Design pattern: middleware.

## System requirments

To run this demo you need to install following software:

- docker (20.x+) ([WIN](https://docs.docker.com/docker-for-windows/install/)|[LINUX](https://docs.docker.com/engine/install/#server)|[MACOS](https://docs.docker.com/docker-for-mac/install/))
- docker-compose ([LINK](https://docs.docker.com/compose/install/))
- make ([WIN](https://sourceforge.net/projects/ezwinports/files/make-4.3-without-guile-w32-bin.zip/download))

Deploying Kubernetes can be resource intensive. Make sure
you have the following resources available:

- at least 2 CPU cores
- at lease 6 GB volatile memory

## Description

In this Demo we use a popular application [httpbin](https://httpbin.org) as backend, which is
running in docker-container `kennethreitz/httpbin`. This app's API is protected 
with api-firewall, which runs as a proxy for protected application.

To make installation of Kubernetes in this Demo easier we are going to use the
kind project: https://kind.sigs.k8s.io/. Also to make this project's installation even more easier,
everything needed for the smooth work is already packed into docker-image that starts Kubernetes cluster 
(to do so many abstraction layers are used).

The `docker-compose.yml` file performs a deployment of temporary Kubernetes cluster. This file
contains all the settings needed to run the project: kind-node building via Dockerfile, 
fix DNS-server (to provide cooperate work of Kubernetes discovery and Docker simultaniosly),
a local docker-registry and dind-service. All commands to work with this Demo are listed in 
`Makefile` file. You can simply run a pre-prepared Demo cluster (with `httpbin` and `kubernetes-dashboard` already installed)
with `make start`. 

**WARNING! Running this demo can be resource intensive. It takes up to 3 minutes to start this Demo environment.**

In this Demo an application backend is available via port 8090, an api-firewall 
work can be shown via 8080 port and a kubernetes-dashboard is available on port 8008.


## Configuration

API-Firewall requires an OpenAPI v3 manifest. This manifest describes the firewall rules,
based on which the requests will be filtered. In this Demo only one manifest is available 
and it's specially made to show the api-firewall capabilities. You can find one 
build into helm-values with yaml-path `manifest.body` in file `volumes/helm/api-firewall.yaml`

## Start/Stop/Restart

Use `make start` command to start this Demo and `make stop` to finish it.

## PoC

The protected copy of testing httpbin app is available via `http://localhost:8080`, 
a vulnerable one via `http://localhost:8090` link.

You can use equal requests for both endpoints to find out how
the response will change. 

Please be advised that the original manifest in `/spec.json` does not match 
the OpenAPI v3 Scheme. Besides, for the Demonstration purposes in this Demo 
a completly different manifest is used. This modified manifest is capable to display
all of api-firewall capabilities. 

Let's start the Demo with following command:

```bash
make start
```

#### Case 1

First of all, let's check that the routes not listed in Manifest are restricted:

```bash
# Dirbust protector. Can block tries to expluatate hidden/intenal endpoints
$ curl -sD - http://localhost:8080/unexists/unexists/unexists
HTTP/1.1 403 Forbidden
Date: Mon, 31 May 2021 06:56:57 GMT
Content-Type: text/plain; charset=utf-8
Content-Length: 0

```

#### Case 2

Now we let's make sure that the "sting" value can't be inserted into "integer" type parameter :

```bash
# Can protect from cache-poisoning DOS attacks vectors
$ curl -sD - http://localhost:8080/cache/arewfser
HTTP/1.1 403 Forbidden
Date: Mon, 31 May 2021 06:58:29 GMT
Content-Type: text/plain; charset=utf-8
Content-Length: 0
Apifw-Request-Id: 0000000200000001

```

#### Case 3

Let's make a new request with "integer" parameter for which a range of values is defined:

```json
...........
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
...........
```

```bash
# Not defined but required
$ curl -sD - http://localhost:8080/get 
HTTP/1.1 403 Forbidden
Date: Mon, 31 May 2021 07:09:08 GMT
Content-Type: text/plain; charset=utf-8
Content-Length: 0
Apifw-Request-Id: 0000000100000001

#Int parameter is 5, which is out of defined range (10-100)
$ curl -sD - http://localhost:8080/get?int=5
HTTP/1.1 403 Forbidden
Date: Mon, 31 May 2021 07:09:27 GMT
Content-Type: text/plain; charset=utf-8
Content-Length: 0
Apifw-Request-Id: 0000000200000001

#Int parameter is 15, which is within defined range (10-100)
$ curl -sD - http://localhost:8080/get?int=15
HTTP/1.1 200 OK
Server: gunicorn/19.9.0
Date: Mon, 31 May 2021 07:09:38 GMT
Content-Type: application/json
Content-Length: 280
Access-Control-Allow-Origin: *
Access-Control-Allow-Credentials: true
Apifw-Request-Id: 0000000300000001

{
  "args": {
    "int": "15"
  }, 
  "headers": {
    "Accept": "*/*", 
    "Apifw-Request-Id": "0000000300000001", 
    "Content-Length": "0", 
    "Host": "backend:80", 
    "User-Agent": "curl/7.68.0"
  }, 
  "origin": "172.22.0.1", 
  "url": "http://backend:80/get?int=15"
}

#Int parameter is 84, which is within defined range (10-100)
$ curl -sD - http://localhost:8080/get?int=84
HTTP/1.1 200 OK
Server: gunicorn/19.9.0
Date: Mon, 31 May 2021 07:09:38 GMT
Content-Type: application/json
Content-Length: 280
Access-Control-Allow-Origin: *
Access-Control-Allow-Credentials: true
Apifw-Request-Id: 0000000300000001

{
  "args": {
    "int": "84"
  }, 
  "headers": {
    "Accept": "*/*", 
    "Apifw-Request-Id": "0000000310000001", 
    "Content-Length": "0", 
    "Host": "backend:80", 
    "User-Agent": "curl/7.68.0"
  }, 
  "origin": "172.22.0.1", 
  "url": "http://backend:80/get?int=84"
}

#Int parameter is 1000, which is out of defined range (10-100)
$ curl -sD - http://localhost:8080/get?int=1000
HTTP/1.1 403 Forbidden
Date: Mon, 31 May 2021 07:09:53 GMT
Content-Type: text/plain; charset=utf-8
Content-Length: 0
Apifw-Request-Id: 0000000400000001

# POTENTIAL EVIL: 8-byte integer overflow can respond
# with stack drop
$ curl -sD - http://localhost:8080/get?int=18446744073710000001
HTTP/1.1 403 Forbidden
Date: Mon, 31 May 2021 07:10:04 GMT
Content-Type: text/plain; charset=utf-8
Content-Length: 0
Apifw-Request-Id: 0000000500000001

```

#### Case 4

Next request with `string` parameter for which regexp is defined. Don't
forget about `int` parameter which is still required for the request:


```json
...........
    {
      "in": "query",
      "name": "str",
      "schema": {
        "type": "string",
        "pattern": "^.{1,10}-\\d{1,10}$"
      }
    },
...........
```

```bash
# First part is "fasxxx.xxxawe" - 13 symbols
$ curl -sD - "http://localhost:8080/get?int=15&str=fasxxx.xxxawe-6354"
HTTP/1.1 403 Forbidden
Date: Mon, 31 May 2021 07:10:42 GMT
Content-Type: text/plain; charset=utf-8
Content-Length: 0
Apifw-Request-Id: 0000000700000001

# First part is "ri0.2-3ur0", second part is "6354"
$ curl -sD - "http://localhost:8080/get?int=15&str=ri0.2-3ur0-6354"
HTTP/1.1 200 OK
Server: gunicorn/19.9.0
Date: Mon, 31 May 2021 07:11:03 GMT
Content-Type: application/json
Content-Length: 331
Access-Control-Allow-Origin: *
Access-Control-Allow-Credentials: true
Apifw-Request-Id: 0000000800000001

{
  "args": {
    "int": "15", 
    "str": "ri0.2-3ur0-6354"
  }, 
  "headers": {
    "Accept": "*/*", 
    "Apifw-Request-Id": "0000000800000001", 
    "Content-Length": "0", 
    "Host": "backend:80", 
    "User-Agent": "curl/7.68.0"
  }, 
  "origin": "172.22.0.1", 
  "url": "http://backend:80/get?int=15&str=ri0.2-3ur0-6354"
}

# Second part "63sss54" contains non-digits
$ curl -sD - "http://localhost:8080/get?int=15&str=faswerffa-63sss54"
HTTP/1.1 403 Forbidden
Date: Mon, 31 May 2021 07:11:23 GMT
Content-Type: text/plain; charset=utf-8
Content-Length: 0
Apifw-Request-Id: 0000000900000001

# POTENTIAL EVIL: SQL Injection
$ curl -sD - 'http://localhost:8080/get?int=15&str=";SELECT%20*%20FROM%20users.credentials;"'
HTTP/1.1 403 Forbidden
Date: Mon, 31 May 2021 07:12:04 GMT
Content-Type: text/plain; charset=utf-8
Content-Length: 0
Apifw-Request-Id: 0000000B00000001

```
