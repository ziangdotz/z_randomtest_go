#### **1. randomtest_stdtest**

- 基于 NIST SP800-22 测试随机性。每组 1000000 bit 数据，共 1000 组

进入目录
```
cd randomtest_stdtest
```
配置 randomtest_stdtest_config.ini
```
TestNum = 1000
TestType = "N"
TestLevel = 2
```
运行
```
go run randomtest_stdtest.go
```

---

- 基于 国密 《GM/T 0005-2021》测试随机性。每组 20000 bit 数据，共 1 组

进入目录
```
cd randomtest_stdtest
```
配置 randomtest_stdtest_config.ini
```
TestNum = 1000
TestType = "G"
TestLevel = 1
```
运行
```
go run randomtest_stdtest.go
```

---

- 基于 国密 《GM/T 0005-2021》测试随机性。每组 1000000 bit 数据，共 1000 组

进入目录
```
cd randomtest_stdtest
```
配置 randomtest_stdtest_config.ini
```
TestNum = 1000
TestType = "G"
TestLevel = 2
```
运行
```
go run randomtest_stdtest.go
```

---

- 基于 国密 《GM/T 0005-2021》测试随机性。每组 100000000 bit 数据，共 1000 组

进入目录
```
cd randomtest_stdtest
```
配置 randomtest_stdtest_config.ini
```
TestNum = 1000
TestType = "G"
TestLevel = 3
```
运行
```
go run randomtest_stdtest.go
```

#### **2. randomtest_stdtest_vue3_wailsv2**

进入目录
```
cd randomtest_stdtest_vue3_wailsv2
```
运行
```
wails dev
```

#### **3. randomtest_web_gin 服务启动**

- 基于 Gin 的随机性测试 Web 服务

进入目录
```
cd randomtest_web_gin
```
配置 ginserver_config.ini
```
[app]
RunMode = "debug" ; "release"

[server]
Port = ":8888"
ReadTimeout = 60
WriteTimeout = 60
IdleTimeout = 120 ; 空闲连接超时 (秒)
MaxRequestBodyMB = 64 ; 请求体大小上限 (MB), 0 = 默认 64
MaxConcurrentJobs = 1 ; 最大并发检测任务数 (CPU 密集), 0 = 默认 1
JobTimeoutSec = 0 ; 异步任务超时 (秒), 0 = 不限
PprofEnable = false ; 是否启用 pprof (仅回环监听, 端口 = Port+1)
```
运行
```
go run main.go
```

#### **4. randomtest_web_gin API 端点**

页面端点:

| 端点 | 说明 |
|---|---|
| `GET /` | 返回内置 Web 控制台首页 (`templates/index.html`) |

同步端点 (阻塞直至检测完成, 响应 `{code, msg, data}`):

| 端点 | 说明 |
|---|---|
| `POST /api/v1/randomtest/handleRandomTestHTTPRequest` | 用户自选检测项组合 |
| `POST /api/v1/randomtest/handleRandomTestHTTPRequestNISTLevel2` | NIST Level2 预设套件 (17 项) |
| `POST /api/v1/randomtest/handleRandomTestHTTPRequestGMLevel2` | GM Level2 预设套件 (25 项) |

异步任务端点 (大文件/长耗时检测推荐, 阶段④新增):

| 端点 | 说明 |
|---|---|
| `POST /api/v1/randomtest/jobs` | 创建任务, 返回 `201 {data: {jobId}}`; body 为检测请求 + `kind` 字段 (`nist2` / `gm2` / `custom`, 缺省 `custom`) |
| `GET /api/v1/randomtest/jobs/{id}` | 查询任务, 返回 `{status, progress, result?}`; `status ∈ queued / running / done / failed / cancelled`; `done` 时携带 `result` |
| `DELETE /api/v1/randomtest/jobs/{id}` | 取消任务; queued/running 可取消, 已结束返回 `409` |

状态码语义: 参数/绑定失败 `400`, 任务不存在 `404`, 并发额度满载 `409` (同步端点与异步任务共用 `MaxConcurrentJobs` 额度), 请求体超限 `413` (上限 `MaxRequestBodyMB`), 内部错误 `500`, 成功 `200/201`。

HTTP 服务支持优雅退出 (Ctrl+C / SIGTERM); `PprofEnable = true` 时在 `127.0.0.1:{Port+1}/debug/pprof/` 提供性能剖析。

