package http_server

import (
	"github.com/danthegoodman1/PSQLGateway/pg"
	"github.com/rs/zerolog"
	"net/http"
)

type (
	QueryRequest struct {
		Queries []*pg.QueryReq
	}

	QueryResponse struct {
		Queries []*pg.QueryRes `json:",omitempty"`
	}
)

func (s *HTTPServer) PostQuery(c *CustomContext) error {
	var body QueryRequest
	if err := ValidateRequest(c, &body); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	logger := zerolog.Ctx(c.Request().Context())
	logger.Info().Msg(c.Request().URL.Path)

	res := QueryResponse{
		Queries: make([]*pg.QueryRes, len(body.Queries)),
	}

	queryRes, err := pg.Query(c.Request().Context(), pg.PGPool, body.Queries)
	if err != nil {
		return c.InternalError(err, "error handling query")
	}

	for i, qr := range queryRes {
		res.Queries[i] = qr
	}

	return c.JSON(http.StatusOK, res)
}
