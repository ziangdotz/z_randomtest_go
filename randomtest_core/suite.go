package randomtest_core

import "fmt"

/*
检测套件模型 (阶段②):

将 CLI / 桌面端 / Web 端三处重复维护的检测项定义与统计逻辑下沉到 core:
- Suite: 一套检测项的有序集合 (标准 + 等级 + 检测项列表)
- Outcome: 单次检测的归一化结果
统计语义与既有 runLevelX 实现一一对应 (含 SerialTest 的双值 RequireAll、
RandomExcursions 系列的 p>0 适用语义), 输出格式由调用方渲染, 保持不变。
*/

// Outcome 单次检测的归一化结果。
type Outcome struct {
	// P/Q 值集合: 多值检测 (如非重叠模板匹配 148 个子项) 含多个值, 单值检测仅 1 个。
	P []float64
	Q []float64
	// Applied 子项是否实际适用, 与 P 一一对应; nil 视为全部适用。
	// 显式承载 RandomExcursions 系列 "p>0 才算适用" 的既有语义
	// (回零次数 J 不足时 C 侧返回全 0 p 值, 该子项不计入 ValuesNum/通过数)。
	Applied []bool
	// RequireAll 聚合判定: true 表示所有 P (及所有 Q) 全部通过才累计一次通过
	// (SerialTest 的 P1/P2 双值语义); false 表示逐子项统计 (多值检测)。
	RequireAll bool
	// Err 单次检测失败 (参数非法等) 不中断整组, 由 Runner 累计上报。
	Err error
}

// TestItem 单个检测项。
type TestItem struct {
	Name string
	Run  func(data []byte) Outcome
}

// Suite 一套检测项。Items 顺序即统计展示顺序, 三个入口 (CLI/桌面端/Web) 保持一致。
type Suite struct {
	Standard string // "NIST" / "GM"
	Level    int
	Items    []TestItem
	// BlockSizeBytes 文件模式分块读取的字节块大小:
	// NIST 为 125000 (1Mbit); GM Level1/2/3 分别为 2500/125000/12500000。
	BlockSizeBytes int
}

// singleItem 单值检测项 (P/Q 各 1 个值)。
func singleItem(name string, run func(data []byte) (float64, float64)) TestItem {
	return TestItem{Name: name, Run: func(d []byte) Outcome {
		p, q := run(d)
		return Outcome{P: []float64{p}, Q: []float64{q}}
	}}
}

// serialItem Serial 检测项: P1/P2、Q1/Q2 双值, 全部通过才计一次通过。
func serialItem(name string, m int) TestItem {
	return TestItem{Name: name, Run: func(d []byte) Outcome {
		p1, q1, p2, q2 := SerialTestBytes(d, m)
		return Outcome{P: []float64{p1, p2}, Q: []float64{q1, q2}, RequireAll: true}
	}}
}

// appliedFromP 由 p 值派生适用标记 (p>0), 对应 RandomExcursions 系列既有语义。
func appliedFromP(ps []float64) []bool {
	applied := make([]bool, len(ps))
	for i, p := range ps {
		applied[i] = p > 0
	}
	return applied
}

// SuiteNISTLevel2 NIST SP800-22 标准等级 2 检测套件 (1Mbit 数据块)。
// 检测项与参数同 CLI/桌面端/Web 的 initTestLevel2NISTFuncResult 定义源。
func SuiteNISTLevel2() *Suite {
	return &Suite{
		Standard:       "NIST",
		Level:          2,
		BlockSizeBytes: 125000,
		Items: []TestItem{
			singleItem("FrequencyTest", FrequencyTestBytes),
			singleItem("BlockFrequencyTest", func(d []byte) (float64, float64) {
				return BlockFrequencyTestBytes(d, 128)
			}),
			singleItem("CumulativeSumsTest (Forward)", func(d []byte) (float64, float64) {
				return CumulativeSumsTestBytes(d, true)
			}),
			singleItem("CumulativeSumsTest (Backward)", func(d []byte) (float64, float64) {
				return CumulativeSumsTestBytes(d, false)
			}),
			singleItem("RunsTest", RunsTestBytes),
			singleItem("LongestRunofOnesTest '1'", func(d []byte) (float64, float64) {
				return LongestRunOfOnesTestBytes(d, true)
			}),
			singleItem("LongestRunofOnesTest '0'", func(d []byte) (float64, float64) {
				return LongestRunOfOnesTestBytes(d, false)
			}),
			singleItem("RankTest", func(d []byte) (float64, float64) {
				return RankTestBytes(d, 32, 32)
			}),
			singleItem("DiscreteFourierTransformTest", DiscreteFourierTransformTestBytes),
			{Name: "NonOverlappingTemplateMatchingTest", Run: func(d []byte) Outcome {
				ps, _, _, err := NonOverlappingTemplateMatchingTestBytes(d, len(d)*8, 9)
				if err != nil {
					return Outcome{Err: err}
				}
				return Outcome{P: ps, Applied: appliedFromP(ps)}
			}},
			{Name: "OverlappingTemplateMatchingTest", Run: func(d []byte) Outcome {
				p, _, err := OverlappingTemplateMatchingTestBytes(d, len(d)*8, 9)
				if err != nil {
					return Outcome{Err: err}
				}
				// C 侧无 Q 值概念, Q 保持 nil (统计与 JSON 输出中该字段为空)
				return Outcome{P: []float64{p}}
			}},
			singleItem("UniversalTest", UniversalTestBytes),
			singleItem("ApproximateEntropyTest", func(d []byte) (float64, float64) {
				return ApproximateEntropyTestBytes(d, 10)
			}),
			{Name: "RandomExcursionsTest", Run: func(d []byte) Outcome {
				ps, _, _, err := RandomExcursionsTestBytes(d, len(d)*8)
				if err != nil {
					return Outcome{Err: err}
				}
				return Outcome{P: ps, Applied: appliedFromP(ps)}
			}},
			{Name: "RandomExcursionsVariantTest", Run: func(d []byte) Outcome {
				ps, _, _, err := RandomExcursionsVariantTestBytes(d, len(d)*8)
				if err != nil {
					return Outcome{Err: err}
				}
				return Outcome{P: ps, Applied: appliedFromP(ps)}
			}},
			serialItem("SerialTest", 16),
			singleItem("LinearComplexityTest", func(d []byte) (float64, float64) {
				return LinearComplexityTestBytes(d, 500)
			}),
		},
	}
}

// SuiteGM GM/T 0005-2021 指定等级检测套件 (level 1/2/3)。
// 检测项与参数同 CLI/桌面端/Web 的 initTestLevelXGMFuncResult 定义源。
func SuiteGM(level int) (*Suite, error) {
	s := &Suite{Standard: "GM", Level: level}
	switch level {
	case 1: // 20000 bits 块
		s.BlockSizeBytes = 2500
		s.Items = []TestItem{
			singleItem("FrequencyTest", FrequencyTestBytes),
			singleItem("BlockFrequencyTest", func(d []byte) (float64, float64) {
				return BlockFrequencyTestBytes(d, 1000)
			}),
			singleItem("PokerTest (4)", func(d []byte) (float64, float64) {
				return PokerTestBytes(d, 4)
			}),
			singleItem("PokerTest (8)", func(d []byte) (float64, float64) {
				return PokerTestBytes(d, 8)
			}),
			serialItem("SerialTest (3)", 3),
			serialItem("SerialTest (5)", 5),
			singleItem("RunsTest", RunsTestBytes),
			singleItem("RunsDistributionTest", RunsDistributionTestBytes),
			singleItem("LongestRunofOnesTest '1'", func(d []byte) (float64, float64) {
				return LongestRunOfOnesTestBytes(d, true)
			}),
			singleItem("LongestRunofOnesTest '0'", func(d []byte) (float64, float64) {
				return LongestRunOfOnesTestBytes(d, false)
			}),
			singleItem("BinaryDerivativeTest (3)", func(d []byte) (float64, float64) {
				return BinaryDerivativeTestBytes(d, 3)
			}),
			singleItem("BinaryDerivativeTest (7)", func(d []byte) (float64, float64) {
				return BinaryDerivativeTestBytes(d, 7)
			}),
			singleItem("AutocorrelationTest (2)", func(d []byte) (float64, float64) {
				return AutocorrelationTestBytes(d, 2)
			}),
			singleItem("AutocorrelationTest (8)", func(d []byte) (float64, float64) {
				return AutocorrelationTestBytes(d, 8)
			}),
			singleItem("AutocorrelationTest (16)", func(d []byte) (float64, float64) {
				return AutocorrelationTestBytes(d, 16)
			}),
			singleItem("CumulativeSumsTest (Forward)", func(d []byte) (float64, float64) {
				return CumulativeSumsTestBytes(d, true)
			}),
			singleItem("CumulativeSumsTest (Backward)", func(d []byte) (float64, float64) {
				return CumulativeSumsTestBytes(d, false)
			}),
			singleItem("ApproximateEntropyTest (2)", func(d []byte) (float64, float64) {
				return ApproximateEntropyTestBytes(d, 2)
			}),
			singleItem("ApproximateEntropyTest (5)", func(d []byte) (float64, float64) {
				return ApproximateEntropyTestBytes(d, 5)
			}),
			singleItem("DiscreteFourierTransformTest", DiscreteFourierTransformTestBytes),
		}
	case 2: // 1000000 bits 块
		s.BlockSizeBytes = 125000
		s.Items = []TestItem{
			singleItem("FrequencyTest", FrequencyTestBytes),
			singleItem("BlockFrequencyTest", func(d []byte) (float64, float64) {
				return BlockFrequencyTestBytes(d, 10000)
			}),
			singleItem("PokerTest (4)", func(d []byte) (float64, float64) {
				return PokerTestBytes(d, 4)
			}),
			singleItem("PokerTest (8)", func(d []byte) (float64, float64) {
				return PokerTestBytes(d, 8)
			}),
			serialItem("SerialTest (3)", 3),
			serialItem("SerialTest (5)", 5),
			singleItem("RunsTest", RunsTestBytes),
			singleItem("RunsDistributionTest", RunsDistributionTestBytes),
			singleItem("LongestRunofOnesTest '1'", func(d []byte) (float64, float64) {
				return LongestRunOfOnesTestBytes(d, true)
			}),
			singleItem("LongestRunofOnesTest '0'", func(d []byte) (float64, float64) {
				return LongestRunOfOnesTestBytes(d, false)
			}),
			singleItem("BinaryDerivativeTest (3)", func(d []byte) (float64, float64) {
				return BinaryDerivativeTestBytes(d, 3)
			}),
			singleItem("BinaryDerivativeTest (7)", func(d []byte) (float64, float64) {
				return BinaryDerivativeTestBytes(d, 7)
			}),
			singleItem("AutocorrelationTest (1)", func(d []byte) (float64, float64) {
				return AutocorrelationTestBytes(d, 1)
			}),
			singleItem("AutocorrelationTest (2)", func(d []byte) (float64, float64) {
				return AutocorrelationTestBytes(d, 2)
			}),
			singleItem("AutocorrelationTest (8)", func(d []byte) (float64, float64) {
				return AutocorrelationTestBytes(d, 8)
			}),
			singleItem("AutocorrelationTest (16)", func(d []byte) (float64, float64) {
				return AutocorrelationTestBytes(d, 16)
			}),
			singleItem("RankTest", func(d []byte) (float64, float64) {
				return RankTestBytes(d, 32, 32)
			}),
			singleItem("CumulativeSumsTest (Forward)", func(d []byte) (float64, float64) {
				return CumulativeSumsTestBytes(d, true)
			}),
			singleItem("CumulativeSumsTest (Backward)", func(d []byte) (float64, float64) {
				return CumulativeSumsTestBytes(d, false)
			}),
			singleItem("ApproximateEntropyTest (2)", func(d []byte) (float64, float64) {
				return ApproximateEntropyTestBytes(d, 2)
			}),
			singleItem("ApproximateEntropyTest (5)", func(d []byte) (float64, float64) {
				return ApproximateEntropyTestBytes(d, 5)
			}),
			singleItem("LinearComplexityTest (500)", func(d []byte) (float64, float64) {
				return LinearComplexityTestBytes(d, 500)
			}),
			singleItem("LinearComplexityTest (1000)", func(d []byte) (float64, float64) {
				return LinearComplexityTestBytes(d, 1000)
			}),
			singleItem("UniversalTest", UniversalTestBytes),
			singleItem("DiscreteFourierTransformTest", DiscreteFourierTransformTestBytes),
		}
	case 3: // 100000000 bits 块
		s.BlockSizeBytes = 12500000
		s.Items = []TestItem{
			singleItem("FrequencyTest", FrequencyTestBytes),
			singleItem("BlockFrequencyTest", func(d []byte) (float64, float64) {
				return BlockFrequencyTestBytes(d, 100000)
			}),
			singleItem("PokerTest (4)", func(d []byte) (float64, float64) {
				return PokerTestBytes(d, 4)
			}),
			singleItem("PokerTest (8)", func(d []byte) (float64, float64) {
				return PokerTestBytes(d, 8)
			}),
			serialItem("SerialTest (3)", 3),
			serialItem("SerialTest (5)", 5),
			serialItem("SerialTest (7)", 7),
			singleItem("RunsTest", RunsTestBytes),
			singleItem("RunsDistributionTest", RunsDistributionTestBytes),
			singleItem("LongestRunofOnesTest '1'", func(d []byte) (float64, float64) {
				return LongestRunOfOnesTestBytes(d, true)
			}),
			singleItem("LongestRunofOnesTest '0'", func(d []byte) (float64, float64) {
				return LongestRunOfOnesTestBytes(d, false)
			}),
			singleItem("BinaryDerivativeTest (3)", func(d []byte) (float64, float64) {
				return BinaryDerivativeTestBytes(d, 3)
			}),
			singleItem("BinaryDerivativeTest (7)", func(d []byte) (float64, float64) {
				return BinaryDerivativeTestBytes(d, 7)
			}),
			singleItem("BinaryDerivativeTest (15)", func(d []byte) (float64, float64) {
				return BinaryDerivativeTestBytes(d, 15)
			}),
			singleItem("AutocorrelationTest (1)", func(d []byte) (float64, float64) {
				return AutocorrelationTestBytes(d, 1)
			}),
			singleItem("AutocorrelationTest (2)", func(d []byte) (float64, float64) {
				return AutocorrelationTestBytes(d, 2)
			}),
			singleItem("AutocorrelationTest (8)", func(d []byte) (float64, float64) {
				return AutocorrelationTestBytes(d, 8)
			}),
			singleItem("AutocorrelationTest (16)", func(d []byte) (float64, float64) {
				return AutocorrelationTestBytes(d, 16)
			}),
			singleItem("AutocorrelationTest (32)", func(d []byte) (float64, float64) {
				return AutocorrelationTestBytes(d, 32)
			}),
			singleItem("RankTest", func(d []byte) (float64, float64) {
				return RankTestBytes(d, 32, 32)
			}),
			singleItem("CumulativeSumsTest (Forward)", func(d []byte) (float64, float64) {
				return CumulativeSumsTestBytes(d, true)
			}),
			singleItem("CumulativeSumsTest (Backward)", func(d []byte) (float64, float64) {
				return CumulativeSumsTestBytes(d, false)
			}),
			singleItem("ApproximateEntropyTest (5)", func(d []byte) (float64, float64) {
				return ApproximateEntropyTestBytes(d, 5)
			}),
			singleItem("ApproximateEntropyTest (7)", func(d []byte) (float64, float64) {
				return ApproximateEntropyTestBytes(d, 7)
			}),
			singleItem("LinearComplexityTest (5000)", func(d []byte) (float64, float64) {
				return LinearComplexityTestBytes(d, 5000)
			}),
			singleItem("UniversalTest", UniversalTestBytes),
			singleItem("DiscreteFourierTransformTest", DiscreteFourierTransformTestBytes),
		}
	default:
		return nil, fmt.Errorf("GM 测试必须为标准等级 1 或 2 或 3")
	}
	return s, nil
}
