# API Firewall
Free API firewall for OpenAPI endpoints. Strict controls for your API inputs and outputs by Swagger/OpenAPI v3 definitions. Reducing attack surface drammatically.

## Hardening

Allow only requests and responses that satisfy your data validation requerements defined by the latest version of OpenAPI. The API FIrewall will block all the rest. 

## Transparency

Zero unknown requests and responses and policy full visibility defined by Swagger/OpenAPI manifest. 

## Performance

Optimized HTTP library, sockets, and JSON assertions to deliver the fastest quest processing time. 

```
$ ab -n100000 -c 10 -p pet.json http://172.17.0.2:8282/v1.0/pets/1
This is ApacheBench, Version 2.3 <$Revision: 1879490 $>

Document Path:          /v1.0/pets/1
Document Length:        18 bytes

Concurrency Level:      10
Time taken for tests:   4.504 seconds
Complete requests:      100000
Failed requests:        0
Non-2xx responses:      100000
Total transferred:      22200000 bytes
Total body sent:        20500000
HTML transferred:       1800000 bytes
Requests per second:    22204.12 [#/sec] (mean)
Time per request:       0.450 [ms] (mean)
Time per request:       0.045 [ms] (mean, across all concurrent requests)
Transfer rate:          4813.78 [Kbytes/sec] received
                        4445.16 kb/s sent
                        9258.94 kb/s total
```

## Quick start

Docker deployment:
```
$ docker pull wallarm:api-firewall
$ docker run -v /opt/sample-api/openapi3/:/tmp -e SERVER_URL=http://172.17.0.1:9090/v1.0/ -e SWAGGER_FILE=/tmp/pets-api.yaml api-firewall
```

Kubernetes deployment:
```
```

Terraform:
```
```

## How it work
