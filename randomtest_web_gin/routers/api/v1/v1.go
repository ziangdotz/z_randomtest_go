package v1

import (
	"errors"
	"log"
	"net/http"

	common "randomtest_web_gin/pkg/common"
	"randomtest_web_gin/service"

	"github.com/gin-gonic/gin"
)

func boolPtr(b bool) *bool { return &b }

func normalizeRandomTestRequest(req *service.RandomTestHTTPRequestStruct) {
	if req.BlockFrequencyTest == nil {
		req.BlockFrequencyTest = &service.RandomTestMethodStruct1{Enable: false, M: 128}
	}
	if req.CumulativeSumsTest == nil {
		req.CumulativeSumsTest = &service.RandomTestMethodStruct2{Enable: boolPtr(false), IsForward: boolPtr(true)}
	}
	if req.LongestRunOfOnesTest == nil {
		req.LongestRunOfOnesTest = &service.RandomTestMethodStruct3{Enable: boolPtr(false), CheckOne: boolPtr(true)}
	}
	if req.RankTest == nil {
		req.RankTest = &service.RandomTestMethodStruct4{Enable: false, R: 32, C: 32}
	}
	if req.NonOverlappingTemplateMatchingTest == nil {
		req.NonOverlappingTemplateMatchingTest = &service.RandomTestMethodStruct5{Enable: false, N: 8, M: 9} // 模板长度 m 仅支持 [2,16]
	}
	if req.OverlappingTemplateMatchingTest == nil {
		req.OverlappingTemplateMatchingTest = &service.RandomTestMethodStruct5{Enable: false, N: 8, M: 1000}
	}
	if req.ApproximateEntropyTest == nil {
		req.ApproximateEntropyTest = &service.RandomTestMethodStruct6{Enable: false, M: 10}
	}
	if req.RandomExcursionsTest == nil {
		req.RandomExcursionsTest = &service.RandomTestMethodStruct7{Enable: false, N: 8}
	}
	if req.RandomExcursionsVariantTest == nil {
		req.RandomExcursionsVariantTest = &service.RandomTestMethodStruct7{Enable: false, N: 8}
	}
	if req.SerialTest == nil {
		req.SerialTest = &service.RandomTestMethodStruct6{Enable: false, M: 16}
	}
	if req.LinearComplexityTest == nil {
		req.LinearComplexityTest = &service.RandomTestMethodStruct6{Enable: false, M: 500}
	}
	if req.PokerTest == nil {
		req.PokerTest = &service.RandomTestMethodStruct6{Enable: false, M: 4}
	}
	if req.BinaryDerivativeTest == nil {
		req.BinaryDerivativeTest = &service.RandomTestMethodStruct8{Enable: false, K: 3}
	}
	if req.AutocorrelationTest == nil {
		req.AutocorrelationTest = &service.RandomTestMethodStruct9{Enable: false, D: 1}
	}
}

// respondBindError 统一处理请求绑定失败 (4.3 状态码语义化): 请求体超限 413, 其余 400。
func respondBindError(commonGin common.Gin, err error) {
	log.Printf("[ERROR] bind request failed: %s", err.Error())
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		commonGin.Response(common.HTTPStatusOf(common.ERROR_REQUEST_BODY_TOO_LARGE), common.ERROR_REQUEST_BODY_TOO_LARGE, nil)
		return
	}
	commonGin.Response(common.HTTPStatusOf(common.ERROR_BIND_FAIL), common.ERROR_BIND_FAIL, nil)
}

// respondServiceError 统一处理检测处理失败 (4.3 状态码语义化): 内部错误 500。
func respondServiceError(commonGin common.Gin, scene string, err error) {
	log.Printf("[ERROR] handle %s random test failed: %s", scene, err.Error())
	commonGin.Response(common.HTTPStatusOf(common.ERROR), common.ERROR, nil)
}

// 根据客户端提交的 RandomTest HTTP 请求，处理随机性测试函数
func HandleRandomTestHTTPRequestAPI(ctx *gin.Context) {
	commonGin := common.Gin{Ctx: ctx}

	var randomTestHTTPRequestService service.RandomTestHTTPRequestStruct
	if err := ctx.ShouldBind(&randomTestHTTPRequestService); err != nil {
		respondBindError(commonGin, err)
		return
	}

	normalizeRandomTestRequest(&randomTestHTTPRequestService)

	// 同步端点受全局检测并发额度保护, 满载返回 409 (阶段④ 4.2)
	if !service.GlobalJobManager().TryAcquire() {
		commonGin.Response(common.HTTPStatusOf(common.ERROR_CONFLICT), common.ERROR_CONFLICT, nil)
		return
	}
	defer service.GlobalJobManager().Release()

	result, err := randomTestHTTPRequestService.HandleRandomTestHTTPRequestService()
	if err != nil {
		respondServiceError(commonGin, "", err)
		return
	}

	// 将结果返回给前端
	commonGin.Response(http.StatusOK, common.OK, gin.H{
		"result": result,
	})
}

// 固定执行 NIST Level2 测试项（参数和组合由后端预设）。
func HandleRandomTestHTTPRequestNISTLevel2API(ctx *gin.Context) {
	commonGin := common.Gin{Ctx: ctx}

	var randomTestHTTPRequestService service.RandomTestHTTPRequestStruct
	if err := ctx.ShouldBind(&randomTestHTTPRequestService); err != nil {
		respondBindError(commonGin, err)
		return
	}

	// 同步端点受全局检测并发额度保护, 满载返回 409 (阶段④ 4.2)
	if !service.GlobalJobManager().TryAcquire() {
		commonGin.Response(common.HTTPStatusOf(common.ERROR_CONFLICT), common.ERROR_CONFLICT, nil)
		return
	}
	defer service.GlobalJobManager().Release()

	result, err := randomTestHTTPRequestService.HandleRandomTestNISTLevel2Service()
	if err != nil {
		respondServiceError(commonGin, "NIST level2", err)
		return
	}

	commonGin.Response(http.StatusOK, common.OK, gin.H{
		"result": result,
	})
}

// createRandomTestJobRequest 异步任务创建请求: 检测请求体 + 任务类型。
type createRandomTestJobRequest struct {
	service.RandomTestHTTPRequestStruct
	Kind string `form:"kind" json:"kind"` // nist2 / gm2 / custom (缺省 custom)
}

// 创建异步检测任务 (阶段④ 4.2): 201 返回 {jobId}。
func HandleCreateRandomTestJobAPI(ctx *gin.Context) {
	commonGin := common.Gin{Ctx: ctx}

	var body createRandomTestJobRequest
	if err := ctx.ShouldBind(&body); err != nil {
		respondBindError(commonGin, err)
		return
	}
	normalizeRandomTestRequest(&body.RandomTestHTTPRequestStruct)

	switch body.Kind {
	case service.JobKindNIST2, service.JobKindGM2, service.JobKindCustom:
	default:
		body.Kind = service.JobKindCustom
	}

	jobID := service.GlobalJobManager().Submit(body.Kind, body.RandomTestHTTPRequestStruct)
	ctx.JSON(http.StatusCreated, common.Response{
		Code: common.OK,
		Msg:  common.GetResponseMsg(common.OK),
		Data: gin.H{"jobId": jobID},
	})
}

// 查询异步检测任务 (阶段④ 4.2): 200 返回 {status, progress, result?}; 不存在 404。
func HandleGetRandomTestJobAPI(ctx *gin.Context) {
	commonGin := common.Gin{Ctx: ctx}

	job, ok := service.GlobalJobManager().Get(ctx.Param("id"))
	if !ok {
		commonGin.Response(common.HTTPStatusOf(common.ERROR_JOB_NOT_FOUND), common.ERROR_JOB_NOT_FOUND, nil)
		return
	}

	data := gin.H{
		"jobId":    job.ID,
		"kind":     job.Kind,
		"status":   job.Status,
		"progress": job.Progress,
	}
	if job.Status == service.JobDone {
		data["result"] = job.Result
	}
	if job.Status == service.JobFailed {
		data["error"] = job.Err
	}
	commonGin.Response(http.StatusOK, common.OK, data)
}

// 取消异步检测任务 (阶段④ 4.2): 200; 不存在 404; 已结束 409。
func HandleCancelRandomTestJobAPI(ctx *gin.Context) {
	commonGin := common.Gin{Ctx: ctx}

	err := service.GlobalJobManager().Cancel(ctx.Param("id"))
	switch {
	case errors.Is(err, service.ErrJobNotFound):
		commonGin.Response(common.HTTPStatusOf(common.ERROR_JOB_NOT_FOUND), common.ERROR_JOB_NOT_FOUND, nil)
		return
	case errors.Is(err, service.ErrJobNotCancellable):
		commonGin.Response(common.HTTPStatusOf(common.ERROR_CONFLICT), common.ERROR_CONFLICT, nil)
		return
	}
	commonGin.Response(http.StatusOK, common.OK, nil)
}

// 固定执行 GM Level2 测试项（参数和组合由后端预设）。
func HandleRandomTestHTTPRequestGMLevel2API(ctx *gin.Context) {
	commonGin := common.Gin{Ctx: ctx}

	var randomTestHTTPRequestService service.RandomTestHTTPRequestStruct
	if err := ctx.ShouldBind(&randomTestHTTPRequestService); err != nil {
		respondBindError(commonGin, err)
		return
	}

	// 同步端点受全局检测并发额度保护, 满载返回 409 (阶段④ 4.2)
	if !service.GlobalJobManager().TryAcquire() {
		commonGin.Response(common.HTTPStatusOf(common.ERROR_CONFLICT), common.ERROR_CONFLICT, nil)
		return
	}
	defer service.GlobalJobManager().Release()

	result, err := randomTestHTTPRequestService.HandleRandomTestGMLevel2Service()
	if err != nil {
		respondServiceError(commonGin, "GM level2", err)
		return
	}

	commonGin.Response(http.StatusOK, common.OK, gin.H{
		"result": result,
	})
}
