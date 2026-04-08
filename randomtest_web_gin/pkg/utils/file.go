package utils

import (
	"io"
	"os"
)

// ReadArrFromBinaryFormatFile 主要实现从 Binary 文件中的读取字节数据并转换为字节数组的功能
func ReadArrFromBinaryFormatFile(filename string) []byte {
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}
	return data
}

// ReadArrFromBinaryFormatFileError 主要实现从 Binary 文件中的读取字节数据并转换为字节数组的功能, 该函数带 error 返回
func ReadArrFromBinaryFormatFileError(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}
	return data, nil
}
