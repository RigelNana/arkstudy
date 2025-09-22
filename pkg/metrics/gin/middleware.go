package gin

import (
	"strconv"
	"time"

	"github.com/RigelNana/arkstudy/pkg/metrics"
	"github.com/gin-gonic/gin"
)

// PrometheusMiddleware 为 Gin 添加 Prometheus 指标
func PrometheusMiddleware(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 继续处理请求
		c.Next()

		// 记录指标
		statusCode := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method + " " + c.FullPath()

		metrics.RecordRequest(serviceName, method, statusCode, time.Since(start))
	}
}
