package proxy

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// LoadBalancer provides round-robin load balancing across multiple backends
// with health checking support. It is lock-free for the hot path (Next).
type LoadBalancer struct {
	backends []string
	healthy  []atomic.Bool
	current  atomic.Uint64

	healthCheckInterval time.Duration
	healthCheckTimeout  time.Duration
	stopCh              chan struct{}
	stopped             atomic.Bool
	wg                  sync.WaitGroup
	logger              zerolog.Logger
}

// LoadBalancerOptions configures the load balancer
type LoadBalancerOptions struct {
	Backends            []string
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration
	Logger              zerolog.Logger
}

// NewLoadBalancer creates a new load balancer with the given backends
func NewLoadBalancer(opts *LoadBalancerOptions) *LoadBalancer {
	if opts.HealthCheckTimeout == 0 {
		opts.HealthCheckTimeout = 1 * time.Second
	}

	lb := &LoadBalancer{
		backends:            opts.Backends,
		healthy:             make([]atomic.Bool, len(opts.Backends)),
		stopCh:              make(chan struct{}),
		healthCheckInterval: opts.HealthCheckInterval,
		healthCheckTimeout:  opts.HealthCheckTimeout,
		logger:              opts.Logger,
	}

	// Mark all backends as healthy initially
	for i := range lb.healthy {
		lb.healthy[i].Store(true)
	}

	// Start health check goroutine if interval is configured
	if opts.HealthCheckInterval > 0 && len(opts.Backends) > 0 {
		lb.wg.Add(1)
		go lb.runHealthChecks()
	}

	return lb
}

// Next returns the next healthy backend using round-robin selection.
// This method is lock-free and safe for concurrent use.
func (lb *LoadBalancer) Next() string {
	n := len(lb.backends)
	if n == 0 {
		return ""
	}

	// Fast path: single backend
	if n == 1 {
		return lb.backends[0]
	}

	// Round-robin with health awareness
	start := lb.current.Add(1)
	for i := 0; i < n; i++ {
		idx := (int(start) + i) % n
		if lb.healthy[idx].Load() {
			return lb.backends[idx]
		}
	}

	// Fallback: return any backend if all appear unhealthy
	// This prevents total failure when health checks are failing
	return lb.backends[int(start)%n]
}

func (lb *LoadBalancer) GetHealthyCount() int {
	count := 0
	for i := range lb.healthy {
		if lb.healthy[i].Load() {
			count++
		}
	}
	return count
}

func (lb *LoadBalancer) GetBackends() []string {
	return append([]string(nil), lb.backends...)
}

func (lb *LoadBalancer) IsHealthy(idx int) bool {
	if idx < 0 || idx >= len(lb.healthy) {
		return false
	}
	return lb.healthy[idx].Load()
}

func (lb *LoadBalancer) SetHealthy(idx int, healthy bool) {
	if idx >= 0 && idx < len(lb.healthy) {
		lb.healthy[idx].Store(healthy)
	}
}

// runHealthChecks periodically checks all backends
func (lb *LoadBalancer) runHealthChecks() {
	defer lb.wg.Done()

	ticker := time.NewTicker(lb.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			lb.checkAll()
		case <-lb.stopCh:
			return
		}
	}
}

// checkAll probes all backends and updates their health status
func (lb *LoadBalancer) checkAll() {
	for i, backend := range lb.backends {
		healthy := lb.probe(backend)
		wasHealthy := lb.healthy[i].Swap(healthy)

		// Log health status changes
		if healthy != wasHealthy {
			if healthy {
				lb.logger.Info().
					Str("backend", backend).
					Msg("Backend became healthy")
			} else {
				lb.logger.Warn().
					Str("backend", backend).
					Msg("Backend became unhealthy")
			}
		}
	}
}

// probe checks if a backend is reachable
func (lb *LoadBalancer) probe(backend string) bool {
	conn, err := net.DialTimeout("tcp", backend, lb.healthCheckTimeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (lb *LoadBalancer) Stop() {
	if lb.stopped.Swap(true) {
		return
	}
	close(lb.stopCh)
	lb.wg.Wait()
}
