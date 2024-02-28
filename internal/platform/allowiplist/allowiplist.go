package allowiplist

import (
	"bufio"
	"io"
	"os"
	"strings"

	"github.com/dgraph-io/ristretto"
	"github.com/sirupsen/logrus"
	"github.com/wallarm/api-firewall/internal/config"
)

const (
	BufferItems = 64
	ElementCost = 1
	// The actual need is 56 (size of ristretto's storeItem struct)
	StoreItemSize = 128
)

type AllowedIPsType struct {
	Cache       *ristretto.Cache
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

	// max cost = total entries * size of ristretto's storeItem struct
	maxCost := StoreItemSize * totalEntries

	logger.Debugf("AllowIPList: cache capacity: %d bytes", maxCost)

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

	// ip's loading to the cache
	s := bufio.NewScanner(f)
	for s.Scan() {
		loadedIP := strings.TrimSpace(s.Text())
		if loadedIP != "" {
			if ok := cache.Set(loadedIP, nil, ElementCost); ok {
				numOfElements += 1
				currentPercent := numOfElements * 100 / totalEntries
				if currentPercent/10 > counter10P {
					counter10P = currentPercent / 10
					logger.Debugf("Allow IP List: loaded %d perecents of ip's. Total elements in the cache: %d", counter10P*10, numOfElements)
				}
			} else {
				logger.Errorf("Allowed IP List: can't add the ip to the cache: %s", s.Text())
			}
			cache.Wait()
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
