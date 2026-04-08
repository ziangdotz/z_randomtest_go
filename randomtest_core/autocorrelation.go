package randomtest_core

import "math"

// bits: 待检测序列
// d: 1/2/8/16/32
func AutocorrelationProto(bits []bool, m int) (float64, float64) {
	n := len(bits)
	if n < 16 {
		return 0.0, 0.0
	}

	Ad := 0
	var V float64 = 0.0
	var P float64 = 0.0
	var Q float64 = 0.0

	for i := 0; i < n-m; i++ {
		if xor(bits[i], bits[i+m]) {
			Ad++
		}
	}

	V = 2.0 * (float64(Ad) - (float64(n-m) / 2.0)) / math.Sqrt(2*float64(n-m)) // 提前对 V 除以 2 的平方根, 避免求 P/Q 时再求解
	P = math.Erfc(math.Abs(V))
	Q = math.Erfc(V) / 2
	debugPrintf("AutocorrelationTest (%d): P-value: %.6f, Q-value: %.6f", m, P, Q)
	isPValueOk(P)
	isQValueOk(Q)
	debugPrint("\n")
	return P, Q
}

// AutocorrelationTest 自相关检测 (bit)
func AutocorrelationTestBits(bits []bool, m int) (float64, float64) {
	return AutocorrelationProto(bits, m)
}

// AutocorrelationTest 自相关检测 (byte)
func AutocorrelationTestBytes(data []byte, m int) (float64, float64) {
	return AutocorrelationProto(Byte2BitArr(data), m)
}
