package randomtest_core

import "math"

// bits: 待检测序列
// k:
func BinaryDerivativeProto(bits []bool, m int) (float64, float64) {
	n := len(bits)
	if n < 7 {
		return 0.0, 0.0
	}

	S := 0
	_bits := make([]bool, len(bits))
	var V float64 = 0.0
	var P float64 = 0.0
	var Q float64 = 0.0

	copy(_bits, bits)
	for i := 0; i < m; i++ {
		for j := 0; j < n-i-1; j++ {
			_bits[j] = xor(_bits[j], _bits[j+1])
		}
	}
	for i := 0; i < n-m; i++ {
		if _bits[i] {
			S++
		} else {
			S--
		}
	}
	V = float64(S) / math.Sqrt(2*float64(n-m)) // 提前对 V 除以 2 的平方根，避免求 P/Q 时再求解
	P = math.Erfc(math.Abs(V))
	Q = math.Erfc(V) / 2
	debugPrintf("BinaryDerivativeTest (%d): P-value: %.6f, Q-value: %.6f", m, P, Q)
	isPValueOk(P)
	isQValueOk(Q)
	debugPrint("\n")
	return P, Q
}

// BinaryDerivativeTest 二元推导检测 (bit)
func BinaryDerivativeTestBits(bits []bool, m int) (float64, float64) {
	return BinaryDerivativeProto(bits, m)
}

// BinaryDerivativeTest 二元推导检测 (byte)
func BinaryDerivativeTestBytes(data []byte, m int) (float64, float64) {
	return BinaryDerivativeProto(Byte2BitArr(data), m)
}
