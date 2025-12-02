package handler

import (
	"fluxqueue/internal/api/http/response"
	"fmt"

	"github.com/labstack/echo/v4"
)

type handler struct {
}

func NewHandler() *handler {
	h := &handler{}
	return h
}

func (h *handler) Enqueue(c echo.Context) error {
	// logger := zerolog.Ctx(c.Request().Context())
	// logger.Info().Msg("enqueue request")
	req := new(EnqueueRequest)
	if err := c.Bind(req); err != nil {
		return response.Wrap(response.ErrBadRequest, fmt.Errorf("error bind: %w", err))
	}
	if err := c.Validate(req); err != nil {
		return response.Wrap(response.ErrBadRequest, fmt.Errorf("error validate: %w", err))
	}
	return response.WithStatusOKResponse("ok", c)
}
