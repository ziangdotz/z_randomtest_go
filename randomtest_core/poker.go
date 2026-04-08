package randomtest_core

// PokerTest 扑克检测 (bit)
// data: 待检测序列
// m:
func PokerTestBits(bits []bool, m int) (float64, float64) {
	n := len(bits)
	if n < 8 {
		return 0.0, 0.0
	}

	N := n / m
	_2m := 1 << uint(m) // 2^m
	patterns := make([]int, _2m)
	var V float64 = 0.0
	var P float64 = 0.0
	var Q float64 = 0.0

	for i := 0; i < N; i++ {
		patterns[subsequencepattern(bits[i*m:], m)]++
	}
	for i := 0; i < _2m; i++ {
		V += float64(patterns[i]) * float64(patterns[i])
	}
	V *= float64(_2m)
	V /= float64(N)
	V -= float64(N)
	P = igamc(float64(_2m-1)/2, V/2)
	Q = P
	debugPrintf("PokerTest (%d): P-value: %.6f, Q-value: %.6f", m, P, Q)
	isPValueOk(P)
	isQValueOk(Q)
	debugPrint("\n")
	return P, Q
}

// PokerTest 扑克检测 (byte)
// data: 待检测序列
// m:
func PokerTestBytes(data []byte, m int) (float64, float64) {
	if len(data) == 0 {
		return 0.0, 0.0
	}

	N := (len(data) * 8) / m
	_2m := 1 << uint(m) // 2^m
	patterns := make([]int, _2m)
	var V float64 = 0.0
	var P float64 = 0.0
	var Q float64 = 0.0

	if m == 8 { // m = 8
		for i := 0; i < N; i++ {
			patterns[data[i]]++
		}
	} else { // m = 4
		for i := 0; i < len(data); i++ {
			patterns[data[i]>>4]++
			patterns[data[i]&0x0f]++
		}
	}
	for i := 0; i < _2m; i++ {
		V += float64(patterns[i]) * float64(patterns[i])
	}
	V *= float64(_2m)
	V /= float64(N)
	V -= float64(N)
	P = igamc(float64(_2m-1)/2, V/2)
	Q = P
	debugPrintf("PokerTest (%d): P-value: %.6f, Q-value: %.6f", m, P, Q)
	isPValueOk(P)
	isQValueOk(Q)
	debugPrint("\n")
	return P, Q
}
