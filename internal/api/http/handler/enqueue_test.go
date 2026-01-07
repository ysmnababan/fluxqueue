package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fluxqueue/utils/validator"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
)

func TestEnqueue(t *testing.T) {
	// common valid JSON body to use in successful paths.
	makeValidBody := func(idempotencyKey string) string {
		// struct shape must match your EnqueueRequest json tags.
		// fields used by the handler: Type, Payload, MaxRetries, IdempotencyKey
		req := map[string]any{
			"type":            "email",
			"payload":         map[string]any{"to": "a@b.test"},
			"max_retries":     3,
			"idempotency_key": idempotencyKey,
		}
		b, _ := json.Marshal(req)
		return string(b)
	}

	t.Run("bind error - invalid JSON", func(t *testing.T) {
		redisMock := NewMockIRedisClient(t)
		h := NewHandler(redisMock) // uses the user's constructor

		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/enqueue", strings.NewReader("not-a-json"))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.Enqueue(c)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error bind")
	})

	t.Run("validate error - validator returns error", func(t *testing.T) {
		redisMock := NewMockIRedisClient(t)
		h := NewHandler(redisMock)

		e := echo.New()
		// install validator that returns error to simulate validation failure
		// e.Validator = &mockValidator{err: errors.New("invalid payload")}
		e.Validator = validator.NewCustomValidator()

		body := `{"key": "value"} `
		req := httptest.NewRequest(http.MethodPost, "/api/v1/enqueue", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Use handler bound to echo instance that has the failing validator
		// If your handler stores echo instance internally, ensure NewHandler doesn't override it.
		// Here we just call the method with the context that references e.
		err := h.Enqueue(c)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error validate")
	})

	t.Run("success - provided idempotency key", func(t *testing.T) {
		redisMock := NewMockIRedisClient(t)
		h := NewHandler(redisMock)

		e := echo.New()
		e.Validator = validator.NewCustomValidator()

		body := makeValidBody("my-idempotency-key")
		req := httptest.NewRequest(http.MethodPost, "/api/v1/enqueue", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Expect Redis LPush to be called. We accept any key/value to avoid depending on h.queueReady name.
		redisMock.On("LPush", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

		err := h.Enqueue(c)
		assert.NoError(t, err)
		// handler should have written 200 OK and a body containing the produced task id
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "task_id")

		redisMock.AssertExpectations(t)
	})

	t.Run("success - no idempotency key (auto-generate)", func(t *testing.T) {
		redisMock := NewMockIRedisClient(t)
		h := NewHandler(redisMock)

		e := echo.New()
		e.Validator = validator.NewCustomValidator()

		body := makeValidBody("") // empty idempotency key should trigger generation
		req := httptest.NewRequest(http.MethodPost, "/api/v1/enqueue", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Capture the pushed JSON so we can assert that an id was generated and present.
		var capturedValue string
		redisMock.On("LPush", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			// args[2] is the value (stringified JSON)
			if v, ok := args.Get(2).(string); ok {
				capturedValue = v
			}
		}).Once()

		err := h.Enqueue(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "task_id")

		// Try to parse the captured value (should be JSON with "id" field present)
		var pushed map[string]any
		if assert.NotEmpty(t, capturedValue) {
			err := json.Unmarshal([]byte(capturedValue), &pushed)
			assert.NoError(t, err)
			// field name in the struct is ID; JSON tag might be "id" or "ID" depending on your model,
			// try both checks: ensure some form of id is present and looks like a UUID.
			var idStr string
			if v, ok := pushed["ID"].(string); ok {
				idStr = v
			} else if v, ok := pushed["id"].(string); ok {
				idStr = v
			}
			assert.NotEmpty(t, idStr)
			_, err = uuid.Parse(idStr)
			assert.NoError(t, err) // id should be a valid uuid
		}

		redisMock.AssertExpectations(t)
	})

	t.Run("redis push error - handler returns wrapped error", func(t *testing.T) {
		redisMock := NewMockIRedisClient(t)
		h := NewHandler(redisMock)

		e := echo.New()
		e.Validator = validator.NewCustomValidator()

		body := makeValidBody("some-key")
		req := httptest.NewRequest(http.MethodPost, "/api/v1/enqueue", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		redisMock.On("LPush", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("redis down")).Once()

		err := h.Enqueue(c)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error redis push")
		redisMock.AssertExpectations(t)
	})
}
