package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"randomtest_core"
	"strings"
	"sync/atomic"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 是 Wails 应用入口，承载窗口上下文和测试运行状态。
type App struct {
	ctx      context.Context
	initialX int
	initialY int
	running  atomic.Bool
}

// TestFormDefaults 定义前端表单初始化参数。
type TestFormDefaults struct {
	PValueThreshold        float64 `json:"pValueThreshold"`
	QValueThreshold        float64 `json:"qValueThreshold"`
	RandomnessTestStandard string  `json:"randomnessTestStandard"`
	RandomnessTestLevel    string  `json:"randomnessTestLevel"`
	InputDataPath          string  `json:"inputDataPath"`
	IsFile                 bool    `json:"isFile"`
	SampleCount            int     `json:"sampleCount"`
	SampleSizeBits         int     `json:"sampleSizeBits"`
	MinimumPassSampleCount int     `json:"minimumPassSampleCount"`
}

// PickInputPathResult 定义目录选择返回值。
type PickInputPathResult struct {
	Path   string `json:"path"`
	IsFile bool   `json:"isFile"`
}

// StartRandomnessTestRequest 定义开始测试请求体。
type StartRandomnessTestRequest struct {
	PValueThreshold        float64 `json:"pValueThreshold"`
	QValueThreshold        float64 `json:"qValueThreshold"`
	InputDataPath          string  `json:"inputDataPath"`
	IsFile                 bool    `json:"isFile"`
	RandomnessTestStandard string  `json:"randomnessTestStandard"`
	RandomnessTestLevel    int     `json:"randomnessTestLevel"`
	SampleCount            int     `json:"sampleCount"`
}

// AnalyzeInputPathRequest 定义路径分析请求体。
type AnalyzeInputPathRequest struct {
	InputDataPath          string  `json:"inputDataPath"`
	IsFile                 bool    `json:"isFile"`
	RandomnessTestStandard string  `json:"randomnessTestStandard"`
	RandomnessTestLevel    int     `json:"randomnessTestLevel"`
	PValueThreshold        float64 `json:"pValueThreshold"`
}

// AnalyzeInputPathResult 定义路径分析结果。
type AnalyzeInputPathResult struct {
	IsFile                 bool `json:"isFile"`
	SampleSizeBits         int  `json:"sampleSizeBits"`
	SampleCount            int  `json:"sampleCount"`
	MinimumPassSampleCount int  `json:"minimumPassSampleCount"`
}

// ------------------------------------------------------------
// App lifecycle
// ------------------------------------------------------------

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	if a.initialX != -1 && a.initialY != -1 {
		go func() {
			time.Sleep(200 * time.Millisecond)
			wailsRuntime.WindowSetPosition(ctx, a.initialX, a.initialY)
			wailsRuntime.WindowShow(ctx)
		}()
	} else {
		wailsRuntime.WindowCenter(ctx)
	}
	fmt.Println("Wails Startup 已启动...")
}

// 响应 Vue + Wails 的 NewWindow 按钮，启动当前程序的新实例
func (a *App) NewWindow(windowDpr float64) {
	/* 计算坐标 */
	x, y := wailsRuntime.WindowGetPosition(a.ctx)
	w, _ := wailsRuntime.WindowGetSize(a.ctx)
	newX := x + w + int(20*windowDpr) // 新窗口在当前窗口右侧，间隔 20 px
	newY := y

	/* 获取当前可执行文件路径 */
	exe, err := os.Executable()
	if err != nil {
		return
	}
	/* 根据操作系统处理启动逻辑 */
	var cmd *exec.Cmd
	cmd = exec.Command(exe, fmt.Sprintf("-x=%d", newX), fmt.Sprintf("-y=%d", newY)) // Windows 直接运行
	_ = cmd.Start()                                                                 // 启动新进程，不阻塞当前进程
}

// ------------------------------------------------------------
// Internal helpers
// ------------------------------------------------------------

func normalizeStandard(standard string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(standard)) {
	case "N", "NIST":
		return "N", nil
	case "G", "GM", "GM/T", "GMT":
		return "G", nil
	default:
		return "", fmt.Errorf("不支持的测试标准: %s", standard)
	}
}

func levelToLabel(level int) string {
	switch level {
	case 1:
		return "1级 (20000 bits)"
	case 2:
		return "2级 (1000000 bits)"
	case 3:
		return "3级 (100000000 bits)"
	default:
		return "2级 (1000000 bits)"
	}
}

func levelToSampleBits(level int) int {
	switch level {
	case 1:
		return 20000
	case 2:
		return 1000000
	case 3:
		return 100000000
	default:
		return 1000000
	}
}

func expectedBlockBytes(standard string, level int) (sampleSizeBits int, blockBytes int, err error) {
	std, err := normalizeStandard(standard)
	if err != nil {
		return 0, 0, err
	}
	if std == "N" {
		if level != 2 {
			return 0, 0, fmt.Errorf("NIST 仅支持 2 级")
		}
		return 1000000, 125000, nil
	}

	switch level {
	case 1:
		return 20000, 2500, nil
	case 2:
		return 1000000, 125000, nil
	case 3:
		return 100000000, 12500000, nil
	default:
		return 0, 0, fmt.Errorf("测试等级必须在 1-3 之间")
	}
}

func calcMinimumPassCount(sampleCount int) int {
	if sampleCount <= 0 {
		return 0
	}
	return int(math.Ceil(float64(sampleCount) * randomtest_core.PassedProportion))
}

func calcMinimumPassCountWithAlpha(alpha float64, sampleCount int) int {
	if sampleCount <= 0 {
		return 0
	}
	_ = alpha
	return int(math.Ceil(float64(sampleCount) * randomtest_core.PassedProportion))
}

// ------------------------------------------------------------
// Public APIs for frontend
// ------------------------------------------------------------

// GetTestFormDefaults 返回前端表单默认参数（来自当前内存配置）。
func (a *App) GetTestFormDefaults() TestFormDefaults {
	alpha, alphaT, _ := randomtest_core.GetAlphaValue()
	testType := strings.ToUpper(strings.TrimSpace(RunStandardRandomTestEg.TestType))
	standard := "未知"
	if testType == "N" {
		standard = "NIST"
	} else if testType == "G" {
		standard = "GM"
	}
	inputPath := strings.TrimSpace(RunStandardRandomTestEg.RootDirPath)
	if RunStandardRandomTestEg.IsFile {
		inputPath = strings.TrimSpace(RunStandardRandomTestEg.FilePath)
	}
	return TestFormDefaults{
		PValueThreshold:        alpha,
		QValueThreshold:        alphaT,
		RandomnessTestStandard: standard,
		RandomnessTestLevel:    levelToLabel(RunStandardRandomTestEg.TestLevel),
		InputDataPath:          inputPath,
		IsFile:                 RunStandardRandomTestEg.IsFile,
		SampleCount:            RunStandardRandomTestEg.TestNum,
		SampleSizeBits:         levelToSampleBits(RunStandardRandomTestEg.TestLevel),
		MinimumPassSampleCount: calcMinimumPassCount(RunStandardRandomTestEg.TestNum),
	}
}

// PickInputPath 仅选择输入目录。
func (a *App) PickInputPath() (PickInputPathResult, error) {
	if a.ctx == nil {
		return PickInputPathResult{}, fmt.Errorf("应用上下文未初始化")
	}
	dirPath, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择输入目录",
	})
	if err != nil {
		return PickInputPathResult{}, err
	}
	if strings.TrimSpace(dirPath) != "" {
		return PickInputPathResult{Path: dirPath, IsFile: false}, nil
	}
	return PickInputPathResult{}, fmt.Errorf("未选择目录")
}

// AnalyzeInputPath 根据当前标准/等级分析输入路径并返回样本统计。
func (a *App) AnalyzeInputPath(req AnalyzeInputPathRequest) (AnalyzeInputPathResult, error) {
	inputPath := strings.TrimSpace(req.InputDataPath)
	if inputPath == "" {
		return AnalyzeInputPathResult{}, fmt.Errorf("输入路径不能为空")
	}
	sampleBits, blockBytes, err := expectedBlockBytes(req.RandomnessTestStandard, req.RandomnessTestLevel)
	if err != nil {
		return AnalyzeInputPathResult{}, err
	}

	st, err := os.Stat(inputPath)
	if err != nil {
		return AnalyzeInputPathResult{}, err
	}

	resolvedIsFile := req.IsFile
	if st != nil {
		resolvedIsFile = !st.IsDir()
	}

	sampleCount := 0
	if resolvedIsFile {
		if blockBytes > 0 {
			sampleCount = int(st.Size()) / blockBytes
		}
	} else {
		entries, readErr := os.ReadDir(inputPath)
		if readErr != nil {
			return AnalyzeInputPathResult{}, readErr
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			info, infoErr := entry.Info()
			if infoErr != nil {
				continue
			}
			if int(info.Size()) == blockBytes {
				sampleCount++
			}
		}
	}

	return AnalyzeInputPathResult{
		IsFile:                 resolvedIsFile,
		SampleSizeBits:         sampleBits,
		SampleCount:            sampleCount,
		MinimumPassSampleCount: calcMinimumPassCountWithAlpha(req.PValueThreshold, sampleCount),
	}, nil
}

// StartRandomnessTest 在点击“开始测试”时接收前端参数并执行测试。
func (a *App) StartRandomnessTest(req StartRandomnessTestRequest) error {
	if !a.running.CompareAndSwap(false, true) {
		return fmt.Errorf("测试任务正在运行，请稍后")
	}
	defer a.running.Store(false)
	ResetStopRandomTest()

	inputPath := strings.TrimSpace(req.InputDataPath)
	if inputPath == "" {
		return fmt.Errorf("输入路径不能为空")
	}

	if err := randomtest_core.SetAlphaValue(req.PValueThreshold, req.QValueThreshold); err != nil {
		return err
	}

	standard, err := normalizeStandard(req.RandomnessTestStandard)
	if err != nil {
		return err
	}
	if req.RandomnessTestLevel < 1 || req.RandomnessTestLevel > 3 {
		return fmt.Errorf("测试等级必须在 1-3 之间")
	}
	if req.SampleCount <= 0 {
		return fmt.Errorf("样本数量必须大于 0")
	}

	resolvedIsFile := req.IsFile
	if st, statErr := os.Stat(inputPath); statErr == nil {
		resolvedIsFile = !st.IsDir()
	} else if ext := strings.ToLower(filepath.Ext(inputPath)); ext != "" {
		resolvedIsFile = true
	}

	RunStandardRandomTestEg.IsFile = resolvedIsFile
	RunStandardRandomTestEg.FilePath = ""
	RunStandardRandomTestEg.RootDirPath = ""
	if resolvedIsFile {
		RunStandardRandomTestEg.FilePath = inputPath
	} else {
		RunStandardRandomTestEg.RootDirPath = inputPath
	}
	RunStandardRandomTestEg.TestType = standard
	RunStandardRandomTestEg.TestLevel = req.RandomnessTestLevel
	RunStandardRandomTestEg.TestNum = req.SampleCount

	SetRandomTestOutputHandler(func(event OutputEvent) {
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "randomtest_output", event)
		}
	})
	defer SetRandomTestOutputHandler(nil)

	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "randomtest_state", map[string]interface{}{"running": true})
		defer wailsRuntime.EventsEmit(a.ctx, "randomtest_state", map[string]interface{}{"running": false})
	}

	err = RunStandardRandomTestEg.HandleRandomTestStandardRunMulti()
	if err != nil {
		if IsStopRandomTestRequested() {
			return fmt.Errorf("测试已停止")
		}
		return err
	}
	return nil
}

// StopRandomnessTest 请求停止当前正在执行的测试任务。
func (a *App) StopRandomnessTest() error {
	if !a.running.Load() {
		return fmt.Errorf("当前没有正在运行的测试任务")
	}
	RequestStopRandomTest()
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "randomtest_output", OutputEvent{
			Type:    "log",
			Level:   "warn",
			Message: "[INFO] 已请求停止测试，等待当前任务安全退出...\n",
		})
	}
	return nil
}

// SaveResultCSV 保存当前最新测试结果到指定 CSV 路径。
func (a *App) SaveResultCSV(filePath string) (string, error) {
	path := strings.TrimSpace(filePath)
	if path == "" {
		return "", fmt.Errorf("结果文件路径不能为空")
	}
	if strings.ToLower(filepath.Ext(path)) != ".csv" {
		path += ".csv"
	}
	if err := SaveLatestResultRowsCSV(path); err != nil {
		return "", err
	}
	return path, nil
}

func sanitizeWindowsFileName(name string) string {
	replacer := strings.NewReplacer(
		"\\", "_",
		"/", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		"\r", "",
		"\n", "",
		"\t", "",
	)
	clean := strings.TrimSpace(replacer.Replace(name))
	if clean == "" {
		return "randomtest_result.csv"
	}
	return clean
}

func currentInputNameForExport() string {
	inputPath := strings.TrimSpace(RunStandardRandomTestEg.RootDirPath)
	if RunStandardRandomTestEg.IsFile {
		inputPath = strings.TrimSpace(RunStandardRandomTestEg.FilePath)
	}
	if inputPath == "" {
		return ""
	}
	base := filepath.Base(strings.TrimRight(inputPath, "\\/"))
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	return base
}

// SaveResultCSVByDialog 弹出资源管理器选择保存目录，并按默认规则命名后保存 CSV。
func (a *App) SaveResultCSVByDialog(standard, level string) (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("应用上下文未初始化")
	}
	dirPath, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择结果保存目录",
	})
	if err != nil {
		return "", err
	}
	dirPath = strings.TrimSpace(dirPath)
	if dirPath == "" {
		return "", fmt.Errorf("未选择保存目录")
	}

	prefixName := currentInputNameForExport()
	if prefixName == "" {
		prefixName = filepath.Base(strings.TrimRight(dirPath, "\\/"))
	}
	fileName := fmt.Sprintf("%s_测试标准%s_测试等级%s.csv", prefixName, strings.TrimSpace(standard), strings.TrimSpace(level))
	targetPath := filepath.Join(dirPath, sanitizeWindowsFileName(fileName))

	if err = SaveLatestResultRowsCSV(targetPath); err != nil {
		return "", err
	}
	return targetPath, nil
}

// // FileLoader 处理本地资源请求
// type FileLoader struct {
// 	http.Handler
// }

// func (h *FileLoader) ServeHTTP(res http.ResponseWriter, req *http.Request) {
// 	rawPath := strings.TrimPrefix(req.URL.Path, "/media-stream/") // 提取路径
// 	/* 关键：URL 解码（处理中文和特殊字符） */
// 	path, err := url.PathUnescape(rawPath)
// 	if err != nil {
// 		http.Error(res, "路径解析失败", http.StatusBadRequest)
// 		return
// 	}
// 	/* 在 Windows 下的兼容处理（去掉路径开头的第一个斜杠，如 /C:/ -> C:/） */
// 	if len(path) > 0 && path[0] == '/' && strings.Contains(path, ":") {
// 		path = path[1:]
// 	}
// 	file, err := os.Open(path) // 读取文件
// 	if err != nil {
// 		http.Error(res, "文件不存在", http.StatusNotFound)
// 		return
// 	}
// 	defer file.Close()
// 	stat, _ := file.Stat()
// 	http.ServeContent(res, req, stat.Name(), stat.ModTime(), file)
// }
