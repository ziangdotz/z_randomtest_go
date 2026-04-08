package main

import (
	"embed"
	"flag"
	"randomtest_core"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func onExit() {}

func main() {
	randomtest_core.SetDebug(false)

	/* 定义并解析命令行参数 */
	startX := flag.Int("x", -1, "starting x position")
	startY := flag.Int("y", -1, "starting y position")
	flag.Parse()

	app := NewApp()
	app.initialX = *startX
	app.initialY = *startY

	appOptions := &options.App{
		Title:         "randomtest_stdtest",
		Width:         1000,
		Height:        680 + 34,
		MinWidth:      1000,
		MinHeight:     680 + 34,
		DisableResize: false, // 禁止调整窗口大小
		Frameless:     true,  // 开启无边框模式
		AlwaysOnTop:   false, // 是否置顶
		StartHidden:   false, // 启动时隐藏窗口
		AssetServer: &assetserver.Options{
			Assets: assets,
			// Handler: &FileLoader{},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.Mica, // Win11 云母效果
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
	}

	err := wails.Run(appOptions)
	if err != nil {
		println("Error:", err.Error())
	}
}
