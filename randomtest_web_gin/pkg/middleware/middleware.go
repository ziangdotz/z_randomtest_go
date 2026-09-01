package middleware

// 阶段④ 4.1: 请求体大小限制中间件。
import (
	"net/http"

	"randomtest_web_gin/pkg/common"

	"github.com/gin-gonic/gin"
)

// LimitRequestBody 限制请求体大小 (字节):
//   - 已知 Content-Length 时超限直接拒绝 413 + 现有 body 结构 (快速路径);
//   - 否则用 http.MaxBytesReader 包装 Body 兜底, 超限读取返回 *http.MaxBytesError,
//     由绑定失败处理处通过 errors.As 识别并返回 413。
func LimitRequestBody(maxBytes int64) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if ctx.Request.ContentLength > maxBytes {
			ctx.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, common.Response{
				Code: common.ERROR_REQUEST_BODY_TOO_LARGE,
				Msg:  common.GetResponseMsg(common.ERROR_REQUEST_BODY_TOO_LARGE),
			})
			return
		}
		if ctx.Request.Body != nil {
			ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxBytes)
		}
		ctx.Next()
	}
}
