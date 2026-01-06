// Package handler provides HTTP request handlers for task enqueue and scheduling endpoints.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"fluxqueue/internal/api/http/response"
	"fluxqueue/internal/model"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type IRedisClient interface {
	LPush(ctx context.Context, key string, value string) error
	ZAdd(ctx context.Context, key string, score float64, member string) error
}

type handler struct {
	redis          IRedisClient
	queueReady     string
	queueScheduled string
}

func NewHandler(redis IRedisClient) *handler {
	h := &handler{
		redis:          redis,
		queueReady:     "queue:ready",
		queueScheduled: "queue:scheduled",
	}
	return h
}

// @Summary Enqueue
// @Description Enqueue
// @Tags ingestor
// @Accept json
// @Produce json
// @Param req body handler.EnqueueRequest true "change this description"
// @Success 200 {object} default.Success "success"
// @Failure 500 {object} default.Failure "error"
// @Failure 400 {object} default.Failure "error"
// @Failure 404 {object} default.Failure "error"
// @Router /api/v1/enqueue [post]
func (h *handler) Enqueue(c echo.Context) error {
	req := new(EnqueueRequest)
	if err := c.Bind(req); err != nil {
		return response.Wrap(response.ErrBadRequest, fmt.Errorf("error bind: %w", err))
	}
	if err := c.Validate(req); err != nil {
		return response.Wrap(response.ErrBadRequest, fmt.Errorf("error validate: %w", err))
	}
	now := time.Now().UTC()
	idempKey := req.IdempotencyKey
	if len(idempKey) == 0 {
		idempKey = uuid.NewString()
	}
	task := &model.Task{
		ID:             uuid.NewString(),
		Type:           req.Type,
		Payload:        req.Payload,
		Attempts:       0,
		MaxRetries:     req.MaxRetries,
		IdempotencyKey: idempKey,
		CreatedAt:      now,
	}

	data, err := json.Marshal(task)
	if err != nil {
		return response.Wrap(response.ErrInternalServerError, fmt.Errorf("error marshalling: %w", err))
	}
	if err := h.redis.LPush(c.Request().Context(), h.queueReady, string(data)); err != nil {
		return response.Wrap(response.ErrInternalServerError, fmt.Errorf("error redis push: %w", err))
	}

	return response.WithStatusOKResponse(
		map[string]any{
			"task_id": task.ID,
		}, c)
}

// @Summary Schedule
// @Description Schedule
// @Tags ingestor
// @Accept json
// @Produce json
// @Param req body handler.ScheduleRequest true "change this description"
// @Success 200 {object} default.Success "success"
// @Failure 500 {object} default.Failure "error"
// @Failure 400 {object} default.Failure "error"
// @Failure 404 {object} default.Failure "error"
// @Router /api/v1/schedule [post]
func (h *handler) Schedule(c echo.Context) error {
	req := new(ScheduleRequest)
	if err := c.Bind(req); err != nil {
		return response.Wrap(response.ErrBadRequest, fmt.Errorf("error bind: %w", err))
	}
	if err := c.Validate(req); err != nil {
		return response.Wrap(response.ErrBadRequest, fmt.Errorf("error validate: %w", err))
	}

	idempKey := req.IdempotencyKey
	if len(idempKey) == 0 {
		idempKey = uuid.NewString()
	}
	now := time.Now().UTC()
	runAt := req.RunAt.UTC()
	task := &model.Task{
		ID:             uuid.NewString(),
		Type:           req.Type,
		Payload:        req.Payload,
		Attempts:       0,
		MaxRetries:     req.MaxRetries,
		IdempotencyKey: idempKey,
		CreatedAt:      now,
		RunAt:          &runAt,
	}

	data, err := json.Marshal(task)
	if err != nil {
		return response.Wrap(response.ErrInternalServerError, fmt.Errorf("error marshalling: %w", err))
	}

	if err := h.redis.ZAdd(c.Request().Context(), h.queueScheduled, float64(runAt.UnixMilli()), string(data)); err != nil {
		return response.Wrap(response.ErrInternalServerError, fmt.Errorf("error redis push: %w", err))
	}

	return response.WithStatusOKResponse(
		map[string]any{
			"task_id": task.ID,
		}, c)
}
