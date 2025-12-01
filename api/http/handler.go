package http

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type handler struct {
}

func NewHandler() *handler {
	h := &handler{}
	return h
}

func (h *handler) Enqueue(c echo.Context) error {
	req := new(EnqueueRequest)
	if err := c.Bind(req); err != nil {
		return c.String(http.StatusBadRequest, "bad request")
	}
	if err := c.Validate(req); err != nil {
		return c.String(http.StatusBadRequest, "validate failed")
	}
	return c.JSON(200, map[string]string{"message": "success"})
}
