package randomtest_core

var parameters = []struct {
	pi     []float64
	k      int
	m      int
	startV int
}{
	{
		pi:     []float64{0.2148, 0.3672, 0.2305, 0.1875},
		k:      3,
		m:      8,
		startV: 1,
	},
	{
		pi:     []float64{0.1174, 0.2430, 0.2494, 0.1752, 0.1027, 0.1124},
		k:      5,
		m:      128,
		startV: 4,
	},
	{
		pi:     []float64{0.086632, 0.208201, 0.248419, 0.193913, 0.121458, 0.068011, 0.073366},
		k:      6,
		m:      10000,
		startV: 10,
	},
}

func selectParameters(n int) int {
	switch {
	case n >= 750000:
		return 2
	case n >= 6272:
		return 1
	default:
		return 0
	}
}

// bits: 待检测序列 (20000 bits: m=128; 1000000 bits / 100000000 bits: m=10000
// m:
func LongestRunOfOnesProto(bits []bool, checkOne bool) (float64, float64) {
	n := len(bits)
	if n < 128 {
		return 0.0, 0.0
	}

	var lr1, mlr1 int
	var b bool
	param := parameters[selectParameters(n)]
	N := n / param.m
	v := make([]float64, param.k+1)
	var V float64 = 0.0
	var P float64 = 0.0
	var Q float64 = 0.0

	for i := 0; i < N; i++ {
		lr1 = 0
		mlr1 = 0

		for j := 0; j < param.m; j++ {
			b, bits = bits[0], bits[1:]
			if checkOne {
				if b {
					lr1++
					mlr1 = max(mlr1, lr1)
				} else {
					lr1 = 0
				}
			} else {
				if b {
					lr1 = 0
				} else {
					lr1++
					mlr1 = max(mlr1, lr1)
				}
			}
		}
		if mlr1 < param.startV {
			mlr1 = param.startV
		} else if mlr1 > param.startV+param.k {
			mlr1 = param.startV + param.k
		}
		v[mlr1-param.startV]++
	}
	for i := 0; i < param.k+1; i++ {
		V += (v[i] - float64(N)*param.pi[i]) * (v[i] - float64(N)*param.pi[i]) / (float64(N) * param.pi[i])
	}
	P = igamc(float64(param.k)/2.0, V/2.0)
	Q = P
	debugPrintf("LongestRunOfOnesTest '%d': P-value: %.6f, Q-value: %.6f", Bool2Int(checkOne), P, Q)
	isPValueOk(P)
	isQValueOk(Q)
	debugPrint("\n")
	return P, Q
}

// LongestRunOfOnesTest 块内最大游程检测 (bit)
func LongestRunOfOnesTestBits(bits []bool, checkOne bool) (float64, float64) {
	return LongestRunOfOnesProto(bits, checkOne)
}

// LongestRunOfOnesTest 块内最大游程检测 (byte)
func LongestRunOfOnesTestBytes(data []byte, checkOne bool) (float64, float64) {
	return LongestRunOfOnesProto(Byte2BitArr(data), checkOne)
}
