package randomtest_core

// RunsDistributionTest 游程分布检测 (bit)
// bits: 待检测序列
func RunsDistributionTestBits(bits []bool) (float64, float64) {
	n := len(bits)
	if n < 100 {
		return 0.0, 0.0
	}

	var V float64 = 0.0
	var P float64 = 0.0
	var Q float64 = 0.0

	k := 0
	for {
		k++
		_2k2 := 1 << uint(k+2)
		if float64(n-k+3)/float64(_2k2) < 5.0 {
			break
		}
	}
	k--
	e := make([]float64, k)
	b := make([]float64, k)
	g := make([]float64, k)
	var cur bool = bits[0]
	cnt := 0
	for i := 0; i < n; i++ {
		if bits[i] == cur {
			cnt++
		} else {
			if cnt > k {
				cnt = k
			}
			if cur {
				b[cnt-1]++
			} else {
				g[cnt-1]++
			}
			cur = bits[i]
			cnt = 1
		}
	}
	if cnt > k { // 特殊处理结尾
		cnt = k
	}
	if cur {
		b[cnt-1]++
	} else {
		g[cnt-1]++
	}
	var T float64 = 0
	for i := 0; i < k; i++ {
		T += b[i] + g[i]
	}
	for i := 0; i < k; i++ {
		if i < k-1 {
			e[i] = T / float64(int(1)<<uint(i+2))
		} else {
			e[i] = T / float64(int(1)<<uint(i+1))
		}
	}
	for i := 0; i < k; i++ {
		V += (b[i] - e[i]) * (b[i] - e[i]) / e[i]
		V += (g[i] - e[i]) * (g[i] - e[i]) / e[i]
	}
	P = igamc(float64(k-1), V/2.0)
	Q = P
	debugPrintf("RunsDistributionTest: P-value: %.6f, Q-value: %.6f", P, Q)
	isPValueOk(P)
	isQValueOk(Q)
	debugPrint("\n")
	return P, Q
}

// RunsDistributionTest 游程分布检测 (byte)
func RunsDistributionTestBytes(data []byte) (float64, float64) {
	return RunsDistributionTestBits(Byte2BitArr(data))
}
