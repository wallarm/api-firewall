package config

import "time"

type DNS struct {
	Nameserver    Nameserver
	Cache         bool          `conf:"default:false"`
	FetchTimeout  time.Duration `conf:"default:1m"`
	LookupTimeout time.Duration `conf:"default:1s"`
}

type Nameserver struct {
	Host  string `conf:""`
	Port  string `conf:"default:53"`
	Proto string `conf:"default:udp"`
}
