package http

import (
	"fluxqueue/internal/api/http/middleware"
	"fluxqueue/pkg/validator"

	"github.com/labstack/echo/v4"
)

func InitServer() *echo.Echo {
	e := echo.New()
	middleware.Setup(e)
	e.Validator = validator.NewCustomValidator()
	return e
}
