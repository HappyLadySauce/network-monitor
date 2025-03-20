package middlewares

import (
	"net/http"
	"strconv"
	"github.com/gin-gonic/gin"
)

// 跨域中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*") // 允许的前端域名
		c.Writer.Header().Set("Access-Control-Allow-Credentials", strconv.FormatBool(true)) // 允许携带凭证
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS") // 允许的请求方法
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization") // 允许的请求头
		// 预检请求缓存24小时(24小时 = 86400秒)
		c.Writer.Header().Set("Access-Control-Max-Age", strconv.Itoa(86400))

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}