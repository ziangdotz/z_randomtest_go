package common

import (
	"github.com/gin-gonic/gin"
)

type Gin struct {
	Ctx *gin.Context
}

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// 根据自定义状态码返回消息标签字段
func GetResponseMsg(responseCode int) string {
	if errMsg, ok := CodeMsg[responseCode]; ok {
		return errMsg
	}

	return CodeMsg[ERROR]
}

// 设置服务端的回复消息字段（JSON 格式）
func (gin *Gin) Response(httpCode, responseCode int, data interface{}) {
	gin.Ctx.JSON(httpCode, Response{
		Code: responseCode,
		Msg:  GetResponseMsg(responseCode),
		Data: data,
	})
}
