package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_requests_total",
		Help: "按路由与状态码统计的请求数",
	}, []string{"route", "code"})

	duration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gateway_request_duration_seconds",
		Help:    "请求耗时分布",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
	}, []string{"route"})

	breakerState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gateway_breaker_state",
		Help: "熔断器状态：0 关闭 1 半开 2 打开",
	}, []string{"route"})
)

func init() {
	prometheus.MustRegister(requests, duration, breakerState)
}

// Handler 暴露 /metrics，供 Prometheus 抓取。
func Handler() http.Handler {
	return promhttp.Handler()
}

// Record 记录一次请求的指标。
func Record(route string, code int, elapsed time.Duration) {
	requests.WithLabelValues(route, strconv.Itoa(code)).Inc()
	duration.WithLabelValues(route).Observe(elapsed.Seconds())
}

// SetBreaker 同步熔断状态，便于告警。
func SetBreaker(route, state string) {
	value := 0.0
	switch state {
	case "half-open":
		value = 1
	case "open":
		value = 2
	}
	breakerState.WithLabelValues(route).Set(value)
}
