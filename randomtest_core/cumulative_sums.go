package randomtest_core

import "math"

func isForward2Str(b bool) string {
	if b {
		return "Forward"
	}
	return "BackWard"
}

func normal_CDF(x float64) float64 {
	return (1 + math.Erf(x/math.Sqrt(2))) / 2
}

// bits: 待检测序列
// isForward: true 前向, false 后向
func CumulativeSumsProto(bits []bool, isForward bool) (float64, float64) {
	n := len(bits)
	if n == 0 {
		return 0.0, 0.0
	}

	var S int = 0
	var Z int = 0
	var P float64 = 1.0
	var Q float64 = 1.0

	for i := 0; i < n; i++ {
		if isForward {
			if bits[i] {
				S++
			} else {
				S--
			}
		} else {
			if bits[n-1-i] {
				S++
			} else {
				S--
			}
		}
		Z = max(Z, abs(S))
	}
	sqrtN := math.Sqrt(float64(n)) // 提前求平方根，避免下面多次求平方根
	for i := ((-n / Z) + 1) / 4; i <= ((n/Z)-1)/4; i++ {
		P -= normal_CDF(float64((4*i+1)*Z)/sqrtN) - normal_CDF(float64((4*i-1)*Z)/sqrtN)
	}
	for i := ((-n / Z) - 3) / 4; i <= ((n/Z)-1)/4; i++ {
		P += normal_CDF(float64((4*i+3)*Z)/sqrtN) - normal_CDF(float64((4*i+1)*Z)/sqrtN)
	}
	Q = P
	debugPrintf("CumulativeSumsTest (%s): P-value: %.6f, Q-value: %.6f", isForward2Str(isForward), P, Q)
	isPValueOk(P)
	isQValueOk(Q)
	debugPrint("\n")
	return P, Q
}

// CumulativeSumsTest 累加和检测 (bits)
func CumulativeSumsTestBits(bits []bool, isForward bool) (float64, float64) {
	return CumulativeSumsProto(bits, isForward)
}

// CumulativeSumsTest 累加和检测 (byte)
func CumulativeSumsTestBytes(data []byte, isForward bool) (float64, float64) {
	return CumulativeSumsProto(Byte2BitArr(data), isForward)
}
