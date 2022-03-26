package blacklist

import (
	"bufio"
	"io"
	"os"

	"github.com/dgraph-io/ristretto"
	"github.com/sirupsen/logrus"
	"github.com/wallarm/api-firewall/internal/config"
)

type BlacklistedTokens struct {
	Cache       *ristretto.Cache
	ElementsNum int
}

func New(cfg *config.APIFWConfiguration, logger *logrus.Logger) (*BlacklistedTokens, error) {

	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: cfg.Blacklist.Cache.NumCounters,
		MaxCost:     cfg.Blacklist.Cache.MaxCost,
		BufferItems: cfg.Blacklist.Cache.BufferItems,
	})
	if err != nil {
		return nil, err
	}

	totalEntries := 0

	// Loading tokens to the cache
	if cfg.Blacklist.Tokens.File != "" {

		f, err := os.Open(cfg.Blacklist.Tokens.File)
		if err != nil {
			return nil, err
		}

		// count entries
		c := bufio.NewScanner(f)
		for c.Scan() {
			totalEntries += 1
		}
		err = c.Err()
		if err != nil {
			return nil, err
		}

		if _, err = f.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}

		logger.Debugf("Blacklist: total entries (lines) found in the file: %d", totalEntries)

		totalEntries10P := totalEntries / 10
		numOfElements := 0
		current10P := 0
		s := bufio.NewScanner(f)
		for s.Scan() {
			if ok := cache.Set(s.Text(), nil, 1); ok {
				numOfElements += 1
				if numOfElements%totalEntries10P == 0 {
					current10P += 10
					logger.Debugf("Blacklist: loaded %d perecents of tokens. Total elements in the cache: %d", current10P, numOfElements)
				}
			} else {
				logger.Errorf("Blacklist: can't add the token to the cache: %s", s.Text())
			}
			cache.Wait()
		}
		err = s.Err()
		if err != nil {
			return nil, err
		}

		if err := f.Close(); err != nil {
			return nil, err
		}
	}

	return &BlacklistedTokens{Cache: cache, ElementsNum: totalEntries}, nil
}
