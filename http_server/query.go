package http_server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/danthegoodman1/SQLGateway/pg"
	"github.com/rs/zerolog"
	"io"
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
	defer c.Request().Body.Close()
	//logger := zerolog.Ctx(c.Request().Context())
	//logger.Info().Msg(c.Request().URL.Path)

	res := QueryResponse{
		Queries: make([]*pg.QueryRes, len(body.Queries)),
	}

	logger := zerolog.Ctx(c.Request().Context())
	if body.TxID != nil {
		logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Str("txID", *body.TxID)
		})
	}

	queryRes, remotePodURL, err := pg.Query(c.Request().Context(), pg.PGPool, body.Queries, body.TxID)
	if errors.Is(err, pg.ErrTxNotFound) {
		return c.String(http.StatusNotFound, "transaction not found, did it timeout?")
	}
	if errors.Is(err, pg.ErrTxOnRemotePod) {
		// Forward the request to the remote pod
		logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Str("remotePod", remotePodURL)
		})
		logger.Debug().Msg("remote transaction found, forwarding")

		bodyJSON, err := json.Marshal(body)
		if err != nil {
			return c.InternalError(err, "failed to marshal req body to JSON")
		}

		ctx, cancel := context.WithTimeout(c.Request().Context(), time.Second*30)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "POST", remotePodURL, bytes.NewReader(bodyJSON))
		if err != nil {
			return c.InternalError(err, "error making http request for remote pod")
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return c.InternalError(err, "error doing request to remote pod")
		}

		resBodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return c.InternalError(err, "error reading body bytes from remote pod response")
		}

		if res.StatusCode != 200 {
			return c.String(res.StatusCode, string(resBodyBytes))
		}
		return c.JSONBlob(http.StatusOK, resBodyBytes)
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
	var body TxIDJSON
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
