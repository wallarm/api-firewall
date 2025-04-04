```yaml
mode: "PROXY"
RequestValidation: "BLOCK"
ResponseValidation: "BLOCK"
CustomBlockStatusCode: 403
AddValidationStatusHeader: false
APISpecs: "openapi.yaml"
APISpecsCustomHeader:
  Name: ""
  Value: ""
PassOptionsRequests: true
SpecificationUpdatePeriod: "0"
Server:
  APIHost: "http://0.0.0.0:8282"
  HealthAPIHost: "0.0.0.0:9999"
  ReadTimeout: "5s"
  WriteTimeout: "5s"
  ReadBufferSize: 8192
  WriteBufferSize: 8192
  MaxRequestBodySize: 4194304
  DisableKeepalive: false
  MaxConnsPerIP: 0
  MaxRequestsPerConn: 0
DNS:
  Nameserver:
    Host: ""
    Port: "53"
    Proto: "udp"
  Cache: false
  FetchTimeout: "1m"
  LookupTimeout: "1s"
Denylist:
  Tokens:
    CookieName: ""
    HeaderName: ""
    TrimBearerPrefix: true
    File: ""
AllowIP:
  File: ""
  HeaderName: ""
ShadowAPI:
  ExcludeList:
    - 404
    - 200
  UnknownParametersDetection: false
TLS:
  CertsPath: "certs"
  CertFile: "localhost.crt"
  CertKey: "localhost.key"
ModSecurity:
  ConfFiles: []
  RulesDir: ""
Endpoints: []
Backend:
  Oauth:
    ValidationType: "JWT"
    JWT:
      SignatureAlgorithm: "RS256"
      PubCertFile: ""
      SecretKey: ""
    Introspection:
      ClientAuthBearerToken: ""
      Endpoint: ""
      EndpointParams: ""
      TokenParamName: ""
      ContentType: ""
      EndpointMethod: "GET"
      RefreshInterval: "10m"
  ProtectedAPI:
  	URL: "http://localhost:3000/v1/"
	RequestHostHeader: ""
	ClientPoolCapacity: 1000
	InsecureConnection: false
	RootCA: ""
	MaxConnsPerHost: 512
	ReadTimeout: "5s"
	WriteTimeout: "5s"
	DialTimeout: "200ms"
	ReadBufferSize: 8192
	WriteBufferSize: 8192
	MaxResponseBodySize: 0
	DeleteAcceptEncoding: false
```