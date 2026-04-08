package randomtest_core

// func selectM(n int) int {
// 	var m int
// 	switch {
// 	case n >= 100000000:
// 		m = 1000000
// 	case n >= 1000000:
// 		m = 10000
// 	case n >= 10000:
// 		m = 1000
// 	case n >= 1000:
// 		m = 100
// 	default:
// 		m = 10
// 	}
// 	return m
// }

// bits: 待检测序列
// m:
func BlockFrequencyProto(bits []bool, blockLength int) (float64, float64) {
	n := len(bits)
	N := n / blockLength
	if N == 0 {
		panic("please provide test bits")
	}

	var b bool
	bits = bits[:N*blockLength]
	var Pi float64
	var V float64 = 0.0
	var P float64 = 0.0
	var Q float64 = 0.0

	for i := 0; i < N; i++ {
		Pi = 0
		for j := 0; j < blockLength; j++ {
			b, bits = bits[0], bits[1:]
			if b {
				Pi++
			}
		}
		Pi = Pi / float64(blockLength)
		Pi = Pi - 0.5
		V += Pi * Pi
	}
	V *= 2.0 * float64(blockLength) // 这一步本来 V 要乘以 4 再除以 2；现在改为 2，免得后一步再除以 2
	P = igamc(float64(N)/2.0, V)
	Q = P
	debugPrintf("BlockFrequencyTest (%d): P-value: %.6f, Q-value: %.6f", blockLength, P, Q)
	isPValueOk(P)
	isQValueOk(Q)
	debugPrint("\n")
	return P, Q
}

// BlockFrequencyTest 块内频数检测 (bit)
func BlockFrequencyTestBits(bits []bool, blockLength int) (float64, float64) {
	return BlockFrequencyProto(bits, blockLength)
}

// BlockFrequencyTest 块内频数检测 (byte)
func BlockFrequencyTestBytes(data []byte, blockLength int) (float64, float64) {
	return BlockFrequencyProto(Byte2BitArr(data), blockLength)
}
