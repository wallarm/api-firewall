package tests

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/foxcpp/go-mockdns"
	"github.com/sirupsen/logrus"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/wallarm/api-firewall/internal/platform/proxy"
)

func TestWithoutRCCDNSCacheBasic(t *testing.T) {

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	var cfg = config.ProxyMode{
		RequestValidation:         "BLOCK",
		ResponseValidation:        "BLOCK",
		CustomBlockStatusCode:     403,
		AddValidationStatusHeader: false,
		ShadowAPI: config.ShadowAPI{
			ExcludeList: []int{404, 401},
		},
		DNS: config.DNS{
			Cache:         true,
			FetchTimeout:  1000 * time.Millisecond,
			LookupTimeout: 400 * time.Millisecond,
		},
	}

	srv, _ := mockdns.NewServer(map[string]mockdns.Zone{
		"example.org.": {
			A: []string{"1.2.3.4", "5.6.7.8"},
		},
	}, false)
	defer srv.Close()

	srUpdatedOrder, _ := mockdns.NewServer(map[string]mockdns.Zone{
		"example.org.": {
			A: []string{"5.6.7.8", "1.2.3.4"},
		},
	}, false)
	defer srUpdatedOrder.Close()

	r := &net.Resolver{}
	srv.PatchNet(r)

	dnsCache, err := proxy.NewDNSResolver(cfg.DNS.FetchTimeout, cfg.DNS.LookupTimeout, r, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer dnsCache.Stop()

	addr, err := dnsCache.Fetch(context.Background(), "example.org")
	if err != nil {
		t.Error(err)
	}

	if addr[0].String() != "1.2.3.4" {
		t.Errorf("Incorrect response from local DNS server. Expected: 1.2.3.4 and got %s",
			addr[0].String())
	}

	srUpdatedOrder.PatchNet(r)

	time.Sleep(600 * time.Millisecond)

	addr, err = dnsCache.Fetch(context.Background(), "example.org")
	if err != nil {
		t.Error(err)
	}

	if addr[0].String() != "1.2.3.4" {
		t.Errorf("Incorrect response from local DNS server. Expected: 1.2.3.4 and got %s",
			addr[0].String())
	}

	time.Sleep(800 * time.Millisecond)

	addr, err = dnsCache.Fetch(context.Background(), "example.org")
	if err != nil {
		t.Error(err)
	}

	if addr[0].String() != "5.6.7.8" {
		t.Errorf("Incorrect response from local DNS server. Expected: 5.6.7.8 and got %s",
			addr[0].String())
	}
}
