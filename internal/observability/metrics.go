package observability

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds engine metrics used for health/SLO visibility.
type Metrics struct {
	jobDuration   *prometheus.HistogramVec
	jobFailures   *prometheus.CounterVec
	pendingOrders prometheus.Gauge
	pushRetry     prometheus.Counter
}

var registerMetricsOnce sync.Once

// NewMetrics creates and registers the process metrics.
func NewMetrics() *Metrics {
	m := &Metrics{
		jobDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "job_duration_seconds",
			Help:    "Scheduler job runtime in seconds.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
		}, []string{"job"}),
		jobFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "job_failures_total",
			Help: "Total number of failed scheduler job runs.",
		}, []string{"job"}),
		pendingOrders: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pending_orders",
			Help: "Current number of orders in PENDING state.",
		}),
		pushRetry: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "push_retry_total",
			Help: "Total number of outbox push retries.",
		}),
	}

	registerMetricsOnce.Do(func() {
		prometheus.MustRegister(m.jobDuration, m.jobFailures, m.pendingOrders, m.pushRetry)
	})

	return m
}

func (m *Metrics) ObserveJobDuration(jobName string, duration time.Duration) {
	m.jobDuration.WithLabelValues(jobName).Observe(duration.Seconds())
}

func (m *Metrics) IncJobFailure(jobName string) {
	m.jobFailures.WithLabelValues(jobName).Inc()
}

func (m *Metrics) SetPendingOrders(count int) {
	m.pendingOrders.Set(float64(count))
}

func (m *Metrics) IncPushRetry() {
	m.pushRetry.Inc()
}
