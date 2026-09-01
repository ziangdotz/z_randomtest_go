package test

// 阶段④ 4.5: Web httptest 集成测试。
//
//	go test ./test -v
//
// 覆盖: 正常检测流 / 413 请求体超限 / 409 并发上限 / 异步任务创建-取消-查询 /
// 404 任务不存在 / 优雅关闭。

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"randomtest_web_gin/pkg/common"
	"randomtest_web_gin/pkg/setting"
	"randomtest_web_gin/routers"
	"randomtest_web_gin/service"

	"math/rand/v2"

	"github.com/gin-gonic/gin"
)

// jobResponse 任务查询响应骨架。
type jobResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		JobId  string                   `json:"jobId"`
		Status string                   `json:"status"`
		Result []map[string]interface{} `json:"result"`
	} `json:"data"`
}

func setupServer(t *testing.T) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// 配置: 请求体上限 1MB (便于 413 用例), 并发检测额度 1 (便于 409 用例)
	setting.ServerSettings = &setting.ServerSetting{
		Port:              ":0",
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxRequestBodyMB:  1,
		MaxConcurrentJobs: 1,
		JobTimeoutSec:     0,
	}
	service.InitJobManager(1, 0, 10*time.Minute)

	srv := httptest.NewServer(routers.InitRouter())
	t.Cleanup(srv.Close)
	return srv
}

// makeUpload 生成 125000 字节 (1Mbit) 确定性随机流的 Base64。
func makeUpload(seed byte) string {
	var s [32]byte
	for i := range s {
		s[i] = seed
	}
	rng := rand.NewChaCha8(s)
	data := make([]byte, 125000)
	_, _ = rng.Read(data)
	return base64.StdEncoding.EncodeToString(data)
}

func postJSON(t *testing.T, url string, body []byte) (int, *http.Response, []byte) {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s 失败: %v", url, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}
	return resp.StatusCode, resp, raw
}

func doRequest(t *testing.T, method, url string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s 失败: %v", method, url, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}
	return resp.StatusCode, raw
}

// waitForJobStatus 轮询任务直至达到期望状态。
func waitForJobStatus(t *testing.T, srv *httptest.Server, id string, want service.JobStatus, timeout time.Duration) jobResponse {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		code, raw := doRequest(t, http.MethodGet, srv.URL+"/api/v1/randomtest/jobs/"+id)
		if code != http.StatusOK {
			t.Fatalf("查询任务 %s 失败: HTTP %d", id, code)
		}
		var jr jobResponse
		if err := json.Unmarshal(raw, &jr); err != nil {
			t.Fatalf("解析任务响应失败: %v", err)
		}
		switch jr.Data.Status {
		case string(want):
			return jr
		case string(service.JobFailed):
			t.Fatalf("任务 %s 意外失败: %s", id, raw)
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("等待任务 %s 达到状态 %s 超时", id, want)
	return jobResponse{}
}

// TestSyncGMLevel2Flow 正常检测流: GM Level2 preset 全 25 项。
func TestSyncGMLevel2Flow(t *testing.T) {
	srv := setupServer(t)

	body, _ := json.Marshal(map[string]interface{}{"uploadFile": makeUpload(0x31)})
	code, _, raw := postJSON(t, srv.URL+"/api/v1/randomtest/handleRandomTestHTTPRequestGMLevel2", body)
	if code != http.StatusOK {
		t.Fatalf("期望 HTTP 200, 实际 %d: %s", code, raw)
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Result []struct {
				TestFuncName string    `json:"TestFuncName"`
				P1Value      []float64 `json:"P1Value"`
				IsPassed     bool      `json:"IsPassed"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if envelope.Code != common.OK {
		t.Fatalf("期望业务码 %d, 实际 %d", common.OK, envelope.Code)
	}
	if len(envelope.Data.Result) != 25 {
		t.Fatalf("GM L2 应有 25 个检测项, 实际 %d", len(envelope.Data.Result))
	}
	if got := envelope.Data.Result[0].TestFuncName; got != "FrequencyTest" {
		t.Fatalf("第一项应为 FrequencyTest, 实际 %s", got)
	}
}

// TestRequestBodyTooLarge 请求体超过 MaxRequestBodyMB → 413 + code=4130。
func TestRequestBodyTooLarge(t *testing.T) {
	srv := setupServer(t)

	bigBody := bytes.Repeat([]byte("a"), 1<<20+128) // 1MB + 128 字节 > 上限
	code, resp, raw := postJSON(t, srv.URL+"/api/v1/randomtest/handleRandomTestHTTPRequestGMLevel2", bigBody)
	if code != http.StatusRequestEntityTooLarge {
		t.Fatalf("期望 HTTP 413, 实际 %d: %s", code, string(raw))
	}
	if ct := resp.Header.Get("Content-Type"); ct == "" {
		t.Fatal("响应缺少 Content-Type")
	}
	if !bytes.Contains(raw, []byte(`"code":4130`)) {
		t.Fatalf("期望业务码 4130, 实际: %s", raw)
	}
}

// TestSyncConflictWhenBusy 并发额度满载时同步端点 → 409 + code=4090。
func TestSyncConflictWhenBusy(t *testing.T) {
	srv := setupServer(t)

	// 提交 GM 异步任务占住唯一并发额度
	createBody, _ := json.Marshal(map[string]interface{}{
		"kind":       "gm2",
		"uploadFile": makeUpload(0x32),
	})
	code, _, raw := postJSON(t, srv.URL+"/api/v1/randomtest/jobs", createBody)
	if code != http.StatusCreated {
		t.Fatalf("创建任务期望 201, 实际 %d: %s", code, raw)
	}
	var created jobResponse
	if err := json.Unmarshal(raw, &created); err != nil || created.Data.JobId == "" {
		t.Fatalf("解析 jobId 失败: %s", raw)
	}
	jobID := created.Data.JobId
	defer func() { _, _ = doRequest(t, http.MethodDelete, srv.URL+"/api/v1/randomtest/jobs/"+jobID) }()

	// 等任务进入 running (额度被占用)
	running := waitForJobStatus(t, srv, jobID, service.JobRunning, 10*time.Second)
	if running.Data.Status != string(service.JobRunning) {
		t.Fatalf("任务应处于 running")
	}

	// 同步端点在额度满载时应返回 409
	body, _ := json.Marshal(map[string]interface{}{"uploadFile": makeUpload(0x33)})
	code, _, raw = postJSON(t, srv.URL+"/api/v1/randomtest/handleRandomTestHTTPRequestGMLevel2", body)
	if code != http.StatusConflict {
		t.Fatalf("满载同步请求期望 409, 实际 %d: %s", code, raw)
	}
	if !bytes.Contains(raw, []byte(`"code":4090`)) {
		t.Fatalf("期望业务码 4090, 实际: %s", raw)
	}

	// 等任务完成释放额度后, 同步端点恢复正常
	waitForJobStatus(t, srv, jobID, service.JobDone, 120*time.Second)
	code, _, raw = postJSON(t, srv.URL+"/api/v1/randomtest/handleRandomTestHTTPRequestGMLevel2", body)
	if code != http.StatusOK {
		t.Fatalf("额度释放后同步请求期望 200, 实际 %d: %s", code, raw)
	}
}

// TestAsyncJobCreateAndCancel 任务创建 → running → 取消 → cancelled; 结果含 25 项 (done 用例)。
func TestAsyncJobCreateAndCancel(t *testing.T) {
	srv := setupServer(t)

	createBody, _ := json.Marshal(map[string]interface{}{
		"kind":       "gm2",
		"uploadFile": makeUpload(0x34),
	})
	code, _, raw := postJSON(t, srv.URL+"/api/v1/randomtest/jobs", createBody)
	if code != http.StatusCreated {
		t.Fatalf("创建任务期望 201, 实际 %d: %s", code, raw)
	}
	var created jobResponse
	if err := json.Unmarshal(raw, &created); err != nil || created.Data.JobId == "" {
		t.Fatalf("解析 jobId 失败: %s", raw)
	}
	jobID := created.Data.JobId

	// 取消: 任务应在下一检测项边界进入 cancelled
	if code, raw := doRequest(t, http.MethodDelete, srv.URL+"/api/v1/randomtest/jobs/"+jobID); code != http.StatusOK {
		t.Fatalf("取消任务期望 200, 实际 %d: %s", code, raw)
	}
	final := waitForJobStatus(t, srv, jobID, service.JobCancelled, 60*time.Second)
	if final.Data.Status != string(service.JobCancelled) {
		t.Fatalf("任务应处于 cancelled, 实际 %s", final.Data.Status)
	}

	// 已取消任务再次取消 → 409
	if code, _ := doRequest(t, http.MethodDelete, srv.URL+"/api/v1/randomtest/jobs/"+jobID); code != http.StatusConflict {
		t.Fatalf("重复取消期望 409, 实际 %d", code)
	}
}

// TestAsyncJobDoneResult 任务正常完成时查询应携带 25 项结果。
func TestAsyncJobDoneResult(t *testing.T) {
	srv := setupServer(t)

	createBody, _ := json.Marshal(map[string]interface{}{
		"kind":       "gm2",
		"uploadFile": makeUpload(0x35),
	})
	code, _, raw := postJSON(t, srv.URL+"/api/v1/randomtest/jobs", createBody)
	if code != http.StatusCreated {
		t.Fatalf("创建任务期望 201, 实际 %d: %s", code, raw)
	}
	var created jobResponse
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}

	final := waitForJobStatus(t, srv, created.Data.JobId, service.JobDone, 120*time.Second)
	if len(final.Data.Result) != 25 {
		t.Fatalf("完成任务结果应为 25 项, 实际 %d", len(final.Data.Result))
	}
}

// TestJobNotFound 不存在的任务查询/取消 → 404 + code=4040。
func TestJobNotFound(t *testing.T) {
	srv := setupServer(t)

	code, raw := doRequest(t, http.MethodGet, srv.URL+"/api/v1/randomtest/jobs/deadbeef")
	if code != http.StatusNotFound || !bytes.Contains(raw, []byte(`"code":4040`)) {
		t.Fatalf("查询不存在任务期望 404/4040, 实际 %d: %s", code, raw)
	}
	code, raw = doRequest(t, http.MethodDelete, srv.URL+"/api/v1/randomtest/jobs/deadbeef")
	if code != http.StatusNotFound {
		t.Fatalf("取消不存在任务期望 404, 实际 %d: %s", code, raw)
	}
}

// TestGracefulShutdown 优雅关闭: 停止接收新连接并等待在途请求完成。
func TestGracefulShutdown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setting.ServerSettings = &setting.ServerSetting{
		Port: ":0", ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second,
		IdleTimeout: 60 * time.Second, MaxRequestBodyMB: 1, MaxConcurrentJobs: 1,
	}
	service.InitJobManager(1, 0, 10*time.Minute)

	srv := &http.Server{Handler: routers.InitRouter()}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()

	// 服务可用性确认 (404 任务查询)
	code, _ := doRequest(t, http.MethodGet, "http://"+ln.Addr().String()+"/api/v1/randomtest/jobs/deadbeef")
	if code != http.StatusNotFound {
		t.Fatalf("关闭前查询期望 404, 实际 %d", code)
	}

	// 优雅关闭应在超时内完成
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("优雅关闭失败: %v", err)
	}
}
