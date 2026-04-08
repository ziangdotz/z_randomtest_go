package service

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"

	"randomtest_core"
)

type RandomTestMethodStruct1 struct {
	Enable bool `form:"enable" json:"enable"`
	M      int  `form:"m" json:"m"` // 检测块长度
}

type RandomTestMethodStruct2 struct {
	Enable    *bool `form:"enable" json:"enable"`
	IsForward *bool `form:"isForward" json:"isForward"` // nil=全选, true=前向, false=后向
}

type RandomTestMethodStruct3 struct {
	Enable   *bool `form:"enable" json:"enable"`
	CheckOne *bool `form:"checkOne" json:"checkOne"` // nil=全选, true=比特1, false=比特0
}

type RandomTestMethodStruct4 struct {
	Enable bool `form:"enable" json:"enable"`
	R      int  `form:"r" json:"r"` // 检测参数 pkg/randomtest/rank.go
	C      int  `form:"c" json:"c"` // 检测参数 pkg/randomtest/rank.go
}

type RandomTestMethodStruct5 struct {
	Enable bool `form:"enable" json:"enable"`
	N      int  `form:"n" json:"n"` // 检测块数
	M      int  `form:"m" json:"m"` // 检测模板长度
}

type RandomTestMethodStruct6 struct {
	Enable bool `form:"enable" json:"enable"`
	M      int  `form:"m" json:"m"` // 检测参数
}

type RandomTestMethodStruct7 struct {
	Enable bool `form:"enable" json:"enable"`
	N      int  `form:"n" json:"n"` // 检测块数
}

type RandomTestMethodStruct8 struct {
	Enable bool `form:"enable" json:"enable"`
	K      int  `form:"k" json:"k"` // 检测参数
}

type RandomTestMethodStruct9 struct {
	Enable bool `form:"enable" json:"enable"`
	D      int  `form:"d" json:"d"` // 检测参数
}

type RandomTestHTTPRequestStruct struct {
	UploadFile                         string                   `form:"uploadFile" json:"uploadFile" binding:"required"` // 上传文件数据 (Base64 编码)
	FrequencyTest                      bool                     `form:"frequencyTest" json:"frequencyTest"`
	BlockFrequencyTest                 *RandomTestMethodStruct1 `form:"blockFrequencyTest" json:"blockFrequencyTest"`
	CumulativeSumsTest                 *RandomTestMethodStruct2 `form:"cumulativeSumsTest" json:"cumulativeSumsTest"`
	RunsTest                           bool                     `form:"runsTest" json:"runsTest"`
	LongestRunOfOnesTest               *RandomTestMethodStruct3 `form:"longestRunOfOnesTest" json:"longestRunOfOnesTest"`
	RankTest                           *RandomTestMethodStruct4 `form:"rankTest" json:"rankTest"`
	DiscreteFourierTransformTest       bool                     `form:"discreteFourierTransformTest" json:"discreteFourierTransformTest"`
	NonOverlappingTemplateMatchingTest *RandomTestMethodStruct5 `form:"nonOverlappingTemplateMatchingTest" json:"nonOverlappingTemplateMatchingTest"`
	OverlappingTemplateMatchingTest    *RandomTestMethodStruct5 `form:"overlappingTemplateMatchingTest" json:"overlappingTemplateMatchingTest"`
	UniversalTest                      bool                     `form:"universalTest" json:"universalTest"`
	ApproximateEntropyTest             *RandomTestMethodStruct6 `form:"approximateEntropyTest" json:"approximateEntropyTest"`
	RandomExcursionsTest               *RandomTestMethodStruct7 `form:"randomExcursionsTest" json:"randomExcursionsTest"`
	RandomExcursionsVariantTest        *RandomTestMethodStruct7 `form:"randomExcursionsVariantTest" json:"randomExcursionsVariantTest"`
	SerialTest                         *RandomTestMethodStruct6 `form:"serialTest" json:"serialTest"`
	LinearComplexityTest               *RandomTestMethodStruct6 `form:"linearComplexityTest" json:"linearComplexityTest"`
	PokerTest                          *RandomTestMethodStruct6 `form:"pokerTest" json:"pokerTest"`
	RunsDistributionTest               bool                     `form:"runsDistributionTest" json:"runsDistributionTest"`
	BinaryDerivativeTest               *RandomTestMethodStruct8 `form:"binaryDerivativeTest" json:"binaryDerivativeTest"`
	AutocorrelationTest                *RandomTestMethodStruct9 `form:"autocorrelationTest" json:"autocorrelationTest"`
}

type TestFuncResultDynamic struct {
	TestFuncName string
	P1Value      []float64
	P2Value      []float64
	Q1Value      []float64
	Q2Value      []float64
	IsPassed     bool
}

func isMetricValuesPassed(values []float64, threshold float64) bool {
	if len(values) == 0 {
		return true
	}

	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
		if value < threshold || value > 1 {
			return false
		}
	}

	return true
}

func calcResultIsPassed(item TestFuncResultDynamic, alpha, alphaT float64) bool {
	if !isMetricValuesPassed(item.P1Value, alpha) {
		return false
	}
	if !isMetricValuesPassed(item.P2Value, alpha) {
		return false
	}
	if !isMetricValuesPassed(item.Q1Value, alphaT) {
		return false
	}
	if !isMetricValuesPassed(item.Q2Value, alphaT) {
		return false
	}
	return true
}

func fillResultPassedStatus(results []TestFuncResultDynamic) {
	alpha, alphaT, _ := randomtest_core.GetAlphaValue()
	for i := range results {
		results[i].IsPassed = calcResultIsPassed(results[i], alpha, alphaT)
	}
}

func (randomTestHTTPRequestStruct *RandomTestHTTPRequestStruct) decodeUploadFileData(maxSize int) ([]byte, error) {
	uploadFileData, err := base64.StdEncoding.DecodeString(randomTestHTTPRequestStruct.UploadFile)
	if err != nil {
		return nil, fmt.Errorf("[Error] Base64 解码失败: %v", err)
	}
	if len(uploadFileData) >= maxSize {
		return nil, errors.New("[Error] 文件数据大小超过限制 (最大 1 GB)")
	}
	return uploadFileData, nil
}

func (randomTestHTTPRequestStruct *RandomTestHTTPRequestStruct) HandleRandomTestHTTPRequestService() ([]TestFuncResultDynamic, error) {
	maxSize := 1 * 1024 * 1024 * 1024

	uploadFileData, err := randomTestHTTPRequestStruct.decodeUploadFileData(maxSize)
	if err != nil {
		return nil, err
	}

	testFuncResultDynamic := []TestFuncResultDynamic{}

	if randomTestHTTPRequestStruct.FrequencyTest {
		pValue, qValue := randomtest_core.FrequencyTestBytes(uploadFileData)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "FrequencyTest",
			P1Value:      []float64{pValue},
			P2Value:      nil,
			Q1Value:      []float64{qValue},
			Q2Value:      nil,
		})
	}

	if randomTestHTTPRequestStruct.BlockFrequencyTest.Enable {
		pValue, qValue := randomtest_core.BlockFrequencyTestBytes(uploadFileData, randomTestHTTPRequestStruct.BlockFrequencyTest.M)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "BlockFrequencyTest",
			P1Value:      []float64{pValue},
			P2Value:      nil,
			Q1Value:      []float64{qValue},
			Q2Value:      nil,
		})
	}

	if randomTestHTTPRequestStruct.CumulativeSumsTest.Enable != nil && *randomTestHTTPRequestStruct.CumulativeSumsTest.Enable {
		isForward := randomTestHTTPRequestStruct.CumulativeSumsTest.IsForward
		if isForward == nil {
			pF, qF := randomtest_core.CumulativeSumsTestBytes(uploadFileData, true)
			pB, qB := randomtest_core.CumulativeSumsTestBytes(uploadFileData, false)
			testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
				TestFuncName: "CumulativeSumsTest (Forward)",
				P1Value:      []float64{pF},
				P2Value:      nil,
				Q1Value:      []float64{qF},
				Q2Value:      nil,
			})
			testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
				TestFuncName: "CumulativeSumsTest (Backward)",
				P1Value:      []float64{pB},
				P2Value:      nil,
				Q1Value:      []float64{qB},
				Q2Value:      nil,
			})
		} else {
			pValue, qValue := randomtest_core.CumulativeSumsTestBytes(uploadFileData, *isForward)
			label := "CumulativeSumsTest (Forward)"
			if !*isForward {
				label = "CumulativeSumsTest (Backward)"
			}
			testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
				TestFuncName: label,
				P1Value:      []float64{pValue},
				P2Value:      nil,
				Q1Value:      []float64{qValue},
				Q2Value:      nil,
			})
		}
	}

	if randomTestHTTPRequestStruct.RunsTest {
		pValue, qValue := randomtest_core.RunsTestBytes(uploadFileData)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "RunsTest",
			P1Value:      []float64{pValue},
			P2Value:      nil,
			Q1Value:      []float64{qValue},
			Q2Value:      nil,
		})
	}

	if randomTestHTTPRequestStruct.LongestRunOfOnesTest.Enable != nil && *randomTestHTTPRequestStruct.LongestRunOfOnesTest.Enable {
		checkOne := randomTestHTTPRequestStruct.LongestRunOfOnesTest.CheckOne
		if checkOne == nil {
			p1, q1 := randomtest_core.LongestRunOfOnesTestBytes(uploadFileData, true)
			p0, q0 := randomtest_core.LongestRunOfOnesTestBytes(uploadFileData, false)
			testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
				TestFuncName: "LongestRunOfOnesTest '1'",
				P1Value:      []float64{p1},
				P2Value:      nil,
				Q1Value:      []float64{q1},
				Q2Value:      nil,
			})
			testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
				TestFuncName: "LongestRunOfOnesTest '0'",
				P1Value:      []float64{p0},
				P2Value:      nil,
				Q1Value:      []float64{q0},
				Q2Value:      nil,
			})
		} else {
			pValue, qValue := randomtest_core.LongestRunOfOnesTestBytes(uploadFileData, *checkOne)
			label := "LongestRunOfOnesTest '1'"
			if !*checkOne {
				label = "LongestRunOfOnesTest '0'"
			}
			testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
				TestFuncName: label,
				P1Value:      []float64{pValue},
				P2Value:      nil,
				Q1Value:      []float64{qValue},
				Q2Value:      nil,
			})
		}
	}

	if randomTestHTTPRequestStruct.RankTest.Enable {
		pValue, qValue := randomtest_core.RankTestBytes(uploadFileData, randomTestHTTPRequestStruct.RankTest.R, randomTestHTTPRequestStruct.RankTest.C)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "RankTest",
			P1Value:      []float64{pValue},
			P2Value:      nil,
			Q1Value:      []float64{qValue},
			Q2Value:      nil,
		})
	}

	if randomTestHTTPRequestStruct.DiscreteFourierTransformTest {
		pValue, qValue := randomtest_core.DiscreteFourierTransformTestBytes(uploadFileData)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "DiscreteFourierTransformTest",
			P1Value:      []float64{pValue},
			P2Value:      nil,
			Q1Value:      []float64{qValue},
			Q2Value:      nil,
		})
	}

	if randomTestHTTPRequestStruct.NonOverlappingTemplateMatchingTest.Enable {
		pValue, _, _ := randomtest_core.NonOverlappingTemplateMatchingTestBytes(
			uploadFileData,
			randomTestHTTPRequestStruct.NonOverlappingTemplateMatchingTest.N,
			randomTestHTTPRequestStruct.NonOverlappingTemplateMatchingTest.M,
		)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "NonOverlappingTemplateMatchingTest",
			P1Value:      pValue,
			P2Value:      nil,
			Q1Value:      nil,
			Q2Value:      nil,
		})
	}

	if randomTestHTTPRequestStruct.OverlappingTemplateMatchingTest.Enable {
		pValue, _ := randomtest_core.OverlappingTemplateMatchingTestBytes(
			uploadFileData,
			randomTestHTTPRequestStruct.OverlappingTemplateMatchingTest.N,
			randomTestHTTPRequestStruct.OverlappingTemplateMatchingTest.M,
		)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "OverlappingTemplateMatchingTest",
			P1Value:      []float64{pValue},
			P2Value:      nil,
			Q1Value:      nil,
			Q2Value:      nil,
		})
	}

	if randomTestHTTPRequestStruct.UniversalTest {
		pValue, qValue := randomtest_core.UniversalTestBytes(uploadFileData)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "UniversalTest",
			P1Value:      []float64{pValue},
			P2Value:      nil,
			Q1Value:      []float64{qValue},
			Q2Value:      nil,
		})
	}

	if randomTestHTTPRequestStruct.ApproximateEntropyTest.Enable {
		pValue, qValue := randomtest_core.ApproximateEntropyTestBytes(uploadFileData, randomTestHTTPRequestStruct.ApproximateEntropyTest.M)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "ApproximateEntropyTest",
			P1Value:      []float64{pValue},
			P2Value:      nil,
			Q1Value:      []float64{qValue},
			Q2Value:      nil,
		})
	}

	if randomTestHTTPRequestStruct.RandomExcursionsTest.Enable {
		pValue, _, _ := randomtest_core.RandomExcursionsTestBytes(uploadFileData, randomTestHTTPRequestStruct.RandomExcursionsTest.N)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "RandomExcursionsTest",
			P1Value:      pValue,
			P2Value:      nil,
			Q1Value:      nil,
			Q2Value:      nil,
		})
	}

	if randomTestHTTPRequestStruct.RandomExcursionsVariantTest.Enable {
		pValue, _, _ := randomtest_core.RandomExcursionsVariantTestBytes(uploadFileData, randomTestHTTPRequestStruct.RandomExcursionsVariantTest.N)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "RandomExcursionsVariantTest",
			P1Value:      pValue,
			P2Value:      nil,
			Q1Value:      nil,
			Q2Value:      nil,
		})
	}

	if randomTestHTTPRequestStruct.SerialTest.Enable {
		pValue1, qValue1, pValue2, qValue2 := randomtest_core.SerialTestBytes(uploadFileData, randomTestHTTPRequestStruct.SerialTest.M)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "SerialTest",
			P1Value:      []float64{pValue1},
			P2Value:      []float64{pValue2},
			Q1Value:      []float64{qValue1},
			Q2Value:      []float64{qValue2},
		})
	}

	if randomTestHTTPRequestStruct.LinearComplexityTest.Enable {
		pValue, qValue := randomtest_core.LinearComplexityTestBytes(uploadFileData, randomTestHTTPRequestStruct.LinearComplexityTest.M)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "LinearComplexityTest",
			P1Value:      []float64{pValue},
			P2Value:      nil,
			Q1Value:      []float64{qValue},
			Q2Value:      nil,
		})
	}

	if randomTestHTTPRequestStruct.PokerTest.Enable {
		pValue, qValue := randomtest_core.PokerTestBytes(uploadFileData, randomTestHTTPRequestStruct.PokerTest.M)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "PokerTest",
			P1Value:      []float64{pValue},
			P2Value:      nil,
			Q1Value:      []float64{qValue},
			Q2Value:      nil,
		})
	}

	if randomTestHTTPRequestStruct.RunsDistributionTest {
		pValue, qValue := randomtest_core.RunsDistributionTestBytes(uploadFileData)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "RunsDistributionTest",
			P1Value:      []float64{pValue},
			P2Value:      nil,
			Q1Value:      []float64{qValue},
			Q2Value:      nil,
		})
	}

	if randomTestHTTPRequestStruct.BinaryDerivativeTest.Enable {
		pValue, qValue := randomtest_core.BinaryDerivativeTestBytes(uploadFileData, randomTestHTTPRequestStruct.BinaryDerivativeTest.K)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "BinaryDerivativeTest",
			P1Value:      []float64{pValue},
			P2Value:      nil,
			Q1Value:      []float64{qValue},
			Q2Value:      nil,
		})
	}

	if randomTestHTTPRequestStruct.AutocorrelationTest.Enable {
		pValue, qValue := randomtest_core.AutocorrelationTestBytes(uploadFileData, randomTestHTTPRequestStruct.AutocorrelationTest.D)
		testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
			TestFuncName: "AutocorrelationTest",
			P1Value:      []float64{pValue},
			P2Value:      nil,
			Q1Value:      []float64{qValue},
			Q2Value:      nil,
		})
	}

	fillResultPassedStatus(testFuncResultDynamic)
	return testFuncResultDynamic, nil
}

func (randomTestHTTPRequestStruct *RandomTestHTTPRequestStruct) HandleRandomTestNISTLevel2Service() ([]TestFuncResultDynamic, error) {
	maxSize := 1 * 1024 * 1024 * 1024

	uploadFileData, err := randomTestHTTPRequestStruct.decodeUploadFileData(maxSize)
	if err != nil {
		return nil, err
	}

	testFuncResultDynamic := []TestFuncResultDynamic{}

	pValue, qValue := randomtest_core.FrequencyTestBytes(uploadFileData)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "FrequencyTest",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.BlockFrequencyTestBytes(uploadFileData, 128)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "BlockFrequencyTest",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.CumulativeSumsTestBytes(uploadFileData, true)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "CumulativeSumsTest (Forward)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.CumulativeSumsTestBytes(uploadFileData, false)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "CumulativeSumsTest (Backward)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.RunsTestBytes(uploadFileData)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "RunsTest",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.LongestRunOfOnesTestBytes(uploadFileData, true)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "LongestRunofOnesTest '1'",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.LongestRunOfOnesTestBytes(uploadFileData, false)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "LongestRunofOnesTest '0'",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.RankTestBytes(uploadFileData, 32, 32)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "RankTest",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.DiscreteFourierTransformTestBytes(uploadFileData)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "DiscreteFourierTransformTest",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValues, _, _ := randomtest_core.NonOverlappingTemplateMatchingTestBytes(uploadFileData, 1000000, 9)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "NonOverlappingTemplateMatchingTest",
		P1Value:      pValues,
	})

	pValue, _ = randomtest_core.OverlappingTemplateMatchingTestBytes(uploadFileData, 1000000, 9)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "OverlappingTemplateMatchingTest",
		P1Value:      []float64{pValue},
	})

	pValue, qValue = randomtest_core.UniversalTestBytes(uploadFileData)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "UniversalTest",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.ApproximateEntropyTestBytes(uploadFileData, 10)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "ApproximateEntropyTest",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValues, _, _ = randomtest_core.RandomExcursionsTestBytes(uploadFileData, 1000000)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "RandomExcursionsTest",
		P1Value:      pValues,
	})

	pValues, _, _ = randomtest_core.RandomExcursionsVariantTestBytes(uploadFileData, 1000000)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "RandomExcursionsVariantTest",
		P1Value:      pValues,
	})

	pValue1, qValue1, pValue2, qValue2 := randomtest_core.SerialTestBytes(uploadFileData, 16)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "SerialTest",
		P1Value:      []float64{pValue1},
		P2Value:      []float64{pValue2},
		Q1Value:      []float64{qValue1},
		Q2Value:      []float64{qValue2},
	})

	pValue, qValue = randomtest_core.LinearComplexityTestBytes(uploadFileData, 500)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "LinearComplexityTest",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	fillResultPassedStatus(testFuncResultDynamic)
	return testFuncResultDynamic, nil
}

func (randomTestHTTPRequestStruct *RandomTestHTTPRequestStruct) HandleRandomTestGMLevel2Service() ([]TestFuncResultDynamic, error) {
	maxSize := 1 * 1024 * 1024 * 1024

	uploadFileData, err := randomTestHTTPRequestStruct.decodeUploadFileData(maxSize)
	if err != nil {
		return nil, err
	}

	testFuncResultDynamic := []TestFuncResultDynamic{}

	pValue, qValue := randomtest_core.FrequencyTestBytes(uploadFileData)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "FrequencyTest",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.BlockFrequencyTestBytes(uploadFileData, 10000)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "BlockFrequencyTest",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.PokerTestBytes(uploadFileData, 4)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "PokerTest (4)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.PokerTestBytes(uploadFileData, 8)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "PokerTest (8)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue1, qValue1, pValue2, qValue2 := randomtest_core.SerialTestBytes(uploadFileData, 3)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "SerialTest (3)",
		P1Value:      []float64{pValue1},
		P2Value:      []float64{pValue2},
		Q1Value:      []float64{qValue1},
		Q2Value:      []float64{qValue2},
	})

	pValue1, qValue1, pValue2, qValue2 = randomtest_core.SerialTestBytes(uploadFileData, 5)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "SerialTest (5)",
		P1Value:      []float64{pValue1},
		P2Value:      []float64{pValue2},
		Q1Value:      []float64{qValue1},
		Q2Value:      []float64{qValue2},
	})

	pValue, qValue = randomtest_core.RunsTestBytes(uploadFileData)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "RunsTest",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.RunsDistributionTestBytes(uploadFileData)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "RunsDistributionTest",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.LongestRunOfOnesTestBytes(uploadFileData, true)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "LongestRunofOnesTest '1'",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.LongestRunOfOnesTestBytes(uploadFileData, false)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "LongestRunofOnesTest '0'",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.BinaryDerivativeTestBytes(uploadFileData, 3)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "BinaryDerivativeTest (3)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.BinaryDerivativeTestBytes(uploadFileData, 7)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "BinaryDerivativeTest (7)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.AutocorrelationTestBytes(uploadFileData, 1)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "AutocorrelationTest (1)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.AutocorrelationTestBytes(uploadFileData, 2)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "AutocorrelationTest (2)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.AutocorrelationTestBytes(uploadFileData, 8)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "AutocorrelationTest (8)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.AutocorrelationTestBytes(uploadFileData, 16)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "AutocorrelationTest (16)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.RankTestBytes(uploadFileData, 32, 32)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "RankTest",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.CumulativeSumsTestBytes(uploadFileData, true)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "CumulativeSumsTest (Forward)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.CumulativeSumsTestBytes(uploadFileData, false)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "CumulativeSumsTest (Backward)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.ApproximateEntropyTestBytes(uploadFileData, 2)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "ApproximateEntropyTest (2)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.ApproximateEntropyTestBytes(uploadFileData, 5)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "ApproximateEntropyTest (5)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.LinearComplexityTestBytes(uploadFileData, 500)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "LinearComplexityTest (500)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.LinearComplexityTestBytes(uploadFileData, 1000)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "LinearComplexityTest (1000)",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.UniversalTestBytes(uploadFileData)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "UniversalTest",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	pValue, qValue = randomtest_core.DiscreteFourierTransformTestBytes(uploadFileData)
	testFuncResultDynamic = append(testFuncResultDynamic, TestFuncResultDynamic{
		TestFuncName: "DiscreteFourierTransformTest",
		P1Value:      []float64{pValue},
		Q1Value:      []float64{qValue},
	})

	fillResultPassedStatus(testFuncResultDynamic)
	return testFuncResultDynamic, nil
}
