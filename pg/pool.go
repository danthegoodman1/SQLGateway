package pg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/danthegoodman1/SQLGateway/gologger"
	"github.com/danthegoodman1/SQLGateway/red"
	"github.com/danthegoodman1/SQLGateway/utils"
	"github.com/go-redis/redis/v9"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/rs/zerolog"
	"io"
	"net/http"
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
		Columns  [][]any `json:",omitempty"`
		Rows     [][]any `json:",omitempty"`
		Error    *string `json:",omitempty"`
		TimeNS   *int64  `json:",omitempty"`
		CacheHit *bool   `json:",omitempty"`
		Cached   *bool   `json:",omitempty"`
	}

	Queryable interface {
		Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
		Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
	}

	QueryRequest struct {
		Queries []*QueryReq
		TxID    *string
	}

	QueryResponse struct {
		Queries []*QueryRes `json:",omitempty"`
	}

	TxIDJSON struct {
		TxID string
	}

	DistributedError struct {
		Err        error
		Remote     bool
		StatusCode int
		ErrString  string
	}
)

var (
	ErrEndTx           = utils.PermError("end tx")
	ErrTxNotFoundLocal = errors.New("transaction not found on local pod, maybe the node restarted with the same name, or the transaction aborted")
)

func Query(ctx context.Context, pool *pgxpool.Pool, queries []*QueryReq, txID *string) (*QueryResponse, *DistributedError) {

	qres := &QueryResponse{
		Queries: make([]*QueryRes, len(queries)),
	}

	logger := zerolog.Ctx(ctx)
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	//if query.IgnoreCache == nil || *query.IgnoreCache == false {
	//	// TODO: Check cache
	//	res.CacheHit = utils.Ptr(true)
	//	// Return if cache hit
	//}
	s := time.Now()
	if txID != nil {
		logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Str("txID", *txID)
		})
		logger.Debug().Msg("transaction detected, handling queries in transaction")

		tx := Manager.GetTx(*txID)
		if tx == nil && red.RedisClient != nil {
			// Check for remote transaction
			txMeta, err := red.GetTransaction(ctx, *txID)
			if errors.Is(err, redis.Nil) {
				return nil, &DistributedError{
					Err: ErrTxNotFound,
				}
			}
			if err != nil {
				return nil, &DistributedError{
					Err: fmt.Errorf("error in red.GetTransaction: %w", err),
				}
			}

			if txMeta.PodID == utils.POD_NAME {
				// The only case would be if this node restarted but maintained the same name, without removing transactions from redis
				return nil, &DistributedError{Err: ErrTxNotFoundLocal}
			}

			logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
				return c.Str("remoteURL", txMeta.PodURL)
			})
			logger.Debug().Msg("remote transaction found, forwarding")

			bodyJSON, err := json.Marshal(QueryRequest{
				Queries: queries,
				TxID:    txID,
			})
			if err != nil {
				return nil, &DistributedError{Err: fmt.Errorf("error in json.Marhsal for remote request body: %w", err)}
			}

			ctx, cancel := context.WithTimeout(ctx, time.Second*30)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s://%s/psql/query", utils.GetHTTPPrefix(), txMeta.PodURL), bytes.NewReader(bodyJSON))
			if err != nil {
				return nil, &DistributedError{Err: fmt.Errorf("error making http request for remote pod: %w", err)}
			}
			req.Header.Set("content-type", "application/json")

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				return nil, &DistributedError{Err: fmt.Errorf("error doing request to remote pod: %w", err)}
			}

			resBodyBytes, err := io.ReadAll(res.Body)
			if err != nil {
				return nil, &DistributedError{Err: fmt.Errorf("error reading body bytes from remote pod response: %w", err)}
			}

			if res.StatusCode != 200 {
				return nil, &DistributedError{Remote: true, StatusCode: res.StatusCode, ErrString: string(resBodyBytes)}
			}

			err = json.Unmarshal(resBodyBytes, &qres)
			if err != nil {
				return nil, &DistributedError{Err: fmt.Errorf("error in json.Unmarhsal for remote response body: %w", err)}
			}

			return qres, nil
		} else if tx == nil {
			logger.Debug().Msgf("transaction %s not found", *txID)
			return nil, &DistributedError{Err: ErrTxNotFound}
		}

		res, err := tx.RunQueries(ctx, queries)
		if err != nil {
			logger.Debug().Msg("error found when running queries in transaction, rolling back")
			err := Manager.RollbackTx(ctx, *txID)
			if err != nil {
				return qres, err
			}
		}
		qres.Queries = res
		return qres, nil
	}

	var queryErr error
	// If single item, don't do in tx
	if len(queries) == 1 {
		queryErr = utils.ReliableExec(ctx, pool, time.Second*60, func(ctx context.Context, conn *pgxpool.Conn) error {
			queryRes := runQuery(ctx, conn, utils.Deref(queries[0].Exec, false), queries[0].Statement, queries[0].Params)
			qres.Queries[0] = queryRes
			if queryRes.Error != nil {
				return ErrEndTx
			}
			return nil
		})
	} else {
		queryErr = utils.ReliableExecInTx(ctx, pool, 60*time.Second, func(ctx context.Context, conn pgx.Tx) (err error) {
			for i, query := range queries {
				queryRes := runQuery(ctx, conn, utils.Deref(query.Exec, false), query.Statement, query.Params)
				qres.Queries[i] = queryRes
				if queryRes.Error != nil {
					return ErrEndTx
				}
			}
			return nil
		})
	}

	if queryErr != nil && !errors.Is(queryErr, ErrEndTx) {
		return nil, &DistributedError{Err: fmt.Errorf("error in transaction execution: %w", queryErr)}
	}

	if utils.TRACES {
		logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
			// metrics
			actionLogs := make([]gologger.Action, 0)

			for i, queryRes := range qres.Queries {
				if queryRes == nil {
					continue
				}
				execd := utils.Deref(queries[i].Exec, false)
				logger.Debug().Interface("queryRes", queryRes).Msg("got query res")
				actionLog := gologger.Action{
					DurationNS: *queryRes.TimeNS,
					Statement:  queries[i].Statement,
					Exec:       execd,
				}
				if !execd {
					actionLog.NumRows = utils.Ptr(len(queryRes.Rows))
				}

				actionLogs = append(actionLogs, actionLog)
			}

			return c.Interface("query_handler", gologger.Action{
				DurationNS: time.Since(s).Nanoseconds(),
			}).Interface("queries", actionLogs)
		})
	}
	return qres, nil
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
	defer func() {
		res.TimeNS = utils.Ptr(time.Since(s).Nanoseconds())
	}()

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

	return
}

//func ShouldCache(ignoreCache, forceCache *bool) bool {
//	if ignoreCache != nil {
//		return *ignoreCache
//	}
//	if forceCache != nil {
//		return *forceCache
//	}
//
//	return utils.CACHE_DEFAULT
//}

//func CRDBIsSelectOnly(statement string) (selectOnly bool, err error) {
//	ast, err := parser.ParseOne(statement)
//	if err != nil {
//		return false, fmt.Errorf("error in parser.ParseOne: %w", err)
//	}
//
//	return ast.AST.StatementTag() == "SELECT", nil
//}
