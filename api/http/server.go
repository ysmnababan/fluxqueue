package http

import (
	"fluxqueue/pkg/validator"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func InitServer(e *echo.Echo) {
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Validator = validator.NewCustomValidator()
}
