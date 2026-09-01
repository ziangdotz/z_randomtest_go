package setting

import (
	"log"
	"time"

	ini "gopkg.in/ini.v1"
)

type AppSetting struct {
	RunMode string
}
type ServerSetting struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	// 阶段④新增:
	IdleTimeout       time.Duration // 空闲连接超时 (秒)
	MaxRequestBodyMB  int           // 请求体大小上限 (MB), <=0 时取默认 64
	MaxConcurrentJobs int           // 最大并发检测任务数 (CPU 密集), <=0 时取默认 1
	JobTimeoutSec     int           // 异步任务超时 (秒), 0 = 不限
	PprofEnable       bool          // 是否启用 pprof (仅回环监听, 端口 = Port+1)
}

var (
	Config *ini.File

	AppSettings    = &AppSetting{}
	ServerSettings = &ServerSetting{}
)

// 定向配置字段
func mapToSection(section string, v interface{}) {
	err := Config.Section(section).MapTo(v)
	if err != nil {
		log.Fatalf("[Error] setting.Config.mapToSection %s err: %v", section, err)
	}
}

// 启动
func Setup() {
	var err error
	Config, err = ini.Load("./ginserver_config.ini")
	if err != nil {
		log.Fatalf("[Error] setting.Setup err: %v", err)
	}

	// 加载 App 设置信息
	mapToSection("app", AppSettings)
	// 加载 Server 服务设置信息
	mapToSection("server", ServerSettings)
	ServerSettings.ReadTimeout *= time.Second
	ServerSettings.WriteTimeout *= time.Second
	ServerSettings.IdleTimeout *= time.Second
	if ServerSettings.MaxRequestBodyMB <= 0 {
		ServerSettings.MaxRequestBodyMB = 64
	}
	if ServerSettings.MaxConcurrentJobs <= 0 {
		ServerSettings.MaxConcurrentJobs = 1
	}
}
