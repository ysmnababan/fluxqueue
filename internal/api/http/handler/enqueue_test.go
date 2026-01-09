package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fluxqueue/utils/validator"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
)

// --- custom binder used to inject values directly into ScheduleRequest ---
type injectBinder struct {
	// fields to inject
	idempotencyKey string
	runAt          time.Time
	payload        any
	errOnBind      error
}

func (b *injectBinder) Bind(i interface{}, c echo.Context) error {
	if b.errOnBind != nil {
		return b.errOnBind
	}
	req, ok := i.(*ScheduleRequest)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "wrong request type")
	}
	payload, _ := b.payload.(map[string]any)
	req.Type = "email"
	req.Payload = payload
	req.MaxRetries = 5
	req.IdempotencyKey = b.idempotencyKey
	req.RunAt = b.runAt
	return nil
}

// --- payload type that fails marshalling ---
type badJSONPayload struct{}

func (badJSONPayload) MarshalJSON() ([]byte, error) {
	return nil, errors.New("marshal failed intentionally")
}

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

func TestScheduleHandler_AllBranches(t *testing.T) {
	// helper to create an echo.Context using real HTTP request and recorder
	newContext := func(e *echo.Echo, body string) (echo.Context, *httptest.ResponseRecorder) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/schedule", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		return c, rec
	}

	t.Run("bind error - invalid JSON", func(t *testing.T) {
		redisMock := NewMockIRedisClient(t)
		h := NewHandler(redisMock)

		e := echo.New()
		c, _ := newContext(e, "not-a-json")

		err := h.Schedule(c)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error bind")
	})

	t.Run("validate error", func(t *testing.T) {
		redisMock := NewMockIRedisClient(t)
		h := NewHandler(redisMock)

		e := echo.New()
		// valid JSON but validator will return error
		runAt := time.Now().Add(time.Hour).UTC()
		bodyMap := map[string]any{
			"type":            "",
			"payload":         map[string]any{"to": "x@y.test"},
			"max_retries":     2,
			"idempotency_key": "some-key",
			"run_at":          runAt.Format(time.RFC3339),
		}
		bs, _ := json.Marshal(bodyMap)
		c, _ := newContext(e, string(bs))

		e.Validator = validator.NewCustomValidator()
		err := h.Schedule(c)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error validate")
	})

	t.Run("success - provided idempotency key", func(t *testing.T) {
		redisMock := NewMockIRedisClient(t)
		h := NewHandler(redisMock)

		e := echo.New()
		e.Validator = validator.NewCustomValidator()

		runAt := time.Now().Add(2 * time.Hour).UTC()
		bodyMap := map[string]any{
			"type":            "email",
			"payload":         map[string]any{"to": "a@b.test"},
			"max_retries":     3,
			"idempotency_key": "provided-key",
			"run_at":          runAt.Format(time.RFC3339),
		}
		bs, _ := json.Marshal(bodyMap)
		c, rec := newContext(e, string(bs))

		// Expect ZAdd called. Capture member string to verify it contains the task ID and runAt.
		var capturedMember string
		redisMock.On("ZAdd", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Run(func(args mock.Arguments) {
				if v, ok := args.Get(3).(string); ok {
					capturedMember = v
				}
			}).Once()

		err := h.Schedule(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "task_id")

		// ensure we pushed JSON that contains an ID which is a UUID
		var pushed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(capturedMember), &pushed))
		var idStr string
		if v, ok := pushed["ID"].(string); ok {
			idStr = v
		} else if v, ok := pushed["id"].(string); ok {
			idStr = v
		}
		_, err = uuid.Parse(idStr)
		assert.NoError(t, err)
		redisMock.AssertExpectations(t)
	})

	t.Run("success - no idempotency key (auto-generate)", func(t *testing.T) {
		redisMock := NewMockIRedisClient(t)
		h := NewHandler(redisMock)

		e := echo.New()

		e.Validator = validator.NewCustomValidator()

		runAt := time.Now().Add(30 * time.Minute).UTC()
		bodyMap := map[string]any{
			"type":        "job",
			"payload":     map[string]any{"foo": "bar"},
			"max_retries": 1,
			"run_at":      runAt.Format(time.RFC3339),
		}
		bs, _ := json.Marshal(bodyMap)
		c, rec := newContext(e, string(bs))

		var capturedMember string
		redisMock.On("ZAdd", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Run(func(args mock.Arguments) {
				if v, ok := args.Get(3).(string); ok {
					capturedMember = v
				}
			}).Once()

		err := h.Schedule(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "task_id")

		// confirm an id was generated inside pushed JSON
		var pushed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(capturedMember), &pushed))
		var idStr string
		if v, ok := pushed["ID"].(string); ok {
			idStr = v
		} else if v, ok := pushed["id"].(string); ok {
			idStr = v
		}
		_, err = uuid.Parse(idStr)
		assert.NoError(t, err)

		redisMock.AssertExpectations(t)
	})

	t.Run("redis push error", func(t *testing.T) {
		redisMock := NewMockIRedisClient(t)
		h := NewHandler(redisMock)

		e := echo.New()

		e.Validator = validator.NewCustomValidator()
		runAt := time.Now().Add(5 * time.Minute).UTC()
		bodyMap := map[string]any{
			"type":        "job",
			"payload":     map[string]any{"x": "y"},
			"max_retries": 1,
			"run_at":      runAt.Format(time.RFC3339),
		}
		bs, _ := json.Marshal(bodyMap)
		c, _ := newContext(e, string(bs))

		redisMock.On("ZAdd", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("redis down")).Once()

		err := h.Schedule(c)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error redis push")
		redisMock.AssertExpectations(t)
	})

	t.Run("json marshal error - payload implements json.Marshaler that returns error", func(t *testing.T) {
		redisMock := NewMockIRedisClient(t)
		h := NewHandler(redisMock)

		e := echo.New()
		// install a binder that injects a payload that will fail json.Marshal
		badRunAt := time.Now().Add(10 * time.Minute).UTC()
		e.Binder = &injectBinder{
			idempotencyKey: "", // empty so handler will generate one
			runAt:          badRunAt,
			payload: map[string]any{
				"bad": badJSONPayload{},
			},
		}
		// ensure validator passes
		e.Validator = validator.NewCustomValidator()

		// create an empty (unused) body; binder will inject values directly
		req := httptest.NewRequest(http.MethodPost, "/api/v1/schedule", strings.NewReader("{}"))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.Schedule(c)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error marshalling")
	})
}
