package http_server

import (
	"context"
	"errors"
	"github.com/danthegoodman1/SQLGateway/pg"
	"github.com/rs/zerolog"
	"net/http"
	"time"
)

func (s *HTTPServer) PostQuery(c *CustomContext) error {
	var body pg.QueryRequest
	if err := ValidateRequest(c, &body); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	defer c.Request().Body.Close()
	//logger := zerolog.Ctx(c.Request().Context())
	//logger.Info().Msg(c.Request().URL.Path)

	logger := zerolog.Ctx(c.Request().Context())
	if body.TxID != nil {
		logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Str("txID", *body.TxID)
		})
	}

	res, err := pg.Query(c.Request().Context(), pg.PGPool, body.Queries, body.TxID)
	if err != nil {
		if errors.Is(err.Err, pg.ErrTxNotFound) {
			return c.String(http.StatusNotFound, "transaction not found, did it timeout?")
		}
		if errors.Is(err.Err, pg.ErrTxNotFoundLocal) {
			return c.String(http.StatusNotFound, err.Err.Error())
		}
		if err.Err != nil {
			return c.InternalError(err.Err, "error handling query")
		}
		if err.Remote {
			return c.String(err.StatusCode, err.ErrString)
		}
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

	return c.JSON(http.StatusOK, pg.TxIDJSON{
		TxID: txID,
	})
}

func (s *HTTPServer) PostCommit(c *CustomContext) error {
	var body pg.TxIDJSON
	if err := ValidateRequest(c, &body); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	defer c.Request().Body.Close()

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
	var body pg.TxIDJSON
	if err := ValidateRequest(c, &body); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	defer c.Request().Body.Close()

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
