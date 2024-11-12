package proxy

// Copyright 2018 The yeqown Author. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"

	"github.com/wallarm/api-firewall/internal/config"
)

const defaultConcurrency = 1000

var defaultLookUpTimeout = 1 * time.Second

var (
	errInvalidCapacitySetting     = errors.New("invalid capacity settings")
	errClosed                     = errors.New("chan closed")
	errFailedDNSCacheResolverInit = errors.New("DNS cache resolver init failed")
)

func (p *chanPool) tryResolveAndFetchOneIP(host string) (string, error) {

	var resolvedIP string

	ipAddrs, err := p.dnsResolver.LookupIPAddr(context.Background(), host)
	if err != nil {
		return "", err
	}

	for _, ip := range ipAddrs {
		if ip.IP.To4() != nil {
			resolvedIP = ip.String()
			return resolvedIP, nil
		}
	}

	for _, ip := range ipAddrs {
		if ip.IP.To16() != nil {
			resolvedIP = ip.String()
			return resolvedIP, nil
		}
	}

	return resolvedIP, nil
}

type HTTPClient interface {
	Do(req *fasthttp.Request, resp *fasthttp.Response) error
}

func (p *chanPool) factory(connAddr string) HTTPClient {

	proxyClient := fasthttp.Client{
		NoDefaultUserAgentHeader:      true,
		DisableHeaderNamesNormalizing: true,
		DisablePathNormalizing:        true,
		Dial: func(addr string) (net.Conn, error) {
			tcpDialer := &fasthttp.TCPDialer{
				Concurrency:          defaultConcurrency,
				Resolver:             p.dnsResolver,
				DisableDNSResolution: p.options.DNSConfig.Cache,
			}
			return tcpDialer.DialTimeout(connAddr, p.options.DialTimeout)
		},
		TLSConfig:           p.tlsConfig,
		MaxConnsPerHost:     p.options.MaxConnsPerHost,
		ReadTimeout:         p.options.ReadTimeout,
		WriteTimeout:        p.options.WriteTimeout,
		ReadBufferSize:      p.options.ReadBufferSize,
		WriteBufferSize:     p.options.WriteBufferSize,
		MaxResponseBodySize: p.options.MaxResponseBodySize,
	}

	return &proxyClient
}

type Pool interface {
	// Get returns a new ReverseProxy from the pool.
	Get() (HTTPClient, string, error)

	// Put the ReverseProxy puts it back to the Pool.
	Put(string, HTTPClient) error

	// Close closes the pool and all its connections. After Close() the pool is
	// no longer usable.
	Close()
}

// Pool interface impelement based on channel
// there is a channel to contain ReverseProxy object,
// and provide Get and Put method to handle with RevsereProxy
type chanPool struct {
	// mutex makes the chanPool woking with goroutine safely
	mutex sync.RWMutex

	// reverseProxyChan chan of getting the *ReverseProxy and putting it back
	reverseProxyChanLB map[string]chan HTTPClient

	// factory is factory method to generate ReverseProxy
	options *Options
	host    string
	port    string

	initResolvedIP string
	initConnAddr   string

	tlsConfig   *tls.Config
	dnsResolver DNSCache
}

type Options struct {
	InitialPoolCapacity int
	ClientPoolCapacity  int
	InsecureConnection  bool
	RootCA              string
	MaxConnsPerHost     int
	DNSConfig           config.DNS

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	DialTimeout  time.Duration

	ReadBufferSize      int
	WriteBufferSize     int
	MaxResponseBodySize int

	Logger *logrus.Logger

	DNSResolver DNSCache
}

// NewChanPool to new a pool with some params
func NewChanPool(hostAddr string, options *Options) (Pool, error) {

	if options.InitialPoolCapacity < 0 || options.ClientPoolCapacity <= 0 || options.InitialPoolCapacity > options.ClientPoolCapacity {
		return nil, errInvalidCapacitySetting
	}

	dnsResolver := options.DNSResolver

	if options.DNSResolver == nil {
		// default DNS resolver
		resolver := &net.Resolver{
			PreferGo:     true,
			StrictErrors: false,
		}

		// init DNS resolver
		dnsCacheOptions := DNSCacheOptions{
			UseCache:      false,
			Logger:        options.Logger,
			LookupTimeout: defaultLookUpTimeout,
		}

		newDnsResolver, err := NewDNSResolver(resolver, &dnsCacheOptions)
		if err != nil {
			return nil, errFailedDNSCacheResolverInit
		}

		dnsResolver = newDnsResolver
	}

	// Get the SystemCertPool, continue with an empty pool on error
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	if options.RootCA != "" {
		// Read in the cert file
		certs, err := os.ReadFile(options.RootCA)
		if err != nil {
			return nil, fmt.Errorf("failed to append %q to RootCAs: %v", options.RootCA, err)
		}

		// Append our cert to the system pool
		if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
			return nil, errors.New("no certs appended, using system certs only")
		}
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: options.InsecureConnection,
		RootCAs:            rootCAs,
	}

	host, port, err := net.SplitHostPort(hostAddr)
	if err != nil {
		return nil, err
	}

	// initialize the chanPool
	pool := &chanPool{
		mutex:              sync.RWMutex{},
		reverseProxyChanLB: make(map[string]chan HTTPClient),
		options:            options,
		host:               host,
		port:               port,
		tlsConfig:          tlsConfig,
		dnsResolver:        dnsResolver,
	}

	ip, err := pool.tryResolveAndFetchOneIP(host)
	if err != nil {
		return nil, err
	}

	var builder strings.Builder

	builder.WriteString(ip)
	builder.WriteString(":")
	builder.WriteString(port)

	pool.initConnAddr = builder.String()

	// create initial connections, if something goes wrong,
	// just close the pool error out.
	for i := 0; i < options.InitialPoolCapacity; i++ {

		ip, err = pool.tryResolveAndFetchOneIP(pool.host)
		if err != nil {
			continue
		}

		builder.Reset()

		builder.WriteString(ip)
		builder.WriteString(":")
		builder.WriteString(port)

		connAddr := builder.String()

		proxy := pool.factory(connAddr)
		if pool.reverseProxyChanLB[ip] == nil {
			pool.reverseProxyChanLB[ip] = make(chan HTTPClient, options.ClientPoolCapacity)
		}
		pool.reverseProxyChanLB[ip] <- proxy
	}

	return pool, nil
}

// Close close the pool
func (p *chanPool) Close() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for ip := range p.reverseProxyChanLB {
		reverseProxyChan := p.reverseProxyChanLB[ip]
		p.reverseProxyChanLB[ip] = nil

		if reverseProxyChan == nil {
			return
		}

		close(reverseProxyChan)
	}

	p.dnsResolver.Stop()

}

// Get a *ReverseProxy from pool, it will get an error while
// reverseProxyChan is nil or pool has been closed
func (p *chanPool) Get() (HTTPClient, string, error) {

	var resolvedIP, connAddr string

	connAddr = p.initConnAddr
	resolvedIP = p.initResolvedIP

	if p.options.DNSConfig.Cache {
		ip, err := p.tryResolveAndFetchOneIP(p.host)
		if err != nil {
			return nil, "", err
		}
		resolvedIP = ip

		var builder strings.Builder

		builder.WriteString(ip)
		builder.WriteString(":")
		builder.WriteString(p.port)

		connAddr = builder.String()
	}

	reverseProxyChan := p.reverseProxyChanLB[resolvedIP]

	if reverseProxyChan == nil {
		p.reverseProxyChanLB[resolvedIP] = make(chan HTTPClient, p.options.ClientPoolCapacity)
		reverseProxyChan = p.reverseProxyChanLB[resolvedIP]
	}

	// wrap our connections with out custom net.Conn implementation (wrapConn
	// method) that puts the connection back to the pool if it's closed.
	select {
	case proxy := <-reverseProxyChan:
		if proxy == nil {
			return nil, resolvedIP, errClosed
		}
		return proxy, resolvedIP, nil
	default:
		proxy := p.factory(connAddr)
		return proxy, resolvedIP, nil
	}
}

// Put ... put a *ReverseProxy object back into chanPool
func (p *chanPool) Put(ip string, proxy HTTPClient) error {
	if proxy == nil {
		return errors.New("proxy is nil. rejecting")
	}

	p.mutex.RLock()
	defer p.mutex.RUnlock()

	reverseProxyChan := p.reverseProxyChanLB[ip]

	if reverseProxyChan == nil {
		// pool is closed, close passed connection
		return nil
	}

	// put the resource back into the pool. If the pool is full, this will
	// block and the default case will be executed.
	select {
	case reverseProxyChan <- proxy:
		return nil
	default:
		return nil
	}
}
