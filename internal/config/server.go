package config

import "time"

type APIFWServer struct {
	APIHost            string        `conf:"default:http://0.0.0.0:8282,env:URL" validate:"required,url"`
	HealthAPIHost      string        `conf:"default:0.0.0.0:9667,env:HEALTH_HOST" validate:"required"`
	ReadTimeout        time.Duration `conf:"default:5s"`
	WriteTimeout       time.Duration `conf:"default:5s"`
	ReadBufferSize     int           `conf:"default:8192"`
	WriteBufferSize    int           `conf:"default:8192"`
	MaxRequestBodySize int           `conf:"default:4194304"`
	DisableKeepalive   bool          `conf:"default:false"`
	MaxConnsPerIP      int           `conf:"default:0"`
	MaxRequestsPerConn int           `conf:"default:0"`
}
