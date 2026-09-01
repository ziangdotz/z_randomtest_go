package randomtest_core

import (
	"encoding/json"
	"flag"
	"math"
	"os"
	"path/filepath"
	"testing"
)

/*
regression 回归快照测试 (阶段①):

固定输入 (3 段各 125000 字节的 ChaCha8 确定性数据) 驱动全部检测函数,
P/Q 值序列化到 testdata/baseline.json (容差 1e-9)。

- 重建基线: go test . -v -run TestRegressionBaseline -args -update-golden
- 此基线是阶段③热路径优化 (位展开查表 / DFT 快速路径 / 缓冲池化) 的
  数值等价性验证依据。
*/

var updateGolden = flag.Bool("update-golden", false, "重建回归基线 testdata/baseline.json")

const (
	regressionBaselinePath   = "testdata/baseline.json"
	regressionBytesPerCase   = 125000 // 每段 1 Mbit
	regressionValueTolerance = 1e-9
)

type regressionCaseResult struct {
	Test string    `json:"test"`
	P    []float64 `json:"p"`
	Q    []float64 `json:"q"`
}

type regressionCase struct {
	Name    string                 `json:"name"`
	Results []regressionCaseResult `json:"results"`
}

type regressionBaseline struct {
	Cases []regressionCase `json:"cases"`
}

// regressionData 用 ChaCha8 固定种子生成确定性检测输入 (每字节种子对应一条独立流)。
func regressionData(seedByte byte) []byte {
	return goldenRandomData(seedByte)
}

// collectRegressionBaseline 以固定输入驱动全部检测项, 采集 P/Q 快照。
func collectRegressionBaseline() regressionBaseline {
	items := goldenTestItems()
	seeds := []struct {
		name string
		seed byte
	}{
		{"chacha8-seed-01", 0x11},
		{"chacha8-seed-02", 0x22},
		{"chacha8-seed-03", 0x33},
	}
	baseline := regressionBaseline{}
	for _, s := range seeds {
		data := regressionData(s.seed)
		results := make([]regressionCaseResult, 0, len(items))
		for _, item := range items {
			o := item.run(data)
			results = append(results, regressionCaseResult{Test: item.name, P: o.pValues, Q: o.qValues})
		}
		baseline.Cases = append(baseline.Cases, regressionCase{Name: s.name, Results: results})
	}
	return baseline
}

// TestRegressionBaseline 数值回归快照: 与 testdata/baseline.json 逐值比对 (容差 1e-9)。
func TestRegressionBaseline(t *testing.T) {
	ResetAlphaValue()
	baseline := collectRegressionBaseline()

	// 采集阶段健壮性校验: 随机流上不应出现 error / 空 P / NaN+Inf
	for _, c := range baseline.Cases {
		for _, r := range c.Results {
			if len(r.P) == 0 {
				t.Fatalf("用例 %s / %s: P 值为空, 采集异常", c.Name, r.Test)
			}
			for _, v := range r.P {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("用例 %s / %s: P 值存在 NaN/Inf, 回归基线不可用", c.Name, r.Test)
				}
			}
			for _, v := range r.Q {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("用例 %s / %s: Q 值存在 NaN/Inf, 回归基线不可用", c.Name, r.Test)
				}
			}
		}
	}

	if *updateGolden {
		bs, err := json.MarshalIndent(baseline, "", "  ")
		if err != nil {
			t.Fatalf("序列化回归基线失败: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(regressionBaselinePath), 0o755); err != nil {
			t.Fatalf("创建 testdata 目录失败: %v", err)
		}
		if err := os.WriteFile(regressionBaselinePath, bs, 0o644); err != nil {
			t.Fatalf("写入回归基线失败: %v", err)
		}
		t.Logf("回归基线已重建: %s (%d 用例)", regressionBaselinePath, len(baseline.Cases))
		return
	}

	bs, err := os.ReadFile(regressionBaselinePath)
	if err != nil {
		t.Fatalf("读取回归基线失败 (先用 'go test . -v -run TestRegressionBaseline -args -update-golden' 生成): %v", err)
	}
	var want regressionBaseline
	if err := json.Unmarshal(bs, &want); err != nil {
		t.Fatalf("解析回归基线失败: %v", err)
	}

	if len(baseline.Cases) != len(want.Cases) {
		t.Fatalf("用例数不一致: got=%d want=%d (基线可能过期, 请用 -update-golden 重建)", len(baseline.Cases), len(want.Cases))
	}
	for ci := range baseline.Cases {
		gotCase, wantCase := baseline.Cases[ci], want.Cases[ci]
		if gotCase.Name != wantCase.Name {
			t.Fatalf("用例 %d 名称不一致: got=%s want=%s", ci, gotCase.Name, wantCase.Name)
		}
		if len(gotCase.Results) != len(wantCase.Results) {
			t.Fatalf("用例 %s 检测项数不一致: got=%d want=%d (基线可能过期, 请用 -update-golden 重建)", gotCase.Name, len(gotCase.Results), len(wantCase.Results))
		}
		for ri := range gotCase.Results {
			gotRes, wantRes := gotCase.Results[ri], wantCase.Results[ri]
			if gotRes.Test != wantRes.Test {
				t.Fatalf("用例 %s 检测项 %d 名称不一致: got=%s want=%s", gotCase.Name, ri, gotRes.Test, wantRes.Test)
			}
			compareRegressionValues(t, gotCase.Name, gotRes.Test, "P", gotRes.P, wantRes.P)
			compareRegressionValues(t, gotCase.Name, gotRes.Test, "Q", gotRes.Q, wantRes.Q)
		}
	}
}

func compareRegressionValues(t *testing.T, caseName, testName, field string, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("用例 %s / %s (%s): 长度不一致 got=%d want=%d", caseName, testName, field, len(got), len(want))
		return
	}
	for i := range got {
		diff := math.Abs(got[i] - want[i])
		if diff > regressionValueTolerance {
			t.Errorf("用例 %s / %s (%s[%d]): 数值漂移 got=%.17g want=%.17g (diff=%.3g > %.0e)",
				caseName, testName, field, i, got[i], want[i], diff, regressionValueTolerance)
		}
	}
}
