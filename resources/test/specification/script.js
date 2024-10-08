/*
 * Swagger API-Firewall testing - OpenAPI 3.0
 * This is a specific Swagger file for api-firewall Wallarm service testing. No real API is described by this speification.    In the specification you can find endpoints witthe different types of the parameters.  **TODO** Remove deprecated endpoints and create endpoints with descriptive names (what is tested)  **TODO** Move all requestBody types to the `components/requestBodies` section  **TODO** Add endpoints, that use different auth schemas https://swagger.io/docs/specification/authentication/
 *
 * OpenAPI spec version: 2.0.0
 *
 * NOTE: This class is auto generated by OpenAPI Generator.
 * https://github.com/OpenAPITools/openapi-generator
 *
 * Generator version: 7.6.0-SNAPSHOT
 */


import http from "k6/http";
import { group, check, sleep } from "k6";

export const options = {
    thresholds: {
      // the rate of successful checks should be 100%
      checks: ['rate==1'],
    },
  };

const BASE_URL = "http://localhost:8282";
// Sleep duration between successive requests.
// You might want to edit the value of this variable or remove calls to the sleep function on the script.
const SLEEP_DURATION = 0.1;
// Global variables should be initialized.
let apiKey = "1tdpjPicl85hVqhZYmQYcVF4iZAVkPHIg2mAeCzPIoJD2y1OVHpMA6h27dp1bSvN";
let headerOptional = true;
let explodeTrueHeader = "role=admin,firstName=Alex";
let ifModifiedSince = 1713967879;
let ifNoneMatch = false;
let explodeFalseHeader = "role,admin,firstName,Alex";
let headerMandatory = 282;

// FIXME by some reason k6 compiler throws exception on spread operator (`...apifwHeaders`), so use variables for now in some cases
const apifwSchemaHeader = "X-WALLARM-SCHEMA-ID";
const apifwSchemaId = 5;
const apifwHeaders = {
    [apifwSchemaHeader]: apifwSchemaId
}

export default function() {
    group("/serialization/query/deepObject", () => {
        let explodeTrueQuery = 'explode_true[role]=admin&explode_true[firstName]=Alex'; // extracted from 'example' field defined at the parameter level of OpenAPI spec

        // Request No. 1: 
        {
            let url = BASE_URL + `/serialization/query/deepObject?explode_true=${explodeTrueQuery}`;
            let params = {headers: apifwHeaders}
            let request = http.get(url, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/string_data_formats", () => {

        // Request No. 1: 
        {
            let url = BASE_URL + `/string_data_formats`;
            let body = {
                date: "2024-04-23",
                'date-time': "2024-04-23T15:57:22.446Z",
                password: "dojsad_knar",
                'byte-string': "U3dhZ2dlciByb2Nrcw==",
                'binary-string': "\\xc5\\xd7\\x14\\x84\\xf8\\xcf\\x9b\\xf4\\xb7oG\\x90G0\\x80K\\x9e2%\\xa9\\xf13\\xb5\\xde\\xa1h\\xf4\\xe2\\x85\\x1f\\x07/\\xcc\\x00\\xfc\\xaa|\\xa6 aqzH\\xe5.)\\xa3\\xfa7\\x9a\\x95?\\xaah\\x93\\xe3.\\xc5\\xa2{\\x94^`_",
                'pattern-string': "123-45-6789"
            };
            let params = {headers: {"Content-Type": "application/json", "Accept": "application/json", [apifwSchemaHeader]: apifwSchemaId}};
            let request = http.post(url, JSON.stringify(body), params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/object_data_formats", () => {

        // Request No. 1: 
        {
            let url = BASE_URL + `/object_data_formats`;
            let body = {
                'limited-array': [
                  "http://instagram.com/photo_1",
                  "http://pixiv.com/photo_2"
                ],
                'mixed-type-array': [
                  "foo",
                  5,
                  -2,
                  "bar"
                ],
                'write-only-param': "1!2qWe7tY",
                AnyValue: {
                  "1": 2,
                  "string": "2024-12-2",
                  "null": null
                },
                'not-integer': "string",
                additionalProp1: {}
              }
            let params = {headers: {"Content-Type": "application/json", "Accept": "application/json", [apifwSchemaHeader]: apifwSchemaId}};
            let request = http.post(url, JSON.stringify(body), params);

            // FIXME https://wallarm.atlassian.net/browse/KERBEROS-1466
            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200 (KERBEROS-1466)": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/serialization/path/matrix/{explode_false}/{explode_true}", () => {
        let explodeTrueQuery = ';role=admin;firstName=Alex'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let explodeFalseQuery = ';explode_false=role,admin,firstName,Alex'; // extracted from 'example' field defined at the parameter level of OpenAPI spec

        // Request No. 1: 
        {
            let url = BASE_URL + `/serialization/path/matrix/${explodeFalseQuery}/${explodeTrueQuery}`;
            let params = {headers: apifwHeaders};
            let request = http.request('TRACE', url, null, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/serialization/path/simple/{explode_false}/{explode_true}", () => {
        let explodeTruePath = 'role=admin,firstName=Alex'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let explodeFalsePath = 'role,admin,firstName,Alex'; // extracted from 'example' field defined at the parameter level of OpenAPI spec

        // Request No. 1: 
        {
            let url = BASE_URL + `/serialization/path/simple/${explodeFalsePath}/${explodeTruePath}`;
            let params = {headers: apifwHeaders};
            let request = http.options(url, null, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/all_params/{uri_param1}/{uri_param2}", () => {
        let queryMandatory = 'mandatory_parameter'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let uriParam1 = 'first_parameter'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let queryOptional = '282'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let uriParam2 = '1337'; // extracted from 'example' field defined at the parameter level of OpenAPI spec

        // Request No. 1: updateUser
        {
            let url = BASE_URL + `/all_params/${uriParam1}/${uriParam2}?query_mandatory=${queryMandatory}&query_optional=${queryOptional}`;
            let body = {
               id: 10,
               username: "theUser",
               firstName: "John",
               lastName: "James",
               email: "john@email.com",
               password: "12345",
               phone: "12345",
               userStatus: 1
             }
            let params = {
                headers: {
                    "Content-Type": "application/json", 
                    "header_mandatory": `${headerMandatory}`, 
                    "header_optional": `${headerOptional}`, 
                    "Accept": "application/json", 
                    [apifwSchemaHeader]: apifwSchemaId
                },
                cookies: {
                    "cookie_mandatory": "cookies are not evil",
                    "cookie_optional": false
                }
            };
            let request = http.put(url, JSON.stringify(body), params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/no_params", () => {

        // Request No. 1: 
        {
            let url = BASE_URL + `/no_params`;
            let params = {headers: apifwHeaders};
            let request = http.get(url, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/common/path/parameters/{parameter1}", () => {
        let offset = '50'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let limit = '20'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let id = '12345'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let parameter1 = '3.14'; // extracted from 'example' field defined at the parameter level of OpenAPI spec

        // Request No. 1: 
        {
            let url = BASE_URL + `/common/path/parameters/${parameter1}?id=${id}&offset=${offset}&limit=${limit}`;
            let params = {headers: apifwHeaders};
            let request = http.get(url, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });

            sleep(SLEEP_DURATION);
        }

        // Request No. 2: 
        {
            let url = BASE_URL + `/common/path/parameters/${parameter1}`;
            let body = {"some string": "this is string"};
            let params = {headers: {"Content-Type": "application/json", "Accept": "application/json", [apifwSchemaHeader]: apifwSchemaId}};
            let request = http.patch(url, JSON.stringify(body), params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/serialization/query/spaceDelimited", () => {
        let explodeTrueQuery = 'explode_true=3&explode_true=4&explode_true=5'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let explodeFalseQuery = 'explode_false=3%204%205'; // extracted from 'example' field defined at the parameter level of OpenAPI spec

        // Request No. 1: 
        {
            let url = BASE_URL + `/serialization/query/spaceDelimited?explode_false=${explodeFalseQuery}&explode_true=${explodeTrueQuery}`;
            let params = {headers: apifwHeaders};
            let request = http.get(url, params);

            // FIXME https://wallarm.atlassian.net/browse/KERBEROS-1468
            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200 (KERBEROS-1468)": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/allOf_data_formats", () => {

        // Request No. 1: 
        {
            let url = BASE_URL + `/allOf_data_formats`;
            // TODO fix JSON
            let body = {
                "id": 10,
                "username": "theUser",
                "firstName": "John",
                "lastName": "James",
                "email": "john@email.com",
                "password": "12345",
                "phone": "12345",
                "userStatus": 1,
                "name": "doggie",
                "category": {
                  "id": 1,
                  "name": "Dogs"
                },
                "photoUrls": [
                  "http://instagram.com/photo_1",
                  "http://pixiv.com/photo_2"
                ],
                "tags": [
                  {
                    "id": 123,
                    "name": "tagName"
                  }
                ],
                "status": "sold"
              }
            let params = {headers: {"Content-Type": "application/json", "Accept": "application/json", [apifwSchemaHeader]: apifwSchemaId}};
            let request = http.post(url, JSON.stringify(body), params);

            // FIXME https://wallarm.atlassian.net/browse/KERBEROS-1469
            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200 (KERBEROS-1469)": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/serialization/query/form", () => {
        let explodeTrueQuery = 'role=admin&firstName=Alex'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let explodeFalseQuery = 'explode_false=role,admin,firstName,Alex'; // extracted from 'example' field defined at the parameter level of OpenAPI spec

        // Request No. 1: 
        {
            let url = BASE_URL + `/serialization/query/form?explode_false=${explodeFalseQuery}&explode_true=${explodeTrueQuery}`;
            let params = {headers: apifwHeaders};
            let request = http.get(url, params);

            // FIXME https://wallarm.atlassian.net/browse/KERBEROS-1468
            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200 (KERBEROS-1468)": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/api/v3/user", () => {

        // Request No. 1: createUser
        {
            let url = BASE_URL + `/api/v3/user`;
            // TODO fix JSON
            let body = {
                "id": 10,
                "username": "theUser",
                "firstName": "John",
                "lastName": "James",
                "email": "john@email.com",
                "password": "12345",
                "phone": "12345",
                "userStatus": 1
              }
            let params = {headers: {"Content-Type": "application/json", "Accept": "application/json", [apifwSchemaHeader]: apifwSchemaId}};
            let request = http.post(url, JSON.stringify(body), params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/no_path_params", () => {

        // Request No. 1: 
        {
            let url = BASE_URL + `/no_path_params`;
            let params = {headers: apifwHeaders};
            let request = http.get(url, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/image", () => {

        // Request No. 1: 
        {
            let url = BASE_URL + `/image`;
            let params = {headers: apifwHeaders};
            let request = http.get(url, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/serialization/header", () => {

        // Request No. 1: 
        {
            let url = BASE_URL + `/serialization/header`;
            let params = {headers: {"explode_false": `${explodeFalseHeader}`, "explode_true": `${explodeTrueHeader}`, "Accept": "application/json", [apifwSchemaHeader]: apifwSchemaId}};
            let request = http.get(url, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/serialization/query/pipeDelimited", () => {
        let explodeTrueQuery = 'explode_true=3&explode_true=4&explode_true=5'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let explodeFalseQuery = 'explode_false=3|4|5'; // extracted from 'example' field defined at the parameter level of OpenAPI spec

        // Request No. 1: 
        {
            let url = BASE_URL + `/serialization/query/pipeDelimited?explode_false=${explodeFalseQuery}&explode_true=${explodeTrueQuery}`;
            let params = {headers: apifwHeaders};
            let request = http.get(url, params);

            // FIXME https://wallarm.atlassian.net/browse/KERBEROS-1468
            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200 (KERBEROS-1468)": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/cookies/set/{name}/{value}", () => {
        let name = 'param_name'
        let value = 'param_value'

        // Request No. 1: 
        {
            let url = BASE_URL + `/cookies/set/${name}/${value}`;
            let params = {headers: apifwHeaders};
            let request = http.get(url, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/test_integer_boundaries", () => {
        let nonExclusive = '15'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let multiple = '160'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let exclusive = '302'; // extracted from 'example' field defined at the parameter level of OpenAPI spec

        // Request No. 1: 
        {
            let url = BASE_URL + `/test_integer_boundaries?non_exclusive=${nonExclusive}&exclusive=${exclusive}&multiple=${multiple}`;
            let params = {headers: apifwHeaders};
            let request = http.get(url, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/cache", () => {

        // Request No. 1: 
        {
            let url = BASE_URL + `/cache`;
            let params = {headers: {"If-Modified-Since": `${ifModifiedSince}`, "If-None-Match": `${ifNoneMatch}`, "Accept": "application/json", [apifwSchemaHeader]: apifwSchemaId}};
            let request = http.get(url, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/serialization/path/label/{explode_false}/{explode_true}", () => {
        let explodeTruePath = '.role=admin.firstName=Alex'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let explodeFalsePath = '.role,admin,firstName,Alex'; // extracted from 'example' field defined at the parameter level of OpenAPI spec

        // Request No. 1: 
        {
            let url = BASE_URL + `/serialization/path/label/${explodeFalsePath}/${explodeTruePath}`;
            let params = {headers: apifwHeaders};
            let request = http.request('HEAD', url, undefined, params);

            // k6 does not allow to get body of the HEAD requests, so we only verify that status code of the response is correct
            check(request, {
                "Is response status 200": (r) => r.status === 200,
            });
        }
    });

    group("/oneOf_data_formats", () => {

        // Request No. 1: 
        {
            let url = BASE_URL + `/oneOf_data_formats`;
            let body = {
                id: 10,
                username: "theUser",
                firstName: "John",
                lastName: "James",
                email: "john@email.com",
                password: "12345",
                phone: "12345",
                userStatus: 1
              };
            let params = {headers: {"Content-Type": "application/json", "Accept": "application/json", [apifwSchemaHeader]: apifwSchemaId}};
            let request = http.post(url, JSON.stringify(body), params);

            // FIXME https://wallarm.atlassian.net/browse/KERBEROS-1469
            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200 (KERBEROS-1469)": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/redirect-to", () => {
        let urlParam = 'http:\/\/test.host'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let statusCode = '302'; // extracted from 'example' field defined at the parameter level of OpenAPI spec

        // Request No. 1: 
        {
            let url = BASE_URL + `/redirect-to?url=${urlParam}&status_code=${statusCode}`;
            let params = {headers: apifwHeaders};
            let request = http.get(url, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/test_query_params", () => {
        let deprecatedParam = 'true'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let jsonParameter = {id: 1, color: "red"};
        let nullableParam = null;
        let reservedAllowedParam = ':\/?#[]@!$&\'()*+,;=';
        let arrayParameter = '3,4,5'; // extracted from 'example' field defined at the parameter level of OpenAPI spec

        // Request No. 1: 
        {
            let url = BASE_URL + `/test_query_params?default_value_param&empty_allowed_param&nullable_param=${nullableParam}&deprecated_param=${deprecatedParam}&reserved_allowed_param=${reservedAllowedParam}&array_parameter=${arrayParameter}&json_parameter=${jsonParameter}`;
            let params = {headers: apifwHeaders};
            let request = http.get(url, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/api/v3/pet", () => {

        // Request No. 1: 
        {
            let url = BASE_URL + `/api/v3/pet`;
            let body = {
                id: 10,
                name: "doggie",
                category: {
                  id: 1,
                  name: "Dogs"
                },
                photoUrls: [
                  "http://instagram.com/photo_1",
                  "http://pixiv.com/photo_2"
                ],
                tags: [
                  {
                    id: 123,
                    name: "tagName"
                  }
                ],
                status: "sold"
              };
            let params = {headers: {"Content-Type": "application/json", "Accept": "application/json", [apifwSchemaHeader]: apifwSchemaId}};
            let request = http.post(url, JSON.stringify(body), params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/api/v3/pet/{petId}", () => {
        let petId = '42'; // extracted from 'example' field defined at the parameter level of OpenAPI spec

        // Request No. 1: deletePet
        {
            let url = BASE_URL + `/api/v3/pet/${petId}`;
            let params = {headers: {"api_key": `${apiKey}`, "Accept": "application/json", [apifwSchemaHeader]: apifwSchemaId}};
            // this is a DELETE method request - if params are also set, empty body must be passed
            let request = http.del(url, {} , params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/no_path_params/{param}", () => {
        let param = 'some_string'; // extracted from 'example' field defined at the parameter level of OpenAPI spec

        // Request No. 1: 
        {
            let url = BASE_URL + `/no_path_params/${param}`;
            let params = {headers: apifwHeaders};
            let request = http.get(url, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/anyOf_data_formats", () => {

        // Request No. 1: 
        {
            let url = BASE_URL + `/anyOf_data_formats`;
            let body = {
                id: 10,
                username: "theUser",
                firstName: "John",
                lastName: "James",
                email: "john@email.com",
                password: "12345",
                phone: "12345",
                userStatus: 1,
                category: {
                    id: 1,
                    name: "Dogs"
                  }
              };
            let params = {headers: {"Content-Type": "application/json", "Accept": "application/json", [apifwSchemaHeader]: apifwSchemaId}};
            let request = http.post(url, JSON.stringify(body), params);

            // FIXME https://wallarm.atlassian.net/browse/KERBEROS-1469
            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200 (KERBEROS-1469)": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/serialization/cookie", () => {

        // Request No. 1: 
        {
            let url = BASE_URL + `/serialization/cookie`;
            let params = {
                headers: apifwHeaders,
                cookies: {
                    'explode_false': 'role,admin,firstName,Alex',
                    'explode_true': 3
                }
            };
            let request = http.get(url, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/path_parameters/{name}/{value}", () => {
        let name = 'Ivan'; // extracted from 'example' field defined at the parameter level of OpenAPI spec
        let value = 'Ivanov'; // extracted from 'example' field defined at the parameter level of OpenAPI spec

        // Request No. 1: 
        {
            let url = BASE_URL + `/path_parameters/${name}/${value}`;
            let params = {headers: apifwHeaders};
            let request = http.get(url, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/cookie_params", () => {

        // Request No. 1: 
        {
            let url = BASE_URL + `/cookie_params`;
            let params = {
                headers: apifwHeaders,
                cookies: {
                    'cookie_mandatory': 'some string',
                    'cookie_optional': 100
                }
            };
            let request = http.get(url, params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });

    group("/content-type-suffix", () => {

        // Request No. 1: 
        {
            let url = BASE_URL + `/content-type-suffix`;
            let body = {id: 1, name: "Dogs"};
            let params = {headers: {"Content-Type": "application/vnd.api+json", "Accept": "application/json", [apifwSchemaHeader]: apifwSchemaId}};
            let request = http.post(url, JSON.stringify(body), params);

            check(request, {
                "Is response status 200": (r) => r.status === 200,
                "Is response body status 200": (r) => r.json("summary.0.status_code") === 200
            });
        }
    });
}
