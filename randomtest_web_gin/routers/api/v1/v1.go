package v1

import (
	"log"
	"net/http"

	common "randomtest_web_gin/pkg/common"
	service "randomtest_web_gin/service"

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
		req.NonOverlappingTemplateMatchingTest = &service.RandomTestMethodStruct5{Enable: false, N: 8, M: 1000}
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

// 根据客户端提交的 RandomTest HTTP 请求，处理随机性测试函数
func HandleRandomTestHTTPRequestAPI(ctx *gin.Context) {
	commonGin := common.Gin{Ctx: ctx}

	var randomTestHTTPRequestService service.RandomTestHTTPRequestStruct
	if err := ctx.ShouldBind(&randomTestHTTPRequestService); err != nil {
		log.Printf("[ERROR] bind request failed: %s", err.Error())
		commonGin.Response(http.StatusOK, common.ERROR_BIND_FAIL, nil)
		return
	}

	normalizeRandomTestRequest(&randomTestHTTPRequestService)

	result, err := randomTestHTTPRequestService.HandleRandomTestHTTPRequestService()
	if err != nil {
		log.Printf("[ERROR] handle random test failed: %s", err.Error())
		commonGin.Response(http.StatusOK, common.ERROR, nil)
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
		log.Printf("[ERROR] bind request failed: %s", err.Error())
		commonGin.Response(http.StatusOK, common.ERROR_BIND_FAIL, nil)
		return
	}

	result, err := randomTestHTTPRequestService.HandleRandomTestNISTLevel2Service()
	if err != nil {
		log.Printf("[ERROR] handle NIST level2 random test failed: %s", err.Error())
		commonGin.Response(http.StatusOK, common.ERROR, nil)
		return
	}

	commonGin.Response(http.StatusOK, common.OK, gin.H{
		"result": result,
	})
}

// 固定执行 GM Level2 测试项（参数和组合由后端预设）。
func HandleRandomTestHTTPRequestGMLevel2API(ctx *gin.Context) {
	commonGin := common.Gin{Ctx: ctx}

	var randomTestHTTPRequestService service.RandomTestHTTPRequestStruct
	if err := ctx.ShouldBind(&randomTestHTTPRequestService); err != nil {
		log.Printf("[ERROR] bind request failed: %s", err.Error())
		commonGin.Response(http.StatusOK, common.ERROR_BIND_FAIL, nil)
		return
	}

	result, err := randomTestHTTPRequestService.HandleRandomTestGMLevel2Service()
	if err != nil {
		log.Printf("[ERROR] handle GM level2 random test failed: %s", err.Error())
		commonGin.Response(http.StatusOK, common.ERROR, nil)
		return
	}

	commonGin.Response(http.StatusOK, common.OK, gin.H{
		"result": result,
	})
}
