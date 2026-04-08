package randomtest_core

import (
	"math"
)

func mutFactorC(L, K int) float64 {
	var v float64
	v = 0.7
	v -= 0.8 / float64(L)
	v += (4.0 + 32.0/float64(L)) * (math.Pow(float64(K), -3.0/float64(L)) / 15.0)
	return v
}

// UniversalTest 通用统计检测 (bit)
// bits: 待检测序列
func UniversalTestBits(bits []bool) (float64, float64) {
	n := len(bits)
	if n == 0 {
		return 0.0, 0.0
	}

	L := 7
	Q := 1280
	T := make([]int, 1<<uint(L))
	mask := (1 << uint(L)) - 1
	var K int = n/L - Q
	var sum float64 = 0.0
	var sigma float64 = 0.0
	var V float64 = 0.0
	var P1 float64 = 0.0
	var Q1 float64 = 0.0
	expected_value := []float64{0, 0, 0, 0, 0, 0, 5.2177052, 6.1962507, 7.1836656, 8.1764248, 9.1723243, 10.170032, 11.168765, 12.168070, 13.167693, 14.167488, 15.167379}
	variance := []float64{0, 0, 0, 0, 0, 0, 2.954, 3.125, 3.238, 3.311, 3.356, 3.384, 3.401, 3.410, 3.416, 3.419, 3.421}

	var tmp int = 0
	var b bool
	for i := 1; i <= Q; i++ {
		for j := 0; j < L; j++ {
			tmp <<= 1
			b, bits = bits[0], bits[1:]
			if b {
				tmp++
			}
		}
		T[tmp&mask] = i
	}
	for i := Q + 1; i <= Q+K; i++ {
		for j := 0; j < L; j++ {
			tmp <<= 1
			b, bits = bits[0], bits[1:]
			if b {
				tmp++
			}
		}
		sum += math.Log(float64(i)-float64(T[tmp&mask])) / math.Log(2.0)
		T[tmp&mask] = i
	}
	sigma = math.Sqrt(variance[L]/float64(K)) * mutFactorC(L, K)
	V = (sum/float64(K) - expected_value[L]) / (sigma * math.Sqrt(2.0)) // 提前对 V 除以 2 的平方根, 避免求 P/Q 时再求解
	P1 = math.Erfc(math.Abs(V))
	Q1 = math.Erfc(V) / 2
	debugPrintf("UniversalTest: P-value: %.6f, Q-value: %.6f", P1, Q1)
	isPValueOk(P1)
	isQValueOk(Q1)
	debugPrint("\n")
	return P1, Q1
}

// UniversalTest 通用统计检测 (byte)
func UniversalTestBytes(data []byte) (float64, float64) {
	return UniversalTestBits(Byte2BitArr(data))
}
