package apifw

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"

	"github.com/valyala/fasthttp"
)

const DefaultIntSchemaID = 1

func TestAPIFWBasic(t *testing.T) {

	apifw, err := NewAPIFirewall(
		WithPathToDB("./wallarm_apifw_test.db"),
	)

	if err != nil {
		t.Fatal(err)
	}

	ctx := new(fasthttp.RequestCtx)
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.SetHost("localhost")

	w := new(bytes.Buffer)
	bw := bufio.NewWriter(w)

	if err := ctx.Request.Write(bw); err != nil {
		t.Fatal(err)
	}
	bw.Flush()

	r := bufio.NewReader(w)
	res, err := apifw.ValidateRequest(DefaultIntSchemaID, r)
	if err != nil {
		t.Error(err)
	}

	if len(res.Summary) != 1 {
		t.Fatalf("expected response with 1 summary for 1 request. Got %d entries in the summary", len(res.Summary))
	}

	if *res.Summary[0].SchemaID != DefaultIntSchemaID {
		t.Errorf("expected schema ID value %d. Got schema ID %d", DefaultIntSchemaID, *res.Summary[0].SchemaID)
	}

	if *res.Summary[0].StatusCode != 200 {
		t.Errorf("expected status code 200. Got status code %d", *res.Summary[0].StatusCode)
	}

	responseStr, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%s", responseStr)
}
