package common

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// Prometheus-compatible metrics counters for monitoring
var (
	// Request counters
	TotalRequests     int64
	SuccessfulRequests int64
	FailedRequests    int64

	// Token counters
	TotalPromptTokens     int64
	TotalCompletionTokens int64

	// Latency (cumulative for avg calculation)
	TotalLatencyMs int64

	// Channel stats
	ChannelRequestCount sync.Map // map[int]int64
	ChannelErrorCount   sync.Map // map[int]int64
	ChannelLatencySum   sync.Map // map[int]int64

	// Rate limiting
	RateLimitedRequests int64

	// Active connections
	ActiveConnections int64
)

// Metrics-related atomic counters accessible from relay layer
var metricsMutex sync.RWMutex

// RecordRequest records a request for metrics
func RecordRequest(channelId int, success bool, latencyMs int64, promptTokens, completionTokens int) {
	atomic.AddInt64(&TotalRequests, 1)
	atomic.AddInt64(&TotalLatencyMs, latencyMs)

	if success {
		atomic.AddInt64(&SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&FailedRequests, 1)
	}

	atomic.AddInt64(&TotalPromptTokens, int64(promptTokens))
	atomic.AddInt64(&TotalCompletionTokens, int64(completionTokens))

	// Per-channel stats
	if channelId > 0 {
		if val, ok := ChannelRequestCount.Load(channelId); ok {
			ChannelRequestCount.Store(channelId, val.(int64)+1)
		} else {
			ChannelRequestCount.Store(channelId, int64(1))
		}

		if !success {
			if val, ok := ChannelErrorCount.Load(channelId); ok {
				ChannelErrorCount.Store(channelId, val.(int64)+1)
			} else {
				ChannelErrorCount.Store(channelId, int64(1))
			}
		}

		if val, ok := ChannelLatencySum.Load(channelId); ok {
			ChannelLatencySum.Store(channelId, val.(int64)+latencyMs)
		} else {
			ChannelLatencySum.Store(channelId, latencyMs)
		}
	}
}

// RecordRateLimited increments the rate-limited counter
func RecordRateLimited() {
	atomic.AddInt64(&RateLimitedRequests, 1)
}

// RecordConnectionStart increments active connections
func RecordConnectionStart() {
	atomic.AddInt64(&ActiveConnections, 1)
}

// RecordConnectionEnd decrements active connections
func RecordConnectionEnd() {
	atomic.AddInt64(&ActiveConnections, -1)
}

// PrometheusMetricsHandler serves metrics in Prometheus text format
func PrometheusMetricsHandler(c *gin.Context) {
	now := time.Now().Unix()
	uptime := now - StartTime

	metrics := []string{
		"# HELP newapi_uptime_seconds Gateway uptime in seconds",
		"# TYPE newapi_uptime_seconds gauge",
		fmt.Sprintf("newapi_uptime_seconds %d", uptime),

		"# HELP newapi_requests_total Total number of API requests",
		"# TYPE newapi_requests_total counter",
		fmt.Sprintf("newapi_requests_total %d", atomic.LoadInt64(&TotalRequests)),

		"# HELP newapi_requests_successful_total Total number of successful requests",
		"# TYPE newapi_requests_successful_total counter",
		fmt.Sprintf("newapi_requests_successful_total %d", atomic.LoadInt64(&SuccessfulRequests)),

		"# HELP newapi_requests_failed_total Total number of failed requests",
		"# TYPE newapi_requests_failed_total counter",
		fmt.Sprintf("newapi_requests_failed_total %d", atomic.LoadInt64(&FailedRequests)),

		"# HELP newapi_prompt_tokens_total Total prompt tokens processed",
		"# TYPE newapi_prompt_tokens_total counter",
		fmt.Sprintf("newapi_prompt_tokens_total %d", atomic.LoadInt64(&TotalPromptTokens)),

		"# HELP newapi_completion_tokens_total Total completion tokens generated",
		"# TYPE newapi_completion_tokens_total counter",
		fmt.Sprintf("newapi_completion_tokens_total %d", atomic.LoadInt64(&TotalCompletionTokens)),

		"# HELP newapi_rate_limited_requests_total Total rate-limited requests",
		"# TYPE newapi_rate_limited_requests_total counter",
		fmt.Sprintf("newapi_rate_limited_requests_total %d", atomic.LoadInt64(&RateLimitedRequests)),

		"# HELP newapi_active_connections Current active WebSocket connections",
		"# TYPE newapi_active_connections gauge",
		fmt.Sprintf("newapi_active_connections %d", atomic.LoadInt64(&ActiveConnections)),

		"# HELP newapi_avg_latency_ms Average request latency in milliseconds",
		"# TYPE newapi_avg_latency_ms gauge",
	}

	total := atomic.LoadInt64(&TotalRequests)
	if total > 0 {
		avgLatency := atomic.LoadInt64(&TotalLatencyMs) / total
		metrics = append(metrics, fmt.Sprintf("newapi_avg_latency_ms %d", avgLatency))
	} else {
		metrics = append(metrics, "newapi_avg_latency_ms 0")
	}

	// Per-channel metrics
	metrics = append(metrics, "# HELP newapi_channel_requests_total Requests per channel", "# TYPE newapi_channel_requests_total counter")
	ChannelRequestCount.Range(func(key, value interface{}) bool {
		metrics = append(metrics, fmt.Sprintf("newapi_channel_requests_total{channel_id=\"%d\"} %d", key, value))
		return true
	})

	metrics = append(metrics, "# HELP newapi_channel_errors_total Errors per channel", "# TYPE newapi_channel_errors_total counter")
	ChannelErrorCount.Range(func(key, value interface{}) bool {
		metrics = append(metrics, fmt.Sprintf("newapi_channel_errors_total{channel_id=\"%d\"} %d", key, value))
		return true
	})

	// Build response
	response := ""
	for _, m := range metrics {
		response += m + "\n"
	}

	c.Header("Content-Type", "text/plain; version=0.0.4")
	c.String(http.StatusOK, response)
}

// HealthCheckHandler provides a simple health check endpoint
func HealthCheckHandler(c *gin.Context) {
	now := time.Now().Unix()
	uptime := now - StartTime

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"version":   Version,
		"uptime":    uptime,
		"requests":  atomic.LoadInt64(&TotalRequests),
	})
}
