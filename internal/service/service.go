// Package service implements task handlers for email sending, report generation, and other background operations.
package service

import (
	"context"
	"errors"
	"fluxqueue/internal/model"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"
)

type IStorage interface {
	PutObject(ctx context.Context, filepath, bucketname, objectName, contentType string) error
	CreateBucketWithCheck(ctx context.Context, bucketName string) error
}

type Service struct {
	storage IStorage
}

func NewTaskService(storage IStorage) *Service {
	return &Service{
		storage: storage,
	}
}

func (s *Service) SendEmail(ctx context.Context, t *model.Task) error {
	// simulate sending an email
	_ = t
	n := rand.Intn(3500) + 200 // in ms
	fmt.Printf("wait for %d sec\n", n)
	time.Sleep(time.Duration(n) * time.Millisecond)
	return simulateError(10)
}

func (s Service) GenerateExcelReport(ctx context.Context, t *model.Task) error {
	_ = t
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	var err error
	err = f.SetCellValue(sheet, "A1", "Task ID")
	if err != nil {
		return err
	}
	err = f.SetCellValue(sheet, "B1", t.ID)
	if err != nil {
		return err
	}
	err = f.SetCellValue(sheet, "A1", "Generated At")
	if err != nil {
		return err
	}
	err = f.SetCellValue(sheet, "B1", time.Now().Format(time.RFC3339))
	if err != nil {
		return err
	}
	tmpDir := os.TempDir()
	// filepath := filepath.Dir(tmpDir, fmt.Sprintf("report-%s.xlsx",t.ID))
	filepath := filepath.Join(tmpDir, fmt.Sprintf("report-%s.xlsx", t.ID))
	// filepath := filepath.Dir(tmpDir)
	if err = f.SaveAs(filepath); err != nil {
		return err
	}

	defer func() {
		err = os.Remove(filepath)
		if err != nil {
			log.Err(err)
		}
	}()
	bucket := "reports"
	err = s.storage.CreateBucketWithCheck(ctx, bucket)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return err
	}

	err = s.storage.PutObject(ctx,
		filepath,
		bucket,
		fmt.Sprintf("report-%s.xlsx", t.ID),
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	if err != nil {
		return err
	}
	return nil
}

// helper function to generate error based on percentage
func simulateError(errorPercent int) error {
	prob := []int{}
	for range 100 {
		val := rand.Intn(100)
		if slices.Contains(prob, val) {
			continue
		}
		prob = append(prob, val)
	}

	if slices.Contains(prob, errorPercent) {
		return errors.New("some error")
	}
	return nil
}
