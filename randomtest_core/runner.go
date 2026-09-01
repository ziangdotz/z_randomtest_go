package randomtest_core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
)

/*
套件运行器 (阶段②):

统一的并发检测引擎, 收敛 CLI/桌面端四段重复的 reader→worker→merge 流水线:
- RunFile: 文件分块模式 (块大小由 Suite.BlockSizeBytes 决定, sync.Pool 复用缓冲)
- RunDir:  目录模式 (文件大小过滤内置, 整文件读取, 所有权转移)
统计聚合语义与既有 runLevelX 实现一致, 输出由调用方通过 Stats() 渲染。
*/

// RunnerOptions SuiteRunner 运行参数。
type RunnerOptions struct {
	// Workers 并发工作协程数, <=0 时取 runtime.NumCPU()。
	Workers int
	// Ctx 取消信号: 触发取消后引擎停止投递与处理, Run* 返回 ctx.Err(),
	// 已完成部分的统计仍可通过 Stats() 获取。
	Ctx context.Context
	// Progress 进度回调, stage 形如 "NIST_FILE" / "GM_DIR", 可为 nil。
	Progress func(stage string, done, total int, currentFile string)
	// OnItemOutcome 目录模式下每个文件每个检测项的原始结果回调 (工作协程内调用,
	// 仅保证同一文件的回调顺序, 跨文件无顺序保证), 供调用方维护文件级明细; 可为 nil。
	OnItemOutcome func(fileName string, itemIdx int, o Outcome)
	// Warn 目录模式下文件大小不符的警告回调 (工作协程外调用), nil 时输出到 stdout。
	Warn func(format string, args ...interface{})
}

// ItemStat 单个检测项跨样本聚合统计, 字段语义与既有渲染逻辑一致。
type ItemStat struct {
	Name      string
	ValuesNum []int // 多值检测的适用计数 (RandomExcursions p>0 语义)
	PCounts   []int // 多值检测逐子项通过计数
	PCount    int
	QCounts   []int // 既有实现初始化但从不递增, 保持结构兼容
	QCount    int
	ErrCount  int   // 单次检测失败的累计次数
	LastErr   error // 最近一次检测失败的原因
}

// SuiteRunner 套件运行器。Stats() 需在对应 Run* 返回后调用。
type SuiteRunner struct {
	suite *Suite
	opt   RunnerOptions
	stats []ItemStat
}

// NewSuiteRunner 创建运行器 (opt.Workers<=0 时取 runtime.NumCPU())。
func NewSuiteRunner(s *Suite, opt RunnerOptions) *SuiteRunner {
	if s == nil {
		panic("NewSuiteRunner: suite 不能为空")
	}
	if opt.Workers <= 0 {
		opt.Workers = runtime.NumCPU()
	}
	if opt.Ctx == nil {
		opt.Ctx = context.Background()
	}
	r := &SuiteRunner{suite: s, opt: opt}
	r.stats = make([]ItemStat, len(s.Items))
	for i, item := range s.Items {
		r.stats[i].Name = item.Name
	}
	return r
}

// newLocalStats 生成工作协程私有的统计副本 (零值切片延迟到首个样本初始化)。
func (r *SuiteRunner) newLocalStats() []ItemStat {
	local := make([]ItemStat, len(r.stats))
	for i := range local {
		local[i].Name = r.stats[i].Name
	}
	return local
}

// applyOutcome 按既有语义累加一次检测结果 (与旧 runLevelX 行为一一对应)。
func (st *ItemStat) applyOutcome(o Outcome) {
	if o.Err != nil {
		st.ErrCount++
		st.LastErr = o.Err
		return
	}
	if o.RequireAll {
		if AllPValuesPassed(o.P) {
			st.PCount++
		}
		if len(o.Q) > 0 && AllQValuesPassed(o.Q) {
			st.QCount++
		}
		return
	}
	if len(o.P) > 1 {
		// 多值检测: 首个样本确定容量, 逐子项统计
		if st.ValuesNum == nil {
			st.ValuesNum = make([]int, len(o.P))
			st.PCounts = make([]int, len(o.P))
		}
		for i, p := range o.P {
			if o.Applied != nil && !o.Applied[i] {
				continue // 本次不适用 (p=0), 不计入
			}
			st.ValuesNum[i]++
			if PValuePassed(p) {
				st.PCounts[i]++
			}
		}
		return
	}
	// 单值检测
	if len(o.P) == 1 && PValuePassed(o.P[0]) {
		st.PCount++
	}
	if len(o.Q) >= 1 && QValuePassed(o.Q[0]) {
		st.QCount++
	}
}

// PValuePassed 单个 P 值是否通过 (与 CLI 的 isPValueOkBool 同口径, 导出供三入口复用)。
func PValuePassed(p float64) bool {
	return p >= Alpha && p <= 1
}

// QValuePassed 单个 Q 值是否通过 (与 CLI 的 isQValueOkBool 同口径)。
func QValuePassed(q float64) bool {
	return q >= AlphaT && q <= 1
}

// AllPValuesPassed 全部 P 值通过 (RequireAll 聚合判定)。
func AllPValuesPassed(ps []float64) bool {
	for _, p := range ps {
		if !PValuePassed(p) {
			return false
		}
	}
	return len(ps) > 0
}

// AllQValuesPassed 全部 Q 值通过 (RequireAll 聚合判定)。
func AllQValuesPassed(qs []float64) bool {
	for _, q := range qs {
		if !QValuePassed(q) {
			return false
		}
	}
	return len(qs) > 0
}

// merge 合并另一份统计 (多值切片按索引累加, 容量不一致视为异常忽略)。
func (st *ItemStat) merge(other *ItemStat) {
	if other.ValuesNum != nil {
		if st.ValuesNum == nil {
			st.ValuesNum = make([]int, len(other.ValuesNum))
			st.PCounts = make([]int, len(other.PCounts))
		}
		for j := range other.ValuesNum {
			st.ValuesNum[j] += other.ValuesNum[j]
			st.PCounts[j] += other.PCounts[j]
		}
	}
	st.PCount += other.PCount
	st.QCount += other.QCount
	st.ErrCount += other.ErrCount
	if other.LastErr != nil {
		st.LastErr = other.LastErr
	}
}

// Merge 将另一个 SuiteRunner 的统计合并进自身 (要求两者基于相同 Suite)。
func (r *SuiteRunner) Merge(o *SuiteRunner) {
	if o == nil || len(o.stats) != len(r.stats) {
		return
	}
	for i := range r.stats {
		r.stats[i].merge(&o.stats[i])
	}
}

// clone 深拷贝统计 (防止外部修改内部状态)。
func (st *ItemStat) clone() ItemStat {
	c := *st
	if st.ValuesNum != nil {
		c.ValuesNum = append([]int(nil), st.ValuesNum...)
		c.PCounts = append([]int(nil), st.PCounts...)
	}
	return c
}

// Stats 返回聚合统计快照。
func (r *SuiteRunner) Stats() []ItemStat {
	out := make([]ItemStat, len(r.stats))
	for i := range r.stats {
		out[i] = r.stats[i].clone()
	}
	return out
}

func (r *SuiteRunner) progress(stage string, done, total int, currentFile string) {
	if r.opt.Progress != nil {
		r.opt.Progress(stage, done, total, currentFile)
	}
}

// runSuite 对单个数据块执行全部检测项并累加到局部统计。
func (r *SuiteRunner) runSuite(data []byte, local []ItemStat, fileName string) {
	for i, item := range r.suite.Items {
		o := item.Run(data)
		local[i].applyOutcome(o)
		if r.opt.OnItemOutcome != nil {
			r.opt.OnItemOutcome(fileName, i, o)
		}
	}
}

// RunFile 文件分块模式: 将 path 内容按 Suite.BlockSizeBytes 分块, 共检测 testNum 块。
func (r *SuiteRunner) RunFile(path string, testNum int) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("[Error] 文件打开失败: %v", err)
	}
	defer file.Close()

	blockSize := r.suite.BlockSizeBytes
	total := testNum
	if total <= 0 {
		return fmt.Errorf("[Error] TestNum 必须大于 0")
	}
	stage := r.suite.Standard + "_FILE"
	r.progress(stage, 0, total, "")

	/* sync.Pool 复用缓冲区, 所有权直接投递, 无数据副本 */
	bufferPool := &sync.Pool{
		New: func() interface{} {
			return make([]byte, blockSize)
		},
	}
	/* 动态调整通道容量: 大块数据降容避免过度内存占用 (沿用既有策略) */
	channelSize := r.opt.Workers * 2
	if blockSize > 1000000 {
		channelSize = r.opt.Workers
	}
	taskChan := make(chan []byte, channelSize)
	resultChan := make(chan []ItemStat, r.opt.Workers)

	var wg sync.WaitGroup
	var processed int64
	var readErrMu sync.Mutex
	var readErrVal error
	setReadErr := func(err error) {
		readErrMu.Lock()
		if readErrVal == nil {
			readErrVal = err
		}
		readErrMu.Unlock()
	}
	progressStep := total / 100
	if progressStep < 1 {
		progressStep = 1
	}

	/* 工作池 */
	for i := 0; i < r.opt.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := r.newLocalStats()
			for {
				select {
				case <-r.opt.Ctx.Done():
					resultChan <- local
					return
				case data, ok := <-taskChan:
					if !ok {
						resultChan <- local
						return
					}
					r.runSuite(data, local, "")
					bufferPool.Put(data)
					done := int(atomic.AddInt64(&processed, 1))
					if done%progressStep == 0 || done == total {
						r.progress(stage, done, total, "")
					}
				}
			}
		}()
	}
	/* 文件读取协程 */
	go func() {
		defer close(taskChan)
		for i := 0; i < total; i++ {
			buf := bufferPool.Get().([]byte)
			n, err := io.ReadFull(file, buf)
			if err != nil {
				bufferPool.Put(buf)
				if err == io.EOF {
					setReadErr(fmt.Errorf("[Error] 文件数据不足, 需要 %d 字节, 仅读取 %d 字节 (已处理 %d/%d 块)", blockSize, n, i, total))
				} else {
					setReadErr(fmt.Errorf("[Error] 文件读取失败 (块 %d/%d): %v", i+1, total, err))
				}
				return
			}
			select {
			case taskChan <- buf:
			case <-r.opt.Ctx.Done():
				bufferPool.Put(buf)
				return
			}
		}
	}()
	/* 结果收集 */
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	for res := range resultChan {
		for i := range r.stats {
			r.stats[i].merge(&res[i])
		}
	}
	readErrMu.Lock()
	localReadErr := readErrVal
	readErrMu.Unlock()
	if localReadErr != nil {
		return localReadErr
	}
	if err := r.opt.Ctx.Err(); err != nil {
		return err
	}
	r.progress(stage, total, total, "")
	return nil
}

// dirTask 目录模式任务。
type dirTask struct {
	FileName string
	Data     []byte
}

// RunDir 目录模式: 只处理大小等于 Suite.BlockSizeBytes 的普通文件, 每文件为一个检测样本。
func (r *SuiteRunner) RunDir(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("[Error] 目录读取失败: %v", err)
	}
	blockSize := r.suite.BlockSizeBytes
	var validFiles []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("[Error] 读取文件信息 %s 失败: %v", entry.Name(), err)
		}
		if info.Size() != int64(blockSize) {
			if r.opt.Warn != nil {
				r.opt.Warn("[WARNING] 文件 %s 大小不符合要求 (%d != %d), 跳过\n", entry.Name(), info.Size(), blockSize)
			} else {
				fmt.Printf("[WARNING] 文件 %s 大小不符合要求 (%d != %d), 跳过\n", entry.Name(), info.Size(), blockSize)
			}
			continue
		}
		validFiles = append(validFiles, entry)
	}
	total := len(validFiles)
	if total == 0 {
		return fmt.Errorf("[Error] 没有符合 %s Level%d 要求的输入文件", r.suite.Standard, r.suite.Level)
	}
	stage := r.suite.Standard + "_DIR"
	r.progress(stage, 0, total, "")

	taskChan := make(chan dirTask, r.opt.Workers*2)
	resultChan := make(chan []ItemStat, r.opt.Workers)

	var wg sync.WaitGroup
	var processed int64
	var readErrMu sync.Mutex
	var readErrVal error
	setReadErr := func(err error) {
		readErrMu.Lock()
		if readErrVal == nil {
			readErrVal = err
		}
		readErrMu.Unlock()
	}
	progressStep := total / 100
	if progressStep < 1 {
		progressStep = 1
	}

	/* 工作池 */
	for i := 0; i < r.opt.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := r.newLocalStats()
			for task := range taskChan {
				r.runSuite(task.Data, local, task.FileName)
				done := int(atomic.AddInt64(&processed, 1))
				if done%progressStep == 0 || done == total {
					r.progress(stage, done, total, task.FileName)
				}
			}
			resultChan <- local
		}()
	}
	/* 文件读取协程 */
	go func() {
		defer close(taskChan)
		for _, entry := range validFiles {
			filePath := filepath.Join(path, entry.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				setReadErr(fmt.Errorf("[Error] 文件 %s 读取失败: %v", filePath, err))
				return
			}
			select {
			case taskChan <- dirTask{FileName: entry.Name(), Data: data}:
			case <-r.opt.Ctx.Done():
				return
			}
		}
	}()
	/* 结果收集 */
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	for res := range resultChan {
		for i := range r.stats {
			r.stats[i].merge(&res[i])
		}
	}
	readErrMu.Lock()
	localReadErr := readErrVal
	readErrMu.Unlock()
	if localReadErr != nil {
		return localReadErr
	}
	if err := r.opt.Ctx.Err(); err != nil {
		return err
	}
	r.progress(stage, total, total, "")
	return nil
}
