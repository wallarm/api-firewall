package proxy

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
)

var (
	errInvalidOptions = errors.New("invalid settings")
	errPoolClosed     = errors.New("pool is closed")
	errNoBackends     = errors.New("no backends available")
)

// HTTPClient is the interface for making HTTP requests.
type HTTPClient interface {
	Do(req *fasthttp.Request, resp *fasthttp.Response) error
}

// Pool manages HTTP client connections to backend servers.
type Pool interface {
	Get() (HTTPClient, string, error)
	Put(string, HTTPClient) error
	Close()
}

// PoolV2 is a lock-free connection pool that leverages
// fasthttp.Client's internal connection pooling.
//
// Key design:
// - Lock-free operation using atomic values
// - Single fasthttp.Client with proper MaxConnsPerHost
// - Zero allocations per request in Get/Put
// - Built-in load balancing with health checks
type PoolV2 struct {
	client    *fasthttp.Client
	lb        *LoadBalancer
	tlsConfig *tls.Config
	closed    atomic.Bool
	logger    zerolog.Logger

	// Cached for metrics/debugging
	host    string
	port    string
	hostPort string
}

// PoolV2Options configures the PoolV2 connection pool
type PoolV2Options struct {
	// Connection settings
	MaxConnsPerHost     int
	MaxIdleConnDuration time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	DialTimeout         time.Duration

	// Buffer sizes
	ReadBufferSize      int
	WriteBufferSize     int
	MaxResponseBodySize int

	// TLS configuration
	InsecureConnection bool
	RootCA             string

	// Load balancing - if empty, uses single backend from hostAddr
	Backends            []string
	HealthCheckInterval time.Duration

	// Logging
	Logger zerolog.Logger
}

// NewPoolV2 creates a new lock-free connection pool
func NewPoolV2(hostAddr string, opts *PoolV2Options) (Pool, error) {
	if opts == nil {
		return nil, errInvalidOptions
	}

	// Parse host address
	host, port, err := net.SplitHostPort(hostAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid host address: %w", err)
	}

	// Build TLS configuration
	tlsConfig, err := BuildTLSConfig(opts.InsecureConnection, opts.RootCA)
	if err != nil {
		return nil, fmt.Errorf("failed to build TLS config: %w", err)
	}

	// Determine backends for load balancer
	backends := opts.Backends
	if len(backends) == 0 {
		// Single backend mode - resolve initial IP
		ips, err := net.LookupIP(host)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve host: %w", err)
		}

		// Collect all resolved IPs as backends
		for _, ip := range ips {
			if ipv4 := ip.To4(); ipv4 != nil {
				backends = append(backends, net.JoinHostPort(ipv4.String(), port))
			}
		}

		// Fallback to IPv6 if no IPv4
		if len(backends) == 0 {
			for _, ip := range ips {
				if ipv6 := ip.To16(); ipv6 != nil {
					backends = append(backends, net.JoinHostPort(ipv6.String(), port))
				}
			}
		}

		// Last resort: use original host
		if len(backends) == 0 {
			backends = []string{hostAddr}
		}
	}

	// Create load balancer
	lb := NewLoadBalancer(&LoadBalancerOptions{
		Backends:            backends,
		HealthCheckInterval: opts.HealthCheckInterval,
		HealthCheckTimeout:  opts.DialTimeout,
		Logger:              opts.Logger,
	})

	p := &PoolV2{
		lb:        lb,
		tlsConfig: tlsConfig,
		logger:    opts.Logger,
		host:      host,
		port:      port,
		hostPort:  net.JoinHostPort(host, port),
	}

	// Create single fasthttp.Client that manages its own connection pool
	p.client = &fasthttp.Client{
		NoDefaultUserAgentHeader:      true,
		DisableHeaderNamesNormalizing: true,
		DisablePathNormalizing:        true,
		MaxConnsPerHost:               opts.MaxConnsPerHost,
		MaxIdleConnDuration:           opts.MaxIdleConnDuration,
		ReadTimeout:                   opts.ReadTimeout,
		WriteTimeout:                  opts.WriteTimeout,
		ReadBufferSize:                opts.ReadBufferSize,
		WriteBufferSize:               opts.WriteBufferSize,
		MaxResponseBodySize:           opts.MaxResponseBodySize,
		TLSConfig:                     tlsConfig,
		Dial: func(addr string) (net.Conn, error) {
			// Use load balancer to select backend
			backend := lb.Next()
			if backend == "" {
				return nil, errNoBackends
			}
			return fasthttp.DialTimeout(backend, opts.DialTimeout)
		},
	}

	p.logger.Info().
		Str("host", host).
		Str("port", port).
		Int("backends", len(backends)).
		Int("max_conns_per_host", opts.MaxConnsPerHost).
		Msg("PoolV2 initialized")

	return p, nil
}

// Get returns the shared HTTP client. The actual backend is selected
// inside the Dial function when the connection is established.
func (p *PoolV2) Get() (HTTPClient, string, error) {
	if p.closed.Load() {
		return nil, "", errPoolClosed
	}

	return p.client, p.hostPort, nil
}

// Put is a no-op -- fasthttp.Client manages connection lifecycle internally.
func (p *PoolV2) Put(ip string, client HTTPClient) error {
	// No-op: fasthttp.Client handles connection reuse automatically
	return nil
}

func (p *PoolV2) Close() {
	if p.closed.Swap(true) {
		// Already closed
		return
	}

	// Stop load balancer health checks
	p.lb.Stop()

	// Close idle connections
	p.client.CloseIdleConnections()

	p.logger.Info().
		Str("host", p.host).
		Str("port", p.port).
		Msg("PoolV2 closed")
}

func (p *PoolV2) Stats() PoolV2Stats {
	return PoolV2Stats{
		Host:           p.host,
		Port:           p.port,
		Backends:       p.lb.GetBackends(),
		HealthyCount:   p.lb.GetHealthyCount(),
		IsClosed:       p.closed.Load(),
	}
}

// PoolV2Stats contains pool statistics
type PoolV2Stats struct {
	Host         string
	Port         string
	Backends     []string
	HealthyCount int
	IsClosed     bool
}

