package main

import (
	"log"
	"net/http"

	"randomtest_web_gin/pkg/setting"
	"randomtest_web_gin/routers"

	"github.com/gin-gonic/gin"
)

// 前置初始化
func init() {
	setting.Setup()
}

// 程序主函数
func main() {
	/* Gin */
	// 设置 Gin 服务的运行模式
	gin.SetMode(setting.AppSettings.RunMode)
	// 设置当前 Server 服务的配置信息
	myServer := &http.Server{
		Addr:         setting.ServerSettings.Port, // 运行地址
		Handler:      routers.InitRouter(),        // 路由处理
		ReadTimeout:  setting.ServerSettings.ReadTimeout,
		WriteTimeout: setting.ServerSettings.WriteTimeout,
	}
	// 启动程序
	if err := myServer.ListenAndServe(); err != nil {
		// 打印错误
		log.Fatalf("[Error] server run err: %v", err.Error())
	}
}
