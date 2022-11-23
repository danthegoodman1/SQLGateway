package pg

import (
	"context"
	"time"

	"github.com/danthegoodman1/SQLGateway/gologger"
	"github.com/danthegoodman1/SQLGateway/utils"
	"github.com/jackc/pgx/v4/pgxpool"
)

var (
	PGPool *pgxpool.Pool

	logger = gologger.NewLogger()
)

func ConnectToDB() error {
	logger.Debug().Msg("connecting to PG...")
	var err error
	config, err := pgxpool.ParseConfig(utils.PG_DSN)
	if err != nil {
		return err
	}

	config.MaxConns = int32(utils.PG_POOL_CONNS)
	config.MinConns = 1
	config.HealthCheckPeriod = time.Second * 5
	config.MaxConnLifetime = time.Minute * 30
	config.MaxConnIdleTime = time.Minute * 30

	PGPool, err = pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		return err
	}
	logger.Debug().Msg("connected to PG")
	return nil
}
