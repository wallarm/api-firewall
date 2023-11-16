package api

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/platform/database"
	"github.com/wallarm/api-firewall/internal/platform/web"
)

type Health struct {
	Build     string
	Logger    *logrus.Logger
	OpenAPIDB database.DBOpenAPILoader
}

// Readiness checks if the Fasthttp connection pool is ready to handle new requests.
func (h *Health) Readiness(ctx *fasthttp.RequestCtx) error {

	status := "ok"
	statusCode := fasthttp.StatusOK

	if !h.OpenAPIDB.IsReady() {
		status = "not ready"
		statusCode = fasthttp.StatusInternalServerError
	}

	data := struct {
		Status string `json:"status"`
	}{
		Status: status,
	}

	return web.Respond(ctx, data, statusCode)
}

// Liveness returns simple status info if the service is alive. If the
// app is deployed to a Kubernetes cluster, it will also return pod, node, and
// namespace details via the Downward API. The Kubernetes environment variables
// need to be set within your Pod/Deployment manifest.
func (h *Health) Liveness(ctx *fasthttp.RequestCtx) error {
	host, err := os.Hostname()
	if err != nil {
		host = "unavailable"
	}

	data := struct {
		Status    string `json:"status,omitempty"`
		Build     string `json:"build,omitempty"`
		Host      string `json:"host,omitempty"`
		Pod       string `json:"pod,omitempty"`
		PodIP     string `json:"podIP,omitempty"`
		Node      string `json:"node,omitempty"`
		Namespace string `json:"namespace,omitempty"`
	}{
		Status:    "up",
		Build:     h.Build,
		Host:      host,
		Pod:       os.Getenv("KUBERNETES_PODNAME"),
		PodIP:     os.Getenv("KUBERNETES_NAMESPACE_POD_IP"),
		Node:      os.Getenv("KUBERNETES_NODENAME"),
		Namespace: os.Getenv("KUBERNETES_NAMESPACE"),
	}

	statusCode := fasthttp.StatusOK
	return web.Respond(ctx, data, statusCode)
}
