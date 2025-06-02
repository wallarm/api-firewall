package config

import "time"

type Metrics struct {
	EndpointName string        `conf:"default:metrics,env:METRICS_ENDPOINT_NAME" validate:"required,url"`
	Host         string        `conf:"default:0.0.0.0:9010,env:METRICS_HOST" validate:"required"`
	Enabled      bool          `conf:"default:false,env:METRICS_ENABLED"`
	ReadTimeout  time.Duration `conf:"default:5s"`
	WriteTimeout time.Duration `conf:"default:5s"`
}
