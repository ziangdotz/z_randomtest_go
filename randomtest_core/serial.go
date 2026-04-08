package randomtest_core

// SerialTest 重叠子序列检测方法 (bit)
// bits: 待检测序列
// m:
func SerialTestBits(bits []bool, m int) (float64, float64, float64, float64) {
	n := len(bits)
	if n < 5 {
		return 0.0, 0.0, 0.0, 0.0
	}

	patterns1 := make([]int, 1<<uint(m))
	patterns2 := make([]int, 1<<uint(m-1))
	patterns3 := make([]int, 1<<uint(m-2))
	var Phi1, Phi2, Phi3 float64 = 0, 0, 0
	var DPhi2, D2Phi2 float64 = 0, 0
	var mask1 int = (1 << uint(m)) - 1
	var mask2 int = (1 << uint(m-1)) - 1
	var mask3 int = (1 << uint(m-2)) - 1
	var P1 float64 = 0.0
	var P2 float64 = 0.0
	var Q1 float64 = 0.0
	var Q2 float64 = 0.0

	tmp := subsequencepattern(bits, m-1)
	for i := m - 1; i < n+m-1; i++ {
		tmp <<= 1
		if bits[i%n] { // i % n is used to avoid appending m-1 bits in the end
			tmp++
		}
		patterns1[tmp&mask1]++
		patterns2[tmp&mask2]++
		patterns3[tmp&mask3]++
	}
	for i := 0; i <= mask1; i++ {
		Phi1 += float64(patterns1[i]) * float64(patterns1[i])
	}
	Phi1 *= float64(mask1 + 1)
	Phi1 /= float64(n)
	Phi1 -= float64(n)
	for i := 0; i <= mask2; i++ {
		Phi2 += float64(patterns2[i]) * float64(patterns2[i])
	}
	Phi2 *= float64(mask2 + 1)
	Phi2 /= float64(n)
	Phi2 -= float64(n)
	for i := 0; i <= mask3; i++ {
		Phi3 += float64(patterns3[i]) * float64(patterns3[i])
	}
	Phi3 *= float64(mask3 + 1)
	Phi3 /= float64(n)
	Phi3 -= float64(n)
	DPhi2 = Phi1 - Phi2
	D2Phi2 = Phi1 - 2*Phi2 + Phi3
	P1 = igamc(float64(len(patterns3)), DPhi2/2.0)
	P2 = igamc(float64(len(patterns3))/2.0, D2Phi2/2.0)
	Q1 = P1
	Q2 = P2
	debugPrintf("SerialTest (%d): P1-value: %.6f, Q1-value: %.6f", m, P1, Q1)
	isPValueOk(P1)
	isQValueOk(Q1)
	debugPrint("\n")
	debugPrintf("SerialTest (%d): P2-value: %.6f, Q2-value: %.6f", m, P2, Q2)
	isPValueOk(P2)
	isQValueOk(Q2)
	debugPrint("\n")
	return P1, P2, Q1, Q2
}

// SerialTest 重叠子序列检测方法 (byte)
func SerialTestBytes(data []byte, m int) (float64, float64, float64, float64) {
	return SerialTestBits(Byte2BitArr(data), m)
}
