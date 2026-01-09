// Package middleware provides HTTP middleware for logging, recovery, CORS, and error handling.
package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"
)

func Setup(e *echo.Echo) {
	e.Use(
		Recover,
		echoMiddleware.CORSWithConfig(echoMiddleware.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
		}),
		EmbedCtxWithLog,
		middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
			LogURI:    true,
			LogStatus: true,
			LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
				log.Info().
					Str("URI", v.URI).
					Int("status", v.Status).Msg("request")
				return nil
			},
		}),
	)
	e.HTTPErrorHandler = CustomHTTPErrorHandler
}
