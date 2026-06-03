package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// HealthCheck wraps common.HealthCheckHandler for the router
func HealthCheck(c *gin.Context) {
	common.HealthCheckHandler(c)
}

// PrometheusMetrics wraps common.PrometheusMetricsHandler for the router
func PrometheusMetrics(c *gin.Context) {
	common.PrometheusMetricsHandler(c)
}
