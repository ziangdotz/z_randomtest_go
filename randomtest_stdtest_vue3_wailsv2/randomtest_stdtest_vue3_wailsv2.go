package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"randomtest_core"
	"randomtest_core/report"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	ini "gopkg.in/ini.v1"
)

var (
	Config *ini.File

	WorkerCount             int
	RunStandardRandomTestEg = &RunStandardRandomTestStruct{}
	outputHandler           OutputEventHandler
	outputHandlerMutex      sync.RWMutex
	stopTestRequested       atomic.Bool
	runCtxMu                sync.Mutex
	runCtx                  context.Context // 当前运行的取消上下文 (nil 表示无运行中的任务)
	runCtxCancel            context.CancelFunc
	latestResultRows        []ResultTableRow
	latestResultRowsMu      sync.RWMutex
)

type RunStandardRandomTestStruct struct {
	IsFile                   bool   `form:"isFile" json:"isFile" binding:"required"`                  // 是否是文件
	FilePath                 string `form:"filePath" json:"filePath"`                                 // 测试文件路径 (IsFile == true)
	TestNum                  int    `form:"TestNum" json:"TestNum"`                                   // 测试字节块数量
	RootDirPath              string `form:"rootDirPath" json:"rootDirPath"`                           // 测试文件根目录路径 (IsFile == false)
	TestType                 string `form:"testFileType" json:"testType" binding:"required"`          // 检测标准类型 (NIST / GM)
	TestLevel                int    `form:"testLevel" json:"testLevel" binding:"required"`            // 检测标准等级 (NIST 默认为 2 级; 主要针对 GM: 1 级-20000 bits, 2 级-1000000 bits, 3 级-100000000 bits)
	EnableSecondaryDetection bool   `form:"enableSecondaryDetection" json:"enableSecondaryDetection"` // 是否启用二级检测（当前用于前后端状态对齐）
}

type OutputEvent struct {
	Type        string           `json:"type"`
	Level       string           `json:"level,omitempty"`
	Message     string           `json:"message,omitempty"`
	Messages    []string         `json:"messages,omitempty"`
	Stage       string           `json:"stage,omitempty"`
	Done        int              `json:"done,omitempty"`
	Total       int              `json:"total,omitempty"`
	Percent     float64          `json:"percent,omitempty"`
	CurrentFile string           `json:"currentFile,omitempty"`
	Rows        []ResultTableRow `json:"rows,omitempty"`
}

type OutputEventHandler func(OutputEvent)

func SetRandomTestOutputHandler(handler OutputEventHandler) {
	outputHandlerMutex.Lock()
	defer outputHandlerMutex.Unlock()
	outputHandler = handler
}

// logBatchMax 日志批量下发阈值: 目录模式单次运行可产生数万行日志,
// 逐行 EventsEmit 会淹没前端渲染线程导致界面卡死, 故合并为批次发送。
const logBatchMax = 50

var (
	logBatchMu    sync.Mutex
	logBatchLines []string
)

func emitOutputEvent(event OutputEvent) {
	if event.Type != "log" {
		dispatchOutputEvent(event)
		return
	}

	logBatchMu.Lock()
	logBatchLines = append(logBatchLines, event.Message)
	var batch []string
	if len(logBatchLines) >= logBatchMax {
		batch = logBatchLines
		logBatchLines = nil
	}
	logBatchMu.Unlock()

	if len(batch) > 0 {
		dispatchOutputEvent(OutputEvent{Type: "log_batch", Messages: batch})
	}
}

// flushLogs 立即下发缓冲的日志行: 在阶段边界与测试结束处调用,
// 保证日志事件先于 result_rows / randomtest_state 事件到达前端。
func flushLogs() {
	logBatchMu.Lock()
	batch := logBatchLines
	logBatchLines = nil
	logBatchMu.Unlock()
	if len(batch) > 0 {
		dispatchOutputEvent(OutputEvent{Type: "log_batch", Messages: batch})
	}
}

func dispatchOutputEvent(event OutputEvent) {
	outputHandlerMutex.RLock()
	handler := outputHandler
	outputHandlerMutex.RUnlock()

	if handler != nil {
		handler(event)
		return
	}

	if event.Type == "progress" {
		fmt.Printf("[PROGRESS] %s: %d/%d (%.2f%%) %s\n", event.Stage, event.Done, event.Total, event.Percent, event.CurrentFile)
		return
	}
	if event.Type == "log_batch" {
		for _, message := range event.Messages {
			fmt.Print(message)
		}
		return
	}
	fmt.Print(event.Message)
}

func emitLogf(level, format string, args ...interface{}) {
	emitOutputEvent(OutputEvent{
		Type:    "log",
		Level:   level,
		Message: fmt.Sprintf(format, args...),
	})
}

func emitErrorf(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	emitOutputEvent(OutputEvent{
		Type:    "log",
		Level:   "error",
		Message: err.Error() + "\n",
	})
	return err
}

func emitProgress(stage string, done, total int, currentFile string) {
	percent := 0.0
	if total > 0 {
		percent = float64(done) * 100 / float64(total)
	}
	emitOutputEvent(OutputEvent{
		Type:        "progress",
		Stage:       stage,
		Done:        done,
		Total:       total,
		Percent:     percent,
		CurrentFile: currentFile,
	})
}

type ResultTableRow struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	PValue string `json:"pValue"`
	QValue string `json:"qValue"`
	Result string `json:"result"`
}

func setLatestResultRows(rows []ResultTableRow) {
	rowsCopy := append([]ResultTableRow(nil), rows...)
	latestResultRowsMu.Lock()
	latestResultRows = rowsCopy
	latestResultRowsMu.Unlock()
	emitOutputEvent(OutputEvent{Type: "result_rows", Rows: rowsCopy})
}

func GetLatestResultRows() []ResultTableRow {
	latestResultRowsMu.RLock()
	defer latestResultRowsMu.RUnlock()
	return append([]ResultTableRow(nil), latestResultRows...)
}

func SaveLatestResultRowsCSV(filePath string) error {
	rows := GetLatestResultRows()
	if len(rows) == 0 {
		return fmt.Errorf("没有可保存的测试结果")
	}
	if strings.TrimSpace(filePath) == "" {
		return fmt.Errorf("CSV 文件路径不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建 CSV 文件失败: %w", err)
	}
	defer f.Close()

	if _, err = f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return fmt.Errorf("写入 UTF-8 BOM 失败: %w", err)
	}

	w := csv.NewWriter(f)
	defer w.Flush()

	if err = w.Write([]string{"序号", "测试项", "P值", "Q值", "结果"}); err != nil {
		return fmt.Errorf("写入 CSV 表头失败: %w", err)
	}
	for _, row := range rows {
		record := []string{
			fmt.Sprintf("%d", row.Index),
			row.Name,
			toExcelTextIfFraction(row.PValue),
			toExcelTextIfFraction(row.QValue),
			row.Result,
		}
		if err = w.Write(record); err != nil {
			return fmt.Errorf("写入 CSV 行失败: %w", err)
		}
	}
	if err = w.Error(); err != nil {
		return fmt.Errorf("刷新 CSV 失败: %w", err)
	}
	return nil
}

func toExcelTextIfFraction(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "-" {
		return value
	}
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 {
		return value
	}
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return value
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return value
	}
	return "=\"" + trimmed + "\""
}

// buildResultRows 生成 GUI 结果表行: 行展开与阈值判定收敛到 report 包;
// NIST 行 Q 不适用 (QApplicable=false) 时 QValue 显示 "-", 整体结果沿用既有口径仅按 P 判定。
func buildResultRows(testNum int, stats []randomtest_core.ItemStat, gm bool) []ResultTableRow {
	itemRows := report.BuildRows(testNum, stats, gm)
	rows := make([]ResultTableRow, 0, len(itemRows))
	for i, ir := range itemRows {
		name := ir.Name
		if ir.Multi {
			name = fmt.Sprintf("%s[%d]", ir.Name, ir.SubIndex)
		}
		qText := "-"
		if ir.QApplicable {
			qText = report.FormatPassFraction(ir.QCount, ir.SampleSize)
		}
		resultText := "通过"
		/* 暂时先不管 QValue (既有 GM 行为, 仅按 P 判定整体结果) */
		if !ir.PPassed {
			resultText = "不通过"
		}
		rows = append(rows, ResultTableRow{
			Index:  i + 1,
			Name:   name,
			PValue: report.FormatPassFraction(ir.PCount, ir.SampleSize),
			QValue: qText,
			Result: resultText,
		})
	}
	return rows
}

func ResetStopRandomTest() {
	stopTestRequested.Store(false)
}

func RequestStopRandomTest() {
	stopTestRequested.Store(true)
	runCtxMu.Lock()
	cancel := runCtxCancel
	runCtxMu.Unlock()
	if cancel != nil {
		cancel() // 通知当前运行中的所有协程退出 (修复 reader 阻塞在 taskChan 发送上的泄漏)
	}
}

// setActiveRunCtx 记录/清除当前运行的取消上下文。
func setActiveRunCtx(ctx context.Context, cancel context.CancelFunc) {
	runCtxMu.Lock()
	defer runCtxMu.Unlock()
	runCtx = ctx
	runCtxCancel = cancel
}

func IsStopRandomTestRequested() bool {
	return stopTestRequested.Load()
}

// suiteFor 按检测标准类型与等级返回对应套件 (检测项定义与统计逻辑收敛到 core)。
func suiteFor(testType string, level int) (*randomtest_core.Suite, error) {
	switch testType {
	case "N": // NIST 测试 (默认标准等级为 2)
		if level != 2 {
			return nil, fmt.Errorf("[Error] NIST 测试必须为默认标准等级 2")
		}
		return randomtest_core.SuiteNISTLevel2(), nil
	case "G": // GM 测试
		suite, err := randomtest_core.SuiteGM(level)
		if err != nil {
			return nil, fmt.Errorf("[Error] %v", err)
		}
		return suite, nil
	default:
		return nil, fmt.Errorf("请选择规定的检测标准类型 (NIST'N' / GM'G')")
	}
}

// printStatsTable 以日志事件输出测试结果统计表 (阈值与行生成收敛到 report 包)。
func printStatsTable(level, testNum int, stats []randomtest_core.ItemStat, gm bool) {
	for _, line := range report.StatsTableLines(level, testNum, stats, gm) {
		emitLogf("info", "%s\n", line)
	}
}

// printErrorFileList 以日志事件输出目录模式不通过文件清单并立即 flush
// (Q 适用性由明细自带的 QApplicable 判定, NIST 仅按 P)。
func printErrorFileList(fileDetails [][]report.FileDetail, stats []randomtest_core.ItemStat) {
	for _, line := range report.ErrorFileListLines(fileDetails, stats) {
		emitLogf("info", "%s\n", line)
	}
	flushLogs()
}

// HandleRandomTestStandardRunMulti 并发执行入口: 文件分块/目录扫描与并发调度全部由
// core 的 SuiteRunner 承担, 进度/日志/结果行通过既有事件协议 (randomtest_output) 上报。
func (runStandardRandomTestStruct *RunStandardRandomTestStruct) HandleRandomTestStandardRunMulti() error {
	emitLogf("info", "\n🚩 [Info] 开始执行并发随机性测试\n")
	flushLogs()
	/* 任何提前返回路径都不丢失缓冲日志 (成功路径此处缓冲已空, 为 no-op) */
	defer flushLogs()
	if IsStopRandomTestRequested() {
		return fmt.Errorf("测试已停止")
	}

	/* 运行级取消上下文: Stop 时通知 Runner 停止投递与处理 */
	runCtxLocal, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	setActiveRunCtx(runCtxLocal, cancelRun)
	defer setActiveRunCtx(nil, nil)

	suite, err := suiteFor(runStandardRandomTestStruct.TestType, runStandardRandomTestStruct.TestLevel)
	if err != nil {
		return emitErrorf("%v", err)
	}
	if runStandardRandomTestStruct.IsFile && strings.TrimSpace(runStandardRandomTestStruct.FilePath) == "" {
		return emitErrorf("[Error] FilePath 为空: 请检查配置文件 FilePath 键 (区分大小写) 与引号格式 (值仅需一对双引号)")
	}
	if runStandardRandomTestStruct.IsFile && runStandardRandomTestStruct.TestNum <= 0 {
		return emitErrorf("[Error] TestNum 必须大于 0")
	}

	/* 进度与统计分母捕获 (Progress 回调在工作协程内并发调用, 用 atomic 避免竞争) */
	var denominatorAtomic int64
	opt := randomtest_core.RunnerOptions{
		Workers: WorkerCount,
		Ctx:     runCtxLocal,
		Progress: func(stage string, done, total int, currentFile string) {
			atomic.StoreInt64(&denominatorAtomic, int64(total))
			emitProgress(stage, done, total, currentFile)
		},
		Warn: func(format string, args ...interface{}) {
			emitLogf("warn", format, args...)
		},
	}

	/* 目录模式: 通过 OnItemOutcome 收集每文件每检测项明细, 判定语义收敛到 report 包 */
	var fileDetails [][]report.FileDetail
	if !runStandardRandomTestStruct.IsFile {
		fileDetails = make([][]report.FileDetail, len(suite.Items))
		var fdMu sync.Mutex
		isGM := suite.Standard == "GM"
		opt.OnItemOutcome = func(fileName string, itemIdx int, o randomtest_core.Outcome) {
			detail := report.NewFileDetail(fileName, o, isGM)
			fdMu.Lock()
			fileDetails[itemIdx] = append(fileDetails[itemIdx], detail)
			fdMu.Unlock()
		}
	}

	runner := randomtest_core.NewSuiteRunner(suite, opt)
	var runErr error
	if runStandardRandomTestStruct.IsFile {
		runErr = runner.RunFile(runStandardRandomTestStruct.FilePath, runStandardRandomTestStruct.TestNum)
	} else {
		runErr = runner.RunDir(runStandardRandomTestStruct.RootDirPath)
	}
	if runErr != nil && runCtxLocal.Err() == nil {
		if runStandardRandomTestStruct.IsFile {
			return emitErrorf("[Error] 处理失败: %v (已完成 %d/%d 个工作线程)", runErr, WorkerCount, WorkerCount)
		}
		return emitErrorf("[Error] 处理失败: %v", runErr)
	}
	if runCtxLocal.Err() != nil {
		return fmt.Errorf("测试已停止")
	}
	flushLogs()

	stats := runner.Stats()
	denominator := int(atomic.LoadInt64(&denominatorAtomic))
	if runStandardRandomTestStruct.IsFile {
		emitLogf("info", "\n✅ 成功处理 %d 个工作线程的结果\n", WorkerCount)
	}
	gm := suite.Standard == "GM"
	printStatsTable(runStandardRandomTestStruct.TestLevel, denominator, stats, gm)
	if !runStandardRandomTestStruct.IsFile {
		printErrorFileList(fileDetails, stats)
	}
	flushLogs()
	setLatestResultRows(buildResultRows(denominator, stats, gm))
	return nil
}

// 定向配置字段
func mapToSection(section string, v interface{}) {
	if Config == nil {
		return
	}
	err := Config.Section(section).MapTo(v)
	if err != nil {
		fmt.Printf("[Error] %s err: %v", section, err)
	}
}

func loadIniConfig(required bool) error {
	configPath := "./randomtest_stdtest_vue3_wailsv2_config.ini"
	config, err := ini.Load(configPath)
	if err != nil {
		if required {
			return fmt.Errorf("[Error] 加载配置失败 (%s): %w", configPath, err)
		}
		return nil
	}
	Config = config
	mapToSection("config", RunStandardRandomTestEg)
	report.SanitizePaths(&RunStandardRandomTestEg.FilePath, &RunStandardRandomTestEg.RootDirPath)
	return nil
}

// 前置初始化
func init() {
	// 并发工作池大小: ini [config] workers 可配 (<=0 或缺省时取 CPU 核数)
	WorkerCount = runtime.NumCPU()
	if err := loadIniConfig(false); err == nil && Config != nil {
		if key, err := Config.Section("config").GetKey("workers"); err == nil {
			if n, err := key.Int(); err == nil && n > 0 {
				WorkerCount = n
			}
		}
	}
}

// 根据 ini 配置执行标准检测流程。
func RunSTDTestFromIniConfig() error {
	if err := loadIniConfig(true); err != nil {
		return err
	}
	randomtest_core.SetDebug(false)
	return RunStandardRandomTestEg.HandleRandomTestStandardRunMulti()
}
