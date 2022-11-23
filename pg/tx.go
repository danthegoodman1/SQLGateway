package pg

import (
	"context"
	"errors"
	"github.com/danthegoodman1/SQLGateway/utils"
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

var (
	ErrTxError = errors.New("transaction error")
)

func (tx *Tx) RunQueries(ctx context.Context, queries []*QueryReq) ([]*QueryRes, error) {
	tx.PoolMu.Lock()
	defer tx.PoolMu.Unlock()

	res := make([]*QueryRes, len(queries))
	for i, query := range queries {
		queryRes := runQuery(ctx, tx.PoolConn, utils.Deref(query.Exec, false), query.Statement, query.Params)
		res[i] = queryRes
		if queryRes.Error != nil {
			return res, ErrTxError
		}
	}
	return res, nil
}
