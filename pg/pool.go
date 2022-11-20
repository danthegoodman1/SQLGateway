package pg

import (
	"context"
	"errors"
	"fmt"
	"github.com/cockroachdb/cockroachdb-parser/pkg/sql/parser"
	"github.com/danthegoodman1/PSQLGateway/utils"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/rs/zerolog"
	"time"
)

type (
	QueryReq struct {
		Statement   string
		Params      []any
		IgnoreCache *bool
		ForceCache  *bool
		Exec        *bool
		TxKey       *string
	}

	QueryRes struct {
		Columns  [][]any        `json:",omitempty"`
		Rows     [][]any        `json:",omitempty"`
		Error    *string        `json:",omitempty"`
		Time     *time.Duration `json:",omitempty"`
		CacheHit *bool          `json:",omitempty"`
		Cached   *bool          `json:",omitempty"`
	}

	Queryable interface {
		Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
		Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	}
)

var (
	ErrEndTx = utils.PermError("end tx")
)

func Query(ctx context.Context, pool *pgxpool.Pool, queries []*QueryReq) ([]*QueryRes, error) {
	res := make([]*QueryRes, len(queries))

	//if query.IgnoreCache == nil || *query.IgnoreCache == false {
	//	// TODO: Check cache
	//	res.CacheHit = utils.Ptr(true)
	//	// Return if cache hit
	//}

	var queryErr error
	// If single item, don't do in tx
	if len(queries) == 1 {
		queryErr = utils.ReliableExec(ctx, pool, time.Second*60, func(ctx context.Context, conn *pgxpool.Conn) error {
			queryRes := runQuery(ctx, conn, utils.Deref(queries[0].Exec, false), queries[0].Statement, queries[0].Params)
			res[0] = queryRes
			if queryRes.Error != nil {
				return ErrEndTx
			}
			return nil
		})
	} else {
		queryErr = utils.ReliableExecInTx(ctx, pool, 60*time.Second, func(ctx context.Context, conn pgx.Tx) (err error) {
			for i, query := range queries {
				queryRes := runQuery(ctx, conn, utils.Deref(query.Exec, false), query.Statement, query.Params)
				res[i] = queryRes
				if queryRes.Error != nil {
					return ErrEndTx
				}
			}
			return nil
		})
	}

	if queryErr != nil && !errors.Is(queryErr, ErrEndTx) {
		return nil, fmt.Errorf("error in transaction execution: %w", queryErr)
	}

	return res, nil
}

func runQuery(ctx context.Context, q Queryable, exec bool, statement string, params []any) (res *QueryRes) {
	res = &QueryRes{
		Rows: make([][]any, 0),
	}

	logger := zerolog.Ctx(ctx)
	logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.Str("statement", statement)
	})

	s := time.Now()

	if exec {
		_, err := q.Exec(ctx, statement, params...)
		if err != nil {
			res.Error = utils.Ptr(err.Error())
			logger.Warn().Err(err).Msg("got exec error")
		}
	} else {
		// Get columns
		rows, err := q.Query(ctx, statement, params...)
		if err != nil {
			res.Error = utils.Ptr(err.Error())
			logger.Warn().Err(err).Msg("got query error")
			return
		}
		colNames := make([]any, len(rows.FieldDescriptions()))
		for i, desc := range rows.FieldDescriptions() {
			colNames[i] = string(desc.Name)
		}
		res.Columns = append(res.Columns, colNames)

		// Get res values
		for rows.Next() {
			rowVals, err := rows.Values()
			if err != nil {
				res.Error = utils.Ptr(err.Error())
				return
			}
			res.Rows = append(res.Rows, rowVals)
		}
	}

	//selectOnly, err := CRDBIsSelectOnly(query.Statement)
	//if err != nil {
	//	return nil, fmt.Errorf("error checking if select only: %w", err)
	//} else if selectOnly && ShouldCache(query.IgnoreCache, query.ForceCache) {
	//	// TODO: Cache result
	//	res.Cached = utils.Ptr(true)
	//}

	res.Time = utils.Ptr(time.Since(s))
	return
}

func ShouldCache(ignoreCache, forceCache *bool) bool {
	if ignoreCache != nil {
		return *ignoreCache
	}
	if forceCache != nil {
		return *forceCache
	}

	return utils.CACHE_DEFAULT
}

func CRDBIsSelectOnly(statement string) (selectOnly bool, err error) {
	ast, err := parser.ParseOne(statement)
	if err != nil {
		return false, fmt.Errorf("error in parser.ParseOne: %w", err)
	}

	return ast.AST.StatementTag() == "SELECT", nil
}
