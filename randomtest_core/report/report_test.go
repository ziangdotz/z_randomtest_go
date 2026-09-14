package report

import (
	"math"
	"randomtest_core"
	"strings"
	"testing"
)

/* 以下用例均基于默认显著性水平 Alpha=0.01, AlphaT=0.0001 */

func TestPassCountRange(t *testing.T) {
	// n=1000: 3σ 区间 [980.56, 999.44] → max=Floor=999, min=Ceil=981
	max, min := PassCountRange(1000)
	if max != 999 || min != 981 {
		t.Fatalf("PassCountRange(1000) = (%d, %d), want (999, 981)", max, min)
	}
	// 边界样本: 980/1000 应判为不通过 (上一轮 int() 截断误判的回归用例)
	if CountInRange(980, 1000) {
		t.Fatal("CountInRange(980, 1000) = true, want false")
	}
	if !CountInRange(981, 1000) {
		t.Fatal("CountInRange(981, 1000) = false, want true")
	}
	if !CountInRange(999, 1000) {
		t.Fatal("CountInRange(999, 1000) = false, want true")
	}
	if CountInRange(1000, 1000) {
		t.Fatal("CountInRange(1000, 1000) = true, want false (超出 max)")
	}
	// sampleSize<=0 不可判定
	if CountInRange(0, 0) {
		t.Fatal("CountInRange(0, 0) = true, want false")
	}
}

func TestPassCountRangeWithAlphaAndMinimum(t *testing.T) {
	maxA, minA := PassCountRangeWithAlpha(0.01, 1000)
	maxG, minG := PassCountRange(1000)
	if maxA != maxG || minA != minG {
		t.Fatalf("WithAlpha(0.01) = (%d,%d) 与全局配置 (%d,%d) 不一致", maxA, minA, maxG, minG)
	}
	if got := MinimumPassCount(1000); got != 981 {
		t.Fatalf("MinimumPassCount(1000) = %d, want 981", got)
	}
	if got := MinimumPassCount(0); got != 0 {
		t.Fatalf("MinimumPassCount(0) = %d, want 0", got)
	}
}

func TestJudgeFileItem(t *testing.T) {
	singlePass := randomtest_core.Outcome{P: []float64{0.5}, Q: []float64{0.5}}
	// NIST: Q 不参与判定, QApplicable=false (与 "Q 不通过" 区分)
	v := JudgeFileItem(singlePass, false)
	if !v.PPassed || v.QPassed || v.QApplicable {
		t.Fatalf("NIST 单值判定 = %+v, want {P:true Q:false QA:false}", v)
	}
	// GM: P/Q 双判定
	v = JudgeFileItem(singlePass, true)
	if !v.PPassed || !v.QPassed || !v.QApplicable {
		t.Fatalf("GM 单值判定 = %+v, want 全 true", v)
	}
	// GM 单值 Q 低于 AlphaT
	v = JudgeFileItem(randomtest_core.Outcome{P: []float64{0.5}, Q: []float64{0.00005}}, true)
	if v.QPassed {
		t.Fatal("Q=0.00005 < AlphaT 应判为不通过")
	}
	// RequireAll (SerialTest): 双值全过才通过
	serial := randomtest_core.Outcome{P: []float64{0.5, 0.5}, Q: []float64{0.5, 0.5}, RequireAll: true}
	if v = JudgeFileItem(serial, true); !v.PPassed || !v.QPassed {
		t.Fatalf("SerialTest 双值全过判定 = %+v, want P/Q 均 true", v)
	}
	serialFail := randomtest_core.Outcome{P: []float64{0.5, 0.001}, Q: []float64{0.5, 0.5}, RequireAll: true}
	if v = JudgeFileItem(serialFail, true); v.PPassed {
		t.Fatal("SerialTest 任一 P 不通过应整体不通过")
	}
	// 多值检测 (RE/REV/NonOverlapping): 保持既有 "恒判定为不通过" 语义
	multi := randomtest_core.Outcome{P: []float64{0.5, 0.5, 0.5}}
	if v = JudgeFileItem(multi, true); v.PPassed || v.QPassed {
		t.Fatalf("多值检测判定 = %+v, want P/Q 均 false (既有语义)", v)
	}
}

func TestFileDetailFailedAndReason(t *testing.T) {
	cases := []struct {
		name       string
		detail     FileDetail
		wantFailed bool
		wantReason string
	}{
		{"NIST Q不适用不算失败", FileDetail{PPassed: true, QPassed: false, QApplicable: false}, false, ""},
		{"GM Q不通过", FileDetail{PPassed: true, QPassed: false, QApplicable: true}, true, "Q不通过 "},
		{"GM P不通过", FileDetail{PPassed: false, QPassed: true, QApplicable: true}, true, "P不通过 "},
		{"GM 双不通过", FileDetail{PPassed: false, QPassed: false, QApplicable: true}, true, "P不通过 Q不通过 "},
		{"GM 双通过", FileDetail{PPassed: true, QPassed: true, QApplicable: true}, false, ""},
	}
	for _, c := range cases {
		if got := c.detail.Failed(); got != c.wantFailed {
			t.Fatalf("%s: Failed() = %v, want %v", c.name, got, c.wantFailed)
		}
		if got := c.detail.FailReason(); got != c.wantReason {
			t.Fatalf("%s: FailReason() = %q, want %q", c.name, got, c.wantReason)
		}
	}
}

func TestBuildRows(t *testing.T) {
	stats := []randomtest_core.ItemStat{
		{Name: "FrequencyTest", PCount: 995, QCount: 990},
		{Name: "RandomExcursionsTest", ValuesNum: []int{1000, 0}, PCounts: []int{995, 0}},
		{Name: "NonOverlappingTemplateMatchingTest", ValuesNum: []int{1000, 1000}, PCounts: []int{500, 995}},
	}
	// NIST: 多值展开, RE/REV 以 ValuesNum 为分母, 其余多值项以 testNum 为分母
	rows := BuildRows(1000, stats, false)
	if len(rows) != 5 {
		t.Fatalf("NIST BuildRows 行数 = %d, want 5", len(rows))
	}
	if rows[0].Multi || rows[0].SampleSize != 1000 || !rows[0].PPassed || rows[0].QApplicable {
		t.Fatalf("NIST 单值行 = %+v 不符合预期", rows[0])
	}
	if !rows[1].Multi || rows[1].SubIndex != 0 || rows[1].SampleSize != 1000 || !rows[1].PPassed {
		t.Fatalf("RE 子项 0 = %+v 不符合预期", rows[1])
	}
	if rows[2].SampleSize != 0 || rows[2].PPassed {
		t.Fatalf("RE 子项 1 (无适用样本) = %+v, want SampleSize=0 且不可判定", rows[2])
	}
	if rows[3].SampleSize != 1000 || rows[3].PPassed {
		t.Fatalf("NonOverlapping 子项 0 (500/1000) = %+v, want 不通过", rows[3])
	}
	if rows[4].SampleSize != 1000 || !rows[4].PPassed {
		t.Fatalf("NonOverlapping 子项 1 (995/1000) = %+v, want 通过 (分母为 testNum)", rows[4])
	}
	// GM: 不展开, 每检测项一行, P/Q 双判定
	gmStats := stats[:1]
	rows = BuildRows(1000, gmStats, true)
	if len(rows) != 1 || !rows[0].QApplicable || !rows[0].PPassed || !rows[0].QPassed {
		t.Fatalf("GM 行 = %+v, want QApplicable 且 P/Q 均通过", rows[0])
	}
}

func TestStatsTableLines(t *testing.T) {
	stats := []randomtest_core.ItemStat{
		{Name: "FrequencyTest", PCount: 995, QCount: 500},
	}
	lines := StatsTableLines(2, 1000, stats, false)
	if lines[0] != "" || lines[1] != "📊 NIST Level2 测试结果统计" {
		t.Fatalf("NIST 表头 = %q / %q, 不符合既有格式", lines[0], lines[1])
	}
	if len(lines) != 7 { // 空行 + 标题 + 3 分隔/表头 + 1 数据行 + 尾分隔
		t.Fatalf("NIST 表行数 = %d, want 7", len(lines))
	}
	if strings.HasSuffix(lines[5], "*") {
		t.Fatalf("NIST 995/1000 应通过, 实际行 = %q", lines[5])
	}
	gmLines := StatsTableLines(2, 1000, stats, true)
	if gmLines[1] != "📊 GM Level2 测试结果统计" {
		t.Fatalf("GM 表头 = %q, 不符合既有格式", gmLines[1])
	}
	if !strings.Contains(gmLines[5], " *") {
		t.Fatalf("GM Q 500/1000 应标 *, 实际行 = %q", gmLines[5])
	}
	if !strings.Contains(gmLines[5], "995/1000  ") {
		t.Fatalf("GM P 列格式不符, 实际行 = %q", gmLines[5])
	}
}

func TestErrorFileListLines(t *testing.T) {
	stats := []randomtest_core.ItemStat{{Name: "FrequencyTest"}}
	// NIST: Q 不适用, P 通过 → 不出现在失败清单
	nistDetails := [][]FileDetail{{
		{FileName: "a.bin", PPassed: true, QPassed: false, QApplicable: false},
	}}
	lines := ErrorFileListLines(nistDetails, stats)
	if !strings.Contains(strings.Join(lines, "\n"), "所有文件均通过") {
		t.Fatal("NIST 全通过时应输出 '所有文件均通过'")
	}
	// GM: Q 不通过 → 出现在清单且仅标 Q
	gmDetails := [][]FileDetail{{
		{FileName: "b.bin", PPassed: true, QPassed: false, QApplicable: true},
	}}
	joined := strings.Join(ErrorFileListLines(gmDetails, stats), "\n")
	if !strings.Contains(joined, "[失败] b.bin") || !strings.Contains(joined, "Q不通过") || strings.Contains(joined, "P不通过") {
		t.Fatalf("GM Q 失败清单不符:\n%s", joined)
	}
}

func TestTrimIniQuotesAndSanitizePaths(t *testing.T) {
	cases := []struct{ in, want string }{
		{`"C:\path\to.bin"`, `C:\path\to.bin`},
		{`""C:\path\to.bin""`, `C:\path\to.bin`},
		{`'C:\path\to.bin'`, `C:\path\to.bin`},
		{`  "C:\x"  `, `C:\x`},
		{`C:\plain`, `C:\plain`},
		{``, ``},
	}
	for _, c := range cases {
		if got := TrimIniQuotes(c.in); got != c.want {
			t.Fatalf("TrimIniQuotes(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	filePath := `""D:\data\a.bin""`
	dirPath := " D:\\data\\dir "
	SanitizePaths(&filePath, &dirPath, nil)
	if filePath != `D:\data\a.bin` || dirPath != `D:\data\dir` {
		t.Fatalf("SanitizePaths 后 = (%q, %q), 不符合预期", filePath, dirPath)
	}
}

func TestValuesAllPassed(t *testing.T) {
	if !ValuesAllPassed(nil, 0.01) || !ValuesAllPassed([]float64{}, 0.01) {
		t.Fatal("空切片应视为不适用 (通过)")
	}
	if !ValuesAllPassed([]float64{0.5, 1.0}, 0.01) {
		t.Fatal("合法值应通过")
	}
	if ValuesAllPassed([]float64{0.001}, 0.01) {
		t.Fatal("低于阈值应不通过")
	}
	if ValuesAllPassed([]float64{1.5}, 0.01) {
		t.Fatal("大于 1 应不通过")
	}
	if ValuesAllPassed([]float64{math.NaN()}, 0.01) {
		t.Fatal("NaN 应不通过")
	}
	if ValuesAllPassed([]float64{math.Inf(1)}, 0.01) {
		t.Fatal("+Inf 应不通过")
	}
}

func TestFormatPassFraction(t *testing.T) {
	if got := FormatPassFraction(980, 1000); got != "980/1000" {
		t.Fatalf("FormatPassFraction = %q, want 980/1000", got)
	}
}
