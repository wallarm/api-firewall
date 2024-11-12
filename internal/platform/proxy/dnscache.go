package proxy

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	// cacheSize is initial size of addr and IP list cache map.
	cacheSize = 64
)

// onRefreshed is called when DNS are refreshed.
var onRefreshed = func() {}

type DNSCache interface {
	LookupIPAddr(context.Context, string) (names []net.IPAddr, err error)
	Refresh()
	Stop()
}

// Resolver is DNS cache resolver which cache DNS resolve results in memory.
type Resolver struct {
	lookupIPAddrFn func(ctx context.Context, host string) ([]net.IPAddr, error)
	lookupTimeout  time.Duration
	useCache       bool

	logger *logrus.Logger

	lock   sync.RWMutex
	cache  map[string][]net.IPAddr
	closer func()
}

type DNSCacheOptions struct {
	UseCache      bool
	Logger        *logrus.Logger
	FetchTimeout  time.Duration
	LookupTimeout time.Duration
}

// NewDNSResolver initializes DNS cache resolver and starts auto refreshing in a new goroutine.
// To stop refreshing, call `Stop()` function.
func NewDNSResolver(resolver *net.Resolver, options *DNSCacheOptions) (DNSCache, error) {

	if options == nil {
		return nil, errors.New("options cannot be nil")
	}

	// copy handler function to avoid race
	onRefreshedFn := onRefreshed
	lookupIPAddrFn := func(ctx context.Context, host string) ([]net.IPAddr, error) {
		addrs, err := resolver.LookupIPAddr(ctx, host)

		if err != nil {
			return nil, err
		}

		return addrs, nil
	}

	r := &Resolver{
		lookupIPAddrFn: lookupIPAddrFn,
		lookupTimeout:  options.LookupTimeout,
		logger:         options.Logger,
		useCache:       options.UseCache,
	}

	if options.UseCache {

		ticker := time.NewTicker(options.FetchTimeout)

		ch := make(chan struct{})
		r.closer = func() {
			ticker.Stop()
			close(ch)
		}

		r.cache = make(map[string][]net.IPAddr, cacheSize)

		go func() {
			for {
				select {
				case <-ticker.C:
					r.Refresh()
					onRefreshedFn()
				case <-ch:
					return
				}
			}
		}()
	}

	return r, nil
}

// fetch fetches IP list from the cache. If IP list of the given addr is not in the cache,
// then it lookups from DNS server by `Lookup` function.
func (r *Resolver) fetch(ctx context.Context, addr string) ([]net.IPAddr, error) {

	// resolve addr and return IPs without caching
	if !r.useCache {
		return r.lookupIPAddrFn(ctx, addr)
	}

	// try to search addr in the cache. In case of fail try to resolve addr and cache the results
	r.lock.RLock()
	ipAddrs, ok := r.cache[addr]
	r.lock.RUnlock()
	if ok {
		return ipAddrs, nil
	}
	return r.lookupIPAddrAndCache(ctx, addr)
}

// lookupIPAddrAndCache lookups IP list from DNS server then it saves result in the cache.
// If you want to get result from the cache use `Fetch` function.
func (r *Resolver) lookupIPAddrAndCache(ctx context.Context, addr string) ([]net.IPAddr, error) {
	ipAddrs, err := r.lookupIPAddrFn(ctx, addr)
	if err != nil {
		return nil, err
	}

	r.lock.Lock()
	r.cache[addr] = ipAddrs
	r.lock.Unlock()
	return ipAddrs, nil
}

// LookupIPAddr lookups IP list from the cache and returns DNS server then it saves result in the cache.
// If you want to get result from the cache use `Fetch` function.
func (r *Resolver) LookupIPAddr(ctx context.Context, addr string) ([]net.IPAddr, error) {
	ipAddrs, err := r.fetch(ctx, addr)
	if err != nil {
		return nil, err
	}

	return ipAddrs, nil
}

// Refresh refreshes IP list cache.
func (r *Resolver) Refresh() {
	r.lock.RLock()
	addrs := make([]string, 0, len(r.cache))
	for addr := range r.cache {
		addrs = append(addrs, addr)
	}
	r.lock.RUnlock()

	for _, addr := range addrs {
		ctx, cancelF := context.WithTimeout(context.Background(), r.lookupTimeout)
		if _, err := r.lookupIPAddrAndCache(ctx, addr); err != nil {
			r.logger.WithFields(logrus.Fields{
				"error": err,
				"addr":  addr,
			}).Error("failed to refresh DNS cache")
		}
		cancelF()
	}
}

// Stop stops auto refreshing.
func (r *Resolver) Stop() {
	if !r.useCache {
		return
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	if r.closer != nil {
		r.closer()
		r.closer = nil
	}
}
