package proxy

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
)

func TestPoolV2_NewPoolV2_ValidConfig(t *testing.T) {
	// Start a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Extract host:port from server URL
	host := server.Listener.Addr().String()

	pool, err := NewPoolV2(host, &PoolV2Options{
		MaxConnsPerHost:     100,
		MaxIdleConnDuration: 10 * time.Second,
		ReadTimeout:         5 * time.Second,
		WriteTimeout:        5 * time.Second,
		DialTimeout:         1 * time.Second,
		Logger:              zerolog.Nop(),
	})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	// Get should work
	client, backend, err := pool.Get()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if client == nil {
		t.Error("expected non-nil client")
	}
	if backend == "" {
		t.Error("expected non-empty backend")
	}
}

func TestPoolV2_GetPut_NoAllocation(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	pool, err := NewPoolV2(listener.Addr().String(), &PoolV2Options{
		MaxConnsPerHost: 100,
		DialTimeout:     1 * time.Second,
		Logger:          zerolog.Nop(),
	})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	// Warm up
	for i := 0; i < 100; i++ {
		client, ip, _ := pool.Get()
		pool.Put(ip, client)
	}

	// Measure allocations
	allocs := testing.AllocsPerRun(1000, func() {
		client, ip, _ := pool.Get()
		pool.Put(ip, client)
	})

	// Should have zero allocations per Get/Put cycle
	if allocs > 0 {
		t.Errorf("expected 0 allocations, got %.2f", allocs)
	}
}

func TestPoolV2_Close_PreventsFurtherGet(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	pool, err := NewPoolV2(listener.Addr().String(), &PoolV2Options{
		MaxConnsPerHost: 100,
		DialTimeout:     1 * time.Second,
		Logger:          zerolog.Nop(),
	})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	// Close the pool
	pool.Close()

	// Get should return error
	_, _, err = pool.Get()
	if err == nil {
		t.Error("expected error after close")
	}
}

func TestPoolV2_ConcurrentGetPut(t *testing.T) {
	// Start a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host := server.Listener.Addr().String()

	pool, err := NewPoolV2(host, &PoolV2Options{
		MaxConnsPerHost: 100,
		DialTimeout:     1 * time.Second,
		Logger:          zerolog.Nop(),
	})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	var wg sync.WaitGroup
	goroutines := 100
	iterations := 1000

	var successCount atomic.Int64
	var errorCount atomic.Int64

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				client, ip, err := pool.Get()
				if err != nil {
					errorCount.Add(1)
					continue
				}
				if client == nil {
					errorCount.Add(1)
					continue
				}
				pool.Put(ip, client)
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()

	expected := int64(goroutines * iterations)
	if successCount.Load() != expected {
		t.Errorf("expected %d successes, got %d (errors: %d)",
			expected, successCount.Load(), errorCount.Load())
	}
}

func TestPoolV2_ActualRequest(t *testing.T) {
	// Start a test server that echoes back
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	host := server.Listener.Addr().String()

	pool, err := NewPoolV2(host, &PoolV2Options{
		MaxConnsPerHost: 100,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		DialTimeout:     1 * time.Second,
		Logger:          zerolog.Nop(),
	})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	// Make actual request
	client, ip, err := pool.Get()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer pool.Put(ip, client)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI("http://" + host + "/test")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err = client.Do(req, resp)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if resp.StatusCode() != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode())
	}

	body := string(resp.Body())
	if body != `{"status":"ok"}` {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestPoolV2_Stats(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	pool, err := NewPoolV2(listener.Addr().String(), &PoolV2Options{
		MaxConnsPerHost: 100,
		DialTimeout:     1 * time.Second,
		Logger:          zerolog.Nop(),
	})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	// Type assertion to get PoolV2 specific methods
	poolV2, ok := pool.(*PoolV2)
	if !ok {
		t.Fatal("expected *PoolV2")
	}

	stats := poolV2.Stats()
	if stats.IsClosed {
		t.Error("expected pool to be open")
	}
	if len(stats.Backends) == 0 {
		t.Error("expected at least one backend")
	}
	if stats.HealthyCount == 0 {
		t.Error("expected at least one healthy backend")
	}
}

func TestPoolV2_InvalidHostAddress(t *testing.T) {
	_, err := NewPoolV2("invalid-no-port", &PoolV2Options{
		MaxConnsPerHost: 100,
		Logger:          zerolog.Nop(),
	})
	if err == nil {
		t.Error("expected error for invalid host address")
	}
}

func TestPoolV2_NilOptions(t *testing.T) {
	_, err := NewPoolV2("127.0.0.1:8080", nil)
	if err == nil {
		t.Error("expected error for nil options")
	}
}

func TestPoolV2_ExplicitBackends(t *testing.T) {
	// Start two test servers
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("server1"))
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("server2"))
	}))
	defer server2.Close()

	pool, err := NewPoolV2("127.0.0.1:0", &PoolV2Options{
		MaxConnsPerHost: 100,
		DialTimeout:     1 * time.Second,
		Backends: []string{
			server1.Listener.Addr().String(),
			server2.Listener.Addr().String(),
		},
		Logger: zerolog.Nop(),
	})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	poolV2 := pool.(*PoolV2)
	stats := poolV2.Stats()
	if len(stats.Backends) != 2 {
		t.Errorf("expected 2 backends, got %d", len(stats.Backends))
	}

	// Make requests - should succeed via one of the backends
	client, _, err := pool.Get()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.SetRequestURI("http://127.0.0.1/test")
	req.Header.SetMethod("GET")

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	err = client.Do(req, resp)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode())
	}

	body := string(resp.Body())
	if body != "server1" && body != "server2" {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestPoolV2_Close_Idempotent(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	pool, err := NewPoolV2(listener.Addr().String(), &PoolV2Options{
		MaxConnsPerHost: 100,
		DialTimeout:     1 * time.Second,
		Logger:          zerolog.Nop(),
	})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	// Close twice - should not panic
	pool.Close()
	pool.Close()
}

func TestPoolV2_Get_ReturnsHostPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String()
	pool, err := NewPoolV2(addr, &PoolV2Options{
		MaxConnsPerHost: 100,
		DialTimeout:     1 * time.Second,
		Logger:          zerolog.Nop(),
	})
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	_, backend, err := pool.Get()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Get should return the original host:port, not a resolved IP
	if backend != addr {
		t.Errorf("expected backend %q, got %q", addr, backend)
	}
}

func TestPoolV2_UnresolvableHost(t *testing.T) {
	_, err := NewPoolV2("this-host-does-not-exist.invalid:8080", &PoolV2Options{
		MaxConnsPerHost: 100,
		Logger:          zerolog.Nop(),
	})
	if err == nil {
		t.Error("expected error for unresolvable host")
	}
}

func TestPoolV2_WithTLSOptions(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	pool, err := NewPoolV2(listener.Addr().String(), &PoolV2Options{
		MaxConnsPerHost:    100,
		DialTimeout:        1 * time.Second,
		InsecureConnection: true,
		Logger:             zerolog.Nop(),
	})
	if err != nil {
		t.Fatalf("failed to create pool with insecure TLS: %v", err)
	}
	defer pool.Close()

	poolV2 := pool.(*PoolV2)
	if poolV2.tlsConfig == nil {
		t.Error("expected non-nil TLS config")
	}
	if !poolV2.tlsConfig.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify to be true")
	}
}

func TestPoolV2_InvalidRootCA(t *testing.T) {
	_, err := NewPoolV2("127.0.0.1:8080", &PoolV2Options{
		MaxConnsPerHost: 100,
		RootCA:          "/nonexistent/ca.pem",
		Logger:          zerolog.Nop(),
	})
	if err == nil {
		t.Error("expected error for invalid root CA")
	}
}

func BenchmarkPoolV2_Get(b *testing.B) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	pool, err := NewPoolV2(listener.Addr().String(), &PoolV2Options{
		MaxConnsPerHost: 100,
		DialTimeout:     1 * time.Second,
		Logger:          zerolog.Nop(),
	})
	if err != nil {
		b.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		client, ip, _ := pool.Get()
		pool.Put(ip, client)
	}
}

func BenchmarkPoolV2_Get_Parallel(b *testing.B) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	pool, err := NewPoolV2(listener.Addr().String(), &PoolV2Options{
		MaxConnsPerHost: 100,
		DialTimeout:     1 * time.Second,
		Logger:          zerolog.Nop(),
	})
	if err != nil {
		b.Fatalf("failed to create pool: %v", err)
	}
	defer pool.Close()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			client, ip, _ := pool.Get()
			pool.Put(ip, client)
		}
	})
}

