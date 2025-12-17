package middleware

import (
	"errors"
	"fluxqueue/internal/api/http/response"
	"fluxqueue/internal/config"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

func CustomHTTPErrorHandler(err error, c echo.Context) {
	ctx := c.Request().Context()
	logger := zerolog.Ctx(ctx)

	var apiErr *response.APIError
	if errors.As(err, &apiErr) {
		logger.Error().
			Err(err).
			Int("status_code", apiErr.Code).
			Msg(apiErr.Message)

		var detail any = apiErr.Err.Error()
		if config.Global.Server.Env == "PRODUCTION" && apiErr.Code == http.StatusInternalServerError {
			detail = nil
		}
		err = c.JSON(apiErr.Code, response.APIResponse{
			Meta: response.Meta{
				Success:    false,
				Message:    apiErr.Message,
				StatusCode: apiErr.StatusCode,
				Detail:     detail,
			},
		})
		if err != nil {
			logger.Error().Err(err)
		}
		return
	}

	logger.Error().
		Err(err).
		Str("path", c.Path()).
		Msg("unhandled internal error")
	err = c.JSON(http.StatusInternalServerError,
		response.APIResponse{
			Meta: response.Meta{
				Success:    false,
				Message:    response.ErrInternalServerError.Message,
				StatusCode: response.ErrInternalServerError.StatusCode,
			},
		})
	if err != nil {
		logger.Error().Err(err)
	}
}
