package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"randomtest_core"
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

type FileDetail struct {
	FileName string
	PPassed  bool
	QPassed  bool
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

func isPassedByRange(passCount, sampleSize int) bool {
	if sampleSize <= 0 {
		return false
	}
	maxV, minV := jisuan(sampleSize)
	return passCount >= minV && passCount <= maxV
}

func formatPassFraction(passCount, sampleSize int) string {
	return fmt.Sprintf("%d/%d", passCount, sampleSize)
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

func buildNISTResultRows(testNum int, stats []randomtest_core.ItemStat) []ResultTableRow {
	rows := make([]ResultTableRow, 0, len(stats))
	index := 1
	for _, item := range stats {
		if item.ValuesNum != nil {
			/* 多值检测: RE/REV 以 ValuesNum[i] 为分母 (p>0 适用语义),
			   其余 (NonOverlappingTemplateMatching) 以 testNum 为分母 */
			isRandomExcursions := item.Name == "RandomExcursionsTest" || item.Name == "RandomExcursionsVariantTest"
			for i, pCount := range item.PCounts {
				sampleSize := testNum
				if isRandomExcursions {
					sampleSize = item.ValuesNum[i]
				}
				pText := formatPassFraction(pCount, sampleSize)
				resultText := "通过"
				if !isPassedByRange(pCount, sampleSize) {
					resultText = "不通过"
				}
				rows = append(rows, ResultTableRow{
					Index:  index,
					Name:   fmt.Sprintf("%s[%d]", item.Name, i),
					PValue: pText,
					QValue: "-",
					Result: resultText,
				})
				index++
			}
			continue
		}

		pText := formatPassFraction(item.PCount, testNum)
		resultText := "通过"
		if !isPassedByRange(item.PCount, testNum) {
			resultText = "不通过"
		}
		rows = append(rows, ResultTableRow{
			Index:  index,
			Name:   item.Name,
			PValue: pText,
			QValue: "-",
			Result: resultText,
		})
		index++
	}
	return rows
}

func buildGMResultRows(testNum int, stats []randomtest_core.ItemStat) []ResultTableRow {
	rows := make([]ResultTableRow, 0, len(stats))
	for i, item := range stats {
		pText := formatPassFraction(item.PCount, testNum)
		qText := formatPassFraction(item.QCount, testNum)
		resultText := "通过"
		/* 暂时先不管 QValue */
		// if !isPassedByRange(item.PCount, testNum) || !isPassedByRange(item.QCount, testNum) {
		if !isPassedByRange(item.PCount, testNum) {
			resultText = "不通过"
		}
		rows = append(rows, ResultTableRow{
			Index:  i + 1,
			Name:   item.Name,
			PValue: pText,
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

func jisuan(sampleSize int) (int, int) {
	sampleSizeF := float64(sampleSize)
	proportion_threshold_max := (randomtest_core.AlphaHAT + 3.0*math.Sqrt((randomtest_core.AlphaHAT*randomtest_core.Alpha)/sampleSizeF)) * sampleSizeF
	proportion_threshold_min := (randomtest_core.AlphaHAT - 3.0*math.Sqrt((randomtest_core.AlphaHAT*randomtest_core.Alpha)/sampleSizeF)) * sampleSizeF
	return int(math.Floor(proportion_threshold_max)), int(math.Ceil(proportion_threshold_min))
}

// fileItemPassed 目录模式单文件判定, 复刻旧实现的 "PCount/QCount 前后增量" 语义:
//   - RequireAll (SerialTest): 双 P (及双 Q) 全过才算通过
//   - 单值检测: p/q 通过即通过
//   - 多值检测 (RE/REV/NonOverlapping): 旧实现 PCount/QCount 不递增, 恒判定为不通过 (保持既有行为)
//   - NIST 套件的 Q 值从不参与统计, QPassed 恒为 false
func fileItemPassed(o randomtest_core.Outcome, gm bool) (bool, bool) {
	pPassed := false
	qPassed := false
	switch {
	case o.RequireAll:
		pPassed = randomtest_core.AllPValuesPassed(o.P)
		if gm {
			qPassed = randomtest_core.AllQValuesPassed(o.Q)
		}
	case len(o.P) == 1:
		pPassed = randomtest_core.PValuePassed(o.P[0])
		if gm && len(o.Q) == 1 {
			qPassed = randomtest_core.QValuePassed(o.Q[0])
		}
	}
	return pPassed, qPassed
}

func printTestLevelXNISTFuncResults(testNum int, stats []randomtest_core.ItemStat) {
	emitLogf("info", "\n📊 NIST Level%d 测试结果统计\n", RunStandardRandomTestEg.TestLevel)
	emitLogf("info", "%s\n", strings.Repeat("-", 75))
	emitLogf("info", "%-45s | %20s\n", "TestFuncName", "PValue Passed-Count")
	emitLogf("info", "%s\n", strings.Repeat("-", 75))
	pPassedNumMax, pPassedNumMin := jisuan(testNum)
	for _, item := range stats {
		if item.ValuesNum != nil {
			/* 多值检测: RE/REV 以 ValuesNum[i] 为分母 (p>0 适用语义),
			   其余 (NonOverlappingTemplateMatching) 以 testNum 为分母 */
			if item.Name == "RandomExcursionsTest" || item.Name == "RandomExcursionsVariantTest" {
				for i, pCount := range item.PCounts {
					pPassedNum1Max, pPassedNum1Min := jisuan(item.ValuesNum[i])
					if pCount < pPassedNum1Min || pCount > pPassedNum1Max {
						emitLogf("info", "%-41s %3d | %9d/%-9d *\n", item.Name, i, pCount, item.ValuesNum[i])
						continue
					}
					emitLogf("info", "%-41s %3d | %9d/%-9d  \n", item.Name, i, pCount, item.ValuesNum[i])
				}
				continue
			}
			for i, pCount := range item.PCounts {
				if pCount < pPassedNumMin || pCount > pPassedNumMax {
					emitLogf("info", "%-41s %3d | %9d/%-9d *\n", item.Name, i, pCount, testNum)
					continue
				}
				emitLogf("info", "%-41s %3d | %9d/%-9d  \n", item.Name, i, pCount, testNum)
			}
			continue
		}
		if item.PCount < pPassedNumMin || item.PCount > pPassedNumMax {
			emitLogf("info", "%-45s | %9d/%-9d *\n", item.Name, item.PCount, testNum)
			continue
		}
		emitLogf("info", "%-45s | %9d/%-9d  \n", item.Name, item.PCount, testNum)
	}
	emitLogf("info", "%s\n", strings.Repeat("-", 75))
}

func printTestLevelXGMFuncResults(testNum int, stats []randomtest_core.ItemStat) {
	emitLogf("info", "\n📊 GM Level%d 测试结果统计\n", RunStandardRandomTestEg.TestLevel)
	emitLogf("info", "%s\n", strings.Repeat("-", 85))
	emitLogf("info", "%-35s | %21s | %21s\n", "TestFuncName", "PValue Passed-Count", "QValue Passed-Count")
	emitLogf("info", "%s\n", strings.Repeat("-", 85))
	pPassedNumMax, pPassedNumMin := jisuan(testNum)
	qPassedNumMax := pPassedNumMax
	qPassedNumMin := pPassedNumMin
	for _, item := range stats {
		pPassedStr := "  "
		qPassedStr := "  "
		if item.PCount < pPassedNumMin || item.PCount > pPassedNumMax {
			pPassedStr = " *"
		}
		if item.QCount < qPassedNumMin || item.QCount > qPassedNumMax {
			qPassedStr = " *"
		}
		emitLogf("info", "%-35s | %9d/%-9d%2s | %9d/%-9d%2s\n", item.Name, item.PCount, testNum, pPassedStr, item.QCount, testNum, qPassedStr)
	}
	emitLogf("info", "%s\n", strings.Repeat("-", 85))
}

func printTestNISTOrGMErrorFileList(fileDetails [][]FileDetail, stats []randomtest_core.ItemStat, gm bool) {
	emitLogf("info", "\n❌ NIST or GM 测试不通过的文件清单\n")
	emitLogf("info", "%s\n", strings.Repeat("=", 85))
	for i, item := range stats {
		hasFail := false
		emitLogf("info", "测试项: %s\n", item.Name)
		for _, detail := range fileDetails[i] {
			/* NIST 套件不统计 Q 值 (QPassed 恒为 false), 仅按 P 判定, 避免清单全量打印 */
			if !detail.PPassed || (gm && !detail.QPassed) {
				status := ""
				if !detail.PPassed {
					status += "P不通过 "
				}
				if gm && !detail.QPassed {
					status += "Q不通过 "
				}
				emitLogf("info", "  - [失败] %-30s: %s\n", detail.FileName, status)
				hasFail = true
			}
		}
		if !hasFail {
			emitLogf("info", "  - 所有文件均通过\n")
		}
		emitLogf("info", "%s\n", strings.Repeat("-", 40))
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

	/* 目录模式: 通过 OnItemOutcome 收集每文件每检测项明细, 判定语义与旧实现一致 */
	var fileDetails [][]FileDetail
	if !runStandardRandomTestStruct.IsFile {
		fileDetails = make([][]FileDetail, len(suite.Items))
		var fdMu sync.Mutex
		isGM := suite.Standard == "GM"
		opt.OnItemOutcome = func(fileName string, itemIdx int, o randomtest_core.Outcome) {
			pPassed, qPassed := fileItemPassed(o, isGM)
			fdMu.Lock()
			fileDetails[itemIdx] = append(fileDetails[itemIdx], FileDetail{
				FileName: fileName,
				PPassed:  pPassed,
				QPassed:  qPassed,
			})
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
	isNIST := runStandardRandomTestStruct.TestType == "N"
	if isNIST {
		printTestLevelXNISTFuncResults(denominator, stats)
	} else {
		printTestLevelXGMFuncResults(denominator, stats)
	}
	if !runStandardRandomTestStruct.IsFile {
		printTestNISTOrGMErrorFileList(fileDetails, stats, suite.Standard == "GM")
	}
	flushLogs()
	if isNIST {
		setLatestResultRows(buildNISTResultRows(denominator, stats))
	} else {
		setLatestResultRows(buildGMResultRows(denominator, stats))
	}
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
	sanitizeConfigPaths()
	return nil
}

// trimIniQuotes 剥离 ini 值残留的首尾引号 (如误写为 ""path"" 时解析残留的内层引号),
// 避免路径混入引号字符导致打开失败。
func trimIniQuotes(s string) string {
	s = strings.TrimSpace(s)
	for len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

// sanitizeConfigPaths 统一清理配置路径字段。
func sanitizeConfigPaths() {
	RunStandardRandomTestEg.FilePath = trimIniQuotes(RunStandardRandomTestEg.FilePath)
	RunStandardRandomTestEg.RootDirPath = trimIniQuotes(RunStandardRandomTestEg.RootDirPath)
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
