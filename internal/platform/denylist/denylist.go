package denylist

import (
	"bufio"
	"io"
	"os"

	"github.com/dgraph-io/ristretto"
	"github.com/sirupsen/logrus"
	"github.com/wallarm/api-firewall/internal/config"
)

type DeniedTokens struct {
	Cache       *ristretto.Cache
	ElementsNum int
}

func New(cfg *config.APIFWConfiguration, logger *logrus.Logger) (*DeniedTokens, error) {

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: cfg.Denylist.Cache.NumCounters,
		MaxCost:     cfg.Denylist.Cache.MaxCost,
		BufferItems: cfg.Denylist.Cache.BufferItems,
	})
	if err != nil {
		return nil, err
	}

	totalEntries := 0

	// Loading tokens to the cache
	if cfg.Denylist.Tokens.File != "" {

		f, err := os.Open(cfg.Denylist.Tokens.File)
		if err != nil {
			return nil, err
		}

		// count non-empty entries
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

		if _, err = f.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}

		logger.Debugf("Denylist: total entries (lines) found in the file: %d", totalEntries)

		totalEntries10P := totalEntries / 10
		numOfElements := 0
		current10P := 0
		s := bufio.NewScanner(f)
		for s.Scan() {
			if s.Text() != "" {
				if ok := cache.Set(s.Text(), nil, 1); ok {
					numOfElements += 1
					if numOfElements%totalEntries10P == 0 {
						current10P += 10
						logger.Debugf("Denylist: loaded %d perecents of tokens. Total elements in the cache: %d", current10P, numOfElements)
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
	}

	return &DeniedTokens{Cache: cache, ElementsNum: totalEntries}, nil
}
