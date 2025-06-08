package metrics

import (
	"time"
)

type Metrics interface {
	IncErrorTypeCounter(err string, schemaID int)
	IncHTTPRequestStat(start time.Time, schemaID int, statusCode int)
	IncHTTPRequestTotalCountOnly(schemaID int, statusCode int)
}
