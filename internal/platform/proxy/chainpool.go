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
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	errInvalidCapacitySetting = errors.New("invalid capacity settings")
	errClosed                 = errors.New("err: chan closed")
)

func resolveDNS(resolver *net.Resolver, host string) (*string, error) {
	// TODO: add resolve host
	// TODO: resolve

	var resolvedIP string

	addrs, err := resolver.LookupHost(context.Background(), host)
	if err != nil {
		return nil, err
	}

	if len(addrs) > 0 {
		resolvedIP = addrs[0]
	}

	if len(addrs) == 0 {
		ip, err := resolver.LookupIP(context.Background(), "udp", host)
		if err != nil {
			return nil, err
		}
		resolvedIP = ip[0].String()
	}

	return &resolvedIP, nil
}

type HTTPClient interface {
	Do(req *fasthttp.Request, resp *fasthttp.Response) error
}

func factory(host string, port string, options *Options, tlsConfig *tls.Config) HTTPClient {

	proxyClient := fasthttp.Client{
		Dial: func(addr string) (net.Conn, error) {
			return fasthttp.DialTimeout(host+":"+port, options.DialTimeout)
		},
		TLSConfig:       tlsConfig,
		MaxConnsPerHost: options.MaxConnsPerHost,
		ReadTimeout:     options.ReadTimeout,
		WriteTimeout:    options.WriteTimeout,
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
	//reverseProxyChan chan HTTPClient
	reverseProxyChanLB map[string]chan HTTPClient

	// factory is factory method to generate ReverseProxy
	// this can be customized
	// factory Factory
	options *Options
	host    string
	port    string

	tlsConfig *tls.Config
	resolver  *net.Resolver
}

type Options struct {
	InitialPoolCapacity int
	ClientPoolCapacity  int
	InsecureConnection  bool
	RootCA              string
	MaxConnsPerHost     int
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	DialTimeout         time.Duration
}

// NewChanPool to new a pool with some params
func NewChanPool(hostAddr string, options *Options) (Pool, error) {
	if options.InitialPoolCapacity < 0 || options.ClientPoolCapacity <= 0 || options.InitialPoolCapacity > options.ClientPoolCapacity {
		return nil, errInvalidCapacitySetting
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
		reverseProxyChanLB: make(map[string]chan HTTPClient), //, options.ClientPoolCapacity),
		options:            options,
		host:               host,
		port:               port,
		tlsConfig:          tlsConfig,
		resolver:           &net.Resolver{},
		//TODO: fix it
	}

	// create initial connections, if something goes wrong,
	// just close the pool error out.
	for i := 0; i < options.InitialPoolCapacity; i++ {
		ip, err := resolveDNS(pool.resolver, pool.host)
		if err != nil {
			continue
		}

		proxy := factory(*ip, pool.port, options, pool.tlsConfig)
		reverseProxyChan := pool.reverseProxyChanLB[*ip]
		if reverseProxyChan == nil {
			pool.reverseProxyChanLB[*ip] = make(chan HTTPClient)
		}
		pool.reverseProxyChanLB[*ip] <- proxy
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
}

// Get a *ReverseProxy from pool, it will get an error while
// reverseProxyChan is nil or pool has been closed
func (p *chanPool) Get() (HTTPClient, string, error) {

	ip, err := resolveDNS(p.resolver, p.host)
	if err != nil {
		return nil, "", err
	}

	reverseProxyChan := p.reverseProxyChanLB[*ip]

	if reverseProxyChan == nil {
		return nil, *ip, errClosed
	}

	// wrap our connections with out custom net.Conn implementation (wrapConn
	// method) that puts the connection back to the pool if it's closed.

	select {
	case proxy := <-reverseProxyChan:
		if proxy == nil {
			return nil, *ip, errClosed
		}
		return proxy, *ip, nil
	default:
		proxy := factory(*ip, p.port, p.options, p.tlsConfig)
		return proxy, *ip, nil
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
