package http_server

import (
	"context"
	"errors"
	"github.com/danthegoodman1/PSQLGateway/pg"
	"net/http"
	"time"
)

type (
	QueryRequest struct {
		Queries []*pg.QueryReq
		TxID    *string
	}

	QueryResponse struct {
		Queries []*pg.QueryRes `json:",omitempty"`
	}

	TxIDJSON struct {
		TxID string
	}
)

func (s *HTTPServer) PostQuery(c *CustomContext) error {
	var body QueryRequest
	if err := ValidateRequest(c, &body); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	//logger := zerolog.Ctx(c.Request().Context())
	//logger.Info().Msg(c.Request().URL.Path)

	res := QueryResponse{
		Queries: make([]*pg.QueryRes, len(body.Queries)),
	}

	queryRes, err := pg.Query(c.Request().Context(), pg.PGPool, body.Queries, body.TxID)
	if errors.Is(err, pg.ErrTxNotFound) {
		return c.String(http.StatusNotFound, "transaction not found, did it timeout?")
	}
	if err != nil {
		return c.InternalError(err, "error handling query")
	}

	for i, qr := range queryRes {
		res.Queries[i] = qr
	}

	return c.JSON(http.StatusOK, res)
}

func (s *HTTPServer) PostBegin(c *CustomContext) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	txID, err := pg.Manager.NewTx(ctx)
	if errors.Is(err, context.DeadlineExceeded) {
		return c.String(http.StatusRequestTimeout, "timed out waiting for free pool connection")
	}
	if err != nil {
		return c.InternalError(err, "error creating new transaction")
	}

	return c.JSON(http.StatusOK, TxIDJSON{
		TxID: txID,
	})
}

func (s *HTTPServer) PostCommit(c *CustomContext) error {
	var body TxIDJSON
	if err := ValidateRequest(c, &body); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	err := pg.Manager.CommitTx(ctx, body.TxID)
	if errors.Is(err, context.DeadlineExceeded) {
		return c.String(http.StatusRequestTimeout, "timed out waiting for free pool connection")
	}
	if err != nil {
		return c.InternalError(err, "error creating new transaction")
	}

	return c.NoContent(http.StatusOK)
}

func (s *HTTPServer) PostRollback(c *CustomContext) error {
	var body TxIDJSON
	if err := ValidateRequest(c, &body); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	err := pg.Manager.RollbackTx(ctx, body.TxID)
	if errors.Is(err, context.DeadlineExceeded) {
		return c.String(http.StatusRequestTimeout, "timed out waiting for free pool connection")
	}
	if err != nil {
		return c.InternalError(err, "error creating new transaction")
	}

	return c.NoContent(http.StatusOK)
}
