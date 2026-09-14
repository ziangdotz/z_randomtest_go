package main

import (
	"fmt"
	"log"
	"randomtest_core"
	"randomtest_core/report"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	ini "gopkg.in/ini.v1"
)

var (
	Config *ini.File

	WorkerCount             int
	RunStandardRandomTestEg = &RunStandardRandomTestStruct{}
)

type RunStandardRandomTestStruct struct {
	IsFile      bool   `form:"isFile" json:"isFile" binding:"required"`         // 是否是文件
	FilePath    string `form:"filePath" json:"filePath"`                        // 测试文件路径 (IsFile == true)
	TestNum     int    `form:"TestNum" json:"TestNum"`                          // 测试字节块数量
	RootDirPath string `form:"rootDirPath" json:"rootDirPath"`                  // 测试文件根目录路径 (IsFile == false)
	TestType    string `form:"testFileType" json:"testType" binding:"required"` // 检测标准类型 (NIST / GM)
	TestLevel   int    `form:"testLevel" json:"testLevel" binding:"required"`   // 检测标准等级 (NIST 默认为 2 级; 主要针对 GM: 1 级-20000 bits, 2 级-1000000 bits, 3 级-100000000 bits)
}

func printProgress(stage string, done, total int, currentFile string) {
	percent := 0.0
	if total > 0 {
		percent = float64(done) * 100 / float64(total)
	}
	if currentFile != "" {
		fmt.Printf("[PROGRESS] %s: %d/%d (%.2f%%) %s\n", stage, done, total, percent, currentFile)
		return
	}
	fmt.Printf("[PROGRESS] %s: %d/%d (%.2f%%)\n", stage, done, total, percent)
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
		return nil, fmt.Errorf("请选择规定的检测标准类型 (NIST / GM)")
	}
}

// printStatsTable 按标准模板输出测试结果统计表 (阈值与行生成收敛到 report 包)。
func printStatsTable(level, testNum int, stats []randomtest_core.ItemStat, gm bool) {
	for _, line := range report.StatsTableLines(level, testNum, stats, gm) {
		fmt.Println(line)
	}
}

// printErrorFileList 输出目录模式不通过文件清单 (Q 适用性由明细自带的 QApplicable 判定)。
func printErrorFileList(fileDetails [][]report.FileDetail, stats []randomtest_core.ItemStat) {
	for _, line := range report.ErrorFileListLines(fileDetails, stats) {
		fmt.Println(line)
	}
}

// run 统一执行入口: 文件分块/目录扫描与并发调度全部由 core 的 SuiteRunner 承担,
// multi 区分单线程/并发两种对外行为 (进度输出、错误包装、成功提示)。
func (runStandardRandomTestStruct *RunStandardRandomTestStruct) run(opt randomtest_core.RunnerOptions, multi bool) error {
	suite, err := suiteFor(runStandardRandomTestStruct.TestType, runStandardRandomTestStruct.TestLevel)
	if err != nil {
		return err
	}

	/* 进度回调与统计分母捕获 (Single 模式无进度输出, 与既有行为一致;
	   Progress 回调在工作协程内并发调用, 用 atomic 避免竞争) */
	var denominatorAtomic int64
	opt.Progress = func(stage string, done, total int, currentFile string) {
		atomic.StoreInt64(&denominatorAtomic, int64(total))
		if multi {
			printProgress(stage, done, total, currentFile)
		}
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

	if runStandardRandomTestStruct.IsFile {
		if strings.TrimSpace(runStandardRandomTestStruct.FilePath) == "" {
			return fmt.Errorf("[Error] FilePath 为空: 请检查配置文件 FilePath 键 (区分大小写) 与引号格式 (值仅需一对双引号)")
		}
		if runStandardRandomTestStruct.TestNum <= 0 {
			return fmt.Errorf("[Error] TestNum 必须大于 0")
		}
	}

	runner := randomtest_core.NewSuiteRunner(suite, opt)
	var runErr error
	if runStandardRandomTestStruct.IsFile {
		runErr = runner.RunFile(runStandardRandomTestStruct.FilePath, runStandardRandomTestStruct.TestNum)
	} else {
		runErr = runner.RunDir(runStandardRandomTestStruct.RootDirPath)
	}
	if runErr != nil {
		if !multi {
			return runErr
		}
		if runStandardRandomTestStruct.IsFile {
			return fmt.Errorf("[Error] 处理失败: %v (已完成 %d/%d 个工作线程)", runErr, opt.Workers, opt.Workers)
		}
		return fmt.Errorf("[Error] 处理失败: %v", runErr)
	}
	stats := runner.Stats()
	denominator := int(atomic.LoadInt64(&denominatorAtomic))
	if multi {
		fmt.Printf("\n✅ 成功处理 %d 个工作线程的结果\n", opt.Workers)
	}
	gm := suite.Standard == "GM"
	printStatsTable(runStandardRandomTestStruct.TestLevel, denominator, stats, gm)
	if !runStandardRandomTestStruct.IsFile && multi {
		printErrorFileList(fileDetails, stats)
	}
	return nil
}

// HandleRandomTestStandardRunSingle 单线程执行 (Workers=1, 无进度输出), 保持原有对外行为。
func (runStandardRandomTestStruct *RunStandardRandomTestStruct) HandleRandomTestStandardRunSingle() error {
	fmt.Println("\n🚩 [Info] 开始执行单线程随机性测试")
	return runStandardRandomTestStruct.run(randomtest_core.RunnerOptions{Workers: 1}, false)
}

// HandleRandomTestStandardRunMulti 并发执行 (Workers=WorkerCount, 含进度输出), 保持原有对外行为。
func (runStandardRandomTestStruct *RunStandardRandomTestStruct) HandleRandomTestStandardRunMulti() error {
	fmt.Println("\n🚩 [Info] 开始执行并发随机性测试")
	return runStandardRandomTestStruct.run(randomtest_core.RunnerOptions{Workers: WorkerCount}, true)
}

// 定向配置字段
func mapToSection(section string, v interface{}) {
	err := Config.Section(section).MapTo(v)
	if err != nil {
		log.Fatalf("[Error] %s err: %v", section, err)
	}
}

// 前置初始化
func init() {
	var err error
	Config, err = ini.Load("./randomtest_stdtest_config.ini")
	if err != nil {
		log.Fatalf("[Error] %v", err)
	}

	// 并发工作池大小: ini [config] workers 可配 (<=0 或缺省时取 CPU 核数)
	WorkerCount = runtime.NumCPU()
	if key, err := Config.Section("config").GetKey("workers"); err == nil {
		if n, err := key.Int(); err == nil && n > 0 {
			WorkerCount = n
		}
	}
	// 加载 config 测试用例信息
	mapToSection("config", RunStandardRandomTestEg)
	report.SanitizePaths(&RunStandardRandomTestEg.FilePath, &RunStandardRandomTestEg.RootDirPath)
}

// 程序主函数
func main() {
	randomtest_core.SetDebug(true)
	// err := RunStandardRandomTestEg.HandleRandomTestStandardRunSingle()
	err := RunStandardRandomTestEg.HandleRandomTestStandardRunMulti()
	if err != nil {
		log.Fatalf("[Error] %v", err)
	}
}
