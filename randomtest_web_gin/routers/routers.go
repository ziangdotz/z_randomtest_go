package routers

import (
	v1 "randomtest_web_gin/routers/api/v1"

	"github.com/gin-gonic/gin"
)

func corsMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		ctx.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(204)
			return
		}

		ctx.Next()
	}
}

// 注册路由
func InitRouter() *gin.Engine {
	xRouter := gin.New()

	/* 设置使用中间件 */
	xRouter.Use(gin.Logger())
	xRouter.Use(gin.Recovery())
	xRouter.Use(corsMiddleware())

	/* 设置路由 */
	// 兜底处理所有 CORS 预检请求，避免 OPTIONS 命中 404
	xRouter.OPTIONS("/*path", func(ctx *gin.Context) {
		ctx.Status(204)
	})

	// 公共访问 Api Version.1 路由组
	commonApiV1 := xRouter.Group("/api/v1")
	{
		// randomtest 路由组
		randomTestHTTPRequestRouterGroup := commonApiV1.Group("/randomtest")
		{
			// 根据客户端提交的 RandomTest HTTP 请求，处理随机性测试函数
			randomTestHTTPRequestRouterGroup.POST("/handleRandomTestHTTPRequest", v1.HandleRandomTestHTTPRequestAPI)
			randomTestHTTPRequestRouterGroup.POST("/handleRandomTestHTTPRequestNISTLevel2", v1.HandleRandomTestHTTPRequestNISTLevel2API)
			randomTestHTTPRequestRouterGroup.POST("/handleRandomTestHTTPRequestGMLevel2", v1.HandleRandomTestHTTPRequestGMLevel2API)
		}
	}

	return xRouter
}
