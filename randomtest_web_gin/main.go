package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"randomtest_web_gin/pkg/setting"
	"randomtest_web_gin/routers"
	"randomtest_web_gin/service"

	"github.com/gin-gonic/gin"
)

// 前置初始化
func init() {
	setting.Setup()
	// 初始化异步任务管理器 (阶段④ 4.2): 并发额度/任务超时来自 ini [server]
	service.InitJobManager(
		setting.ServerSettings.MaxConcurrentJobs,
		time.Duration(setting.ServerSettings.JobTimeoutSec)*time.Second,
		10*time.Minute,
	)
}

// startPprof 启动仅回环监听的 pprof 服务, 端口 = Port+1 (阶段④ 4.4)。
func startPprof(port string) {
	_, portStr, err := net.SplitHostPort(port)
	if err != nil {
		portStr = strings.TrimPrefix(port, ":")
	}
	pprofPort := 6061
	if p, err := strconv.Atoi(portStr); err == nil {
		pprofPort = p + 1
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	addr := fmt.Sprintf("127.0.0.1:%d", pprofPort)
	go func() {
		log.Printf("[INFO] pprof serving at http://%s/debug/pprof/", addr)
		if err := http.ListenAndServe(addr, mux); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[WARN] pprof server exited: %v", err)
		}
	}()
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
		IdleTimeout:  setting.ServerSettings.IdleTimeout, // 空闲连接超时 (阶段④ 4.4)
	}

	// pprof 独立监听 (阶段④ 4.4): ini [server] PprofEnable 开关
	if setting.ServerSettings.PprofEnable {
		startPprof(setting.ServerSettings.Port)
	}

	// 优雅退出 (阶段④ 4.4): Ctrl+C / SIGTERM → 停止接收新连接, 等待在途请求完成
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("[INFO] server listening on %s", myServer.Addr)
		if err := myServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[Error] server run err: %v", err.Error())
		}
	}()

	<-ctx.Done()
	log.Println("[INFO] shutdown signal received, draining connections...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := myServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Error] server shutdown err: %v", err)
	}
	log.Println("[INFO] server stopped")
}
