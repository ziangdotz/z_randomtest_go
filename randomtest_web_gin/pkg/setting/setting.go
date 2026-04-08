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
}
