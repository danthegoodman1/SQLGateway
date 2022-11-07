package http_server

import (
	"context"
	"fmt"
	"github.com/danthegoodman1/PSQLGateway/pg"
	"github.com/danthegoodman1/PSQLGateway/utils"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	pg_query "github.com/pganalyze/pg_query_go/v2"
	"github.com/rs/zerolog"
	"net/http"
	"time"
)

type (
	QueryRequest struct {
		Queries []*ReqQuery
	}

	ReqQuery struct {
		Statement   string
		Params      []any
		IgnoreCache *bool
	}

	ResQuery struct {
		Rows     [][]any        `json:",omitempty"`
		Error    *string        `json:",omitempty"`
		Time     *time.Duration `json:",omitempty"`
		CacheHit *bool          `json:",omitempty"`
		Cached   *bool          `json:",omitempty"`
	}

	QueryResponse struct {
		Queries []*ResQuery `json:",omitempty"`
	}
)

func (s *HTTPServer) PostQuery(c *CustomContext) error {
	var body QueryRequest
	if err := ValidateRequest(c, &body); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	logger := zerolog.Ctx(c.Request().Context())
	logger.Info().Msg(c.Request().URL.Path)

	res, err := HandleQueryRequest(c.Request().Context(), body, c.Request().URL.String() == "/exec")
	if err != nil {
		return c.InternalError(err, "error handling query request")
	}

	return c.JSON(http.StatusOK, res)
}

func HandleQueryRequest(ctx context.Context, req QueryRequest, exec bool) (QueryResponse, error) {
	logger := zerolog.Ctx(ctx)
	logger.Debug().Msg("handling query request")

	qr := QueryResponse{
		Queries: make([]*ResQuery, len(req.Queries)),
	}

QueryLoop:
	for i, query := range req.Queries {
		row := &ResQuery{
			Rows: [][]any{},
		}
		qr.Queries[i] = row

		s := time.Now()
		if query.IgnoreCache == nil || *query.IgnoreCache == false {
			// TODO: Check cache
			row.CacheHit = utils.Ptr(true)
			// Return if cache hit
		}

		var rows pgx.Rows
		err := utils.ReliableExec(ctx, pg.PGPool, 30*time.Second, func(ctx context.Context, conn *pgxpool.Conn) (err error) {
			rows, err = conn.Query(ctx, query.Statement, query.Params...)
			return err
		})
		row.Time = utils.Ptr(time.Since(s))
		if err != nil {
			row.Error = utils.Ptr(err.Error())
		} else if !exec {
			// Get columns
			colNames := make([]any, len(rows.FieldDescriptions()))
			for i, desc := range rows.FieldDescriptions() {
				colNames[i] = string(desc.Name)
			}
			row.Rows = append(row.Rows, colNames)

			// Get row values
			for rows.Next() {
				rowVals, err := rows.Values()
				if err != nil {
					row.Error = utils.Ptr(err.Error())
					continue QueryLoop
				}
				row.Rows = append(row.Rows, rowVals)
			}

			selectOnly, err := IsSelectOnly(query.Statement)
			if err != nil {
				logger.Error().Err(err).Str("statement", query.Statement).Msg("error checking if select only")
			} else if selectOnly && query.IgnoreCache == nil || *query.IgnoreCache == false {
				// TODO: Cache result
				row.Cached = utils.Ptr(true)
			}
		}
	}

	return qr, nil
}

func IsSelectOnly(statement string) (selectOnly bool, err error) {
	result, err := pg_query.Parse(statement)
	if err != nil {
		err = fmt.Errorf("error in pg_query.Parse: %w", err)
		return
	}
	for _, stmt := range result.Stmts {
		if stmt.Stmt.GetSelectStmt() == nil {
			return
		}
	}

	return true, nil
}
