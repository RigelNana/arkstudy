package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP/gRPC 请求指标
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "requests_total",
			Help: "Total number of requests",
		},
		[]string{"service", "method", "status"},
	)

	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"service", "method"},
	)

	// 数据库连接池指标
	DatabaseConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "database_connections",
			Help: "Current database connections",
		},
		[]string{"service", "status"},
	)

	// 消息队列指标
	KafkaMessagesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_messages_total",
			Help: "Total number of Kafka messages",
		},
		[]string{"service", "topic", "status"},
	)

	// 业务指标
	ActiveUsers = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_users_total",
			Help: "Number of active users",
		},
	)

	MaterialsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "materials_processed_total",
			Help: "Total number of materials processed",
		},
		[]string{"service", "type", "status"},
	)

	VectorSearchLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "vector_search_duration_seconds",
			Help: "Vector search latency in seconds",
		},
	)
)

func init() {
	// 注册所有指标
	prometheus.MustRegister(
		RequestsTotal,
		RequestDuration,
		DatabaseConnections,
		KafkaMessagesTotal,
		ActiveUsers,
		MaterialsProcessed,
		VectorSearchLatency,
	)
}

// StartMetricsServer 启动独立的 metrics HTTP 服务器
func StartMetricsServer(port string) {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			panic("failed to start metrics server: " + err.Error())
		}
	}()
}

// RecordRequest 记录请求指标的助手函数
func RecordRequest(service, method, status string, duration time.Duration) {
	RequestsTotal.WithLabelValues(service, method, status).Inc()
	RequestDuration.WithLabelValues(service, method).Observe(duration.Seconds())
}
