package pg

import (
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"sync"
	"time"
)

type (
	Tx struct {
		ID         string
		PoolConn   *pgxpool.Conn
		Tx         pgx.Tx
		Expires    time.Time
		CancelChan chan bool
		Exited     bool
		PoolMu     *sync.Mutex
	}
)
