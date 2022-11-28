package cache

import (
	"fmt"
	"github.com/danthegoodman1/SQLGateway/gologger"
	"github.com/danthegoodman1/SQLGateway/utils"
	"github.com/dgraph-io/ristretto"
)

var (
	logger = gologger.NewLogger()

	// Megabyte
	mb int64 = 1_048_576
)

type (
	CacheManager struct {
		memCache *ristretto.Cache
	}
)

func NewCacheManager() (*CacheManager, error) {
	cm := &CacheManager{}

	if utils.REDIS_ADDR == "" {
		// use memory cache
		logger.Debug().Msg("setting up in-memory cache")
		cache, err := ristretto.NewCache(&ristretto.Config{
			NumCounters: utils.CACHE_MAX_VALUES,
			MaxCost:     mb * utils.CACHE_MAX_COST_MB,
			BufferItems: 64,
		})
		if err != nil {
			return nil, fmt.Errorf("error in ristretto.NewCache: %w", err)
		}
		cm.memCache = cache
	} else {
		logger.Debug().Msg("Redis detected, using Redis for cache")
	}
}

func (cm *CacheManager) Set() {

}

func (cm *CacheManager) Get() {

}

func (cm *CacheManager) Shutdown() error {

}
