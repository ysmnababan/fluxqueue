package validator

import (
	"net/http"
	"time"

	valid "github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type customValidator struct {
	validator *valid.Validate
}

func NewCustomValidator() *customValidator {
	newValidator := valid.New()

	// add custom translation
	_ = newValidator.RegisterValidation("is-date", validateIsDate)

	return &customValidator{
		validator: newValidator,
	}
}

func (cv *customValidator) Validate(i interface{}) error {
	err := cv.validator.Struct(i)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

func validateIsDate(fl valid.FieldLevel) bool {
	field := fl.Field().String()
	if field == "" {
		return true
	}
	// Try parsing with date and time format first
	if _, err := time.Parse("2006-01-02 15:04:05", field); err == nil {
		return true
	}
	// Fallback to date only
	if _, err := time.Parse("2006-01-02", field); err == nil {
		return true
	}
	return false
}
