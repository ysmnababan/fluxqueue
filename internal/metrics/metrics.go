// Package metric provides Prometheus metrics for monitoring HTTP requests.
package metric

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	RequestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_requests_total",
			Help: "Total number of requests processed by the MyApp web server.",
		},
		[]string{"path", "status"},
	)

	ErrorCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_requests_errors_total",
			Help: "Total number of error requests processed by the MyApp web server.",
		},
		[]string{"path", "status"},
	)
	TaskProcessedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tasks_processed_total",
			Help: "Total number of successfully processed tasks",
		},
		[]string{"type"},
	)
	TaskFailedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tasks_failed_total",
			Help: "Total number of permanently failed tasks",
		},
		[]string{"type"},
	)
	TaskRetriedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tasks_retried_total",
			Help: "Total number of task retry attempts",
		},
		[]string{"type"},
	)
)

func init() {
	prometheus.MustRegister(RequestCount)
	prometheus.MustRegister(ErrorCount)
	prometheus.MustRegister(TaskProcessedTotal)
	prometheus.MustRegister(TaskFailedTotal)
	prometheus.MustRegister(TaskRetriedTotal)
}

func TrackMetrics(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		path := c.Request().URL.Path
		if path == "/metrics" || path == "/favicon.ico" {
			return next(c)
		}
		status := c.Response().Status
		RequestCount.WithLabelValues(path, http.StatusText(status)).Inc()
		if status >= 400 {
			ErrorCount.WithLabelValues(path, http.StatusText(status)).Inc()
		}
		return next(c)
	}
}
