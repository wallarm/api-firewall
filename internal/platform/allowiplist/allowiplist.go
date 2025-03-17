package allowiplist

import (
	"bufio"
	"net"
	"net/netip"
	"os"
	"strings"

	"github.com/dgraph-io/ristretto"
	"github.com/rs/zerolog"
	"github.com/wallarm/api-firewall/internal/config"
)

const (
	BufferItems = 64
	ElementCost = 1

	// The actual need is 56 (size of ristretto's storeItem struct)
	StoreItemSize = 56
)

type AllowedIPsType struct {
	Cache       *ristretto.Cache
	ElementsNum int64
}

func getIPsFromCIDR(subnet string) ([]string, error) {

	var ips []string

	p, err := netip.ParsePrefix(subnet)
	if err != nil {
		return ips, err
	}

	p = p.Masked()

	for addr := p.Addr(); p.Contains(addr); addr = addr.Next() {
		ips = append(ips, addr.String())
	}

	if len(ips) <= 2 {
		return ips, nil
	}

	return ips[1 : len(ips)-1], nil
}

func New(cfg *config.AllowIP, logger zerolog.Logger) (*AllowedIPsType, error) {

	if cfg.File == "" {
		return nil, nil
	}

	var totalEntries int64

	var ips []string

	// open IPs cache storage
	f, err := os.Open(cfg.File)
	if err != nil {
		return nil, err
	}

	// count non-empty entries and total cache capacity in bytes
	c := bufio.NewScanner(f)
	for c.Scan() {
		if c.Text() != "" {

			if strings.Contains(c.Text(), "/") {
				subnetIPs, err := getIPsFromCIDR(c.Text())
				if err != nil {
					logger.Debug().Msgf("Allowlist (IP): entry with the key %s is not a valid subnet. Error: %v", c.Text(), err)
					continue
				}
				ips = append(ips, subnetIPs...)
				totalEntries += int64(len(subnetIPs))
				continue
			}

			if ip := net.ParseIP(c.Text()); ip != nil {
				totalEntries += 1
				ips = append(ips, ip.String())
				continue
			} else {
				logger.Debug().Msgf("Allowlist (IP): entry with the key %s is not a valid IP address. Error: %v", c.Text(), err)
			}

		}
	}
	err = c.Err()
	if err != nil {
		return nil, err
	}

	logger.Debug().Msgf("Allowlist (IP): total entries (lines) found in the file: %d", totalEntries)

	// max cost = total entries * size of ristretto's storeItem struct
	maxCost := totalEntries * (StoreItemSize + ElementCost)

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: maxCost * 10, // recommended value
		MaxCost:     maxCost,
		BufferItems: BufferItems,
	})
	if err != nil {
		return nil, err
	}

	var numOfElements int64

	// 10% counter
	var counter10P int64

	// ips loading to the cache
	for _, ip := range ips {

		if ok := cache.Set(ip, nil, ElementCost); ok {
			numOfElements += 1

			currentPercent := numOfElements * 100 / totalEntries
			if currentPercent/10 > counter10P {
				counter10P = currentPercent / 10
				logger.Debug().Msgf("Allowlist (IP): loaded %d perecents of IPs. Total elements in the cache: %d", counter10P*10, numOfElements)
			}
		} else {
			logger.Debug().Msgf("Allowlist (IP): can't add the token to the cache: %s", ip)
		}
		cache.Wait()

	}

	if err := f.Close(); err != nil {
		return nil, err
	}

	return &AllowedIPsType{Cache: cache, ElementsNum: totalEntries}, nil
}
