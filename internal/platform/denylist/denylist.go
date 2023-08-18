package denylist

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
)

type DeniedTokens struct {
	Cache       *ristretto.Cache
	ElementsNum int64
}

func New(cfg *config.Denylist, logger *logrus.Logger) (*DeniedTokens, error) {

	if cfg.Tokens.File == "" {
		return nil, nil
	}

	var totalEntries int64
	var totalCacheCapacity int64

	// open tokens storage
	f, err := os.Open(cfg.Tokens.File)
	if err != nil {
		return nil, err
	}

	// count non-empty entries and total cache capacity in bytes
	c := bufio.NewScanner(f)
	for c.Scan() {
		if c.Text() != "" {
			totalCacheCapacity += int64(len(c.Text()))
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

	logger.Debugf("Denylist: total entries (lines) found in the file: %d", totalEntries)

	// max cost = total bytes found in the storage + 5%
	maxCost := totalCacheCapacity + totalCacheCapacity/20

	logger.Debugf("Denylist: cache capacity: %d bytes", maxCost)

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: maxCost * 10, // recommended value
		MaxCost:     maxCost,
		BufferItems: BufferItems,
	})
	if err != nil {
		return nil, err
	}

	var numOfElements int64
	totalEntries10P := totalEntries / 10

	// 10% counter
	counter10P := 0

	// tokens loading to the cache
	s := bufio.NewScanner(f)
	for s.Scan() {
		if s.Text() != "" {
			if ok := cache.Set(strings.TrimSpace(s.Text()), nil, ElementCost); ok {
				numOfElements += 1
				if numOfElements%totalEntries10P == 0 {
					counter10P += 10
					logger.Debugf("Denylist: loaded %d perecents of tokens. Total elements in the cache: %d", counter10P, numOfElements)
				}
			} else {
				logger.Errorf("Denylist: can't add the token to the cache: %s", s.Text())
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

	return &DeniedTokens{Cache: cache, ElementsNum: totalEntries}, nil
}
