package randomtest_core

import "math"

// bits: 待检测序列
// m:
func ApproximateEntropyProto(bits []bool, m int) (float64, float64) {
	n := len(bits)
	if n == 0 {
		return 0.0, 0.0
	}

	numOfBlocks := float64(n)
	var pattern []int
	r := 0
	var ApEn [2]float64
	var V float64 = 0.0
	var P float64 = 0.0

	for blockSize := m; blockSize <= m+1; blockSize++ {
		powLen := 1 << uint(blockSize)
		pattern = make([]int, powLen)
		for i := 0; i < n; i++ {
			k := 1
			for j := 0; j < blockSize; j++ {
				k <<= 1
				if bits[(i+j)%n] {
					k++
				}
			}
			pattern[k-powLen]++
		}
		sum := float64(0.0)
		for i := 0; i < powLen; i++ {
			if pattern[i] > 0 {
				sum += float64(pattern[i]) * math.Log(float64(pattern[i])/numOfBlocks)
			}
		}
		sum /= numOfBlocks
		ApEn[r] = sum
		r++
	}
	apen := ApEn[0] - ApEn[1]
	V = 2.0 * numOfBlocks * (math.Log(2) - apen)
	_2mMinus1 := 1 << uint(m-1)
	P = igamc(float64(_2mMinus1), V/2.0)
	debugPrintf("ApproximateEntropyTest (%d): P-value: %.6f, Q-value: %.6f", m, P, P)
	isPValueOk(P)
	isQValueOk(P)
	debugPrint("\n")
	return P, P
}

// ApproximateEntropyTest 近似熵检测 (bit)
func ApproximateEntropyTestBits(bits []bool, m int) (float64, float64) {
	return ApproximateEntropyProto(bits, m)
}

// ApproximateEntropyTest 近似熵检测 (byte)
func ApproximateEntropyTestBytes(data []byte, m int) (float64, float64) {
	return ApproximateEntropyProto(Byte2BitArr(data), m)
}
