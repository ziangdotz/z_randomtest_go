package randomtest_core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// assertStatsEqual 比较两组统计快照是否一致 (整型精确比较)。
func assertStatsEqual(t *testing.T, got, want []ItemStat) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("统计项数不一致: got=%d want=%d", len(got), len(want))
	}
	for i := range got {
		g, w := got[i], want[i]
		if g.Name != w.Name {
			t.Fatalf("项 %d 名称不一致: got=%s want=%s", i, g.Name, w.Name)
		}
		if g.PCount != w.PCount || g.QCount != w.QCount || g.ErrCount != w.ErrCount {
			t.Fatalf("项 %s 计数不一致: got=(P:%d Q:%d Err:%d) want=(P:%d Q:%d Err:%d)",
				g.Name, g.PCount, g.QCount, g.ErrCount, w.PCount, w.QCount, w.ErrCount)
		}
		if len(g.ValuesNum) != len(w.ValuesNum) || len(g.PCounts) != len(w.PCounts) {
			t.Fatalf("项 %s 多值切片长度不一致", g.Name)
		}
		for j := range g.ValuesNum {
			if g.ValuesNum[j] != w.ValuesNum[j] || g.PCounts[j] != w.PCounts[j] {
				t.Fatalf("项 %s 子项 %d 计数不一致: got=(V:%d P:%d) want=(V:%d P:%d)",
					g.Name, j, g.ValuesNum[j], g.PCounts[j], w.ValuesNum[j], w.PCounts[j])
			}
		}
	}
}

// runSuiteDirect 顺序执行套件并直接聚合统计 (作为 Runner 的参照实现)。
func runSuiteDirect(s *Suite, blocks [][]byte) []ItemStat {
	r := NewSuiteRunner(s, RunnerOptions{Workers: 1})
	local := r.newLocalStats()
	for _, block := range blocks {
		r.runSuite(block, local, "")
	}
	for i := range r.stats {
		r.stats[i].merge(&local[i])
	}
	return r.Stats()
}

// TestSuiteRunnerFileMatchesDirect: Runner 并发统计结果与顺序执行完全一致 (NIST 套件)。
func TestSuiteRunnerFileMatchesDirect(t *testing.T) {
	ResetAlphaValue()
	dir := t.TempDir()
	data := append(goldenRandomData(0x77), goldenRandomData(0x78)...)
	path := filepath.Join(dir, "sample.bin")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	suite := SuiteNISTLevel2()
	runner := NewSuiteRunner(suite, RunnerOptions{Workers: 3})
	if err := runner.RunFile(path, 2); err != nil {
		t.Fatalf("RunFile 失败: %v", err)
	}
	want := runSuiteDirect(suite, [][]byte{data[:suite.BlockSizeBytes], data[suite.BlockSizeBytes:]})
	assertStatsEqual(t, runner.Stats(), want)
}

// TestSuiteRunnerDirMatchesDirect: 目录模式统计与顺序执行一致 (GM L1 套件),
// 且 OnItemOutcome 回调次数 = 文件数 × 检测项数。
func TestSuiteRunnerDirMatchesDirect(t *testing.T) {
	ResetAlphaValue()
	dir := t.TempDir()
	suite, err := SuiteGM(1)
	if err != nil {
		t.Fatalf("SuiteGM(1) 失败: %v", err)
	}
	var blocks [][]byte
	for i, seed := range []byte{0x61, 0x62} {
		// GM L1 块大小为 2500 字节 (20000 bits), 取随机流前缀保持确定性
		data := goldenRandomData(seed)[:suite.BlockSizeBytes]
		if err := os.WriteFile(filepath.Join(dir, "f"+string(rune('0'+i))+".bin"), data, 0o644); err != nil {
			t.Fatalf("写入测试文件失败: %v", err)
		}
		blocks = append(blocks, data)
	}
	// 大小不符的文件应被跳过
	if err := os.WriteFile(filepath.Join(dir, "bad_size.bin"), make([]byte, 10), 0o644); err != nil {
		t.Fatalf("写入干扰文件失败: %v", err)
	}

	var cbCount int64
	runner := NewSuiteRunner(suite, RunnerOptions{
		Workers: 2,
		OnItemOutcome: func(fileName string, itemIdx int, o Outcome) {
			atomic.AddInt64(&cbCount, 1) // 回调在 worker goroutine 并发调用
		},
	})
	if err := runner.RunDir(dir); err != nil {
		t.Fatalf("RunDir 失败: %v", err)
	}
	want := runSuiteDirect(suite, blocks)
	assertStatsEqual(t, runner.Stats(), want)
	if got := int(atomic.LoadInt64(&cbCount)); got != len(blocks)*len(suite.Items) {
		t.Fatalf("OnItemOutcome 回调次数不一致: got=%d want=%d", got, len(blocks)*len(suite.Items))
	}
}

// TestSuiteGMInvalidLevel: 非法等级应报错。
func TestSuiteGMInvalidLevel(t *testing.T) {
	if _, err := SuiteGM(9); err == nil {
		t.Fatal("SuiteGM(9) 应返回错误")
	}
}

// TestItemStatApplyOutcome: 统计聚合语义 (RequireAll / Applied / Err)。
func TestItemStatApplyOutcome(t *testing.T) {
	// RequireAll: 双 P 全过才计 PCount, 双 Q 全过才计 QCount
	st := &ItemStat{}
	st.applyOutcome(Outcome{P: []float64{0.5, 0.9}, Q: []float64{0.5, 0.9}, RequireAll: true})
	if st.PCount != 1 || st.QCount != 1 {
		t.Fatalf("RequireAll 全过: got=(P:%d Q:%d) want=(1 1)", st.PCount, st.QCount)
	}
	st = &ItemStat{}
	st.applyOutcome(Outcome{P: []float64{0.5, 0.001}, Q: []float64{0.5, 0.9}, RequireAll: true})
	if st.PCount != 0 || st.QCount != 1 {
		t.Fatalf("RequireAll 部分失败: got=(P:%d Q:%d) want=(0 1)", st.PCount, st.QCount)
	}
	// 多值: Applied=false 不计入; p>0 计适用; 仅通过值计通过
	st = &ItemStat{}
	st.applyOutcome(Outcome{
		P:       []float64{0.5, 0.0, 0.001},
		Applied: []bool{true, false, true},
	})
	if st.ValuesNum[0] != 1 || st.ValuesNum[1] != 0 || st.ValuesNum[2] != 1 {
		t.Fatalf("多值适用计数: got=%v want=[1 0 1]", st.ValuesNum)
	}
	if st.PCounts[0] != 1 || st.PCounts[2] != 0 {
		t.Fatalf("多值通过计数: got=%v want=[1 0 0]", st.PCounts)
	}
	// Err: 累计不中断
	st = &ItemStat{}
	st.applyOutcome(Outcome{Err: errors.New("boom")})
	if st.ErrCount != 1 || st.LastErr == nil {
		t.Fatalf("错误统计: got=(%d %v)", st.ErrCount, st.LastErr)
	}
}

// TestSuiteRunnerCancel: 取消后 RunFile 返回 context.Canceled。
func TestSuiteRunnerCancel(t *testing.T) {
	ResetAlphaValue()
	dir := t.TempDir()
	suite := SuiteNISTLevel2()
	data := goldenRandomData(0x79)
	path := filepath.Join(dir, "cancel.bin")
	// 生成足够多的块让引擎持续工作
	blocks := make([]byte, 0, suite.BlockSizeBytes*8)
	for i := 0; i < 8; i++ {
		blocks = append(blocks, data...)
	}
	if err := os.WriteFile(path, blocks, 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runner := NewSuiteRunner(suite, RunnerOptions{Workers: 1, Ctx: ctx})
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	err := runner.RunFile(path, 8)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("取消后 RunFile 应返回 context.Canceled, got=%v", err)
	}
}
