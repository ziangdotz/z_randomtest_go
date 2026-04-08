package common

const (
	OK       = 200
	NOTFOUND = 404
	ERROR    = 500

	REQUEST_METHOD_OPTIONS = 1001
	ERROR_READ_FILE_FAIL   = 1101
	ERROR_WRITE_FILE_FAIL  = 1002

	ERROR_REQUEST_PARAMS = 2001
	ERROR_BIND_FAIL      = 2002
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
}
