package proxy

import (
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestLoadBalancer_Next_SingleBackend(t *testing.T) {
	lb := NewLoadBalancer(&LoadBalancerOptions{
		Backends: []string{"127.0.0.1:8080"},
		Logger:   zerolog.Nop(),
	})
	defer lb.Stop()

	backend := lb.Next()
	if backend != "127.0.0.1:8080" {
		t.Errorf("expected 127.0.0.1:8080, got %s", backend)
	}
}

func TestLoadBalancer_Next_RoundRobin(t *testing.T) {
	backends := []string{"127.0.0.1:8080", "127.0.0.1:8081", "127.0.0.1:8082"}
	lb := NewLoadBalancer(&LoadBalancerOptions{
		Backends: backends,
		Logger:   zerolog.Nop(),
	})
	defer lb.Stop()

	// Track how many times each backend is selected
	counts := make(map[string]int)

	// Call Next many times
	iterations := 300
	for i := 0; i < iterations; i++ {
		backend := lb.Next()
		counts[backend]++
	}

	// Each backend should be called approximately equal times
	expectedPerBackend := iterations / len(backends)
	tolerance := 5 // Allow some variance due to round-robin starting point

	for _, backend := range backends {
		count := counts[backend]
		if count < expectedPerBackend-tolerance || count > expectedPerBackend+tolerance {
			t.Errorf("backend %s called %d times, expected around %d", backend, count, expectedPerBackend)
		}
	}
}

func TestLoadBalancer_Next_SkipsUnhealthy(t *testing.T) {
	backends := []string{"127.0.0.1:8080", "127.0.0.1:8081", "127.0.0.1:8082"}
	lb := NewLoadBalancer(&LoadBalancerOptions{
		Backends: backends,
		Logger:   zerolog.Nop(),
	})
	defer lb.Stop()

	// Mark first backend as unhealthy
	lb.SetHealthy(0, false)

	// Count selections
	counts := make(map[string]int)
	iterations := 100
	for i := 0; i < iterations; i++ {
		backend := lb.Next()
		counts[backend]++
	}

	// First backend should not be selected
	if counts["127.0.0.1:8080"] > 0 {
		t.Errorf("unhealthy backend was selected %d times", counts["127.0.0.1:8080"])
	}

	// Other backends should share the load
	if counts["127.0.0.1:8081"] == 0 {
		t.Error("backend 8081 was never selected")
	}
	if counts["127.0.0.1:8082"] == 0 {
		t.Error("backend 8082 was never selected")
	}
}

func TestLoadBalancer_Next_FallbackWhenAllUnhealthy(t *testing.T) {
	backends := []string{"127.0.0.1:8080", "127.0.0.1:8081"}
	lb := NewLoadBalancer(&LoadBalancerOptions{
		Backends: backends,
		Logger:   zerolog.Nop(),
	})
	defer lb.Stop()

	// Mark all backends as unhealthy
	lb.SetHealthy(0, false)
	lb.SetHealthy(1, false)

	// Should still return a backend (fallback behavior)
	backend := lb.Next()
	if backend == "" {
		t.Error("expected a backend even when all are unhealthy")
	}
}

func TestLoadBalancer_Next_EmptyBackends(t *testing.T) {
	lb := NewLoadBalancer(&LoadBalancerOptions{
		Backends: []string{},
		Logger:   zerolog.Nop(),
	})
	defer lb.Stop()

	backend := lb.Next()
	if backend != "" {
		t.Errorf("expected empty string for empty backends, got %s", backend)
	}
}

func TestLoadBalancer_GetHealthyCount(t *testing.T) {
	backends := []string{"127.0.0.1:8080", "127.0.0.1:8081", "127.0.0.1:8082"}
	lb := NewLoadBalancer(&LoadBalancerOptions{
		Backends: backends,
		Logger:   zerolog.Nop(),
	})
	defer lb.Stop()

	// All should be healthy initially
	if count := lb.GetHealthyCount(); count != 3 {
		t.Errorf("expected 3 healthy, got %d", count)
	}

	// Mark one unhealthy
	lb.SetHealthy(1, false)
	if count := lb.GetHealthyCount(); count != 2 {
		t.Errorf("expected 2 healthy, got %d", count)
	}
}

func TestLoadBalancer_ConcurrentAccess(t *testing.T) {
	backends := []string{"127.0.0.1:8080", "127.0.0.1:8081", "127.0.0.1:8082"}
	lb := NewLoadBalancer(&LoadBalancerOptions{
		Backends: backends,
		Logger:   zerolog.Nop(),
	})
	defer lb.Stop()

	var wg sync.WaitGroup
	iterations := 10000
	goroutines := 10

	var successCount atomic.Int64

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				backend := lb.Next()
				if backend != "" {
					successCount.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	expected := int64(goroutines * iterations)
	if successCount.Load() != expected {
		t.Errorf("expected %d successful calls, got %d", expected, successCount.Load())
	}
}

func TestLoadBalancer_HealthCheck_Integration(t *testing.T) {
	// Start a simple TCP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer listener.Close()

	// Accept connections in background
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	backends := []string{listener.Addr().String(), "127.0.0.1:1"} // One reachable, one not
	lb := NewLoadBalancer(&LoadBalancerOptions{
		Backends:            backends,
		HealthCheckInterval: 100 * time.Millisecond,
		HealthCheckTimeout:  50 * time.Millisecond,
		Logger:              zerolog.Nop(),
	})
	defer lb.Stop()

	// Wait for health check to run
	time.Sleep(200 * time.Millisecond)

	// First backend should be healthy, second should be unhealthy
	if !lb.IsHealthy(0) {
		t.Error("expected first backend to be healthy")
	}
	if lb.IsHealthy(1) {
		t.Error("expected second backend to be unhealthy")
	}
}

func BenchmarkLoadBalancer_Next(b *testing.B) {
	backends := []string{
		"127.0.0.1:8080",
		"127.0.0.1:8081",
		"127.0.0.1:8082",
		"127.0.0.1:8083",
	}
	lb := NewLoadBalancer(&LoadBalancerOptions{
		Backends: backends,
		Logger:   zerolog.Nop(),
	})
	defer lb.Stop()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = lb.Next()
	}
}

func BenchmarkLoadBalancer_Next_Parallel(b *testing.B) {
	backends := []string{
		"127.0.0.1:8080",
		"127.0.0.1:8081",
		"127.0.0.1:8082",
		"127.0.0.1:8083",
	}
	lb := NewLoadBalancer(&LoadBalancerOptions{
		Backends: backends,
		Logger:   zerolog.Nop(),
	})
	defer lb.Stop()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = lb.Next()
		}
	})
}
