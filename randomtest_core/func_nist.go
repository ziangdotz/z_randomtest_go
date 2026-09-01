package randomtest_core

/*
#include <stdlib.h>
#include <func_nist.h>
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

var (
	cTemplateMu   sync.Mutex
	cTemplatePtr  = map[int]unsafe.Pointer{} // m → C 堆上的模板数据常驻副本
	cTemplateSize = map[int]int{}
)

// syncNistCDebug 将调试开关同步到 C 侧 NIST 实现。
func syncNistCDebug(enabled bool) {
	if enabled {
		C.SetNistDebug(1)
	} else {
		C.SetNistDebug(0)
	}
}

// cTemplateData 返回模板文本在 C 堆上的常驻副本 (按需加载, 进程内复用, 总体积 <1MB)。
func cTemplateData(m int) (unsafe.Pointer, int, error) {
	if m < TemplateMinM || m > TemplateMaxM {
		return nil, 0, fmt.Errorf("模板长度 m=%d 超出支持范围 [%d,%d]", m, TemplateMinM, TemplateMaxM)
	}
	rawMap, err := loadTemplates()
	if err != nil {
		return nil, 0, err
	}
	cTemplateMu.Lock()
	defer cTemplateMu.Unlock()
	if p, ok := cTemplatePtr[m]; ok {
		return p, cTemplateSize[m], nil
	}
	data := rawMap[m]
	cp := C.CBytes(data)
	cTemplatePtr[m] = cp
	cTemplateSize[m] = len(data)
	return cp, len(data), nil
}

// nistCError 将 C 侧错误码转换为 error (0 = 成功)。
func nistCError(testName string, code C.int) error {
	switch code {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("%s: 参数非法 (错误码 1)", testName)
	case 2:
		return fmt.Errorf("%s: 模板数据不可用 (错误码 2)", testName)
	case 3:
		return fmt.Errorf("%s: 内存分配失败 (错误码 3)", testName)
	default:
		return fmt.Errorf("%s: 未知错误码 %d", testName, int(code))
	}
}

// cDoublesToSlice 将 C 堆上的 double 数组拷贝为 Go 切片并释放 C 内存。
func cDoublesToSlice(p *C.double, num int) ([]float64, error) {
	if p == nil {
		return nil, fmt.Errorf("C 侧返回空指针")
	}
	defer C.free(unsafe.Pointer(p))
	if num <= 0 {
		return []float64{}, nil
	}
	goSlice := (*[1 << 30]C.double)(unsafe.Pointer(p))[:num:num]
	out := make([]float64, num)
	for i := 0; i < num; i++ {
		out[i] = float64(goSlice[i])
	}
	return out, nil
}

func goBytes2CUCharArray(goBytes []byte) (*C.uchar, func()) {
	bits := make([]byte, len(goBytes)*8)
	BytesToBitBytes(bits, goBytes) // 查表展开 (大端位序), 替代逐位循环移位
	// 分配 C 内存并拷贝数据（自动处理 NULL 终止符问题）
	cData := (*C.uchar)(C.CBytes(bits)) // 转换为 unsigned char*
	return cData, func() { C.free(unsafe.Pointer(cData)) }
}

// NonOverlappingTemplateMatchingTestBytes 非重叠模板匹配检测 (byte)
// data: 待检测序列
// n: 序列比特数
// m: 模板长度 (支持范围 [2,16])
func NonOverlappingTemplateMatchingTestBytes(data []byte, n, m int) ([]float64, []float64, int, error) {
	tmplPtr, tmplLen, err := cTemplateData(m)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("NonOverlappingTemplateMatchingTest: %w", err)
	}
	cData, free := goBytes2CUCharArray(data)
	defer free()
	var cPValuesNum C.int
	var cPValues *C.double
	code := C.NonOverlappingTemplateMatchings_NIST(cData, C.int(n), C.int(m), (*C.uchar)(tmplPtr), C.int(tmplLen), &cPValues, &cPValuesNum)
	if err := nistCError("NonOverlappingTemplateMatchingTest", code); err != nil {
		return nil, nil, 0, err
	}
	goPValues, err := cDoublesToSlice(cPValues, int(cPValuesNum))
	if err != nil {
		return nil, nil, 0, fmt.Errorf("NonOverlappingTemplateMatchingTest: %w", err)
	}
	for i, pValue := range goPValues {
		debugPrintf("NonOverlappingTemplateMatchingTest %d: P-value: %.6f", i+1, pValue)
		isPValueOk(pValue)
		debugPrint("\n")
	}
	return goPValues, nil, len(goPValues), nil
}

// OverlappingTemplateMatchingTest 重叠模板匹配检测 (byte)
// data: 待检测序列
// n: 序列比特数
// m: 模板长度
func OverlappingTemplateMatchingTestBytes(data []byte, n, m int) (float64, float64, error) {
	cData, free := goBytes2CUCharArray(data)
	defer free()
	var cPValue C.double
	code := C.OverlappingTemplateMatchings_NIST(cData, C.int(n), C.int(m), &cPValue)
	if err := nistCError("OverlappingTemplateMatchingTest", code); err != nil {
		return 0, 0, err
	}
	goPValue := float64(cPValue)
	debugPrintf("OverlappingTemplateMatchingTest: P-value: %.6f", goPValue)
	isPValueOk(goPValue)
	debugPrint("\n")
	return goPValue, 0, nil
}

// RandomExcursionsTest 随机游程检测 (byte)
// data: 待检测序列
// n: 序列比特数
func RandomExcursionsTestBytes(data []byte, n int) ([]float64, []float64, int, error) {
	cData, free := goBytes2CUCharArray(data)
	defer free()
	var cPValuesNum C.int
	var cPValues *C.double
	code := C.RandomExcursions_NIST(cData, C.int(n), &cPValues, &cPValuesNum)
	if err := nistCError("RandomExcursionsTest", code); err != nil {
		return nil, nil, 0, err
	}
	goPValues, err := cDoublesToSlice(cPValues, int(cPValuesNum))
	if err != nil {
		return nil, nil, 0, fmt.Errorf("RandomExcursionsTest: %w", err)
	}
	for i, pValue := range goPValues {
		debugPrintf("RandomExcursionsTest %d: P-value: %.6f", i+1, pValue)
		isPValueOk(pValue)
		debugPrint("\n")
	}
	return goPValues, nil, len(goPValues), nil
}

// RandomExcursionsVariantTest 随机游程变量检测 (byte)
// data: 待检测序列
// n: 序列比特数
func RandomExcursionsVariantTestBytes(data []byte, n int) ([]float64, []float64, int, error) {
	cData, free := goBytes2CUCharArray(data)
	defer free()
	var cPValuesNum C.int
	var cPValues *C.double
	code := C.RandomExcursionsVariant_NIST(cData, C.int(n), &cPValues, &cPValuesNum)
	if err := nistCError("RandomExcursionsVariantTest", code); err != nil {
		return nil, nil, 0, err
	}
	goPValues, err := cDoublesToSlice(cPValues, int(cPValuesNum))
	if err != nil {
		return nil, nil, 0, fmt.Errorf("RandomExcursionsVariantTest: %w", err)
	}
	for i, pValue := range goPValues {
		debugPrintf("RandomExcursionsVariantTest %d: P-value: %.6f", i+1, pValue)
		isPValueOk(pValue)
		debugPrint("\n")
	}
	return goPValues, nil, len(goPValues), nil
}
