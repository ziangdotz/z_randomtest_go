package common

import "net/http"

const (
	OK       = 200
	NOTFOUND = 404
	ERROR    = 500

	REQUEST_METHOD_OPTIONS = 1001
	ERROR_READ_FILE_FAIL   = 1101
	ERROR_WRITE_FILE_FAIL  = 1002

	ERROR_REQUEST_PARAMS = 2001
	ERROR_BIND_FAIL      = 2002

	ERROR_JOB_NOT_FOUND          = 4040 // 任务不存在或已过期
	ERROR_CONFLICT               = 4090 // 并发任务数已达上限
	ERROR_REQUEST_BODY_TOO_LARGE = 4130 // 请求体超过大小限制
)

var CodeMsg = map[int]string{
	OK:       "操作成功",
	NOTFOUND: "404 错误.",
	ERROR:    "500 错误",

	REQUEST_METHOD_OPTIONS: "请求方式为 OPTIONS",
	ERROR_READ_FILE_FAIL:   "服务端读取文件失败",
	ERROR_WRITE_FILE_FAIL:  "服务端写入文件失败",

	ERROR_REQUEST_PARAMS: "请求参数信息错误",
	ERROR_BIND_FAIL:      "绑定数据结构失败",

	ERROR_JOB_NOT_FOUND:          "任务不存在或已过期",
	ERROR_CONFLICT:               "并发任务数已达上限",
	ERROR_REQUEST_BODY_TOO_LARGE: "请求体超过大小限制",
}

// HTTPStatusOf 业务码 → HTTP 状态码映射 (阶段④ 4.3 状态码语义化):
// 参数/绑定失败 400、任务不存在 404、并发冲突 409、请求体超限 413、
// 内部错误 500; 成功维持 200 (201 由创建任务端点单独使用)。
func HTTPStatusOf(responseCode int) int {
	switch responseCode {
	case OK:
		return http.StatusOK
	case ERROR_JOB_NOT_FOUND, NOTFOUND:
		return http.StatusNotFound
	case ERROR_REQUEST_PARAMS, ERROR_BIND_FAIL:
		return http.StatusBadRequest
	case ERROR_CONFLICT:
		return http.StatusConflict
	case ERROR_REQUEST_BODY_TOO_LARGE:
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusInternalServerError
	}
}
