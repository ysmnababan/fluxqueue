// Package metric provides Prometheus metrics for monitoring HTTP requests.
package metric

import (
	"github.com/prometheus/client_golang/prometheus"
	// "github.com/prometheus/client_golang/prometheus/promhttp"
)

var HTTPRequestsTotal = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "total_request",
		Help: "Total number of HTTP requests",
	},
)

func init() {
	prometheus.MustRegister(HTTPRequestsTotal)
}
