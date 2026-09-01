package randomtest_core

import (
	"math"
	"math/cmplx"
	"sync"
)

func ceilPow2(N int) int {
	i := 2
	for {
		if i >= N {
			return i
		}
		i <<= 1
	}
}

// fftCache 用于缓存不同长度的 FFT 预计算数据
var fftCache sync.Map

// bufferPool 用于复用复数切片，减少内存抖动
var bufferPool = sync.Pool{
	New: func() interface{} {
		return nil // 动态根据需要的长度分配
	},
}

// getBuffer 从 Pool 获取或创建足够长的切片
func getBuffer(size int) []complex128 {
	val := bufferPool.Get()
	if val == nil {
		return make([]complex128, size)
	}
	buf := val.([]complex128)
	if cap(buf) < size {
		return make([]complex128, size)
	}
	return buf[:size]
}

// getFFTInstance 获取预计算过的 FFT 对象
func getFFTInstance(N int) (*FFT, error) {
	if val, ok := fftCache.Load(N); ok {
		return val.(*FFT), nil
	}
	f, err := New(N)
	if err != nil {
		return nil, err
	}
	fftCache.Store(N, &f)
	return &f, nil
}

// bits: 待检测序列
func DiscreteFourierTransformProto(bits []bool) (float64, float64) {
	n := len(bits)
	if n == 0 {
		panic("please provide test bits")
	}

	// Step 1 数据预处理: 0->-1, 1->1
	N := ceilPow2(n)
	rr := getBuffer(N)
	defer bufferPool.Put(rr) // 函数结束时回收内存
	for i := 0; i < n; i++ {
		if bits[i] {
			rr[i] = complex(1.0, 0)
		} else {
			rr[i] = complex(-1.0, 0)
		}
	}
	for i := n; i < N; i++ {
		rr[i] = 0
	} // 初始化 rr，未使用的部分补 0 (Padding)
	return dftTransformAndScore(rr, n)
}

// DiscreteFourierTransformProtoBytes bytes 快速路径: 直接从字节流查表填充 ±1,
// 跳过 []bool 中间层 (省 1MB/块 分配与遍历)。
func DiscreteFourierTransformProtoBytes(data []byte) (float64, float64) {
	n := len(data) * 8
	if n == 0 {
		panic("please provide test bits")
	}

	N := ceilPow2(n)
	rr := getBuffer(N)
	defer bufferPool.Put(rr) // 函数结束时回收内存
	for i, b := range data {
		row := &bitsExpandTable[b]
		base := i * 8
		for j := 0; j < 8; j++ {
			if row[j] != 0 {
				rr[base+j] = complex(1.0, 0)
			} else {
				rr[base+j] = complex(-1.0, 0)
			}
		}
	}
	for i := n; i < N; i++ {
		rr[i] = 0
	} // Padding
	return dftTransformAndScore(rr, n)
}

// dftTransformAndScore DFT Step 2-6: 变换 + 阈值统计 + P/Q 计算 (rr 被覆写)。
func dftTransformAndScore(rr []complex128, n int) (float64, float64) {
	// Step 2 傅里叶变换
	f, err := getFFTInstance(len(rr))
	if err != nil {
		panic(err)
	}
	f.Transform(rr)
	// Step 3 计算阈值 T = sqrt(log(1/0.05) * n)
	T := math.Sqrt(2.995732274 * float64(n))
	// Step 4 计算理论预期值 N0 = 0.95 * n / 2
	N_0 := 0.95 * float64(n) / 2
	// Step 5 统计模值小于 T 的数量 N1
	var N_1 int = 0
	for i := 0; i < n/2-1; i++ {
		if cmplx.Abs(rr[i]) < T {
			N_1++
		}
	}
	// Step 6
	V := (float64(N_1) - N_0) / math.Sqrt(0.95*0.05*float64(2.0*n)/3.8) // 提前对 V 除以 2 的平方根, 避免求 P/Q 时再求解
	P := math.Erfc(math.Abs(V))
	Q := math.Erfc(V) / 2
	debugPrintf("DiscreteFourierTransformTest: P-value: %.6f, Q-value: %.6f", P, Q)
	isPValueOk(P)
	isQValueOk(Q)
	debugPrint("\n")
	return P, Q
}

// DiscreteFourierTransformTest 离散傅里叶检测 (bit)
func DiscreteFourierTransformTestBits(bits []bool) (float64, float64) {
	return DiscreteFourierTransformProto(bits)
}

// DiscreteFourierTransformTest 离散傅里叶检测 (byte)
func DiscreteFourierTransformTestBytes(data []byte) (float64, float64) {
	return DiscreteFourierTransformProtoBytes(data)
}
