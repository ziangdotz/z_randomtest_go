package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
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
	fmt.Printf("\n📊 NIST Level%d 测试结果统计\n", RunStandardRandomTestEg.TestLevel)
	fmt.Println(strings.Repeat("-", 75))
	fmt.Printf("%-45s | %20s\n", "TestFuncName", "PValue Passed-Count")
	fmt.Println(strings.Repeat("-", 75))
	pPassedNumMax, pPassedNumMin := jisuan(testNum)
	for _, testFuncResultStaticItem := range testFuncResultStatics {
		if testFuncResultStaticItem.TestFunc2 != nil {
			if testFuncResultStaticItem.TestFuncName == "RandomExcursionsTest" || testFuncResultStaticItem.TestFuncName == "RandomExcursionsVariantTest" {
				for i, pCount := range testFuncResultStaticItem.PCounts {
					pPassedNum1Max, pPassedNum1Min := jisuan(testFuncResultStaticItem.ValuesNum[i])
					if pCount < pPassedNum1Min || pCount > pPassedNum1Max {
						fmt.Printf("%-41s %3d | %9d/%-9d *\n", testFuncResultStaticItem.TestFuncName, i, pCount, testFuncResultStaticItem.ValuesNum[i])
						continue
					}
					fmt.Printf("%-41s %3d | %9d/%-9d  \n", testFuncResultStaticItem.TestFuncName, i, pCount, testFuncResultStaticItem.ValuesNum[i])
				}
				continue
			}
			for i, pCount := range testFuncResultStaticItem.PCounts {
				if pCount < pPassedNumMin || pCount > pPassedNumMax {
					fmt.Printf("%-41s %3d | %9d/%-9d *\n", testFuncResultStaticItem.TestFuncName, i, pCount, testNum)
					continue
				}
				fmt.Printf("%-41s %3d | %9d/%-9d  \n", testFuncResultStaticItem.TestFuncName, i, pCount, testNum)
			}
			continue
		}
		// if testFuncResultStaticItem.TestFunc1 != nil {
		if testFuncResultStaticItem.PCount < pPassedNumMin || testFuncResultStaticItem.PCount > pPassedNumMax {
			fmt.Printf("%-45s | %9d/%-9d *\n", testFuncResultStaticItem.TestFuncName, testFuncResultStaticItem.PCount, testNum)
			continue
		}
		fmt.Printf("%-45s | %9d/%-9d  \n", testFuncResultStaticItem.TestFuncName, testFuncResultStaticItem.PCount, testNum)
		// }
	}
	fmt.Println(strings.Repeat("-", 75))
}

func printTestLevelXGMFuncResults(testNum int, testFuncResultStatics []TestFuncResultStatic) {
	fmt.Printf("\n📊 GM Level%d 测试结果统计\n", RunStandardRandomTestEg.TestLevel)
	fmt.Println(strings.Repeat("-", 85))
	fmt.Printf("%-35s | %21s | %21s\n", "TestFuncName", "PValue Passed-Count", "QValue Passed-Count")
	fmt.Println(strings.Repeat("-", 85))
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
		fmt.Printf("%-35s | %9d/%-9d%2s | %9d/%-9d%2s\n", testFuncResultStaticItem.TestFuncName, testFuncResultStaticItem.PCount, testNum, pPassedStr, testFuncResultStaticItem.QCount, testNum, qPassedStr)
	}
	fmt.Println(strings.Repeat("-", 85))
}

func printTestNISTOrGMErrorFileList(results []TestFuncResultStatic) {
	fmt.Println("\n❌ NIST or GM 测试不通过的文件清单")
	fmt.Println(strings.Repeat("=", 85))
	for _, test := range results {
		hasFail := false
		fmt.Printf("测试项: %s\n", test.TestFuncName)
		for _, detail := range test.FileDetails {
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

func (runStandardRandomTestStruct *RunStandardRandomTestStruct) HandleRandomTestStandardRunSingle() error {
	fmt.Println("\n🚩 [Info] 开始执行单线程随机性测试")

	if runStandardRandomTestStruct.IsFile {
		file, err := os.Open(runStandardRandomTestStruct.FilePath)
		if err != nil {
			return fmt.Errorf("[Error] 文件打开失败: %v", err)
		}
		defer file.Close()

		switch runStandardRandomTestStruct.TestType {
		case "N": // NIST 测试
			if runStandardRandomTestStruct.TestLevel == 2 { // NIST 测试 (默认标准等级为 2)
				testLevel2NISTFuncResults := initTestLevel2NISTFuncResult()
				// 每次读取 runStandardRandomTestStruct.FilePath 文件中的 1000000 bit 数据到 data 对象中, 共读取 runStandardRandomTestStruct.TestNum 次, 每次都调用 runLevel2NIST() 函数
				const nistBlockSize = 125000 // 1000000 bits = 125000 bytes
				buf := make([]byte, nistBlockSize)
				for i := 0; i < runStandardRandomTestStruct.TestNum; i++ {
					n, err := io.ReadFull(file, buf)
					if err == io.EOF {
						return fmt.Errorf("[Error] 文件数据不足, 需要 %d 字节, 仅读取 %d 字节", nistBlockSize, n)
					}
					if err != nil {
						return fmt.Errorf("[Error] 文件读取错误: %v", err)
					}
					runLevel2NIST(buf, testLevel2NISTFuncResults)
				}
				printTestLevelXNISTFuncResults(runStandardRandomTestStruct.TestNum, testLevel2NISTFuncResults)
			} else {
				return fmt.Errorf("[Error] NIST 测试必须为默认标准等级 2")
			}
		case "G": // GM 测试
			var (
				gmBlockSize             int
				testFunc                func([]byte, []TestFuncResultStatic)
				testLevelXGMFuncResults []TestFuncResultStatic
			)

			switch runStandardRandomTestStruct.TestLevel {
			case 1: // GM 测试 (标准等级为 1)
				// 每次读取 runStandardRandomTestStruct.FilePath 文件中的 20000 bit 数据到 data 对象中, 共读取 runStandardRandomTestStruct.TestNum 次, 每次都调用 runLevel1GM() 函数
				gmBlockSize = 2500 // 20000 bits = 2500 bytes
				testFunc = runLevel1GM
				testLevelXGMFuncResults = initTestLevel1GMFuncResult()
			case 2: // GM 测试 (标准等级为 2)
				// 每次读取 runStandardRandomTestStruct.FilePath 文件中的 1000000 bit 数据到 data 对象中, 共读取 runStandardRandomTestStruct.TestNum 次, 每次都调用 runLevel2GM() 函数
				gmBlockSize = 125000 // 1000000 bits = 125000 bytes
				testFunc = runLevel2GM
				testLevelXGMFuncResults = initTestLevel2GMFuncResult()
			case 3: // GM 测试 (标准等级为 3)
				// 每次读取 runStandardRandomTestStruct.FilePath 文件中的 100000000 bit 数据到 data 对象中, 共读取 runStandardRandomTestStruct.TestNum 次, 每次都调用 runLevel3GM() 函数
				gmBlockSize = 12500000 // 100000000 bits = 12500000 bytes
				testFunc = runLevel3GM
				testLevelXGMFuncResults = initTestLevel3GMFuncResult()
			default:
				return fmt.Errorf("[Error] GM 测试必须为标准等级 1 或 2 或 3")
			}

			buf := make([]byte, gmBlockSize)
			for i := 0; i < runStandardRandomTestStruct.TestNum; i++ {
				n, err := io.ReadFull(file, buf)
				if err == io.EOF {
					return fmt.Errorf("[Error] 文件数据不足, 需要 %d 字节, 仅读取 %d 字节", gmBlockSize, n)
				}
				if err != nil {
					return fmt.Errorf("[Error] 文件读取错误: %v", err)
				}
				testFunc(buf, testLevelXGMFuncResults)
			}
			printTestLevelXGMFuncResults(runStandardRandomTestStruct.TestNum, testLevelXGMFuncResults)
		default:
			return fmt.Errorf("请选择规定的检测标准类型 (NIST / GM)")
		}
	} else {
		files, err := ioutil.ReadDir(runStandardRandomTestStruct.RootDirPath)
		if err != nil {
			return fmt.Errorf("[Error] 目录读取失败: %v", err)
		}
		filesNum := 0

		switch runStandardRandomTestStruct.TestType {
		case "N": // NIST 测试
			if runStandardRandomTestStruct.TestLevel == 2 { // NIST 测试 (默认标准等级为 2)
				testLevel2NISTFuncResults := initTestLevel2NISTFuncResult()
				// 每次读取 runStandardRandomTestStruct.RootDirPath 目录中一个文件, 文件大小需要是 1000000 bit, 读取的数据到 data 对象中, 共读取目录下的文件数量次, 每次都调用 runLevel2NIST() 函数
				const nistFileSize = 125000 // 1000000 bits = 125000 bytes
				for _, fileInfo := range files {
					if fileInfo.IsDir() {
						continue
					}
					if fileInfo.Size() != nistFileSize {
						fmt.Printf("[WARNNING] 警告: 文件 %s 大小不符合要求 (%d != %d), 跳过\n", fileInfo.Name(), fileInfo.Size(), nistFileSize)
						continue
					}
					filePath := filepath.Join(runStandardRandomTestStruct.RootDirPath, fileInfo.Name())
					data, err := os.ReadFile(filePath)
					if err != nil {
						return fmt.Errorf("[Error] 读取文件 %s 失败: %v", filePath, err)
					}
					filesNum++
					runLevel2NIST(data, testLevel2NISTFuncResults)
				}
				printTestLevelXNISTFuncResults(filesNum, testLevel2NISTFuncResults)
			} else {
				return fmt.Errorf("[Error] NIST 测试必须为默认标准等级 2")
			}
		case "G": // GM 测试
			var (
				gmFileSize              int
				testLevelXGMFuncResults []TestFuncResultStatic
				testFunc                func([]byte, []TestFuncResultStatic)
			)

			switch runStandardRandomTestStruct.TestLevel {
			case 1: // GM 测试 (标准等级为 1)
				// 每次读取 runStandardRandomTestStruct.RootDirPath 目录中一个文件, 文件大小需要是 20000 bit，读取的数据到 data 对象中，共读取目录下的文件数量次, 每次都调用 runLevel1GM() 函数
				gmFileSize = 2500 // 20000 bits = 2500 bytes
				testFunc = runLevel1GM
				testLevelXGMFuncResults = initTestLevel1GMFuncResult()
			case 2: // GM 测试 (标准等级为 2)
				// 每次读取 runStandardRandomTestStruct.RootDirPath 目录中一个文件, 文件大小需要是 1000000 bit，读取的数据到 data 对象中，共读取目录下的文件数量次, 每次都调用 runLevel2GM() 函数
				gmFileSize = 125000 // 1000000 bits = 125000 bytes
				testFunc = runLevel2GM
				testLevelXGMFuncResults = initTestLevel2GMFuncResult()
			case 3: // GM 测试 (标准等级为 3)
				// 每次读取 runStandardRandomTestStruct.RootDirPath 目录中一个文件, 文件大小需要是 100000000 bit，读取的数据到 data 对象中，共读取目录下的文件数量次, 每次都调用 runLevel3GM() 函数
				gmFileSize = 12500000 // 100000000 bits = 12500000 bytes
				testFunc = runLevel3GM
				testLevelXGMFuncResults = initTestLevel3GMFuncResult()
			default:
				return fmt.Errorf("[Error] GM 测试必须为标准等级 1 或 2 或 3")
			}

			for _, fileInfo := range files {
				if fileInfo.IsDir() {
					continue
				}
				if fileInfo.Size() != int64(gmFileSize) {
					fmt.Printf("[WARNNING] 警告: 文件 %s 大小不符合要求 (%d != %d), 跳过\n", fileInfo.Name(), fileInfo.Size(), gmFileSize)
					continue
				}
				filePath := filepath.Join(runStandardRandomTestStruct.RootDirPath, fileInfo.Name())
				data, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("[Error] 读取文件 %s 失败: %v", filePath, err)
				}
				filesNum++
				testFunc(data, testLevelXGMFuncResults)
			}
			printTestLevelXGMFuncResults(filesNum, testLevelXGMFuncResults)
		default:
			return fmt.Errorf("[Error] 请选择规定的检测标准类型 (NIST'N' / GM'G')")
		}
	}
	return nil
}

func (runStandardRandomTestStruct *RunStandardRandomTestStruct) HandleRandomTestStandardRunMulti() error {
	fmt.Println("\n🚩 [Info] 开始执行并发随机性测试")

	if runStandardRandomTestStruct.IsFile {
		file, err := os.Open(runStandardRandomTestStruct.FilePath)
		if err != nil {
			return fmt.Errorf("[Error] 文件打开失败: %v", err)
		}
		defer file.Close()

		switch runStandardRandomTestStruct.TestType {
		case "N": // NIST 测试
			if runStandardRandomTestStruct.TestLevel == 2 { // NIST 测试 (默认标准等级为 2)
				// 每次读取 runStandardRandomTestStruct.FilePath 文件中的 1000000 bit 数据到 data 对象中, 共读取 runStandardRandomTestStruct.TestNum 次, 每次都调用 runLevel2NIST() 函数
				/** 并发处理核心逻辑 **/
				total := runStandardRandomTestStruct.TestNum
				if total <= 0 {
					return fmt.Errorf("[Error] TestNum 必须大于 0")
				}
				printProgress("NIST_FILE", 0, total, "")
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
							runLevel2NIST(data, workerResults)
							done := int(atomic.AddInt64(&processed, 1))
							if done%progressStep == 0 || done == total {
								printProgress("NIST_FILE", done, total, "")
							}
						}
						resultChan <- workerResults
					}()
				}
				/* 文件读取协程 */
				go func() {
					defer close(taskChan)
					for i := 0; i < total; i++ {
						buf := make([]byte, nistBlockSize)
						n, err := io.ReadFull(file, buf)
						if err == io.EOF {
							readMutex.Lock()
							readErr = fmt.Errorf("[Error] 文件数据不足, 需要 %d 字节, 仅读取 %d 字节 (已处理 %d/%d 块)", nistBlockSize, n, i, total)
							readMutex.Unlock()
							return
						}
						if err != nil {
							readMutex.Lock()
							readErr = fmt.Errorf("[Error] 文件读取失败 (块 %d/%d): %v", i+1, total, err)
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
					return fmt.Errorf("[Error] 处理失败: %v (已完成 %d/%d 个工作线程)", localReadErr, resultsProcessed, WorkerCount)
				}
				printProgress("NIST_FILE", total, total, "")
				fmt.Printf("\n✅ 成功处理 %d 个工作线程的结果\n", resultsProcessed)
				printTestLevelXNISTFuncResults(total, finalResults)
			} else {
				return fmt.Errorf("[Error] NIST 测试必须为默认标准等级 2")
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
				return fmt.Errorf("[Error] GM 测试必须为标准等级 1 或 2 或 3")
			}

			total := runStandardRandomTestStruct.TestNum
			if total <= 0 {
				return fmt.Errorf("[Error] TestNum 必须大于 0")
			}
			printProgress("GM_FILE", 0, total, "")

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
				go func(workerID int) {
					defer wg.Done()
					workerResults := testInitFunc()
					for data := range taskChan {
						// 处理数据
						testFunc(data, workerResults)
						done := int(atomic.AddInt64(&processed, 1))
						if done%progressStep == 0 || done == total {
							printProgress("GM_FILE", done, total, "")
						}
						// 归还缓冲区到池中
						bufferPool.Put(data[:gmBlockSize])
					}
					resultChan <- workerResults
				}(i)
			}
			/* 文件读取协程 */
			go func() {
				defer close(taskChan)
				/* 使用 bufio.Reader 提高读取性能 */
				reader := io.Reader(file)
				if gmBlockSize >= 1024*1024 { // 对于大块, 增加缓冲区
					reader = io.NewSectionReader(file, 0, 1<<63-1)
				}
				for i := 0; i < total; i++ {
					buf := bufferPool.Get().([]byte) // 从池中获取缓冲区
					/* 读取数据块 */
					n, err := io.ReadFull(reader, buf[:gmBlockSize])
					if err == io.EOF {
						once.Do(func() {
							select {
							case errChan <- fmt.Errorf("[Error] 文件数据不足, 需要 %d 字节, 仅读取 %d 字节 (已处理 %d/%d 块)", gmBlockSize, n, i, total):
							default:
							}
						})
						bufferPool.Put(buf)
						return
					}
					if err != nil {
						once.Do(func() {
							select {
							case errChan <- fmt.Errorf("[Error] 文件读取失败 (块 %d/%d): %v", i+1, total, err):
							default:
							}
						})
						bufferPool.Put(buf)
						return
					}
					/* 创建数据副本发送到通道 (避免数据竞争) */
					dataCopy := make([]byte, gmBlockSize)
					copy(dataCopy, buf[:gmBlockSize])
					bufferPool.Put(buf)
					select {
					case taskChan <- dataCopy:
					case <-errChan: // 如果发生错误, 停止读取
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
					return fmt.Errorf("[Error] 处理失败: %v (已完成 %d/%d 个工作线程)", err, resultsProcessed, WorkerCount)
				}
			default:
			}
			printProgress("GM_FILE", total, total, "")
			fmt.Printf("\n✅ 成功处理 %d 个工作线程的结果\n", resultsProcessed)
			printTestLevelXGMFuncResults(total, finalResults)
		default:
			return fmt.Errorf("请选择规定的检测标准类型 (NIST'N' / GM'G')")
		}
	} else {
		files, err := ioutil.ReadDir(runStandardRandomTestStruct.RootDirPath)
		if err != nil {
			return fmt.Errorf("[Error] 目录读取失败: %v", err)
		}
		filesNum := 0

		switch runStandardRandomTestStruct.TestType {
		case "N": // NIST 测试
			if runStandardRandomTestStruct.TestLevel == 2 { // NIST 测试 (默认标准等级为 2)
				// 每次读取 runStandardRandomTestStruct.RootDirPath 目录中一个文件, 文件大小需要是 1000000 bit, 读取的数据到 data 对象中, 共读取目录下的文件数量次, 每次都调用 runLevel2NIST() 函数
				/** 并发处理核心逻辑 **/
				const nistFileSize = 125000 // 1000000 bits = 125000 bytes
				validFiles := make([]os.FileInfo, 0, len(files))
				for _, fileInfo := range files {
					if fileInfo.IsDir() {
						continue
					}
					if fileInfo.Size() != nistFileSize {
						fmt.Printf("[WARNING] 文件 %s 大小不符合要求 (%d != %d), 跳过\n", fileInfo.Name(), fileInfo.Size(), nistFileSize)
						continue
					}
					validFiles = append(validFiles, fileInfo)
				}
				total := len(validFiles)
				if total == 0 {
					return fmt.Errorf("[Error] 没有符合 NIST Level2 要求的输入文件")
				}
				printProgress("NIST_DIR", 0, total, "")

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
								printProgress("NIST_DIR", done, total, task.FileName)
							}
						}
						resultChan <- workerResults
					}()
				}
				/* 文件读取协程 */
				go func() {
					defer close(taskChan)
					for _, fileInfo := range validFiles {
						filePath := filepath.Join(runStandardRandomTestStruct.RootDirPath, fileInfo.Name())
						data, err := os.ReadFile(filePath)
						if err != nil {
							readMutex.Lock()
							readErr = fmt.Errorf("[Error] 文件 %s 读取失败: %v", filePath, err)
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
					return fmt.Errorf("[Error] 处理失败: %v", localReadErr)
				}
				printProgress("NIST_DIR", total, total, "")
				printTestLevelXNISTFuncResults(filesNum, finalResults)
				printTestNISTOrGMErrorFileList(finalResults)
			} else {
				return fmt.Errorf("[Error] NIST 测试必须为默认标准等级 2")
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
				return fmt.Errorf("[Error] GM 测试必须为标准等级 1 或 2 或 3")
			}

			/** 并发处理核心逻辑 **/
			validFiles := make([]os.FileInfo, 0, len(files))
			for _, fileInfo := range files {
				if fileInfo.IsDir() {
					continue
				}
				if fileInfo.Size() != gmFileSize {
					fmt.Printf("[WARNING] 文件 %s 大小不符合要求 (%d != %d), 跳过\n", fileInfo.Name(), fileInfo.Size(), gmFileSize)
					continue
				}
				validFiles = append(validFiles, fileInfo)
			}
			total := len(validFiles)
			if total == 0 {
				return fmt.Errorf("[Error] 没有符合 GM Level%d 要求的输入文件", runStandardRandomTestStruct.TestLevel)
			}
			printProgress("GM_DIR", 0, total, "")

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
							printProgress("GM_DIR", done, total, task.FileName)
						}
					}
					resultChan <- workerResults
				}()
			}
			/* 文件读取协程 */
			go func() {
				defer close(taskChan)
				for _, fileInfo := range validFiles {
					filePath := filepath.Join(runStandardRandomTestStruct.RootDirPath, fileInfo.Name())
					data, err := os.ReadFile(filePath)
					if err != nil {
						readMutex.Lock()
						readErr = fmt.Errorf("[Error] 文件 %s 读取失败: %v", filePath, err)
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
				return fmt.Errorf("[Error] 处理失败: %v", localReadErr)
			}
			printProgress("GM_DIR", total, total, "")
			printTestLevelXGMFuncResults(filesNum, finalResults)
			printTestNISTOrGMErrorFileList(finalResults)
		default:
			return fmt.Errorf("请选择规定的检测标准类型 (NIST'N' / GM'G')")
		}
	}
	return nil
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

	WorkerCount = runtime.NumCPU() * 2 // 动态工作池大小
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
