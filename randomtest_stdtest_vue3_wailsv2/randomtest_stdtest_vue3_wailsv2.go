package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
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

type RandomTestTask struct {
	FileName string
	Data     []byte
}

type TestFuncResultStatic struct {
	TestFuncName string
	ValuesNum    []int
	PCount       int
	PCounts      []int
	QCount       int
	QCounts      []int
	TestFunc1    func([]byte) (float64, float64)
	TestFunc2    func([]byte) ([]float64, []float64, int)
	FileDetails  []FileDetail
}

type OutputEvent struct {
	Type        string           `json:"type"`
	Level       string           `json:"level,omitempty"`
	Message     string           `json:"message,omitempty"`
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

func emitOutputEvent(event OutputEvent) {
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

func buildNISTResultRows(testNum int, stats []TestFuncResultStatic) []ResultTableRow {
	rows := make([]ResultTableRow, 0, len(stats))
	index := 1
	for _, item := range stats {
		if item.TestFunc2 != nil {
			for i := range item.PCounts {
				sampleSize := testNum
				if item.TestFuncName == "RandomExcursionsTest" || item.TestFuncName == "RandomExcursionsVariantTest" {
					sampleSize = item.ValuesNum[i]
				}
				pText := formatPassFraction(item.PCounts[i], sampleSize)
				resultText := "通过"
				if !isPassedByRange(item.PCounts[i], sampleSize) {
					resultText = "不通过"
				}
				rows = append(rows, ResultTableRow{
					Index:  index,
					Name:   fmt.Sprintf("%s[%d]", item.TestFuncName, i),
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
			Name:   item.TestFuncName,
			PValue: pText,
			QValue: "-",
			Result: resultText,
		})
		index++
	}
	return rows
}

func buildGMResultRows(testNum int, stats []TestFuncResultStatic) []ResultTableRow {
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
			Name:   item.TestFuncName,
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
}

func IsStopRandomTestRequested() bool {
	return stopTestRequested.Load()
}

func initTestLevel2NISTFuncResult() []TestFuncResultStatic {
	return []TestFuncResultStatic{
		{"FrequencyTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.FrequencyTestBytes(d)
		}, nil, nil},
		{"BlockFrequencyTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.BlockFrequencyTestBytes(d, 128)
		}, nil, nil},
		{"CumulativeSumsTest (Forward)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.CumulativeSumsTestBytes(d, true)
		}, nil, nil},
		{"CumulativeSumsTest (Backward)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.CumulativeSumsTestBytes(d, false)
		}, nil, nil},
		{"RunsTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.RunsTestBytes(d)
		}, nil, nil},
		{"LongestRunofOnesTest '1'", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.LongestRunOfOnesTestBytes(d, true)
		}, nil, nil},
		{"LongestRunofOnesTest '0'", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.LongestRunOfOnesTestBytes(d, false)
		}, nil, nil},
		{"RankTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.RankTestBytes(d, 32, 32)
		}, nil, nil},
		{"DiscreteFourierTransformTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.DiscreteFourierTransformTestBytes(d)
		}, nil, nil},
		{"NonOverlappingTemplateMatchingTest", make([]int, 148), 0, make([]int, 148), 0, []int{}, nil, func(d []byte) ([]float64, []float64, int) {
			return randomtest_core.NonOverlappingTemplateMatchingTestBytes(d, 1000000, 9)
		}, nil}, // 148 对应 template9
		{"OverlappingTemplateMatchingTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.OverlappingTemplateMatchingTestBytes(d, 1000000, 9)
		}, nil, nil},
		{"UniversalTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.UniversalTestBytes(d)
		}, nil, nil},
		{"ApproximateEntropyTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.ApproximateEntropyTestBytes(d, 10)
		}, nil, nil},
		{"RandomExcursionsTest", make([]int, 8), 0, make([]int, 8), 0, []int{}, nil, func(d []byte) ([]float64, []float64, int) {
			return randomtest_core.RandomExcursionsTestBytes(d, 1000000)
		}, nil},
		{"RandomExcursionsVariantTest", make([]int, 18), 0, make([]int, 18), 0, []int{}, nil, func(d []byte) ([]float64, []float64, int) {
			return randomtest_core.RandomExcursionsVariantTestBytes(d, 1000000)
		}, nil},
		{"SerialTest", nil, 0, nil, 0, nil, nil, nil, nil},
		{"LinearComplexityTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.LinearComplexityTestBytes(d, 500)
		}, nil, nil},
	}
}

func initTestLevel1GMFuncResult() []TestFuncResultStatic {
	return []TestFuncResultStatic{
		{"FrequencyTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.FrequencyTestBytes(d)
		}, nil, nil},
		{"BlockFrequencyTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.BlockFrequencyTestBytes(d, 1000)
		}, nil, nil},
		{"PokerTest (4)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.PokerTestBytes(d, 4)
		}, nil, nil},
		{"PokerTest (8)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.PokerTestBytes(d, 8)
		}, nil, nil},
		{"SerialTest (3)", nil, 0, nil, 0, nil, nil, nil, nil},
		{"SerialTest (5)", nil, 0, nil, 0, nil, nil, nil, nil},
		{"RunsTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.RunsTestBytes(d)
		}, nil, nil},
		{"RunsDistributionTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.RunsDistributionTestBytes(d)
		}, nil, nil},
		{"LongestRunofOnesTest '1'", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.LongestRunOfOnesTestBytes(d, true)
		}, nil, nil},
		{"LongestRunofOnesTest '0'", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.LongestRunOfOnesTestBytes(d, false)
		}, nil, nil},
		{"BinaryDerivativeTest (3)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.BinaryDerivativeTestBytes(d, 3)
		}, nil, nil},
		{"BinaryDerivativeTest (7)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.BinaryDerivativeTestBytes(d, 7)
		}, nil, nil},
		{"AutocorrelationTest (2)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.AutocorrelationTestBytes(d, 2)
		}, nil, nil},
		{"AutocorrelationTest (8)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.AutocorrelationTestBytes(d, 8)
		}, nil, nil},
		{"AutocorrelationTest (16)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.AutocorrelationTestBytes(d, 16)
		}, nil, nil},
		{"CumulativeSumsTest (Forward)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.CumulativeSumsTestBytes(d, true)
		}, nil, nil},
		{"CumulativeSumsTest (Backward)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.CumulativeSumsTestBytes(d, false)
		}, nil, nil},
		{"ApproximateEntropyTest (2)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.ApproximateEntropyTestBytes(d, 2)
		}, nil, nil},
		{"ApproximateEntropyTest (5)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.ApproximateEntropyTestBytes(d, 5)
		}, nil, nil},
		{"DiscreteFourierTransformTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.DiscreteFourierTransformTestBytes(d)
		}, nil, nil},
	}
}

func initTestLevel2GMFuncResult() []TestFuncResultStatic {
	return []TestFuncResultStatic{
		{"FrequencyTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.FrequencyTestBytes(d)
		}, nil, nil},
		{"BlockFrequencyTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.BlockFrequencyTestBytes(d, 10000)
		}, nil, nil},
		{"PokerTest (4)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.PokerTestBytes(d, 4)
		}, nil, nil},
		{"PokerTest (8)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.PokerTestBytes(d, 8)
		}, nil, nil},
		{"SerialTest (3)", nil, 0, nil, 0, nil, nil, nil, nil},
		{"SerialTest (5)", nil, 0, nil, 0, nil, nil, nil, nil},
		{"RunsTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.RunsTestBytes(d)
		}, nil, nil},
		{"RunsDistributionTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.RunsDistributionTestBytes(d)
		}, nil, nil},
		{"LongestRunofOnesTest '1'", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.LongestRunOfOnesTestBytes(d, true)
		}, nil, nil},
		{"LongestRunofOnesTest '0'", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.LongestRunOfOnesTestBytes(d, false)
		}, nil, nil},
		{"BinaryDerivativeTest (3)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.BinaryDerivativeTestBytes(d, 3)
		}, nil, nil},
		{"BinaryDerivativeTest (7)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.BinaryDerivativeTestBytes(d, 7)
		}, nil, nil},
		{"AutocorrelationTest (1)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.AutocorrelationTestBytes(d, 1)
		}, nil, nil},
		{"AutocorrelationTest (2)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.AutocorrelationTestBytes(d, 2)
		}, nil, nil},
		{"AutocorrelationTest (8)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.AutocorrelationTestBytes(d, 8)
		}, nil, nil},
		{"AutocorrelationTest (16)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.AutocorrelationTestBytes(d, 16)
		}, nil, nil},
		{"RankTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.RankTestBytes(d, 32, 32)
		}, nil, nil},
		{"CumulativeSumsTest (Forward)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.CumulativeSumsTestBytes(d, true)
		}, nil, nil},
		{"CumulativeSumsTest (Backward)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.CumulativeSumsTestBytes(d, false)
		}, nil, nil},
		{"ApproximateEntropyTest (2)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.ApproximateEntropyTestBytes(d, 2)
		}, nil, nil},
		{"ApproximateEntropyTest (5)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.ApproximateEntropyTestBytes(d, 5)
		}, nil, nil},
		{"LinearComplexityTest (500)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.LinearComplexityTestBytes(d, 500)
		}, nil, nil},
		{"LinearComplexityTest (1000)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.LinearComplexityTestBytes(d, 1000)
		}, nil, nil},
		{"UniversalTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.UniversalTestBytes(d)
		}, nil, nil},
		{"DiscreteFourierTransformTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.DiscreteFourierTransformTestBytes(d)
		}, nil, nil},
	}
}

func initTestLevel3GMFuncResult() []TestFuncResultStatic {
	return []TestFuncResultStatic{
		{"FrequencyTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.FrequencyTestBytes(d)
		}, nil, nil},
		{"BlockFrequencyTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.BlockFrequencyTestBytes(d, 100000)
		}, nil, nil},
		{"PokerTest (4)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.PokerTestBytes(d, 4)
		}, nil, nil},
		{"PokerTest (8)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.PokerTestBytes(d, 8)
		}, nil, nil},
		{"SerialTest (3)", nil, 0, nil, 0, nil, nil, nil, nil},
		{"SerialTest (5)", nil, 0, nil, 0, nil, nil, nil, nil},
		{"SerialTest (7)", nil, 0, nil, 0, nil, nil, nil, nil},
		{"RunsTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.RunsTestBytes(d)
		}, nil, nil},
		{"RunsDistributionTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.RunsDistributionTestBytes(d)
		}, nil, nil},
		{"LongestRunofOnesTest '1'", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.LongestRunOfOnesTestBytes(d, true)
		}, nil, nil},
		{"LongestRunofOnesTest '0'", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.LongestRunOfOnesTestBytes(d, false)
		}, nil, nil},
		{"BinaryDerivativeTest (3)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.BinaryDerivativeTestBytes(d, 3)
		}, nil, nil},
		{"BinaryDerivativeTest (7)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.BinaryDerivativeTestBytes(d, 7)
		}, nil, nil},
		{"BinaryDerivativeTest (15)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.BinaryDerivativeTestBytes(d, 15)
		}, nil, nil},
		{"AutocorrelationTest (1)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.AutocorrelationTestBytes(d, 1)
		}, nil, nil},
		{"AutocorrelationTest (2)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.AutocorrelationTestBytes(d, 2)
		}, nil, nil},
		{"AutocorrelationTest (8)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.AutocorrelationTestBytes(d, 8)
		}, nil, nil},
		{"AutocorrelationTest (16)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.AutocorrelationTestBytes(d, 16)
		}, nil, nil},
		{"AutocorrelationTest (32)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.AutocorrelationTestBytes(d, 32)
		}, nil, nil},
		{"RankTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.RankTestBytes(d, 32, 32)
		}, nil, nil},
		{"CumulativeSumsTest (Forward)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.CumulativeSumsTestBytes(d, true)
		}, nil, nil},
		{"CumulativeSumsTest (Backward)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.CumulativeSumsTestBytes(d, false)
		}, nil, nil},
		{"ApproximateEntropyTest (5)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.ApproximateEntropyTestBytes(d, 5)
		}, nil, nil},
		{"ApproximateEntropyTest (7)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.ApproximateEntropyTestBytes(d, 7)
		}, nil, nil},
		{"LinearComplexityTest (5000)", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.LinearComplexityTestBytes(d, 5000)
		}, nil, nil},
		{"UniversalTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.UniversalTestBytes(d)
		}, nil, nil},
		{"DiscreteFourierTransformTest", nil, 0, nil, 0, nil, func(d []byte) (float64, float64) {
			return randomtest_core.DiscreteFourierTransformTestBytes(d)
		}, nil, nil},
	}
}

func jisuan(sampleSize int) (int, int) {
	sampleSizeF := float64(sampleSize)
	proportion_threshold_max := (randomtest_core.AlphaHAT + 3.0*math.Sqrt((randomtest_core.AlphaHAT*randomtest_core.Alpha)/sampleSizeF)) * sampleSizeF
	proportion_threshold_min := (randomtest_core.AlphaHAT - 3.0*math.Sqrt((randomtest_core.AlphaHAT*randomtest_core.Alpha)/sampleSizeF)) * sampleSizeF
	return int(proportion_threshold_max), int(proportion_threshold_min)
}

func printTestLevelXNISTFuncResults(testNum int, testFuncResultStatics []TestFuncResultStatic) {
	emitLogf("info", "\n📊 NIST Level%d 测试结果统计\n", RunStandardRandomTestEg.TestLevel)
	emitLogf("info", "%s\n", strings.Repeat("-", 75))
	emitLogf("info", "%-45s | %20s\n", "TestFuncName", "PValue Passed-Count")
	emitLogf("info", "%s\n", strings.Repeat("-", 75))
	pPassedNumMax, pPassedNumMin := jisuan(testNum)
	for _, testFuncResultStaticItem := range testFuncResultStatics {
		if testFuncResultStaticItem.TestFunc2 != nil {
			if testFuncResultStaticItem.TestFuncName == "RandomExcursionsTest" || testFuncResultStaticItem.TestFuncName == "RandomExcursionsVariantTest" {
				for i, pCount := range testFuncResultStaticItem.PCounts {
					pPassedNum1Max, pPassedNum1Min := jisuan(testFuncResultStaticItem.ValuesNum[i])
					if pCount < pPassedNum1Min || pCount > pPassedNum1Max {
						emitLogf("info", "%-41s %3d | %9d/%-9d *\n", testFuncResultStaticItem.TestFuncName, i, pCount, testFuncResultStaticItem.ValuesNum[i])
						continue
					}
					emitLogf("info", "%-41s %3d | %9d/%-9d  \n", testFuncResultStaticItem.TestFuncName, i, pCount, testFuncResultStaticItem.ValuesNum[i])
				}
				continue
			}
			for i, pCount := range testFuncResultStaticItem.PCounts {
				if pCount < pPassedNumMin || pCount > pPassedNumMax {
					emitLogf("info", "%-41s %3d | %9d/%-9d *\n", testFuncResultStaticItem.TestFuncName, i, pCount, testNum)
					continue
				}
				emitLogf("info", "%-41s %3d | %9d/%-9d  \n", testFuncResultStaticItem.TestFuncName, i, pCount, testNum)
			}
			continue
		}
		// if testFuncResultStaticItem.TestFunc1 != nil {
		if testFuncResultStaticItem.PCount < pPassedNumMin || testFuncResultStaticItem.PCount > pPassedNumMax {
			emitLogf("info", "%-45s | %9d/%-9d *\n", testFuncResultStaticItem.TestFuncName, testFuncResultStaticItem.PCount, testNum)
			continue
		}
		emitLogf("info", "%-45s | %9d/%-9d  \n", testFuncResultStaticItem.TestFuncName, testFuncResultStaticItem.PCount, testNum)
		// }
	}
	emitLogf("info", "%s\n", strings.Repeat("-", 75))
}

func printTestLevelXGMFuncResults(testNum int, testFuncResultStatics []TestFuncResultStatic) {
	emitLogf("info", "\n📊 GM Level%d 测试结果统计\n", RunStandardRandomTestEg.TestLevel)
	emitLogf("info", "%s\n", strings.Repeat("-", 85))
	emitLogf("info", "%-35s | %21s | %21s\n", "TestFuncName", "PValue Passed-Count", "QValue Passed-Count")
	emitLogf("info", "%s\n", strings.Repeat("-", 85))
	pPassedNumMax, pPassedNumMin := jisuan(testNum)
	qPassedNumMax := pPassedNumMax
	qPassedNumMin := pPassedNumMin
	for _, testFuncResultStaticItem := range testFuncResultStatics {
		pPassedStr := "  "
		qPassedStr := "  "
		if testFuncResultStaticItem.PCount < pPassedNumMin || testFuncResultStaticItem.PCount > pPassedNumMax {
			pPassedStr = " *"
		}
		if testFuncResultStaticItem.QCount < qPassedNumMin || testFuncResultStaticItem.QCount > qPassedNumMax {
			qPassedStr = " *"
		}
		emitLogf("info", "%-35s | %9d/%-9d%2s | %9d/%-9d%2s\n", testFuncResultStaticItem.TestFuncName, testFuncResultStaticItem.PCount, testNum, pPassedStr, testFuncResultStaticItem.QCount, testNum, qPassedStr)
	}
	emitLogf("info", "%s\n", strings.Repeat("-", 85))
}

func printTestNISTOrGMErrorFileList(results []TestFuncResultStatic) {
	emitLogf("info", "\n❌ NIST or GM 测试不通过的文件清单\n")
	emitLogf("info", "%s\n", strings.Repeat("=", 85))
	for _, test := range results {
		hasFail := false
		emitLogf("info", "测试项: %s\n", test.TestFuncName)
		for _, detail := range test.FileDetails {
			if !detail.PPassed || !detail.QPassed {
				status := ""
				if !detail.PPassed {
					status += "P不通过 "
				}
				if !detail.QPassed {
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
}

// 验证 P 值是否通过检测
func isPValueOkBool(pValue float64) bool {
	return pValue >= randomtest_core.Alpha && pValue <= 1 // ✅ pValue Pass 或 ❌ pValue Fail
}

// 验证 Q 值是否通过检测
func isQValueOkBool(qValue float64) bool {
	return qValue >= randomtest_core.AlphaT && qValue <= 1 // ✅ qValue Pass 或 ❌ qValue Fail
}

func runLevel2NIST(data []byte, testLevel2NISTFuncResults []TestFuncResultStatic) {
	// fmt.Println("\n🚩 [Info] 运行 NIST 测试 (标准等级为 2)")
	for i := range testLevel2NISTFuncResults {
		item := &testLevel2NISTFuncResults[i]
		switch item.TestFuncName {
		case "SerialTest":
			pValue1, _, pValue2, _ := randomtest_core.SerialTestBytes(data, 16)
			if isPValueOkBool(pValue1) && isPValueOkBool(pValue2) {
				item.PCount++
			}
		default:
			if item.TestFunc1 != nil {
				if pValue, _ := item.TestFunc1(data); isPValueOkBool(pValue) {
					item.PCount++
				}
			}
			if item.TestFunc2 != nil {
				pValues, _, pValuesNum := item.TestFunc2(data)
				for i := 0; i < pValuesNum; i++ {
					if item.TestFuncName == "RandomExcursionsTest" || item.TestFuncName == "RandomExcursionsVariantTest" {
						if pValues[i] > 0 {
							item.ValuesNum[i]++
						}
						if isPValueOkBool(pValues[i]) {
							item.PCounts[i]++
						}
						continue
					}
					if isPValueOkBool(pValues[i]) {
						item.PCounts[i]++
					}
				}
			}
		}
	}
}

func runLevel1GM(data []byte, testLevel1GMFuncResults []TestFuncResultStatic) {
	// fmt.Println("\n🚩 [Info] 运行 GM 测试 (标准等级为 1)")
	for i := range testLevel1GMFuncResults {
		item := &testLevel1GMFuncResults[i]
		switch item.TestFuncName {
		case "SerialTest (3)":
			pValue1, qValue1, pValue2, qValue2 := randomtest_core.SerialTestBytes(data, 3)
			if isPValueOkBool(pValue1) && isPValueOkBool(pValue2) {
				item.PCount++
			}
			if isQValueOkBool(qValue1) && isQValueOkBool(qValue2) {
				item.QCount++
			}
		case "SerialTest (5)":
			pValue1, qValue1, pValue2, qValue2 := randomtest_core.SerialTestBytes(data, 5)
			if isPValueOkBool(pValue1) && isPValueOkBool(pValue2) {
				item.PCount++
			}
			if isQValueOkBool(qValue1) && isQValueOkBool(qValue2) {
				item.QCount++
			}
		default:
			if item.TestFunc1 != nil {
				pValue, qValue := item.TestFunc1(data)
				if isPValueOkBool(pValue) {
					item.PCount++
				}
				if isQValueOkBool(qValue) {
					item.QCount++
				}
			}
		}
	}
}

func runLevel2GM(data []byte, testLevel2GMFuncResults []TestFuncResultStatic) {
	// fmt.Println("\n🚩 [Info] 运行 GM 测试 (标准等级为 2)")
	for i := range testLevel2GMFuncResults {
		item := &testLevel2GMFuncResults[i]
		switch item.TestFuncName {
		case "SerialTest (3)":
			pValue1, qValue1, pValue2, qValue2 := randomtest_core.SerialTestBytes(data, 3)
			if isPValueOkBool(pValue1) && isPValueOkBool(pValue2) {
				item.PCount++
			}
			if isQValueOkBool(qValue1) && isQValueOkBool(qValue2) {
				item.QCount++
			}
		case "SerialTest (5)":
			pValue1, qValue1, pValue2, qValue2 := randomtest_core.SerialTestBytes(data, 5)
			if isPValueOkBool(pValue1) && isPValueOkBool(pValue2) {
				item.PCount++
			}
			if isQValueOkBool(qValue1) && isQValueOkBool(qValue2) {
				item.QCount++
			}
		default:
			if item.TestFunc1 != nil {
				pValue, qValue := item.TestFunc1(data)
				if isPValueOkBool(pValue) {
					item.PCount++
				}
				if isQValueOkBool(qValue) {
					item.QCount++
				}
			}
		}
	}
}

func runLevel3GM(data []byte, testLevel3GMFuncResults []TestFuncResultStatic) {
	// fmt.Println("\n🚩 [Info] 运行 GM 测试 (标准等级为 3)")
	for i := range testLevel3GMFuncResults {
		item := &testLevel3GMFuncResults[i]
		switch item.TestFuncName {
		case "SerialTest (3)":
			pValue1, qValue1, pValue2, qValue2 := randomtest_core.SerialTestBytes(data, 3)
			if isPValueOkBool(pValue1) && isPValueOkBool(pValue2) {
				item.PCount++
			}
			if isQValueOkBool(qValue1) && isQValueOkBool(qValue2) {
				item.QCount++
			}
		case "SerialTest (5)":
			pValue1, qValue1, pValue2, qValue2 := randomtest_core.SerialTestBytes(data, 5)
			if isPValueOkBool(pValue1) && isPValueOkBool(pValue2) {
				item.PCount++
			}
			if isQValueOkBool(qValue1) && isQValueOkBool(qValue2) {
				item.QCount++
			}
		case "SerialTest (7)":
			pValue1, qValue1, pValue2, qValue2 := randomtest_core.SerialTestBytes(data, 7)
			if isPValueOkBool(pValue1) && isPValueOkBool(pValue2) {
				item.PCount++
			}
			if isQValueOkBool(qValue1) && isQValueOkBool(qValue2) {
				item.QCount++
			}
		default:
			if item.TestFunc1 != nil {
				pValue, qValue := item.TestFunc1(data)
				if isPValueOkBool(pValue) {
					item.PCount++
				}
				if isQValueOkBool(qValue) {
					item.QCount++
				}
			}
		}
	}
}

func (runStandardRandomTestStruct *RunStandardRandomTestStruct) HandleRandomTestStandardRunMulti() error {
	emitLogf("info", "\n🚩 [Info] 开始执行并发随机性测试\n")
	if IsStopRandomTestRequested() {
		return fmt.Errorf("测试已停止")
	}

	if runStandardRandomTestStruct.IsFile {
		file, err := os.Open(runStandardRandomTestStruct.FilePath)
		if err != nil {
			return emitErrorf("[Error] 文件打开失败: %v", err)
		}
		defer file.Close()

		switch runStandardRandomTestStruct.TestType {
		case "N": // NIST 测试
			if runStandardRandomTestStruct.TestLevel == 2 { // NIST 测试 (默认标准等级为 2)
				total := runStandardRandomTestStruct.TestNum
				if total <= 0 {
					return emitErrorf("[Error] TestNum 必须大于 0")
				}
				emitProgress("NIST_FILE", 0, total, "")

				const nistBlockSize = 125000 // 1000000 bits = 125000 bytes
				taskChan := make(chan []byte, WorkerCount*2)
				resultChan := make(chan []TestFuncResultStatic, WorkerCount)
				var wg sync.WaitGroup
				var readErr error
				var readMutex sync.Mutex
				var processed int64
				progressStep := total / 100
				if progressStep < 1 {
					progressStep = 1
				}

				/* 工作池初始化 */
				for i := 0; i < WorkerCount; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						workerResults := initTestLevel2NISTFuncResult()
						for data := range taskChan {
							if IsStopRandomTestRequested() {
								break
							}
							runLevel2NIST(data, workerResults)
							done := int(atomic.AddInt64(&processed, 1))
							if done%progressStep == 0 || done == total {
								emitProgress("NIST_FILE", done, total, "")
							}
						}
						resultChan <- workerResults
					}()
				}
				/* 文件读取协程 */
				go func() {
					defer close(taskChan)
					for i := 0; i < total; i++ {
						if IsStopRandomTestRequested() {
							return
						}
						buf := make([]byte, nistBlockSize)
						n, err := io.ReadFull(file, buf)
						if err == io.EOF {
							readMutex.Lock()
							readErr = emitErrorf("[Error] 文件数据不足, 需要 %d 字节, 仅读取 %d 字节 (已处理 %d/%d 块)", nistBlockSize, n, i, total)
							readMutex.Unlock()
							return
						}
						if err != nil {
							readMutex.Lock()
							readErr = emitErrorf("[Error] 文件读取失败 (块 %d/%d): %v", i+1, total, err)
							readMutex.Unlock()
							return
						}
						taskChan <- buf
					}
				}()
				/* 结果收集协程 */
				go func() {
					wg.Wait()
					close(resultChan)
				}()
				/* 结果合并 */
				finalResults := initTestLevel2NISTFuncResult()
				resultsProcessed := 0
				for res := range resultChan {
					resultsProcessed++
					for i := range finalResults {
						final := &finalResults[i]
						partial := &res[i]
						if final.ValuesNum != nil && partial.ValuesNum != nil {
							for j := range final.ValuesNum {
								final.ValuesNum[j] += partial.ValuesNum[j]
							}
						}
						final.PCount += partial.PCount
						if final.PCounts != nil && partial.PCounts != nil {
							for j := range final.PCounts {
								final.PCounts[j] += partial.PCounts[j]
							}
						}
						final.QCount += partial.QCount
						if final.QCounts != nil && partial.QCounts != nil {
							for j := range final.QCounts {
								final.QCounts[j] += partial.QCounts[j]
							}
						}
					} // 合并每个 worker 的结果
					res = nil // 清空引用，帮助 GC
				}
				/* 检查错误 */
				readMutex.Lock()
				localReadErr := readErr
				readMutex.Unlock()
				if localReadErr != nil {
					return emitErrorf("[Error] 处理失败: %v (已完成 %d/%d 个工作线程)", localReadErr, resultsProcessed, WorkerCount)
				}
				if IsStopRandomTestRequested() {
					return fmt.Errorf("测试已停止")
				}
				emitProgress("NIST_FILE", total, total, "")
				emitLogf("info", "\n✅ 成功处理 %d 个工作线程的结果\n", resultsProcessed)
				printTestLevelXNISTFuncResults(total, finalResults)
				setLatestResultRows(buildNISTResultRows(total, finalResults))
			} else {
				return emitErrorf("[Error] NIST 测试必须为默认标准等级 2")
			}
		case "G": // GM 测试
			var (
				gmBlockSize  int
				testInitFunc func() []TestFuncResultStatic
				testFunc     func([]byte, []TestFuncResultStatic)
			)

			switch runStandardRandomTestStruct.TestLevel {
			case 1: // GM 测试 (标准等级为 1)
				// 每次读取 runStandardRandomTestStruct.FilePath 文件中的 20000 bit 数据到 data 对象中, 共读取 runStandardRandomTestStruct.TestNum 次, 每次都调用 runLevel1GM() 函数
				gmBlockSize = 2500 // 20000 bits = 2500 bytes
				testInitFunc = initTestLevel1GMFuncResult
				testFunc = runLevel1GM
			case 2: // GM 测试 (标准等级为 2)
				// 每次读取 runStandardRandomTestStruct.FilePath 文件中的 1000000 bit 数据到 data 对象中, 共读取 runStandardRandomTestStruct.TestNum 次, 每次都调用 runLevel2GM() 函数
				gmBlockSize = 125000 // 1000000 bits = 125000 bytes
				testInitFunc = initTestLevel2GMFuncResult
				testFunc = runLevel2GM
			case 3: // GM 测试 (标准等级为 3)
				// 每次读取 runStandardRandomTestStruct.FilePath 文件中的 100000000 bit 数据到 data 对象中, 共读取 runStandardRandomTestStruct.TestNum 次, 每次都调用 runLevel3GM() 函数
				gmBlockSize = 12500000 // 100000000 bits = 12500000 bytes
				testInitFunc = initTestLevel3GMFuncResult
				testFunc = runLevel3GM
			default:
				return emitErrorf("[Error] GM 测试必须为标准等级 1 或 2 或 3")
			}

			total := runStandardRandomTestStruct.TestNum
			if total <= 0 {
				return emitErrorf("[Error] TestNum 必须大于 0")
			}
			emitProgress("GM_FILE", 0, total, "")

			/** 并发处理核心逻辑 **/
			/* 使用 sync.Pool 复用缓冲区, 减少 GC 压力 */
			bufferPool := &sync.Pool{
				New: func() interface{} {
					return make([]byte, gmBlockSize)
				},
			}
			/* 动态调整通道容量，避免过度内存占用 */
			channelSize := WorkerCount * 2
			if gmBlockSize > 1000000 { // 对于大块数据, 减小通道容量
				channelSize = WorkerCount
			}
			taskChan := make(chan []byte, channelSize)
			resultChan := make(chan []TestFuncResultStatic, WorkerCount)
			errChan := make(chan error, 1) // 用于传递错误
			var wg sync.WaitGroup
			var once sync.Once // 确保只记录一次错误
			var processed int64
			progressStep := total / 100
			if progressStep < 1 {
				progressStep = 1
			}
			/* 工作池初始化 */
			for i := 0; i < WorkerCount; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					workerResults := testInitFunc()
					for data := range taskChan {
						if IsStopRandomTestRequested() {
							break
						}
						testFunc(data, workerResults)
						done := int(atomic.AddInt64(&processed, 1))
						if done%progressStep == 0 || done == total {
							emitProgress("GM_FILE", done, total, "")
						}
						bufferPool.Put(data[:gmBlockSize])
					}
					resultChan <- workerResults
				}()
			}
			/* 文件读取协程 */
			go func() {
				defer close(taskChan)
				for i := 0; i < total; i++ {
					if IsStopRandomTestRequested() {
						return
					}
					buf := bufferPool.Get().([]byte) // 从池中获取缓冲区
					/* 读取数据块 */
					n, err := io.ReadFull(file, buf[:gmBlockSize])
					if err == io.EOF {
						once.Do(func() {
							select {
							case errChan <- emitErrorf("[Error] 文件数据不足, 需要 %d 字节, 仅读取 %d 字节 (已处理 %d/%d 块)", gmBlockSize, n, i, total):
							default:
							}
						})
						bufferPool.Put(buf)
						return
					}
					if err != nil {
						once.Do(func() {
							select {
							case errChan <- emitErrorf("[Error] 文件读取失败 (块 %d/%d): %v", i+1, total, err):
							default:
							}
						})
						bufferPool.Put(buf)
						return
					}

					select {
					case taskChan <- buf[:gmBlockSize]:
					case <-errChan: // 如果发生错误, 停止读取
						bufferPool.Put(buf)
						return
					}
				}
			}()
			/* 结果收集协程 */
			go func() {
				wg.Wait()
				close(resultChan)
			}()
			/* 结果合并 */
			finalResults := testInitFunc()
			resultsProcessed := 0
			for res := range resultChan {
				resultsProcessed++
				for i := range finalResults {
					final := &finalResults[i]
					partial := &res[i]
					/* 合并 ValuesNum */
					if final.ValuesNum != nil && partial.ValuesNum != nil {
						for j := range final.ValuesNum {
							final.ValuesNum[j] += partial.ValuesNum[j]
						}
					}
					/* 合并 PCount 和 PCounts */
					final.PCount += partial.PCount
					if final.PCounts != nil && partial.PCounts != nil {
						for j := range final.PCounts {
							final.PCounts[j] += partial.PCounts[j]
						}
					}
					/* 合并 QCount 和 QCounts */
					final.QCount += partial.QCount
					if final.QCounts != nil && partial.QCounts != nil {
						for j := range final.QCounts {
							final.QCounts[j] += partial.QCounts[j]
						}
					}
				}
				res = nil // 清空引用，帮助 GC
			}
			/* 检查错误 */
			select {
			case err := <-errChan:
				if err != nil {
					return emitErrorf("[Error] 处理失败: %v (已完成 %d/%d 个工作线程)", err, resultsProcessed, WorkerCount)
				}
			default:
			}
			if IsStopRandomTestRequested() {
				return fmt.Errorf("测试已停止")
			}
			emitProgress("GM_FILE", total, total, "")
			emitLogf("info", "\n✅ 成功处理 %d 个工作线程的结果\n", resultsProcessed)
			printTestLevelXGMFuncResults(total, finalResults)
			setLatestResultRows(buildGMResultRows(total, finalResults))
		default:
			return emitErrorf("请选择规定的检测标准类型 (NIST'N' / GM'G')")
		}
	} else {
		files, err := ioutil.ReadDir(runStandardRandomTestStruct.RootDirPath)
		if err != nil {
			return emitErrorf("[Error] 目录读取失败: %v", err)
		}
		filesNum := 0

		switch runStandardRandomTestStruct.TestType {
		case "N": // NIST 测试
			if runStandardRandomTestStruct.TestLevel == 2 { // NIST 测试 (默认标准等级为 2)
				// 每次读取目录下一个符合要求的文件, 每个文件执行一次 runLevel2NIST()
				const nistFileSize = 125000 // 1000000 bits = 125000 bytes
				validFiles := make([]os.FileInfo, 0, len(files))
				for _, fileInfo := range files {
					if fileInfo.IsDir() {
						continue
					}
					if fileInfo.Size() != nistFileSize {
						emitLogf("warn", "[WARNING] 文件 %s 大小不符合要求 (%d != %d), 跳过\n", fileInfo.Name(), fileInfo.Size(), nistFileSize)
						continue
					}
					validFiles = append(validFiles, fileInfo)
				}
				total := len(validFiles)
				if total == 0 {
					return emitErrorf("[Error] 没有符合 NIST Level2 要求的输入文件")
				}
				emitProgress("NIST_DIR", 0, total, "")

				taskChan := make(chan RandomTestTask, WorkerCount*2)
				resultChan := make(chan []TestFuncResultStatic, WorkerCount)
				var wg sync.WaitGroup
				var readErr error
				var readMutex sync.Mutex
				var processed int64
				progressStep := total / 100
				if progressStep < 1 {
					progressStep = 1
				}
				/* 工作池初始化 */
				for i := 0; i < WorkerCount; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						workerResults := initTestLevel2NISTFuncResult()
						for task := range taskChan {
							if IsStopRandomTestRequested() {
								break
							}
							// 记录本次测试前后的 PCount/QCount 变化来判断当前文件是否通过
							for j := range workerResults {
								item := &workerResults[j]
								oldP, oldQ := item.PCount, item.QCount
								// 执行测试函数
								runLevel2NIST(task.Data, workerResults[j:j+1])
								// 记录该文件在当前测试项的结果
								item.FileDetails = append(item.FileDetails, FileDetail{
									FileName: task.FileName,
									PPassed:  item.PCount > oldP,
									QPassed:  item.QCount > oldQ,
								})
							}
							done := int(atomic.AddInt64(&processed, 1))
							if done%progressStep == 0 || done == total {
								emitProgress("NIST_DIR", done, total, task.FileName)
							}
						}
						resultChan <- workerResults
					}()
				}
				/* 文件读取协程 */
				go func() {
					defer close(taskChan)
					for _, fileInfo := range validFiles {
						if IsStopRandomTestRequested() {
							return
						}
						filePath := filepath.Join(runStandardRandomTestStruct.RootDirPath, fileInfo.Name())
						data, err := os.ReadFile(filePath)
						if err != nil {
							readMutex.Lock()
							readErr = emitErrorf("[Error] 文件 %s 读取失败: %v", filePath, err)
							readMutex.Unlock()
							return
						}
						filesNum++
						taskChan <- RandomTestTask{FileName: fileInfo.Name(), Data: data} // 传输带文件详情的 Task
					}
				}()
				/* 结果收集协程 */
				go func() {
					wg.Wait()
					close(resultChan)
				}()
				/* 结果合并 */
				finalResults := initTestLevel2NISTFuncResult()
				for res := range resultChan {
					for i := range finalResults {
						final := &finalResults[i]
						partial := &res[i]
						if final.ValuesNum != nil && partial.ValuesNum != nil {
							for j := range final.ValuesNum {
								final.ValuesNum[j] += partial.ValuesNum[j]
							}
						}
						final.PCount += partial.PCount
						if final.PCounts != nil && partial.PCounts != nil {
							for j := range final.PCounts {
								final.PCounts[j] += partial.PCounts[j]
							}
						}
						final.QCount += partial.QCount
						if final.QCounts != nil && partial.QCounts != nil {
							for j := range final.QCounts {
								final.QCounts[j] += partial.QCounts[j]
							}
						}
						final.FileDetails = append(final.FileDetails, partial.FileDetails...) // 合并文件详情
					} // 合并每个 worker 的结果
				}
				/* 检查读取错误 */
				readMutex.Lock()
				localReadErr := readErr
				readMutex.Unlock()
				if localReadErr != nil {
					return emitErrorf("[Error] 处理失败: %v", readErr)
				}
				if IsStopRandomTestRequested() {
					return fmt.Errorf("测试已停止")
				}
				emitProgress("NIST_DIR", total, total, "")
				printTestLevelXNISTFuncResults(filesNum, finalResults)
				printTestNISTOrGMErrorFileList(finalResults)
				setLatestResultRows(buildNISTResultRows(filesNum, finalResults))
			} else {
				return emitErrorf("[Error] NIST 测试必须为默认标准等级 2")
			}
		case "G": // GM 测试
			var (
				gmFileSize   int64
				testInitFunc func() []TestFuncResultStatic
				testFunc     func([]byte, []TestFuncResultStatic)
			)

			switch runStandardRandomTestStruct.TestLevel {
			case 1: // GM 测试 (标准等级为 1)
				// 每次读取 runStandardRandomTestStruct.RootDirPath 目录中一个文件, 文件大小需要是 20000 bit，读取的数据到 data 对象中，共读取目录下的文件数量次, 每次都调用 runLevel1GM() 函数
				gmFileSize = 2500 // 20000 bits = 2500 bytes
				testInitFunc = initTestLevel1GMFuncResult
				testFunc = runLevel1GM
			case 2: // GM 测试 (标准等级为 2)
				// 每次读取 runStandardRandomTestStruct.RootDirPath 目录中一个文件, 文件大小需要是 1000000 bit，读取的数据到 data 对象中，共读取目录下的文件数量次, 每次都调用 runLevel2GM() 函数
				gmFileSize = 125000 // 1000000 bits = 125000 bytes
				testInitFunc = initTestLevel2GMFuncResult
				testFunc = runLevel2GM
			case 3: // GM 测试 (标准等级为 3)
				// 每次读取 runStandardRandomTestStruct.RootDirPath 目录中一个文件, 文件大小需要是 100000000 bit，读取的数据到 data 对象中，共读取目录下的文件数量次, 每次都调用 runLevel3GM() 函数
				gmFileSize = 12500000 // 100000000 bits = 12500000 bytes
				testInitFunc = initTestLevel3GMFuncResult
				testFunc = runLevel3GM
			default:
				return emitErrorf("[Error] GM 测试必须为标准等级 1 或 2 或 3")
			}

			/** 并发处理核心逻辑 **/
			validFiles := make([]os.FileInfo, 0, len(files))
			for _, fileInfo := range files {
				if fileInfo.IsDir() {
					continue
				}
				if fileInfo.Size() != gmFileSize {
					emitLogf("warn", "[WARNING] 文件 %s 大小不符合要求 (%d != %d), 跳过\n", fileInfo.Name(), fileInfo.Size(), gmFileSize)
					continue
				}
				validFiles = append(validFiles, fileInfo)
			}
			total := len(validFiles)
			if total == 0 {
				return emitErrorf("[Error] 没有符合 GM Level%d 要求的输入文件", runStandardRandomTestStruct.TestLevel)
			}
			emitProgress("GM_DIR", 0, total, "")

			taskChan := make(chan RandomTestTask, WorkerCount*2)
			resultChan := make(chan []TestFuncResultStatic, WorkerCount)
			var wg sync.WaitGroup
			var readErr error
			var readMutex sync.Mutex
			var processed int64
			progressStep := total / 100
			if progressStep < 1 {
				progressStep = 1
			}
			/* 工作池初始化 */
			for i := 0; i < WorkerCount; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					workerResults := testInitFunc()
					for task := range taskChan {
						if IsStopRandomTestRequested() {
							break
						}
						// 记录本次测试前后的 PCount/QCount 变化来判断当前文件是否通过
						for j := range workerResults {
							item := &workerResults[j]
							oldP, oldQ := item.PCount, item.QCount
							// 执行测试函数
							testFunc(task.Data, workerResults[j:j+1])
							// 记录该文件在当前测试项的结果
							item.FileDetails = append(item.FileDetails, FileDetail{
								FileName: task.FileName,
								PPassed:  item.PCount > oldP,
								QPassed:  item.QCount > oldQ,
							})
						}
						done := int(atomic.AddInt64(&processed, 1))
						if done%progressStep == 0 || done == total {
							emitProgress("GM_DIR", done, total, task.FileName)
						}
					}
					resultChan <- workerResults
				}()
			}
			/* 文件读取协程 */
			go func() {
				defer close(taskChan)
				for _, fileInfo := range validFiles {
					if IsStopRandomTestRequested() {
						return
					}
					filePath := filepath.Join(runStandardRandomTestStruct.RootDirPath, fileInfo.Name())
					data, err := os.ReadFile(filePath)
					if err != nil {
						readMutex.Lock()
						readErr = emitErrorf("[Error] 文件 %s 读取失败: %v", filePath, err)
						readMutex.Unlock()
						return
					}
					filesNum++
					taskChan <- RandomTestTask{FileName: fileInfo.Name(), Data: data} // 传输带文件详情的 Task
				}
			}()
			/* 结果收集协程 */
			go func() {
				wg.Wait()
				close(resultChan)
			}()
			/* 结果合并 */
			finalResults := testInitFunc()
			for res := range resultChan {
				for i := range finalResults {
					final := &finalResults[i]
					partial := &res[i]
					if final.ValuesNum != nil && partial.ValuesNum != nil {
						for j := range final.ValuesNum {
							final.ValuesNum[j] += partial.ValuesNum[j]
						}
					}
					final.PCount += partial.PCount
					if final.PCounts != nil && partial.PCounts != nil {
						for j := range final.PCounts {
							final.PCounts[j] += partial.PCounts[j]
						}
					}
					final.QCount += partial.QCount
					if final.QCounts != nil && partial.QCounts != nil {
						for j := range final.QCounts {
							final.QCounts[j] += partial.QCounts[j]
						}
					}
					final.FileDetails = append(final.FileDetails, partial.FileDetails...) // 合并文件详情
				} // 合并每个 worker 的结果
				res = nil
			}
			/* 检查读取错误 */
			readMutex.Lock()
			localReadErr := readErr
			readMutex.Unlock()
			if localReadErr != nil {
				return emitErrorf("[Error] 处理失败: %v", localReadErr)
			}
			if IsStopRandomTestRequested() {
				return fmt.Errorf("测试已停止")
			}
			emitProgress("GM_DIR", total, total, "")
			printTestLevelXGMFuncResults(filesNum, finalResults)
			printTestNISTOrGMErrorFileList(finalResults)
			setLatestResultRows(buildGMResultRows(filesNum, finalResults))
		default:
			return emitErrorf("请选择规定的检测标准类型 (NIST'N' / GM'G')")
		}
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
	return nil
}

// 前置初始化
func init() {
	WorkerCount = runtime.NumCPU() * 2 // 动态工作池大小
	_ = loadIniConfig(false)
}

// 根据 ini 配置执行标准检测流程。
func RunSTDTestFromIniConfig() error {
	if err := loadIniConfig(true); err != nil {
		return err
	}
	randomtest_core.SetDebug(false)
	return RunStandardRandomTestEg.HandleRandomTestStandardRunMulti()
}
