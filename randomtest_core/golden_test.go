package randomtest_core

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
)

/*
golden known-answer 测试安全网 (阶段①):

1. 构造性非随机序列必须被检出: 全 0、全 1、0101 交替、短周期重复块。
   "检出" = 显式 error / NaN+Inf 值 / 任一 P < Alpha (或 Q < AlphaT)。
2. ChaCha8 固定种子的高质量随机流必须整体通过:
   每个检测项在 30 组 × 1Mbit 上通过率 ≥ 26/30 (容忍统计波动)。

覆盖全部导出检测函数 (含 CGO 的 4 个 NIST 函数)。
参数与 CLI/桌面端/Web 端常用 Level2 配置一致。
此层同时是阶段③热路径优化的数值等价性护航基线之一。
*/

const goldenBytesPerGroup = 125000 // 每组 1 Mbit

type goldenOutcome struct {
	pValues []float64
	qValues []float64
	err     error
}

type goldenTestItem struct {
	name string
	run  func(data []byte) goldenOutcome
}

func goldenFrom2(p, q float64) goldenOutcome {
	return goldenOutcome{pValues: []float64{p}, qValues: []float64{q}}
}

func goldenFromPOnly(p float64) goldenOutcome {
	return goldenOutcome{pValues: []float64{p}}
}

func goldenFrom4(p1, q1, p2, q2 float64) goldenOutcome {
	return goldenOutcome{pValues: []float64{p1, p2}, qValues: []float64{q1, q2}}
}

func goldenFromSlice(ps, qs []float64, err error) goldenOutcome {
	return goldenOutcome{pValues: ps, qValues: qs, err: err}
}

// goldenTestItems 返回覆盖全部导出检测函数的检测项表。
// 注意: OverlappingTemplateMatching 的 C 侧实现无 Q 值概念 (Go 层返回占位 0), 故仅收集 P。
func goldenTestItems() []goldenTestItem {
	return []goldenTestItem{
		{"Frequency", func(d []byte) goldenOutcome {
			p, q := FrequencyTestBytes(d)
			return goldenFrom2(p, q)
		}},
		{"BlockFrequency(128)", func(d []byte) goldenOutcome {
			p, q := BlockFrequencyTestBytes(d, 128)
			return goldenFrom2(p, q)
		}},
		{"CumulativeSums(Forward)", func(d []byte) goldenOutcome {
			p, q := CumulativeSumsTestBytes(d, true)
			return goldenFrom2(p, q)
		}},
		{"CumulativeSums(Backward)", func(d []byte) goldenOutcome {
			p, q := CumulativeSumsTestBytes(d, false)
			return goldenFrom2(p, q)
		}},
		{"Runs", func(d []byte) goldenOutcome {
			p, q := RunsTestBytes(d)
			return goldenFrom2(p, q)
		}},
		{"LongestRunOfOnes('1')", func(d []byte) goldenOutcome {
			p, q := LongestRunOfOnesTestBytes(d, true)
			return goldenFrom2(p, q)
		}},
		{"LongestRunOfOnes('0')", func(d []byte) goldenOutcome {
			p, q := LongestRunOfOnesTestBytes(d, false)
			return goldenFrom2(p, q)
		}},
		{"Rank(32x32)", func(d []byte) goldenOutcome {
			p, q := RankTestBytes(d, 32, 32)
			return goldenFrom2(p, q)
		}},
		{"DiscreteFourierTransform", func(d []byte) goldenOutcome {
			p, q := DiscreteFourierTransformTestBytes(d)
			return goldenFrom2(p, q)
		}},
		{"NonOverlappingTemplateMatching(m=9)", func(d []byte) goldenOutcome {
			ps, qs, _, err := NonOverlappingTemplateMatchingTestBytes(d, len(d)*8, 9)
			return goldenFromSlice(ps, qs, err)
		}},
		{"OverlappingTemplateMatching(m=9)", func(d []byte) goldenOutcome {
			p, _, err := OverlappingTemplateMatchingTestBytes(d, len(d)*8, 9)
			return goldenOutcome{pValues: []float64{p}, err: err}
		}},
		{"Universal", func(d []byte) goldenOutcome {
			p, q := UniversalTestBytes(d)
			return goldenFrom2(p, q)
		}},
		{"ApproximateEntropy(m=10)", func(d []byte) goldenOutcome {
			p, q := ApproximateEntropyTestBytes(d, 10)
			return goldenFrom2(p, q)
		}},
		{"RandomExcursions", func(d []byte) goldenOutcome {
			ps, qs, _, err := RandomExcursionsTestBytes(d, len(d)*8)
			return goldenFromSlice(ps, qs, err)
		}},
		{"RandomExcursionsVariant", func(d []byte) goldenOutcome {
			ps, qs, _, err := RandomExcursionsVariantTestBytes(d, len(d)*8)
			return goldenFromSlice(ps, qs, err)
		}},
		{"Serial(m=16)", func(d []byte) goldenOutcome {
			p1, q1, p2, q2 := SerialTestBytes(d, 16)
			return goldenFrom4(p1, q1, p2, q2)
		}},
		{"LinearComplexity(m=500)", func(d []byte) goldenOutcome {
			p, q := LinearComplexityTestBytes(d, 500)
			return goldenFrom2(p, q)
		}},
		{"Poker(m=4)", func(d []byte) goldenOutcome {
			p, q := PokerTestBytes(d, 4)
			return goldenFrom2(p, q)
		}},
		{"RunsDistribution", func(d []byte) goldenOutcome {
			p, q := RunsDistributionTestBytes(d)
			return goldenFrom2(p, q)
		}},
		{"BinaryDerivative(m=3)", func(d []byte) goldenOutcome {
			p, q := BinaryDerivativeTestBytes(d, 3)
			return goldenFrom2(p, q)
		}},
		{"Autocorrelation(m=2)", func(d []byte) goldenOutcome {
			p, q := AutocorrelationTestBytes(d, 2)
			return goldenFrom2(p, q)
		}},
	}
}

// goldenValuesPassed 判定一组 P (或 Q) 值是否通过:
// 多值项 (len ≥ 20, 如 NonOverlapping 的 148 个模板 p 值) 允许 5% 失败比例, 其余不允许失败。
func goldenValuesPassed(values []float64, threshold float64) (bool, string) {
	if len(values) == 0 {
		return true, ""
	}
	allow := 0
	if len(values) >= 20 {
		allow = len(values) / 20
	}
	failures := 0
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false, "存在 NaN/Inf 值"
		}
		if v < threshold {
			failures++
		}
	}
	if failures > allow {
		return false, fmt.Sprintf("%d/%d 个值低于阈值 %.4f (允许 %d)", failures, len(values), threshold, allow)
	}
	return true, ""
}

// goldenOutcomeApplicable 判断该次检测是否实际适用:
// RandomExcursions 系列在回零次数 J 不足 (J < MAX(0.005·√n, 500)) 时, C 侧返回全 0 p 值
// 表示本次不适用 (与 NIST STS 及本仓库 CLI 的 p>0 计数语义一致), 不算失败。
func goldenOutcomeApplicable(o goldenOutcome) bool {
	if o.err != nil || len(o.pValues) == 0 {
		return true
	}
	for _, p := range o.pValues {
		if p != 0 {
			return true
		}
	}
	return false
}

// goldenItemPassed 判定随机流上一次检测是否整体通过 (回归护航口径)。
func goldenItemPassed(o goldenOutcome) (bool, string) {
	if o.err != nil {
		return false, o.err.Error()
	}
	if passed, detail := goldenValuesPassed(o.pValues, Alpha); !passed {
		return false, "P: " + detail
	}
	if passed, detail := goldenValuesPassed(o.qValues, AlphaT); !passed {
		return false, "Q: " + detail
	}
	return true, ""
}

// goldenItemFailed 判定一次检测是否"检出"非随机 (known-answer 口径):
// 显式 error、NaN/Inf 值、任一 P < Alpha (或 Q < AlphaT) 均视为检出。
func goldenItemFailed(o goldenOutcome) (bool, string) {
	if o.err != nil {
		return true, o.err.Error()
	}
	for _, p := range o.pValues {
		if math.IsNaN(p) || math.IsInf(p, 0) {
			return true, "P 值为 NaN/Inf"
		}
		if p < Alpha {
			return true, fmt.Sprintf("P=%.6f < Alpha", p)
		}
	}
	for _, q := range o.qValues {
		if math.IsNaN(q) || math.IsInf(q, 0) {
			return true, "Q 值为 NaN/Inf"
		}
		if q < AlphaT {
			return true, fmt.Sprintf("Q=%.6f < AlphaT", q)
		}
	}
	return false, "未检出"
}

func goldenFillData(fill byte) []byte {
	data := make([]byte, goldenBytesPerGroup)
	for i := range data {
		data[i] = fill
	}
	return data
}

// goldenRandomData 用 ChaCha8 固定种子生成确定性随机流 (每字节种子对应一条独立流)。
func goldenRandomData(seedByte byte) []byte {
	var seed [32]byte
	for i := range seed {
		seed[i] = seedByte
	}
	rng := rand.NewChaCha8(seed)
	data := make([]byte, goldenBytesPerGroup)
	_, _ = rng.Read(data)
	return data
}

// TestGoldenConstructiveMustFail 构造性非随机序列必须被检测套件检出。
func TestGoldenConstructiveMustFail(t *testing.T) {
	ResetAlphaValue()
	cases := []struct {
		name string
		fill byte
	}{
		{"全 0 序列", 0x00},
		{"全 1 序列", 0xFF},
		{"0101 交替序列", 0x55},
		{"短周期块 0011 序列", 0x33},
	}
	items := goldenTestItems()
	itemByName := make(map[string]goldenTestItem, len(items))
	for _, item := range items {
		itemByName[item.name] = item
	}
	dataByFill := make(map[byte][]byte, len(cases))

	for _, tc := range cases {
		data := goldenFillData(tc.fill)
		dataByFill[tc.fill] = data
		detected := 0
		for _, item := range items {
			failed, detail := goldenItemFailed(item.run(data))
			if failed {
				detected++
			}
			t.Logf("%s / %s: 检出=%v (%s)", tc.name, item.name, failed, detail)
		}
		if detected < 10 {
			t.Errorf("%s: 检出数 %d/%d 低于下限 10, 套件检出能力异常", tc.name, detected, len(items))
		} else {
			t.Logf("%s: 检出 %d/%d", tc.name, detected, len(items))
		}
	}

	// 核心检出能力的精确断言 (统计量行为确定的组合)
	mustFail := []struct {
		itemName string
		fill     byte
		fillName string
	}{
		{"Frequency", 0x00, "全 0"},
		{"Frequency", 0xFF, "全 1"},
		{"Runs", 0x55, "0101 交替"},
		{"DiscreteFourierTransform", 0x55, "0101 交替"},
		{"DiscreteFourierTransform", 0x33, "短周期 0011"},
	}
	for _, mf := range mustFail {
		failed, detail := goldenItemFailed(itemByName[mf.itemName].run(dataByFill[mf.fill]))
		if !failed {
			t.Errorf("%s 对%s序列未检出 (应 FAIL): %s", mf.itemName, mf.fillName, detail)
		}
	}
}

// TestGoldenRandomStreamMustPass ChaCha8 固定种子随机流必须整体通过全部检测项。
// 输入由固定种子确定性生成, 结果完全可重现; RE/REV 的“J 不足不适用”组不计入通过率分母。
func TestGoldenRandomStreamMustPass(t *testing.T) {
	ResetAlphaValue()
	groups := 30
	passRatioNum := 26 // 通过率要求: 通过数/适用数 ≥ 26/30 (容忍统计波动)
	if testing.Short() {
		groups = 6
		passRatioNum = 24
	}
	items := goldenTestItems()
	passCount := make([]int, len(items))
	applicableCount := make([]int, len(items))
	failureNotes := make([][]string, len(items))
	for g := 0; g < groups; g++ {
		data := goldenRandomData(byte(0x40 + g))
		for i, item := range items {
			o := item.run(data)
			if !goldenOutcomeApplicable(o) {
				continue // 本次不适用 (RE/REV J 不足), 不计入分母
			}
			applicableCount[i]++
			if passed, detail := goldenItemPassed(o); passed {
				passCount[i]++
			} else {
				failureNotes[i] = append(failureNotes[i], fmt.Sprintf("组 %d: %s", g, detail))
			}
		}
	}
	for i, item := range items {
		if applicableCount[i] == 0 {
			t.Errorf("%s: %d 组随机流均不适用, 检测项疑似失效", item.name, groups)
			continue
		}
		// 通过率: passed×30 ≥ 26×applicable (即通过比例 ≥ 26/30)
		if passCount[i]*30 < passRatioNum*applicableCount[i] {
			t.Errorf("%s: 随机流通过率 %d/%d 低于 %d/30, 失败详情: %v", item.name, passCount[i], applicableCount[i], passRatioNum, failureNotes[i])
		} else {
			t.Logf("%s: 随机流通过率 %d/%d (适用口径)", item.name, passCount[i], applicableCount[i])
		}
	}
}
