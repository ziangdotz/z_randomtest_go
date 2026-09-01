package main

import (
	"fmt"
	"log"
	"math"
	"randomtest_core"
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

type FileDetail struct {
	FileName string
	PPassed  bool
	QPassed  bool
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

func jisuan(sampleSize int) (int, int) {
	sampleSizeF := float64(sampleSize)
	proportion_threshold_max := (randomtest_core.AlphaHAT + 3.0*math.Sqrt((randomtest_core.AlphaHAT*randomtest_core.Alpha)/sampleSizeF)) * sampleSizeF
	proportion_threshold_min := (randomtest_core.AlphaHAT - 3.0*math.Sqrt((randomtest_core.AlphaHAT*randomtest_core.Alpha)/sampleSizeF)) * sampleSizeF
	return int(proportion_threshold_max), int(proportion_threshold_min)
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
	fmt.Printf("\n📊 NIST Level%d 测试结果统计\n", RunStandardRandomTestEg.TestLevel)
	fmt.Println(strings.Repeat("-", 75))
	fmt.Printf("%-45s | %20s\n", "TestFuncName", "PValue Passed-Count")
	fmt.Println(strings.Repeat("-", 75))
	pPassedNumMax, pPassedNumMin := jisuan(testNum)
	for _, item := range stats {
		if item.ValuesNum != nil {
			/* 多值检测: RE/REV 以 ValuesNum[i] 为分母 (p>0 适用语义),
			   其余 (NonOverlappingTemplateMatching) 以 testNum 为分母 */
			if item.Name == "RandomExcursionsTest" || item.Name == "RandomExcursionsVariantTest" {
				for i, pCount := range item.PCounts {
					pPassedNum1Max, pPassedNum1Min := jisuan(item.ValuesNum[i])
					if pCount < pPassedNum1Min || pCount > pPassedNum1Max {
						fmt.Printf("%-41s %3d | %9d/%-9d *\n", item.Name, i, pCount, item.ValuesNum[i])
						continue
					}
					fmt.Printf("%-41s %3d | %9d/%-9d  \n", item.Name, i, pCount, item.ValuesNum[i])
				}
				continue
			}
			for i, pCount := range item.PCounts {
				if pCount < pPassedNumMin || pCount > pPassedNumMax {
					fmt.Printf("%-41s %3d | %9d/%-9d *\n", item.Name, i, pCount, testNum)
					continue
				}
				fmt.Printf("%-41s %3d | %9d/%-9d  \n", item.Name, i, pCount, testNum)
			}
			continue
		}
		if item.PCount < pPassedNumMin || item.PCount > pPassedNumMax {
			fmt.Printf("%-45s | %9d/%-9d *\n", item.Name, item.PCount, testNum)
			continue
		}
		fmt.Printf("%-45s | %9d/%-9d  \n", item.Name, item.PCount, testNum)
	}
	fmt.Println(strings.Repeat("-", 75))
}

func printTestLevelXGMFuncResults(testNum int, stats []randomtest_core.ItemStat) {
	fmt.Printf("\n📊 GM Level%d 测试结果统计\n", RunStandardRandomTestEg.TestLevel)
	fmt.Println(strings.Repeat("-", 85))
	fmt.Printf("%-35s | %21s | %21s\n", "TestFuncName", "PValue Passed-Count", "QValue Passed-Count")
	fmt.Println(strings.Repeat("-", 85))
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
		fmt.Printf("%-35s | %9d/%-9d%2s | %9d/%-9d%2s\n", item.Name, item.PCount, testNum, pPassedStr, item.QCount, testNum, qPassedStr)
	}
	fmt.Println(strings.Repeat("-", 85))
}

func printTestNISTOrGMErrorFileList(fileDetails [][]FileDetail, stats []randomtest_core.ItemStat) {
	fmt.Println("\n❌ NIST or GM 测试不通过的文件清单")
	fmt.Println(strings.Repeat("=", 85))
	for i, item := range stats {
		hasFail := false
		fmt.Printf("测试项: %s\n", item.Name)
		for _, detail := range fileDetails[i] {
			if !detail.PPassed || !detail.QPassed {
				status := ""
				if !detail.PPassed {
					status += "P不通过 "
				}
				if !detail.QPassed {
					status += "Q不通过 "
				}
				fmt.Printf("  - [失败] %-30s: %s\n", detail.FileName, status)
				hasFail = true
			}
		}
		if !hasFail {
			fmt.Println("  - 所有文件均通过")
		}
		fmt.Println(strings.Repeat("-", 40))
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

	if runStandardRandomTestStruct.IsFile {
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
	if runStandardRandomTestStruct.TestType == "N" {
		printTestLevelXNISTFuncResults(denominator, stats)
	} else {
		printTestLevelXGMFuncResults(denominator, stats)
	}
	if !runStandardRandomTestStruct.IsFile && multi {
		printTestNISTOrGMErrorFileList(fileDetails, stats)
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
