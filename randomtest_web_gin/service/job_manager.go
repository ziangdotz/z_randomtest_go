package service

// 阶段④ 4.2: 异步检测任务管理。
//
// 端点:
//	POST   /api/v1/randomtest/jobs        创建任务 (201, 返回 jobId)
//	GET    /api/v1/randomtest/jobs/{id}   查询状态/进度/结果
//	DELETE /api/v1/randomtest/jobs/{id}   取消任务 (queued/running 可取消)
//
// 并发控制: 全局信号量 (maxConcurrentJobs, CPU 密集型检测默认串行),
// 同步端点与异步任务共用同一信号量, 满载时同步端点返回 409。
// 结果保留 TTL (默认 10 分钟), 在每次创建任务时惰性清理过期任务。

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// JobStatus 任务状态机: queued → running → done/failed/cancelled。
type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobDone      JobStatus = "done"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

// 任务类型: 后端预设套件 (取消粒度为单项检测) 或用户自选组合 (不中断)。
const (
	JobKindNIST2  = "nist2"
	JobKindGM2    = "gm2"
	JobKindCustom = "custom"
)

// ErrJobNotFound / ErrJobNotCancellable 取消操作的两种失败语义 (分别映射 404/409)。
var (
	ErrJobNotFound       = errors.New("job not found")
	ErrJobNotCancellable = errors.New("job not cancellable")
)

// Job 异步任务实例 (Status/Progress/Result 受 mu 保护, worker 写、查询读)。
type Job struct {
	mu        sync.Mutex
	ID        string
	Kind      string
	Status    JobStatus
	Progress  string
	Result    []TestFuncResultDynamic
	Err       string
	CreatedAt time.Time
	UpdatedAt time.Time

	cancel context.CancelFunc
}

// JobSnapshot 任务状态快照 (供 HTTP 响应使用, 避免暴露内部锁)。
type JobSnapshot struct {
	ID        string
	Kind      string
	Status    JobStatus
	Progress  string
	Result    []TestFuncResultDynamic
	Err       string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (j *Job) snapshot() JobSnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	return JobSnapshot{
		ID:        j.ID,
		Kind:      j.Kind,
		Status:    j.Status,
		Progress:  j.Progress,
		Result:    j.Result,
		Err:       j.Err,
		CreatedAt: j.CreatedAt,
		UpdatedAt: j.UpdatedAt,
	}
}

func (j *Job) update(status JobStatus, progress, errMsg string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = status
	if progress != "" {
		j.Progress = progress
	}
	j.Err = errMsg
	j.UpdatedAt = time.Now()
}

// JobManager 任务管理器: sync.Map 存储 + 并发信号量 + 超时与 TTL。
type JobManager struct {
	jobs    sync.Map      // id → *Job
	sem     chan struct{} // 并发检测信号量
	timeout time.Duration // 任务级超时, 0 = 不限
	ttl     time.Duration // 结果保留时间
}

// NewJobManager maxConcurrent<=0 时取 1 (CPU 密集型检测默认串行)。
func NewJobManager(maxConcurrent int, timeout, ttl time.Duration) *JobManager {
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &JobManager{
		sem:     make(chan struct{}, maxConcurrent),
		timeout: timeout,
		ttl:     ttl,
	}
}

var defaultJobManager = NewJobManager(1, 0, 10*time.Minute)

// GlobalJobManager 返回全局任务管理器 (未显式初始化时为默认配置)。
func GlobalJobManager() *JobManager { return defaultJobManager }

// InitJobManager 由入口 (main) 在读取配置后显式初始化全局任务管理器。
func InitJobManager(maxConcurrent int, timeout, ttl time.Duration) {
	defaultJobManager = NewJobManager(maxConcurrent, timeout, ttl)
}

// TryAcquire 非阻塞获取检测并发额度 (同步端点满载保护, 满载返回 false → 409)。
func (m *JobManager) TryAcquire() bool {
	select {
	case m.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release 归还检测并发额度。
func (m *JobManager) Release() {
	<-m.sem
}

// Submit 创建任务并异步执行, 返回任务 ID。
func (m *JobManager) Submit(kind string, req RandomTestHTTPRequestStruct) string {
	id := newJobID()
	ctx, cancel := context.WithCancel(context.Background())
	job := &Job{
		ID:        id,
		Kind:      kind,
		Status:    JobQueued,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		cancel:    cancel,
	}
	m.jobs.Store(id, job)
	m.cleanupExpired()
	go m.run(job, ctx, req)
	return id
}

// Get 返回任务快照。
func (m *JobManager) Get(id string) (JobSnapshot, bool) {
	v, ok := m.jobs.Load(id)
	if !ok {
		return JobSnapshot{}, false
	}
	return v.(*Job).snapshot(), true
}

// Cancel 取消任务: queued/running 可取消, 不存在返回 ErrJobNotFound,
// 已结束 (done/failed/cancelled) 返回 ErrJobNotCancellable。
func (m *JobManager) Cancel(id string) error {
	v, ok := m.jobs.Load(id)
	if !ok {
		return ErrJobNotFound
	}
	job := v.(*Job)
	job.mu.Lock()
	status := job.Status
	job.mu.Unlock()
	switch status {
	case JobQueued, JobRunning:
		job.cancel() // queued: 退出排队等待; running: 在下一检测项边界中断
		return nil
	default:
		return ErrJobNotCancellable
	}
}

// run 任务执行协程: 排队获取信号量 → 执行 → 归还。
func (m *JobManager) run(job *Job, ctx context.Context, req RandomTestHTTPRequestStruct) {
	// 排队等待并发额度
	select {
	case m.sem <- struct{}{}:
	case <-ctx.Done():
		job.update(JobCancelled, "", "任务已取消")
		return
	}
	defer m.Release()

	if ctx.Err() != nil {
		job.update(JobCancelled, "", "任务已取消")
		return
	}

	execCtx := ctx
	if m.timeout > 0 {
		var timeoutCancel context.CancelFunc
		execCtx, timeoutCancel = context.WithTimeout(ctx, m.timeout)
		defer timeoutCancel()
	}

	job.update(JobRunning, "", "")
	result, err := executeJob(execCtx, job.Kind, req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			job.update(JobCancelled, "", "任务已取消")
			return
		}
		if errors.Is(err, context.DeadlineExceeded) {
			job.update(JobCancelled, "", "任务超时")
			return
		}
		job.update(JobFailed, "", err.Error())
		return
	}
	job.mu.Lock()
	job.Status = JobDone
	job.Result = result
	job.Progress = "completed"
	job.UpdatedAt = time.Now()
	job.mu.Unlock()
}

// executeJob 按任务类型执行检测; custom 为用户自选组合, 不支持中途取消。
func executeJob(ctx context.Context, kind string, req RandomTestHTTPRequestStruct) ([]TestFuncResultDynamic, error) {
	switch kind {
	case JobKindNIST2:
		return req.HandleRandomTestNISTLevel2ServiceCtx(ctx)
	case JobKindGM2:
		return req.HandleRandomTestGMLevel2ServiceCtx(ctx)
	default:
		return req.HandleRandomTestHTTPRequestService()
	}
}

// cleanupExpired 惰性清理超过 TTL 的已结束任务。
func (m *JobManager) cleanupExpired() {
	now := time.Now()
	m.jobs.Range(func(key, value interface{}) bool {
		job := value.(*Job)
		job.mu.Lock()
		status, updatedAt := job.Status, job.UpdatedAt
		job.mu.Unlock()
		switch status {
		case JobDone, JobFailed, JobCancelled:
			if now.Sub(updatedAt) > m.ttl {
				m.jobs.Delete(key)
			}
		}
		return true
	})
}

// newJobID 生成 16 字符十六进制任务 ID。
func newJobID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
