package randomtest_core

import (
	"math"
	"math/bits"
)

// FrequencyTest 单比特频数检测 (bit)
func FrequencyTestBits(bits []bool) (float64, float64) {
	n := len(bits)
	if n == 0 {
		return 0.0, 0.0
	}

	S := 0
	var V float64 = 0.0
	var P float64 = 0.0
	var Q float64 = 0.0

	for _, bit := range bits {
		if bit {
			S++
		} else {
			S--
		}
	}
	V = float64(S) / math.Sqrt(float64(2*n)) // 这里提前处理 (n->2*n), 减少 math.Sqrt() 的调用
	P = math.Erfc(math.Abs(V))
	Q = math.Erfc(V) / 2
	debugPrintf("FrequencyTest: P-value: %.6f, Q-value: %.6f", P, Q)
	isPValueOk(P)
	isQValueOk(Q)
	debugPrint("\n")
	return P, Q
}

// FrequencyTest 单比特频数检测 (byte)
func FrequencyTestBytes(data []byte) (float64, float64) {
	n := len(data) * 8
	if n == 0 {
		return 0.0, 0.0
	}

	S := 0
	var V float64 = 0.0
	var P float64 = 0.0
	var Q float64 = 0.0

	for _, b := range data {
		S += bits.OnesCount8(b)<<1 - 8 // S += (bits.OnesCount8(b) - (8 - bits.OnesCount8(b)))
	}
	V = float64(S) / math.Sqrt(float64(2*n)) // 这里提前处理 (n->2*n), 减少 math.Sqrt() 的调用
	P = math.Erfc(math.Abs(V))
	Q = math.Erfc(V) / 2
	debugPrintf("FrequencyTest: P-value: %.6f, Q-value: %.6f", P, Q)
	isPValueOk(P)
	isQValueOk(Q)
	debugPrint("\n")
	return P, Q
}
