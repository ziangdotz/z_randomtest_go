package randomtest_core

import (
	"math"
)

// RunsTest 游程总数检测 (bit)
// bits: 待检测序列
func RunsTestBits(bits []bool) (float64, float64) {
	n := len(bits)
	if n == 0 {
		return 0.0, 0.0
	}

	var V_obs int = 1
	var Pi float64 = 0
	var V float64 = 0.0
	var P float64 = 0.0
	var Q float64 = 0.0

	for i := 0; i < n-1; i++ {
		if bits[i] != bits[i+1] {
			V_obs++
		}
		if bits[i] {
			Pi++
		}
	}
	if bits[n-1] {
		Pi++
	}
	Pi /= float64(n)
	V = (float64(V_obs) - 2.0*float64(n)*Pi*(1.0-Pi)) / (2.0 * math.Sqrt(float64(2*n)) * Pi * (1.0 - Pi)) // Step 4/Step 5 的除以 math.Sqrt(2) 放到这里提前处理，减少 math.Sqrt() 的调用
	P = math.Erfc(math.Abs(V))
	Q = math.Erfc(V) / 2.0
	debugPrintf("RunsTest: P-value: %.6f, Q-value: %.6f", P, Q)
	isPValueOk(P)
	isQValueOk(Q)
	debugPrint("\n")
	return P, Q
}

// RunsTest 游程总数检测 (byte)
func RunsTestBytes(data []byte) (float64, float64) {
	return RunsTestBits(Byte2BitArr(data))
}
