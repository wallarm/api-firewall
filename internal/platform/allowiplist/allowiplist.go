package allowiplist

import (
	"bufio"
	"io"
	"net"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/wallarm/api-firewall/internal/config"
	"github.com/yl2chen/cidranger"
)

type AllowedIPsType struct {
	Cache       cidranger.Ranger
	ElementsNum int64
}

func New(cfg *config.AllowIP, logger *logrus.Logger) (*AllowedIPsType, error) {

	if cfg.File == "" {
		return nil, nil
	}

	var totalEntries int64

	// open IPs cache storage
	f, err := os.Open(cfg.File)
	if err != nil {
		return nil, err
	}

	// count non-empty entries and total cache capacity in bytes
	c := bufio.NewScanner(f)
	for c.Scan() {
		if c.Text() != "" {
			totalEntries += 1
		}
	}
	err = c.Err()
	if err != nil {
		return nil, err
	}

	// go to the beginning of the storage file
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	logger.Debugf("AllowIPList: total entries (lines) found in the file: %d", totalEntries)

	cache := cidranger.NewPCTrieRanger()

	var numOfElements int64

	// 10% counter
	var counter10P int64

	// ip's loading to the cache
	s := bufio.NewScanner(f)
	for s.Scan() {
		loadedEntry := strings.TrimSpace(s.Text())
		if loadedEntry != "" {

			ipEntry := strings.Split(loadedEntry, "/")

			ip := net.ParseIP(ipEntry[0])
			if ip == nil {
				logger.Errorf("allow IP: %s IP address is not valid", loadedEntry)
				continue
			}

			if ip.To4() != nil && len(ipEntry) == 1 {
				loadedEntry += "/32"
			}

			if ip.To4() == nil && len(ipEntry) == 1 {
				loadedEntry += "/128"
			}

			_, network, err := net.ParseCIDR(loadedEntry)
			if err != nil {
				logger.Debugf("allow IP: entry with the key %s has not been parsed. Error: %v", loadedEntry, err)
				continue
			}

			if err := cache.Insert(cidranger.NewBasicRangerEntry(*network)); err != nil {
				logger.Debugf("allow IP: entry with the key %s has not been loaded. Error: %v", loadedEntry, err)
			}

			numOfElements += 1

			currentPercent := numOfElements * 100 / totalEntries
			if currentPercent/10 > counter10P {
				counter10P = currentPercent / 10
				logger.Debugf("Allow IP List: loaded %d perecents of ip's. Total elements in the cache: %d", counter10P*10, numOfElements)
			}

		}
	}

	err = s.Err()
	if err != nil {
		return nil, err
	}

	if err := f.Close(); err != nil {
		return nil, err
	}

	return &AllowedIPsType{Cache: cache, ElementsNum: totalEntries}, nil
}
