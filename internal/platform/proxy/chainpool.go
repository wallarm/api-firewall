package proxy

// Copyright 2018 The yeqown Author. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"sync"

	"github.com/valyala/fasthttp"
	"github.com/wallarm/api-firewall/internal/config"
)

var (
	errFactoryNotHelp         = errors.New("factory is not able to fill the pool")
	errInvalidCapacitySetting = errors.New("invalid capacity settings")
	errClosed                 = errors.New("err: chan closed")
)

type HTTPClient interface {
	Do(req *fasthttp.Request, resp *fasthttp.Response) error
}

func factory(hostAddr string, server *config.Server, tlsConfig *tls.Config) (HTTPClient, error) {

	var proxyClient = &fasthttp.Client{
		Dial: func(addr string) (net.Conn, error) {
			return fasthttp.DialTimeout(hostAddr, server.DialTimeout)
		},
		TLSConfig:       tlsConfig,
		MaxConnsPerHost: server.MaxConnsPerHost,
		ReadTimeout:     server.ReadTimeout,
		WriteTimeout:    server.WriteTimeout,
	}
	return proxyClient, nil
}

type Pool interface {
	// Get returns a new ReverseProxy from the pool.
	Get() (HTTPClient, error)

	// Put the ReverseProxy puts it back to the Pool.
	Put(HTTPClient) error

	// Close closes the pool and all its connections. After Close() the pool is
	// no longer usable.
	Close()

	// Len returns the current number of connections of the pool.
	Len() int
}

// Pool interface impelement based on channel
// there is a channel to contain ReverseProxy object,
// and provide Get and Put method to handle with RevsereProxy
type chanPool struct {
	// mutex makes the chanPool woking with goroutine safely
	mutex sync.RWMutex

	// reverseProxyChan chan of getting the *ReverseProxy and putting it back
	reverseProxyChan chan HTTPClient

	// factory is factory method to generate ReverseProxy
	// this can be customized
	// factory Factory
	server *config.Server
	host   string

	tlsConfig *tls.Config
}

// NewChanPool to new a pool with some params
func NewChanPool(initialCap, maxCap int, hostAddr string, server *config.Server) (Pool, error) {
	if initialCap < 0 || maxCap <= 0 || initialCap > maxCap {
		return nil, errInvalidCapacitySetting
	}

	// Get the SystemCertPool, continue with an empty pool on error
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	if server.RootCA != "" {

		// Read in the cert file
		certs, err := ioutil.ReadFile(server.RootCA)
		if err != nil {
			return nil, fmt.Errorf("failed to append %q to RootCAs: %v", server.RootCA, err)
		}

		// Append our cert to the system pool
		if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
			return nil, errors.New("no certs appended, using system certs only")
		}
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: server.InsecureConnection,
		RootCAs:            rootCAs,
	}

	// initialize the chanPool
	pool := &chanPool{
		mutex:            sync.RWMutex{},
		reverseProxyChan: make(chan HTTPClient, maxCap),
		server:           server,
		host:             hostAddr,
		tlsConfig:        tlsConfig,
	}

	// create initial connections, if something goes wrong,
	// just close the pool error out.
	for i := 0; i < initialCap; i++ {
		proxy, err := factory(hostAddr, server, tlsConfig)
		if err != nil {
			return nil, errFactoryNotHelp
		}
		pool.reverseProxyChan <- proxy
	}

	return pool, nil
}

// getConnsAndFactory ... get a copy of chanPool's reverseProxyChan and factory
func (p *chanPool) getConnsAndFactory() chan HTTPClient {
	p.mutex.RLock()
	reverseProxyChan := p.reverseProxyChan
	p.mutex.RUnlock()
	return reverseProxyChan
}

// Close close the pool
func (p *chanPool) Close() {
	p.mutex.Lock()
	reverseProxyChan := p.reverseProxyChan
	p.reverseProxyChan = nil
	p.mutex.Unlock()

	if reverseProxyChan == nil {
		return
	}

	close(reverseProxyChan)
}

// Get a *ReverseProxy from pool, it will get an error while
// reverseProxyChan is nil or pool has been closed
func (p *chanPool) Get() (HTTPClient, error) {

	if p.reverseProxyChan == nil {
		return nil, errClosed
	}

	// wrap our connections with out custom net.Conn implementation (wrapConn
	// method) that puts the connection back to the pool if it's closed.
	select {
	case proxy := <-p.reverseProxyChan:
		if proxy == nil {
			return nil, errClosed
		}
		return proxy, nil
	default:
		proxy, err := factory(p.host, p.server, p.tlsConfig)
		if err != nil {
			return nil, err
		}
		return proxy, nil
	}
}

// Put ... put a *ReverseProxy object back into chanPool
func (p *chanPool) Put(proxy HTTPClient) error {
	if proxy == nil {
		return errors.New("proxy is nil. rejecting")
	}

	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.reverseProxyChan == nil {
		// pool is closed, close passed connection
		return nil
	}

	// put the resource back into the pool. If the pool is full, this will
	// block and the default case will be executed.
	select {
	case p.reverseProxyChan <- proxy:
		return nil
	default:
		return nil
	}
}

// Len get chanPool channel length
func (p *chanPool) Len() int {
	reverseProxyChan := p.getConnsAndFactory()
	return len(reverseProxyChan)
}
