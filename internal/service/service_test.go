package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"fluxqueue/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSendEmail(t *testing.T) {
	ctx := context.Background()

	t.Run("body is not string", func(t *testing.T) {
		storage := NewMockIStorage(t)
		mail := NewMockIEmail(t)
		service := NewTaskService(storage, mail)

		task := &model.Task{
			Payload: map[string]any{
				"body":    123,
				"to":      "a@b.com",
				"subject": "hello",
			},
		}

		err := service.SendEmail(ctx, task)
		assert.Error(t, err)
		assert.Equal(t, "body is not string ", err.Error())

		// Email must NOT be called
		mail.AssertNotCalled(t, "SendEmail", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("to is not string", func(t *testing.T) {
		storage := NewMockIStorage(t)
		mail := NewMockIEmail(t)
		service := NewTaskService(storage, mail)

		task := &model.Task{
			Payload: map[string]any{
				"body":    "content",
				"to":      999,
				"subject": "hello",
			},
		}

		err := service.SendEmail(ctx, task)
		assert.Error(t, err)
		assert.Equal(t, "target is not string ", err.Error())

		mail.AssertNotCalled(t, "SendEmail", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("subject is not string", func(t *testing.T) {
		storage := NewMockIStorage(t)
		mail := NewMockIEmail(t)
		service := NewTaskService(storage, mail)

		task := &model.Task{
			Payload: map[string]any{
				"body":    "content",
				"to":      "a@b.com",
				"subject": true,
			},
		}

		err := service.SendEmail(ctx, task)
		assert.Error(t, err)
		assert.Equal(t, "subject is not string ", err.Error())

		mail.AssertNotCalled(t, "SendEmail", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("success - email sent", func(t *testing.T) {
		storage := NewMockIStorage(t)
		mail := NewMockIEmail(t)
		service := NewTaskService(storage, mail)

		task := &model.Task{
			Payload: map[string]any{
				"body":    "hello world",
				"to":      "a@b.com",
				"subject": "test",
			},
		}

		mail.
			On("SendEmail", "a@b.com", "test", "hello world").
			Return(nil).
			Once()

		err := service.SendEmail(ctx, task)
		assert.NoError(t, err)

		mail.AssertExpectations(t)
	})

	t.Run("email service returns error", func(t *testing.T) {
		storage := NewMockIStorage(t)
		mail := NewMockIEmail(t)
		service := NewTaskService(storage, mail)

		task := &model.Task{
			Payload: map[string]any{
				"body":    "hello",
				"to":      "a@b.com",
				"subject": "oops",
			},
		}

		mail.
			On("SendEmail", "a@b.com", "oops", "hello").
			Return(errors.New("smtp down")).
			Once()

		err := service.SendEmail(ctx, task)
		assert.Error(t, err)
		assert.Equal(t, "smtp down", err.Error())

		mail.AssertExpectations(t)
	})
}

func TestGenerateExcelReport(t *testing.T) {
	ctx := context.Background()

	makeService := func(storage *MockIStorage) Service {
		return Service{
			storage: storage,
		}
	}

	makeTask := func() *model.Task {
		return &model.Task{
			ID: "task-123",
		}
	}

	t.Run("success", func(t *testing.T) {
		storage := NewMockIStorage(t)
		svc := makeService(storage)

		storage.
			On("CreateBucketWithCheck", mock.Anything, "reports").
			Return(nil).
			Once()

		storage.
			On(
				"PutObject",
				mock.Anything,
				mock.Anything, // temp filepath
				"reports",
				"report-task-123.xlsx",
				"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			).
			Return(nil).
			Once()

		err := svc.GenerateExcelReport(ctx, makeTask())
		assert.NoError(t, err)

		storage.AssertExpectations(t)
	})

	t.Run("save file error", func(t *testing.T) {
		storage := NewMockIStorage(t)
		svc := makeService(storage)

		// Force SaveAs to fail
		origTmp := os.Getenv("TMPDIR")
		fmt.Println(origTmp)
		t.Cleanup(func() {
			_ = os.Setenv("TMPDIR", origTmp)
		})
		_ = os.Setenv("TMPDIR", "/non-existent-dir")

		err := svc.GenerateExcelReport(ctx, makeTask())
		assert.Error(t, err)
	})

	t.Run("bucket already exists is ignored", func(t *testing.T) {
		storage := NewMockIStorage(t)
		svc := makeService(storage)

		storage.
			On("CreateBucketWithCheck", mock.Anything, "reports").
			Return(errors.New("bucket already exists")).
			Once()

		storage.
			On("PutObject", mock.Anything, mock.Anything, "reports",
				"report-task-123.xlsx",
				"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			).
			Return(nil).
			Once()

		err := svc.GenerateExcelReport(ctx, makeTask())
		assert.NoError(t, err)

		storage.AssertExpectations(t)
	})

	t.Run("bucket creation fatal error", func(t *testing.T) {
		storage := NewMockIStorage(t)
		svc := makeService(storage)

		storage.
			On("CreateBucketWithCheck", mock.Anything, "reports").
			Return(errors.New("permission denied")).
			Once()

		err := svc.GenerateExcelReport(ctx, makeTask())
		assert.Error(t, err)

		storage.AssertExpectations(t)
	})

	t.Run("put object error", func(t *testing.T) {
		storage := NewMockIStorage(t)
		svc := makeService(storage)

		storage.
			On("CreateBucketWithCheck", mock.Anything, "reports").
			Return(nil).
			Once()

		storage.
			On("PutObject", mock.Anything, mock.Anything, "reports",
				"report-task-123.xlsx",
				"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			).
			Return(errors.New("upload failed")).
			Once()

		err := svc.GenerateExcelReport(ctx, makeTask())
		assert.Error(t, err)

		storage.AssertExpectations(t)
	})
}
