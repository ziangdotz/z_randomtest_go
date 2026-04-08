package randomtest_core

/*
#include <stdlib.h>
#include <func_nist.h>
*/
import "C"

import (
	"fmt"
	"path/filepath"
	"runtime"
	"unsafe"
)

var templateRootDir string

func syncNistCDebug(enabled bool) {
	if enabled {
		C.SetNistDebug(C.int(1))
		return
	}
	C.SetNistDebug(C.int(0))
}

func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("无法获取当前文件路径")
	}
	templateRootDir = filepath.Join(filepath.Dir(filename), "templates")
}

func goBytes2CUCharArray(goBytes []byte) (*C.uchar, func()) {
	bits := make([]byte, len(goBytes)*8)
	for i := range bits {
		byteIndex := i / 8
		bitOffset := 7 - (i % 8) // 大端序
		bit := (goBytes[byteIndex] >> bitOffset) & 0x01
		bits[i] = bit
	}
	// 分配 C 内存并拷贝数据（自动处理 NULL 终止符问题）
	cData := (*C.uchar)(C.CBytes(bits)) // 转换为 unsigned char*
	return cData, func() { C.free(unsafe.Pointer(cData)) }
}

// NonOverlappingTemplateMatchingTest 非重叠模板匹配检测 (byte)
// data: 待检测序列
// n:
// m:
func NonOverlappingTemplateMatchingTestBytes(data []byte, n, m int) ([]float64, []float64, int) {
	cData, free := goBytes2CUCharArray(data)
	defer free()
	templateFilePath := C.CString(filepath.Join(templateRootDir, fmt.Sprintf("template%d", m)))
	defer C.free(unsafe.Pointer(templateFilePath))
	var cPValuesNum C.int
	cPValues := C.NonOverlappingTemplateMatchings_NIST(cData, C.int(n), C.int(m), templateFilePath, &cPValuesNum)
	defer C.free(unsafe.Pointer(cPValues))
	goPValuesNum := int(cPValuesNum)
	// fmt.Printf("goPValuesNum: %d\n", goPValuesNum)
	goSlice := (*[1 << 30]C.double)(unsafe.Pointer(cPValues))[:goPValuesNum:goPValuesNum]
	goPValues := make([]float64, goPValuesNum)
	for i := 0; i < goPValuesNum; i++ {
		goPValues[i] = float64(goSlice[i])
	}
	for i, pValue := range goPValues {
		debugPrintf("NonOverlappingTemplateMatchingTest %d: P-value: %.6f", i+1, pValue)
		isPValueOk(pValue)
		debugPrint("\n")
	}
	return goPValues, nil, goPValuesNum
}

// OverlappingTemplateMatchingTest 重叠模板匹配检测 (byte)
// byte: 待检测序列
// n:
// m:
func OverlappingTemplateMatchingTestBytes(data []byte, n, m int) (float64, float64) {
	cData, free := goBytes2CUCharArray(data)
	defer free()
	cPValue := C.OverlappingTemplateMatchings_NIST(cData, C.int(n), C.int(m))
	goPValue := float64(cPValue)
	debugPrintf("OverlappingTemplateMatchingTest: P-value: %.6f", goPValue)
	isPValueOk(goPValue)
	debugPrint("\n")
	return goPValue, 0
}

// RandomExcursionsTest 随机游程检测 (byte)
// byte: 待检测序列
// n:
func RandomExcursionsTestBytes(data []byte, n int) ([]float64, []float64, int) {
	cData, free := goBytes2CUCharArray(data)
	defer free()
	var cPValuesNum C.int
	cPValues := C.RandomExcursions_NIST(cData, C.int(n), &cPValuesNum)
	defer C.free(unsafe.Pointer(cPValues))
	goPValuesNum := int(cPValuesNum)
	// fmt.Printf("goPValuesNum: %d\n", goPValuesNum)
	goSlice := (*[1 << 30]C.double)(unsafe.Pointer(cPValues))[:goPValuesNum:goPValuesNum]
	goPValues := make([]float64, goPValuesNum)
	for i := 0; i < goPValuesNum; i++ {
		goPValues[i] = float64(goSlice[i])
	}
	for i, pValue := range goPValues {
		debugPrintf("RandomExcursionsTest %d: P-value: %.6f", i+1, pValue)
		isPValueOk(pValue)
		debugPrint("\n")
	}
	return goPValues, nil, goPValuesNum
}

// RandomExcursionsVariantTest 随机游程变量检测 (byte)
// byte: 待检测序列
// n:
func RandomExcursionsVariantTestBytes(data []byte, n int) ([]float64, []float64, int) {
	cData, free := goBytes2CUCharArray(data)
	defer free()
	var cPValuesNum C.int
	cPValues := C.RandomExcursionsVariant_NIST(cData, C.int(n), &cPValuesNum)
	defer C.free(unsafe.Pointer(cPValues))
	goPValuesNum := int(cPValuesNum)
	// fmt.Printf("goPValuesNum: %d\n", goPValuesNum)
	goSlice := (*[1 << 30]C.double)(unsafe.Pointer(cPValues))[:goPValuesNum:goPValuesNum]
	goPValues := make([]float64, goPValuesNum)
	for i := 0; i < goPValuesNum; i++ {
		goPValues[i] = float64(goSlice[i])
	}
	for i, pValue := range goPValues {
		debugPrintf("RandomExcursionsVariantTest %d: P-value: %.6f", i+1, pValue)
		isPValueOk(pValue)
		debugPrint("\n")
	}
	return goPValues, nil, goPValuesNum
}
