# z_randomtest_go 项目评审与优化建议

> 评审对象：`C:\Users\ziangdotz\Desktop\Projects\z_randomtest_go`（HEAD `2bd6733`，工作区干净）
> 评审方式：全量阅读源码 + **实际编译并在本机运行验证**（Go 1.25.2 / mingw-w64 g++ / cgo 可用）。
> 文中标注 **[已实测]** 的结论均由本机命令复现，证据见文末附录 A；标注 **[已核实非缺陷]** 的项见附录 B。
> 本次评审**未修改任何仓库文件**（临时探针与缓存已全部删除，`git status` 为空）。

---

## 一、项目结构速览

| 模块 | 角色 | 规模 | 备注 |
|---|---|---|---|
| `randomtest_core` | 算法库（唯一真源） | ~2400 行 Go + 4 个 C/C++ 文件 | 19 项统计检测；其中 4 项（非重叠模板、重叠模板、随机游程、随机游程变量）由 cgo 调 C++ |
| `randomtest_stdtest` | CLI 入口 | 195 行 | 读 `randomtest_stdtest_config.ini` |
| `randomtest_stdtest_vue3_wailsv2` | Wails v2 桌面端 | ~840 行 Go + Vue3 | 读同名 ini |
| `randomtest_web_gin` | Gin Web 服务 | ~1300 行 Go | 同步 3 端点 + 异步任务 3 端点，内置单文件控制台 `templates/index.html`(1371 行) |

**值得肯定的设计**（后续改造请保留）：

1. `randomtest_core` 的 `Suite` / `TestItem` / `Outcome` 模型（`suite.go:15-45`）把"检测项定义 + 判定语义"抽到了一处，CLI 与桌面端已改为共用；`SuiteRunner`（`runner.go`）把 reader→worker→merge 流水线收敛，`report` 包把阈值/报表行/ini 清洗收敛 —— 这是一次正确的重构方向。
2. `runner.go:241-250` 用 `sync.Pool` 复用块缓冲并转移所有权，`runner.go:247-250` 按块大小动态降容，是正确的内存策略。
3. `linear_complexity.go:13-39` 的 `lcBuffers` 池化、`discrete_fourier_transform.go:20-53` 的 FFT 实例缓存，都是有价值的微优化。
4. `templates.go` 用 `go:embed` 嵌入 15 个模板文件（1MB 级），消除了运行时路径依赖 —— **这个做法应当推广到 `templates/index.html` 和两个 `.ini`**（见 P1-2）。
5. 异步任务已有排队/取消/超时/TTL 的完整状态机（`job_manager.go:24-33`），`main.go:80-98` 有优雅退出。

**核心问题一句话总结**：库层用 `panic` 充当错误处理、Web 层把用户参数原样转发给库层、异步任务在无 `recover` 的 goroutine 里跑 —— 三者叠加，让**一个匿名 HTTP 请求就能终止整个服务进程**（P0-1，已实测）。

---

## 二、优先级总览

| 级别 | 编号 | 问题 | 证据 |
|---|---|---|---|
| **P0** | P0-1 | 一个未授权 HTTP 请求即可打挂 Web 服务（参数无校验 → panic → 进程退出） | **[已实测]** |
| **P0** | P0-2 | `Alpha`/`AlphaT` 全局变量存在数据竞争，判定结果不确定 | **[已实测 -race]** |
| **P0** | P0-3 | Web 端 7 个 API 测试当前**全部**跑不起来 | **[已实测]** |
| **P0** | P0-4 | C++ 层 `cycle` 数组越界写、`n` 不受缓冲区长度约束、`4n` 字节分配 | 源码核实 |
| **P0** | P0-5 | 桌面端与 CLI 对同一输入给出**不同样本数/不同结论** | 源码核实 |
| P1 | P1-1 | Web 二进制离开源码目录就**不能**提供控制台页与配置 | 源码核实 |
| P1 | P1-2 | 四个模块**不能独立构建**（`GOWORK=off` 失败） | **[已实测]** |
| P1 | P1-3 | 桌面端在干净检出上**无法编译**（embed 未跟踪的 `frontend/dist`） | **[已实测]** |
| P1 | P1-4 | 三入口约 400 行重复逻辑未下沉 core | 源码核实 |
| P1 | P1-5 | 回归基线自录、无外部基准；CLI 模块零测试 | 源码核实 |
| P1 | P1-6 | 无 CI、无 LICENSE（却内联了 NIST STS / cephes 源码） | 源码核实 |
| P2 | P2-1 | Web 单请求内存放大到 ~1.2 GB（base64 + `[]bool` 8 倍展开） | 源码核算 |
| P2 | P2-2 | 每个检测项重复做字节→比特展开；C 桥再展开一次 | 源码核实 |
| P2 | P2-3 | C 层可优化的循环/重复计算 | 源码核实 |
| P2 | P2-4 | DFT 常数 `3.8` 无出处（实测表现良好，**不要按教科书"修正"**） | **[已实测]** |
| P3 | — | 死代码/死目录/死资源清理清单 | 源码核实 |

---

## 三、P0 —— 必须优先修复

### P0-1　一个未授权 HTTP 请求即可打挂 Web 服务　**[已实测]**

**复现（真实路由 + 真实 JSON body，1 KB 上传）**：

```
POST /api/v1/randomtest/jobs
{"uploadFile":"<1KB base64>","rankTest":{"enable":true,"r":64,"c":64}}

→ HTTP 201 Created  {"code":200,"msg":"操作成功","data":{"jobId":"7cc928963a6867c7"}}
→ panic: runtime error: index out of range [32] with length 32
     randomtest_core.RankProto(...)            rank.go:98
     randomtest_core.RankTestBytes(...)        rank.go:131
     service.(*RandomTestHTTPRequestStruct).HandleRandomTestHTTPRequestService(...)  randomtest.go:282
     service.executeJob(...)                   job_manager.go:249
     service.(*JobManager).run(...)            job_manager.go:220
     created by service.(*JobManager).Submit  job_manager.go:163
→ 进程退出
```

**根因是三处独立缺陷的叠加**：

1. **库层用 `panic` 当错误处理，且不做参数校验。** `rank.go:79-82` 把矩阵硬编码为 32×32，`rank.go:94-99` 却按用户给的 `R/C` 索引：
   ```go
   matrix := make([][]int, 32)          // rank.go:79  固定 32
   for i := 0; i < 32; i++ { matrix[i] = make([]int, 32) }
   ...
   for j := 0; j < R; j++ { for k := 0; k < C; k++ { ...; matrix[j][k] = ... } }  // rank.go:92-99
   ```
   实测（附录 A.2）确认下列参数会 panic 而非返回错误：

   | 检测项 | 参数 | panic |
   |---|---|---|
   | `BlockFrequencyTest` | `m=0` | integer divide by zero |
   | `RankTest` | `r=0,c=0` / `r=64,c=64` / `r=32,c=64` | divide by zero / index out of range |
   | `PokerTest` | `m=0` / `m=3` / `m=1000` | divide by zero / index out of range |
   | `SerialTest` | `m=0` / `m=1` | index out of range |
   | `ApproximateEntropyTest` | `m=1000` | index out of range（`1<<1000` 巨量分配） |
   | `LinearComplexityTest` | `m=0` | integer divide by zero |
   | `UniversalTest` | 输入短于 `1280*7` 比特 | slice bounds out of range |

   另有 5 处显式 `panic`：`discrete_fourier_transform.go:59,84,112`、`fft.go:67,96`、`block_frequency.go:26`。
   **全仓库 `grep -n "recover()"` 结果为空**。

2. **HTTP 层把用户参数原样转发。** `v1.go:16-59` 的 `normalizeRandomTestRequest` 只填 nil 默认值，**不做任何范围校验**；`randomtest.go:282` 直接把 `RankTest.R/C` 传给检测函数。`index.html` 也没有前端约束。

3. **异步任务在无 `recover` 的 goroutine 中执行。** `job_manager.go:163` 起 `go m.run(...)`，`job_manager.go:220` → `executeJob` → `job_manager.go:249`。`gin.Recovery()`（`routers.go:34`）只能救**请求 goroutine**，救不了这个后台 goroutine，因此是**进程级宕机**。
   注：同步端点（`/handleRandomTest*`）因为跑在请求 goroutine 里会被 `Recovery` 兜成 500，所以问题只暴露在异步端点 —— 这也解释了为什么它至今没被发现。

**修复建议（三层都要做，缺一不可）**：

- **库层（根治）**：所有导出检测函数在入口做参数校验并**返回 error**，禁止 panic。
  - 建议引入统一的参数契约：`type Params struct{...}` + `func (Params) Validate() error`，或给每个 `*Bytes` 函数加前置校验，例如 `RankTestBytes` 要求 `R,C ∈ [1,32] && R*C <= len(data)*8`。
  - 把所有 `panic(...)` 改为返回 `Outcome{Err: ...}`；`Outcome.Err` 通道**已经存在**（`suite.go:27-28`），RoadRunner 也已经在累计 `ErrCount`（`runner.go:88-91`），只是生产者没用它。
  - 这是最小改动、收益最大的一步：`runner.go:214-222` 与 `randomtest.go:143-152` 都已具备承载 error 的结构。
- **桥接层（纵深防御）**：`item.Run(data)` 内部包一层 `recover` 并转成 `Outcome{Err:...}`，这样即使未来新增检测项忘了校验，也只是该项失败而不是进程崩溃。建议放在 `runner.go:214-222` 的 `runSuite` 和 `randomtest.go:147` 两处。
- **HTTP 层（快速失败）**：在 `v1.go` 的 `normalizeRandomTestRequest` 之后加 `validateRandomTestRequest(*req) error`，对每个 `M/N/R/C/K/D` 做上下界校验并返回 400。参数边界可直接由 core 导出（例如 `randomtest_core.Limits{RankMin, RankMax, TemplateMinM, TemplateMaxM}`）。

**顺带修复的 API 语义缺陷**：`executeJob`（`job_manager.go:242-251`）的 `custom` 分支调用的是**无 ctx 版本** `HandleRandomTestHTTPRequestService()`，因此：
- `DELETE /jobs/{id}` 对 `custom` 任务返回 200，但任务**不会停止**，且继续独占唯一的并发额度（`MaxConcurrentJobs=1`）；
- 任务结束后还会把状态覆盖成 `done`，与"已取消"的语义矛盾。
  建议给自定义路径加 `ctx` 并在每个检测项之间检查 `ctx.Err()`（照搬 `runPresetSuiteServiceCtx`，`randomtest.go:141-155` 已有现成写法）。

---

### P0-2　`Alpha` 全局变量数据竞争　**[已实测 -race]**

`randomtest_core/common.go:14-18` 用包级变量保存显著性水平，`SetAlphaValue`（`common.go:25-38`）加锁写，但**所有读者都没有加锁**：

```
$ go test -race -run TestProbeAlphaRace .
WARNING: DATA RACE
Read at 0x0001402b92c8 by goroutine 10:
  randomtest_core.PValuePassed()   .../randomtest_core/runner.go:130
Previous write at 0x0001402b92c8 by goroutine 9:
  randomtest_core.SetAlphaValue()  .../randomtest_core/common.go:33
```

无锁读者：`runner.go:129-136`（`PValuePassed`/`QValuePassed`，**被工作协程并发调用**）、`utils.go:229,238`（`isPValueOk`/`isQValueOk`，被每个检测函数调用）、`report/report.go:29-32,36-48`（`PassCountRange`/`CountInRange`）。

**危害**：判定结论不确定（同一次运行内不同 worker 可能用不同 alpha），且 `go test -race` 一旦覆盖到就会失败。桌面端每次点"开始测试"都会改全局（`app.go:311`），而 `SetAlphaValue` 在**校验之前**调用（校验在 `app.go:315-324`），所以一次被拒绝的启动（等级越界、空目录导致 `sampleCount=0`）也会永久污染后续所有判定。

**修复建议（按推荐度排序）**：

1. **彻底去掉全局**：把 alpha 变成显式参数。`RunnerOptions` 增加 `Alpha/AlphaT float64` 字段（`runner.go:24-37`），`report` 的判定函数改为 `CountInRange(pass, n, alpha)`、`BuildRows(testNum, stats, gm, alpha)`，`Outcome` 判定不再读全局。全局变量可保留为"默认值提供者"但只读。
2. 过渡方案：`atomic.Value` / `atomic.Uint64`（`math.Float64bits`）承载，读者无锁但**原子**；这能消除 race，但仍无法表达"同一次运行内多个 alpha"。
3. 若坚持用全局：至少把 `SetAlphaValue` 调用挪到校验之后（`app.go:311` → `:324` 之后），并给所有读者加 `RLock`（性能敏感路径不推荐）。

---

### P0-3　Web 端 7 个 API 测试当前全部跑不起来　**[已实测]**

```
$ cd randomtest_web_gin && go test ./test/ -count=1
--- FAIL: TestSyncGMLevel2Flow (0.00s)
panic: html/template: pattern matches no files: `templates/*` [recovered, repanicked]
  html/template.Must(...)
  github.com/gin-gonic/gin.(*Engine).LoadHTMLGlob(...)  gin.go:275
  randomtest_web_gin/routers.InitRouter()               routers.go:49
  randomtest_web_gin/test.setupServer(...)              api_test.go:59
```

`routers.go:49` 的 `LoadHTMLGlob("templates/*")` 内部是 `template.Must(template.ParseGlob(...))`，**glob 不匹配就 panic**；`go test ./test` 的工作目录是 `randomtest_web_gin/test`，所以 7 个 API 测试在 `setupServer` 里就死了 —— 这个模块目前**实际上处于零测试覆盖状态**。

**修复建议**（同时解决 P1-1）：把模板改为 embed，用 `LoadHTMLFS`：

```go
//go:embed templates/index.html
var templatesFS embed.FS
sub, _ := fs.Sub(templatesFS, "templates")
xRouter.LoadHTMLFS(http.FS(sub), "index.html")
```

一步同时修好"测试可运行"和"二进制离开源码目录不能服务首页"两个问题。

---

### P0-4　C++ 层内存安全问题

以下三处**已逐行核实**（含可达性推演），另附 cgo 桥接的安全隐患。

**(a) `cycle` 数组越界写 —— 堆溢出（高危）**

```c
cycle = (int *)calloc(MAX(1000, n / 100), sizeof(int));   // func_nist.cpp:352   容量 cap，合法下标 0..cap-1
...
J++;
if (J > MAX(1000, n / 100)) { ...return 1; }              // func_nist.cpp:376   条件应 >=
cycle[J] = i;                                             // func_nist.cpp:386   J == cap 时越界写 4 字节
...
cycle[J] = n;                                             // func_nist.cpp:393   完全没有边界检查
```
`J == cap` 必然先于 `J == cap+1` 出现，所以越界写**可达**。触发条件：`S_k[i] == 0` 的零穿越次数达到 cap。`n = 1000000`（NIST L2 块）时 cap = 10000，第 10000 次零穿越发生在 `i = 19999`；**交替序列 `0101...`** 每次奇偶都归零，必然命中。随后 `free(cycle)` 可能因堆元数据被破坏而崩溃。这处缺陷继承自 NIST STS 参考实现（`randomExcursions.c` 同样如此），但**不能因此不修** —— 随机性检测工具的输入恰恰可能来自被测的异常发生器。

**修复**：`calloc(MAX(1000, n/100) + 2, sizeof(int))`，并把守卫改为 `if (J >= cap) { 释放; return 1; }`，同时给 `:393` 加写前检查。

**(b) `n` 不受缓冲区长度约束 —— 越界读（高危）**

C 侧入口签名只有 `n`，没有 `buf_len`（`func_nist.h:28-33`），循环按 `n` 访问 `epsilon[i]`；而 Go 桥接只转发位长：

```go
cData, free := goBytes2CUCharArray(data)                       // 实际只分配 len(data)*8 字节
code := C.NonOverlappingTemplateMatchings_NIST(cData, C.int(n), ...)   // func_nist.go:105（:129/:148/:172 同理）
```
`n` 在 HTTP 上由客户端提供（`randomtest.go:303-308, 321-326, 361-362, 375-376`），而控制台表单把它默认成 `1000000`（`index.html:597,608,632,641`，`min="1"` 无上界），`v1.go:16-59` 的归一化**又不校验 `n` 与上传数据长度的关系**。因此"上传 1 KB + `n=1000000`"会让 C 侧越界读约 1 MB 内存 → 段错误或返回垃圾 p 值。

**修复**：C 侧增加 `int buf_len` 参数，入口校验 `n <= buf_len`，否则返回错误码 1；Go 桥接同时校验 `n <= len(data)*8`；HTTP 层再校验一次（纵深防御）。

**(c) `S_k = calloc(n, sizeof(int))` —— 每个输入比特 4 字节（中危）**

`func_nist.cpp:351`、`:497`。`n = 1e9` 时单次分配 4 GB，且数组会被完整写入，属于与 (b) 无关的独立 OOM/DoS 面。**修复**：在 C 侧对 `n` 设上界（返回错误码 1），或把 `S_k` 换成更小的类型 / 流式计算。

**(d) 其他并发与健壮性问题（中危）**

- `cephes.cpp:13` 的 `int sgngam` 是可变全局，`cephes_lgam` 每次调用都会写（`:134,148,150,184,188` 写、`:200` 读），而 `NonOverlapping` 会从多个 worker 协程并发调用它 → C 侧数据竞争（`go test -race` **看不见**）。建议改 `_Thread_local` 或改为出参。
- `func_nist.cpp:11-16` 的 `g_nist_debug_enabled` 是非原子 int，`SetDebug`（`common.go:57-60`）是公开 API 且运行时会被调用，而 worker 并发读 → 竞争。建议 `_Atomic int` 或仅在 init 期设置一次；调试输出也应走 stderr/回调，而不是从多线程写 fd 1（服务进程的 stdout）。
- `func_nist.cpp:242-247, 265-266, 308-317`：`OverlappingTemplateMatchings_NIST` 未校验 `m <= M(1032)`；`m > M` 会让 `log(eta)` 产生 NaN，而 `isNegative`/`isGreaterThanOne` 对 NaN 均为 false，于是**带成功码返回 NaN p 值**。同理 `:312, :462, :543`。建议入口校验参数并加 `isfinite` 守卫。
- `func_nist.go:45-47, 88`：`C.CBytes` 的返回值未判空，失败指针会被永久缓存（模板场景），`NonOverlapping` 也不判空 `epsilon`（对比 `:242/:339/:485` 有判空）。
- `func_nist.cpp:397-400, 524-527`：用 `p = 0.0` 表示"本次不适用"，与"真的算出 p=0"无法区分；Go 侧据此用 `p > 0` 判定适用性（`suite.go:64-70`、`report.go:150-154`），会把一个极端非随机序列（p 下溢为 0）静默排除在统计之外，而不是计入失败。建议用独立的状态数组/计数表达适用性。

---

### P0-5　桌面端与 CLI 对同一输入给出不同结论

| # | 问题 | 位置 |
|---|---|---|
| a | **桌面端忽略 ini 的 `TestNum`**：前端用"文件大小 ÷ 块大小"覆盖样本数并回传 | `RandomtestMain.vue:213`（覆盖）→ `:282`（回传）→ `app.go:266-270, 343`；CLI 用 `TestNum`：`randomtest_stdtest.go:26, 121` |
| b | **标准类型词表不一致**：桌面端接受 `NIST/GM/GM/T/GMT`，CLI/桌面 runner 只认 `"N"`/`"G"` | `app.go:119-128` vs `randomtest_stdtest.go:46-60`、`randomtest_stdtest_vue3_wailsv2.go:306-320` |
| c | **alpha 只在桌面端可改**：CLI/Web 没有 alpha 入口，却共享同一个进程全局（见 P0-2） | `app.go:311`；CLI 全文无 `SetAlphaValue` |
| d | **同一次桌面端运行内自相矛盾**：结果表只按 P 判"通过"，失败清单按 P+Q 判 | `randomtest_stdtest_vue3_wailsv2.go:262-266` vs `report/report.go:108-125, 196-207` |

实测数据佐证 a：仓库自带 `data/data_0.bin` 为 125,000,000 字节 = 1000 个 NIST 块；桌面端 ini 写 `TestNum=1000`，CLI ini 写 `TestNum=2` —— 同一份数据，桌面端测 1000 个样本、CLI 测 2 个，通过计数与阈值判定自然不同。

**修复建议**：
- 把"样本数如何确定"变成 core 的显式规则（例如 `core.ResolveSampleCount(isFile, path, testNum)`），三入口共用；桌面端如果确实想"测整个文件"，应把它作为**显式选项**并在 UI 上显示实际样本数，而不是静默覆盖。
- `normalizeStandard` 下沉到 core，让三入口共享同一词表（并统一错误文案）。
- `report` 的判定 API 改为显式接收 alpha（见 P0-2 修复建议 1）。
- 桌面端结果表把 Q 纳入判定，或增加独立的 Q 列，与失败清单口径一致。

---

## 四、P1 —— 架构与工程化

### P1-1　Web 产物离开源码目录即不可用

- `routers.go:49` 用相对路径 `"templates/*"` 加载页面 → `release/` 里的 `z_randomtest_web_windows_amd64.exe` 必须放在含 `templates/` 的目录、且 CWD 正确才能服务 `/`。
- `setting.go:43` 硬编码 `ini.Load("./ginserver_config.ini")`，`randomtest_stdtest.go:170` 同样；配置与 CWD 强绑定，且 `setup()` 失败时 `log.Fatalf` 只留一行日志。

**建议**：模板 embed（见 P0-3）；配置路径改为"`-config` flag > 环境变量 > 可执行文件同目录 > CWD"的查找顺序；配置文件缺失时给出明确提示而不是直接退出。

### P1-2　四个模块不能独立构建　**[已实测]**

```
$ cd randomtest_web_gin && GOWORK=off go build ./...
service\randomtest.go:9:2: package randomtest_core is not in std (...\src\randomtest_core)
service\randomtest.go:10:2: package randomtest_core/report is not in std
```

`randomtest_web_gin/go.mod`（以及另外两个消费方）都**没有 `require randomtest_core`**，完全依赖根目录 `go.work`。后果：CI 无法按模块构建、`go mod tidy` 行为异常、`go install` 不可用、任何 `GOWORK=off` 的工具链集成都会失败。

**建议**：给每个消费模块补

```
require randomtest_core v0.0.0
replace randomtest_core => ../randomtest_core
```

并保留 `go.work` 用于本地开发；CI 中显式用 `GOWORK=off` 构建以证明模块独立性。

### P1-3　桌面端在干净检出上无法编译　**[已实测]**

`randomtest_stdtest_vue3_wailsv2/main.go:14` 是 `//go:embed all:frontend/dist`，而 `frontend/dist` 被 `.gitignore:3` 忽略且未跟踪（`git ls-files` 无 dist 条目）：

```
pattern all:frontend/dist: no matching files found   → go build / go test / go vet 全部失败
```

**建议**：二者择一 —— (a) 提交 `dist`（约 123 KB，代价小、可复现）；(b) 在 `frontend/dist` 放占位文件并在模块 Makefile 中强制先 `npm ci && npm run build`。推荐 (a) + 发布前校验 dist 比 src 新。

### P1-4　三入口重复逻辑未下沉 core（约 400 行）

| 重复内容 | 位置 | 收敛建议 |
|---|---|---|
| `suiteFor(testType, level)` 几乎逐字复制 | `randomtest_stdtest.go:45-61`、`randomtest_stdtest_vue3_wailsv2.go:305-321` | 移到 core：`core.SuiteFor(...)` |
| 等级→比特/字节表被重新推导 | `app.go:130-178`（硬编码 `1000000/125000`） vs `suite.go:44,78,148,200,265` | 由 `Suite.BlockSizeBytes` 派生，或 core 导出 `BlockBytes(std, level)` |
| 进度回调 + 分母捕获 + 明细收集 + 错误包装 + 报表打印（约 85 行） | `randomtest_stdtest.go:79-145` vs `randomtest_stdtest_vue3_wailsv2.go:341-426` | core 提供 `RunAndCollect(...) (stats, details, denominator, err)`，各入口只留输出 sink |
| 3σ 最小通过数公式存在 3 份（含 JS 一份） | `report/report.go:18-26`、`app.go:189-195`、`RandomtestMain.vue:160-169` | 删掉 JS 版，统一用 `AnalyzeInputPath` 返回值 |
| **Web 自定义检测的 430 行 if 链** | `randomtest_web_gin/service/randomtest.go:168-457` | 见下 |

**Web 自定义路径是本项目最大的重复**：它把 `Suite` 已经抽象好的东西又用 18 个 `if` 手写了一遍，导致 (1) 参数校验无处安放（P0-1），(2) 检测项顺序与标准套件不一致（`PokerTest`/`RunsDistributionTest`/`BinaryDerivativeTest`/`AutocorrelationTest` 被追加到末尾而不是按规范顺序），(3) 与预设路径判定口径分散。
**建议**：由请求动态构造 `*randomtest_core.Suite`（每个检测项一个 `TestItem`，参数闭包捕获），然后复用 `runPresetSuiteServiceCtx`。这一处改造可望删掉 250+ 行，并让 P0-1 的校验点收敛为**唯一一处**。

### P1-5　测试与回归基线

**好消息**：19 个统计函数都有 golden 数据点（`golden_test.go:54-141`），且 `report` 包有单测、runner 有并发/取消测试。

**问题**：

1. **基线是自录的**：`regression_test.go:52-73` 用被测代码生成基线，`make update_core_golden`（`Makefile:48-51`）一键覆盖 `testdata/baseline.json`。它能捕捉"内部漂移"，但**没有外部基准**，因此系统性错误的 p 值会被永久固化。
   → 建议至少加一个外部已知答案向量（固定字节序列的公开 p 值，或 NIST STS 参考输出），把快照降级为"漂移哨兵"。
2. **容差是绝对 1e-9**（`regression_test.go:28`）：对 p≈1 的项在 MinGW/Windows 与 musl/Linux 之间可能因 libm/FMA 差异而误报；对极小 p 又毫无意义。→ 改为 `|Δ| <= 1e-9 + 1e-9*|want|`，并在基线里记录 Go/编译器版本与 commit。
3. **参数覆盖有盲区**：只覆盖了 NIST L2 的参数集；`SuiteGM(2)`/`SuiteGM(3)` **从未在任何测试中构造过**，而它们是 CLI/Web 的预设套件。
   → 让 golden 表**由 suite 派生**（`for _, s := range suites { for _, it := range s.Items }`），这样"出厂即被钉住"。
4. **`randomtest_stdtest`（CLI）模块零测试**；`Makefile` 也没有 web/CLI 的测试目标。
5. `golden_test.go:171-181` 在 `len(pValues)==0` 时返回"适用且通过"，即"检测项退化为不返回值"会被判成功。
6. `report_test.go:52-57` 的断言是**同义反复**（`PassCountRange` 就是 `PassCountRangeWithAlpha` 的转发，且测试从不改 alpha）。
7. `bench_test.go` 没有 make 目标、没有 `-benchmem`，实际从未被执行。

**建议**：新增 `make test_all`（四模块 `go test ./... -count=1`）、`make test_race`（四模块 `-race`）、`make bench_core -benchmem`；CI 中跑全套并上报覆盖率。

### P1-6　构建、CI、许可证

| # | 问题 | 位置 | 建议 |
|---|---|---|---|
| a | **无 CI**（`.github/workflows` 不存在） | — | 加 `ci.yml`：`gofmt -l` / `go vet` / `go build` / `go test` / `go test -race` × 4 模块 × (ubuntu, windows)；外加 musl 交叉编译作业 |
| b | **无 LICENSE**，却内联了 NIST STS 与 cephes 源码 | `func_nist.cpp/.h`、`cephes.cpp/.h` | 加 `LICENSE` + `THIRD_PARTY_NOTICES`（NIST STS 为公有领域、cephes 有其自身条款，需如实声明） |
| c | 构建 recipe 是 **cmd.exe 专用语法** | `Makefile:64-66, 85-87` | 改为 shell 无关的单行 `CGO_ENABLED=1 GOOS=windows ... go build ...` |
| d | **硬编码本机绝对工具链路径**（`x86_64-linux-musl-gcc.exe`、`zig.exe`），且 `ZIG_PATH` 从未被引用 | `Makefile:14-16` | `MUSL_CC ?= x86_64-linux-musl-gcc`，从 PATH 解析并允许覆盖；删除死变量 |
| e | `clean` 被列为**每个 build 目标的前置依赖**，会删掉全部 5 个产物（交叉构建时互相清除）；`go clean` 在工作区根目录是空操作；`clean` 只能删 5 个硬编码文件名，而 `release/` 里实际存在的 `z_randomtest_web_*.exe`、`*_upx.exe` 从没被清过 | `Makefile:61,69,82,90,113-119` | 从 build 前置依赖中移除 `clean`；`clean` 改为清理 `release/*` 与 wails `build/bin` |
| f | Windows 构建缺少 `-trimpath`（Linux 有）；**所有目标都没有版本戳**，`git tag` 为空，产物无法对应 commit | `Makefile:64-67, 85-88` | 统一 `-trimpath` + `-X main.version/-X main.commit/-X main.buildTime`，启动时打印 |
| g | `.PHONY` 漏掉了 5 个 `build_*`、`build_all`、`run_*` | `Makefile:36` | 补全 |
| h | `RELEASE_STDTEST_DARWIN_ARM64` 及对应 clean 行是死代码 | `Makefile:32,118` | 删除 |
| i | `release/` 内容与 Makefile 产物名不一致、无校验和、`release/randomtest_stdtest_config.ini` 里**残留个人目录路径**；`release/README.md` 把 Web 服务的启动命令写成了 stdtest 的 | `release/` | 删除陈旧目录，改用 `make release` 装配产物 + 配置 + `SHA256SUMS`，剔除个人路径 |
| j | `frontend/wailsjs/**` 与 `frontend/src/wailsjs/**` **重复且已经漂移**（前者只有 `NewWindow`，后者有 8 个方法；`wails.json:5` 指定 `wailsjsdir: ./frontend/src`） | `randomtest_stdtest_vue3_wailsv2/frontend/` | `git rm -r --cached frontend/wailsjs`，并把 `frontend/wailsjs/`、`package.json.md5` 加进 `.gitignore` |
| k | `npm install` 而非 `npm ci`（已提交 lockfile 却不用）；Wails CLI 版本未固定；UPX 按机器条件启用导致产物大小不一致 | wails 模块 `Makefile:5-17,33,41` | 改 `npm ci`，固定 Wails CLI 版本，UPX 改为显式可选 |
| l | 无 `.golangci.yml` / dependabot / `.editorconfig` / CONTRIBUTING | — | 补 lint 配置（至少 `errcheck, govet, staticcheck, ineffassign`）与依赖更新自动化 |

### P1-7　可观测性

- 全局只有 std `log` + 手写 `[INFO]/[Error]` 前缀（`main.go:54,85,92,96,98`）与 gin 默认文本日志（`routers.go:33`）：无结构化字段、无请求 ID、无 per-job 耗时。
- 无 `/healthz`、`/readyz`、`/metrics`；唯一的运维端点是需要手动开启的回环 pprof（`main.go:37-59`）。
- `Job.Progress` 是 string 且只在结束时被赋成 `"completed"`（`job_manager.go:236`），运行期间**没有进度**；前端只能轮询（`index.html:1268`）。而 `SuiteRunner` 已经支持 `Progress` 回调（`runner.go:31`）—— 把回调接进 job 即可。
- 无版本号输出（见 P1-6f）。
- **建议**：`log/slog` JSON handler + 级别来自 ini；`/healthz` + `/metrics`（任务数、队列长度、各检测项耗时、错误计数）；异步任务加 `?progress=1` 或 SSE 推送进度；入库每次运行的摘要（耗时、块/秒、ErrCount）。

---

## 五、P2 —— 性能优化建议（含量化）

### P2-1　Web 单请求内存放大到 ~1.2 GB

链路：`MaxRequestBodyMB = 64`（`ginserver_config.ini`）→ gin 缓冲 64 MB JSON → base64 解码得 ~48 MB 原始数据 → 每个检测项调用 `*Bytes` 时 `Byte2BitArr`（`utils.go:184-199`）展开成 **8 倍**的 `[]bool` = **384 MB**；若走 cgo 检测项，`goBytes2CUCharArray`（`func_nist.go:84-90`）再分配 384 MB 的 C 缓冲 + 384 MB 临时 Go 切片。预设套件 17/25 项顺序执行，每项都重新展开一次。

**建议（按收益排序）**：
1. **改用二进制上传**：`Content-Type: application/octet-stream` 或 multipart，省掉 base64 的 33% 膨胀和解码时的双份驻留；前端用 `FileReader.readAsArrayBuffer` 而不是 `readAsDataURL`（`index.html:925-936` 当前用后者）。
2. **比特表示只做一次**：把展开结果提升到请求/任务级别，`TestItem` 签名考虑增加 `RunBits(bits []bool)`，或者统一传 packed bytes 并在 C 侧展开（能够顺便省掉 P2-2）。
3. **换用位压缩表示**：`[]bool`（1 字节/位）改为 `[]uint64`（1 位/位），1 Mbit 从 128 KB 降到 16 KB，内存与缓存命中率提升 64 倍。这对 CLI 的 1000 块 × 100 万比特场景同样是数量级收益。
4. 降低默认 `MaxRequestBodyMB`（64 → 8~16），并把"单请求最大比特数"作为显式配置项。

### P2-2　重复的字节→比特展开

`serial.go:72`、`poker.go:41`、`rank.go:131`、`frequency.go:39` 等每个 `*Bytes` 包装函数都独立调用 `Byte2BitArr`。在一个 125 KB 块上跑 19 项检测，就是 19 次 1 MB 的分配与填充。
**建议**：`SuiteRunner.runSuite`（`runner.go:214-222`）把展开结果算一次并向下传递；同时 `func_nist.go:85-88` 里 Go 侧展开一次、`C.CBytes` 再拷一次，可以合并为"直接分配 C 缓冲并原地展开"。

### P2-3　C 层可优化点

- `func_nist.cpp:182`：`pow(varWj, 0.5)` 在 8 块循环内、每个模板重复计算 → 提到循环外。
- `func_nist.cpp:531-541`：`RandomExcursionsVariant` 对 `S_k` 重扫 18 次（18n）→ 一趟扫描 + 18 个计数器。
- `func_nist.go:16-18,45-47`：模板 C 副本常驻不释放（<1MB，已正确加锁与 `sync.Once`，属可接受的设计，但建议补一个 teardown API 供测试使用）。
- `func_nist.cpp:113-124, 152-155` 计算了从未被使用的 `nu[6]`/`pi[6]`（NIST 原实现也如此，非正确性问题，可留可删）。

### P2-4　DFT 检测的常数 `3.8`　**[已实测：表现良好，请勿"修正"]**

`discrete_fourier_transform.go:127`：

```go
V := (float64(N_1) - N_0) / math.Sqrt(0.95*0.05*float64(2.0*n)/3.8)
P := math.Erfc(math.Abs(V))
```

这个 `3.8` 没有注释说明出处，看起来像是把 `√2` 折进分母时写错了。为判断它到底是 bug 还是"有意标定"，我用 1500 个真随机 1 Mbit 块实测了三套公式的拒绝率（名义值 1% / 5%）：

| 公式 | α=0.01 拒绝率 | α=0.05 拒绝率 |
|---|---|---|
| **本仓库实现** | **1.47%** | **5.80%** |
| NIST STS 参考式 `d=(N1-N0)/√(2·0.95·0.05·n)`, `p=erfc(|d|/√2)` | 0.00% | 0.00% |
| 二项理论式 `z=(N1-N0)/√(0.95·0.05·n/2)`, `p=erfc(|z|/√2)` | 0.00% | 1.00% |

**结论：现实现接近名义水平（略偏保守超标），另外两个"教科书公式"反而严重失效（几乎不拒绝）。因此这不是 bug，千万不要按教科书公式去"修正"它。**

**但仍建议改进**：
1. 在 `discrete_fourier_transform.go:115-128` 加注释说明 `3.8` 的来源与标定依据（否则下一位维护者很可能把它"修"坏）。
2. 把上面这个标定实验固化成测试（例如 200 块、容忍拒绝率 0.2%~4%），这样任何人改动该常数都会立刻被拦下。
3. 记录 DFT 检测（以及 NIST 原版）已知的 p 值非均匀性背景，供使用者判读时参考。

---

## 六、P3 —— 清理清单（低风险、可顺手做）

| 项 | 位置 | 说明 |
|---|---|---|
| 未使用的文件读取工具，且风格为 `panic` | `randomtest_web_gin/pkg/utils/file.go` 全文 | 全仓库无引用（仅自引用）；`...Error` 版本号称返回 error 却只会 panic |
| 空包文件 | `randomtest_web_gin/service/service.go` | 仅一行 `package service`，无任何声明 |
| 死字段 | `randomtest_core/runner.go:45` `QCounts` | 声明后注释为"从不递增"，全仓库仅此一处出现（`grep QCounts`） |
| 空目录残留 | `randomtest_web_gin/8ckup`、`middlewares/`、`models/`、根 `.vscode/` | 均为 0 条目；`8ckup` 是 `backup` 的拼写错误，且被 `.gitignore:1` 专门忽略 |
| 3.65 MB 死资源 | `randomtest_core/templates/template1{7,8,9}.Z`、`template2{0,1}.Z`、`templates/dataInfo` | `templates.go:14` 明确说明这些 `.Z` 是"历史遗留不可用资源"，`//go:embed`（`:17`）也未包含它们 |
| 陈旧基线与产物 | `release/` 整目录 | 见 P1-6i |
| `.gitignore` 缺口 | 根 `.gitignore` | 缺 `.vscode/`、`.idea/`、`node_modules/`、`*.test`、`*.log`、`.DS_Store`、`Thumbs.db` |
| `.gitattributes` 缺口 | `.gitattributes:5-6` | 只把 `*.bin`/`*.exe` 声明为 binary；建议补 `*.Z`、`*.png`、`*.ico`（避免被 `eol=lf` 规范化破坏） |
| 不可移植的配置 | `randomtest_stdtest/randomtest_stdtest_config.ini:3,5` | 指向 `D:\Projects\z_randomtest_go\data\...`，在本机已不存在（`Test-Path` = False），`make run_stdtest` 开箱即失败 |
| 死 Makefile 变量/目标 | `Makefile:16` `ZIG_PATH`、`:32,118` DARWIN | 见 P1-6d/h |
| 桌面端死代码 | `app.go:42 EnableSecondaryDetection`、`app.go:464-489` 注释掉的 `FileLoader`、`main.go:17 onExit`、`main.go:37` 注释与 `DisableResize:false` 矛盾 | — |

---

## 七、建议落地路线图

按"风险 × 改动量"排序，建议拆成 6 个 PR：

**PR-1（阻塞性修复，最高优先）**
1. 库层参数校验 + 消除 `panic()`，改用 `Outcome.Err`（P0-1 第 1 层）
2. `runSuite` / 预设运行路径加 `recover` 兜底（P0-1 第 2 层）
3. `v1.go` 加 `validateRandomTestRequest`（P0-1 第 3 层）
4. `recover`/`defer` 覆盖 `JobManager.run`，保证任何检测项异常都只让任务 `failed`
5. 回归测试：把附录 A.2 那张参数表变成 table-driven 测试（断言返回 error 而非 panic）

**PR-2（并发正确性）**
1. alpha 去全局化，改为 `RunnerOptions` / `report` API 显式参数（P0-2）
2. 桌面端 `SetAlphaValue` 调用移到校验之后（`app.go:311`）
3. 消除桌面端 `RunStandardRandomTestEg` 全局写（`app.go:333-343`）与 `a.ctx` 竞争（`app.go:82`）
4. `go test -race ./...` 覆盖四模块并纳入 CI

**PR-3（可构建、可测试）**
1. 模板改 `//go:embed` + `LoadHTMLFS`（P0-3 + P1-1，一步修好 7 个 API 测试）
2. 消费模块补 `require` + `replace`，`GOWORK=off` 可构建（P1-2）
3. 提交 `frontend/dist` 或加占位与构建前置（P1-3）
4. 新增 `make test_all` / `test_race` / `bench_core`；加 `.github/workflows/ci.yml`（P1-5、P1-6a）
5. `Makefile`：移除 build 对 `clean` 的依赖、去硬编码工具链、统一 `-trimpath` 与版本戳（P1-6c/d/e/f）

**PR-4（内存与性能）**
1. Web 改二进制上传 + 减少 `[]bool` 膨胀（P2-1）
2. 比特展开在块级别只做一次（P2-2）
3. C 层小优化 + `cycle` 越界修复 + `buf_len` 校验（P0-4a/b + P2-3）
   —— `cycle` 越界与 `n` 校验也可以提前到 PR-1，取决于你们对 C 层改动的排期

**PR-5（架构收敛）**
1. Web 自定义路径改为动态构造 `Suite`，删除 430 行 if 链（P1-4）
2. `suiteFor` / `normalizeStandard` / 等级→字节表 / 运行流程下沉 core（P1-4）
3. 样本数规则统一，修掉桌面端与 CLI 的口径差异（P0-5a）

**PR-6（工程化收尾）**
1. LICENSE + THIRD_PARTY_NOTICES（P1-6b）
2. `log/slog` + `/healthz` + `/metrics` + job 进度回调（P1-7）
3. 回归基线加外部已知答案向量、容差改相对、按 suite 派生 golden 表（P1-5）
4. P3 清理清单一次性收尾

---

## 附录 A　实测证据（本机复现）

### A.1　DFT 公式标定（1500 个真随机 1 Mbit 块）

```
PROBE-DFT-COMPARE blocks=1500 (expected 1% at a=0.01, 5% at a=0.05)
  in-repo  formula erfc(|(N1-N0)/sqrt(0.95*0.05*2n/3.8)|): a=.01 -> 1.47%   a=.05 -> 5.80%
  NIST STS formula d=(N1-N0)/sqrt(2*0.95*0.05n), erfc(|d|/sqrt2): a=.01 -> 0.00%   a=.05 -> 0.00%
  binomial theory z=(N1-N0)/sqrt(0.95*0.05n/2), erfc(|z|/sqrt2):  a=.01 -> 0.00%   a=.05 -> 1.00%
```

### A.2　用户参数导致的 panic（HTTP 可直接触达）

```
PROBE-PANIC BlockFrequency M=0           -> PANIC: runtime error: integer divide by zero
PROBE-PANIC Rank R=100 C=100             -> PANIC: runtime error: index out of range [32] with length 32
PROBE-PANIC Rank R=0 C=0                 -> PANIC: runtime error: integer divide by zero
PROBE-PANIC Rank R=32 C=64               -> PANIC: runtime error: index out of range [32] with length 32
PROBE-PANIC Poker m=0                    -> PANIC: runtime error: integer divide by zero
PROBE-PANIC Poker m=3                    -> PANIC: runtime error: index out of range [9] with length 8
PROBE-PANIC Poker m=1000                 -> PANIC: runtime error: index out of range [6] with length 0
PROBE-PANIC Serial m=1                   -> PANIC: runtime error: index out of range [0] with length 0
PROBE-PANIC Serial m=0                   -> PANIC: runtime error: index out of range [-1]
PROBE-PANIC ApproxEntropy m=1000         -> PANIC: runtime error: index out of range [8860813739491930529] with length 0
PROBE-PANIC LinearComplexity m=0         -> PANIC: runtime error: integer divide by zero
PROBE-PANIC Universal short              -> PANIC: runtime error: slice bounds out of range [1:0]
```

### A.3　未授权请求打挂进程

```
POST /api/v1/randomtest/jobs  {"uploadFile":<1KB base64>,"rankTest":{"enable":true,"r":64,"c":64}}
PROBE-JOB create -> HTTP 201 body={"code":200,"msg":"操作成功","data":{"jobId":"7cc928963a6867c7"}}
panic: runtime error: index out of range [32] with length 32
  randomtest_core.RankProto(...)  rank.go:98
  ... service.(*JobManager).run(...)  job_manager.go:220
  created by service.(*JobManager).Submit  job_manager.go:163
FAIL	randomtest_web_gin/test	2.058s       ← 进程终止
```

### A.4　`Alpha` 数据竞争（`go test -race`）

```
WARNING: DATA RACE
Read at 0x0001402b92c8 by goroutine 10:
  randomtest_core.PValuePassed()   runner.go:130
Previous write at 0x0001402b92c8 by goroutine 9:
  randomtest_core.SetAlphaValue()  common.go:33
--- FAIL: TestProbeAlphaRace (0.06s)
```

### A.5　Web 端全部 API 测试失败

```
$ cd randomtest_web_gin && go test ./test/ -count=1
panic: html/template: pattern matches no files: `templates/*` [recovered, repanicked]
  gin.(*Engine).LoadHTMLGlob(...)  gin.go:275
  routers.InitRouter()             routers.go:49
  test.setupServer(...)            api_test.go:59
FAIL	randomtest_web_gin/test	1.876s
```

### A.6　模块不可独立构建

```
$ cd randomtest_web_gin && GOWORK=off go build ./...
service\randomtest.go:9:2: package randomtest_core is not in std (...\src\randomtest_core)
service\randomtest.go:10:2: package randomtest_core/report is not in std (...\src\randomtest_core\report)
```

### A.7　桌面端 embed 缺失

```
//go:embed all:frontend/dist        main.go:14
frontend/dist                       → .gitignore:3 忽略、git ls-files 无条目
=> pattern all:frontend/dist: no matching files found（go build / go test / go vet 全部失败）
```

### A.8　`cycle` 越界写的可达性推演

```
func_nist.cpp:352  calloc(MAX(1000, n/100))     n=1_000_000 → cap = 10000，合法下标 0..9999
func_nist.cpp:376  if (J > cap) return 1;       J == 10000 时条件为 false，放行
func_nist.cpp:386  cycle[J] = i;                → 写 cycle[10000]，越界 4 字节
触发序列：0101...01（交替），S_k 在奇数下标全部归零，第 10000 次归零发生在 i = 19999
func_nist.cpp:393  cycle[J] = n;                无任何边界检查（J 可能已被 :391 自增）
```

---

## 附录 B　经核实**不是**缺陷的项（避免误改）

1. **`TestMain` 未写 `os.Exit(m.Run())`**（`randomtest_stdtest_vue3_wailsv2_test.go:10-14`）——**不是 bug**。我在本机用最小模块实测：`TestMain` 丢弃 `m.Run()` 返回值时，失败用例仍会让进程以退出码 1 结束。不必修改。
2. **`longest_run_of_ones.go:3-38` 的分类表与 m 选择**（8/128/10000、startV 1/4/10、pi 表）与 NIST SP800-22 一致，**不是缺陷**。
3. **cgo 错误码协议完整**：`func_nist.cpp` 四个导出函数的每条返回路径都给出 0/1/2/3，失败时 `*out_pvalues` 均置 NULL，未发现内存泄漏分支。
4. **没有 C++ 异常跨越 C ABI**：两个 TU 只用 C 标准库/math，无 `new/delete/throw`/STL 容器，定义继承 `func_nist.h` 的 `extern "C"`；也没有 `cerr/cout` 输出。
5. **字节→比特展开位序正确**：`utils.go:166-182` 按 `v >> (7-j)` 为 MSB-first，与 NIST STS 的 `convertToBits`（mask 0x80）一致。
6. **`templates.go` 的模板缓存并发安全**：`cTemplateMu` + `sync.Once` 使用正确，且重活不在锁内。
7. **桌面端同步调用不会阻塞 UI 线程**：Wails v2 为每个绑定调用起独立 goroutine。真正的问题只是那些共享全局变量的竞争（P0-2 相关项），而不是"UI 卡死"。
8. **CLI 与桌面端的统计聚合/分母逻辑是共用的**（都经 `report.BuildRows`/`StatsTableLines`），两者结论差异**只来自输入口径**（P0-5a/b/c），而不是聚合代码本身。
9. **`runSuiteDirect` 的分母原子量有正确的 happens-before 边**（worker 发送 → `resultChan` 排空 → 运行后读取），且 `RunFile` 的取消分支发送次数恰好填满 `Workers` 容量的 result channel，不会阻塞。
10. **`templates/*.Z` 之外的模板行为与 SKIP 逻辑自洽**：实测 `template2..16` 分别含 2,4,6,12,20,40,74,148,284,568,1116,2232,4424,8848,17622 个整数、每行恰好 m 个，SKIP 永不越界（最坏 m=16 时消耗 17612/17622）。

---

## 附：本次评审的操作记录（便于复核）

- 临时探针文件：`randomtest_core/zz_dsh_probe_test.go`、`randomtest_web_gin/test/zz_dsh_probe_test.go` —— **已删除**
- 本地 Go 构建缓存：`.dsh_gocache/`、`.dsh_tmp/`（根目录与 `randomtest_core/` 下）—— **已删除**
- 结束时 `git status --short` 输出为空，工作区与评审前一致
