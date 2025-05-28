package config

type Metrics struct {
	Endpoint string `conf:"default:metrics,env:METRICS_ENDPOINT" validate:"required,url"`
	Host     string `conf:"default:0.0.0.0:9090,env:METRICS_HOST" validate:"required"`
	Enabled  bool   `conf:"default:false,env:METRICS_ENABLED"`
}
