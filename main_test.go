package main

import (
	"context"
	"github.com/danthegoodman1/SQLGateway/pg"
	"github.com/danthegoodman1/SQLGateway/red"
	"github.com/danthegoodman1/SQLGateway/utils"
	"os"
	"testing"
)

func TestMain(t *testing.M) {
	if err := pg.ConnectToDB(); err != nil {
		logger.Error().Err(err).Msg("error connecting to PG Pool")
		os.Exit(1)
	}

	if utils.REDIS_ADDR != "" {
		if err := red.ConnectRedis(); err != nil {
			logger.Error().Err(err).Msg("error connecting to Redis")
			os.Exit(1)
		}
	}

	c := t.Run()

	pg.PGPool.Close()
	err := red.Shutdown(context.Background())
	if err != nil {
		logger.Error().Err(err).Msg("error shutting down to Redis")
		os.Exit(1)
	}
	logger.Debug().Msg("done tests")
	os.Exit(c)
}
