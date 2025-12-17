package middleware

import (
	"runtime/debug"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

func Recover(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		defer func(c echo.Context) {
			if r := recover(); r != nil {
				stackTrace := debug.Stack()
				log.Error().Any("error", r).RawJSON("stackTrace", stackTrace).Send()

				err := c.JSON(500, map[string]any{
					"message": "something went wrong",
				})
				if err != nil {
					log.Error().Err(err)
				}
			}
		}(c)

		return next(c)
	}
}
