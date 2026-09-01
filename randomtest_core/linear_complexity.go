package randomtest_core

import (
	"math"
	"sync"
)

// lcBuffers 线性复杂度检测的工作缓冲 (阶段③: 池化复用, 消除每块 4 次 []int 分配)。
type lcBuffers struct {
	B_, C, P, T []int
}

var lcBufPool = sync.Pool{New: func() interface{} { return new(lcBuffers) }}

// ensure 保证四个缓冲容量足够 (不足时整体重新分配), 并截取到所需长度。
func (b *lcBuffers) ensure(M int) {
	if cap(b.B_) < M {
		b.B_ = make([]int, M)
		b.C = make([]int, M)
		b.P = make([]int, M)
		b.T = make([]int, M)
		return
	}
	b.B_ = b.B_[:M]
	b.C = b.C[:M]
	b.P = b.P[:M]
	b.T = b.T[:M]
}

func linearComplexity(a []bool, M int) int {
	var N_ int = 0
	var L int = 0
	var m int = -1
	var d int = 0

	bufs := lcBufPool.Get().(*lcBuffers)
	defer lcBufPool.Put(bufs)
	bufs.ensure(M)
	B_, C, P, T := bufs.B_, bufs.C, bufs.P, bufs.T

	for i := 0; i < M; i++ {
		B_[i] = 0
		C[i] = 0
		T[i] = 0
		P[i] = 0
	}
	C[0] = 1
	B_[0] = 1
	for {
		if !(N_ < M) {
			break
		}
		d = Bool2Int(a[N_])
		for i := 1; i <= L; i++ {
			d += C[i] * Bool2Int(a[N_-i])
		}
		d = d % 2
		if d == 1 {
			for i := 0; i < M; i++ {
				T[i] = C[i]
				P[i] = 0
			}
			for j := 0; j < M; j++ {
				if B_[j] == 1 {
					P[j+N_-m] = 1
				}
			}
			for i := 0; i < M; i++ {
				C[i] = (C[i] + P[i]) % 2
			}
			if L <= N_/2 {
				L = N_ + 1 - L
				m = N_
				for i := 0; i < M; i++ {
					B_[i] = T[i]
				}
			}
		}
		N_++
	}
	return L
}

// bits: 待检测序列
// m:
func LinearComplexityProto(bits []bool, m int) (float64, float64) {
	n := len(bits)
	N := n / m
	if N == 0 {
		return 0.0, 0.0
	}

	var arr []bool
	var complexity int
	var T float64
	v := []float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}
	var V float64 = 0.0
	var P float64 = 0.0
	var Q float64 = 0.0
	pi := []float64{0.010417, 0.03125, 0.12500, 0.5000, 0.25000, 0.06250, 0.020833}

	arr = make([]bool, m)
	var _1_m float64
	if m%2 == 0 {
		_1_m = 1.0
	} else {
		_1_m = -1.0
	}
	miu := float64(m)/2.0 + (9.0+_1_m)/36.0 - (float64(m)/3.0+2.0/9.0)/math.Pow(2.0, float64(m))
	for i := 0; i < N; i++ {
		for j := 0; j < m; j++ {
			arr[j], bits = bits[0], bits[1:]
		}
		complexity = linearComplexity(arr, m)
		if m%2 == 0 {
			_1_m = 1.0
		} else {
			_1_m = -1.0
		}
		T = _1_m*(float64(complexity)-miu) + 2.0/9.0
		if T <= -2.5 {
			v[0]++
		} else if T <= -1.5 {
			v[1]++
		} else if T <= -0.5 {
			v[2]++
		} else if T <= 0.5 {
			v[3]++
		} else if T <= 1.5 {
			v[4]++
		} else if T <= 2.5 {
			v[5]++
		} else {
			v[6]++
		}
	}
	for i := 0; i < 7; i++ {
		V += math.Pow(v[i]-float64(N)*pi[i], 2.0) / (float64(N) * pi[i])
	}
	P = igamc(3.0, V/2.0)
	Q = P
	debugPrintf("LinearComplexityTest (%d): P-value: %.6f, Q-value: %.6f", m, P, Q)
	isPValueOk(P)
	isQValueOk(Q)
	debugPrint("\n")
	return P, Q
}

// LinearComplexityTest 线型复杂度检测 (bit)
func LinearComplexityTestBits(bits []bool, m int) (float64, float64) {
	return LinearComplexityProto(bits, m)
}

// LinearComplexityTest 线型复杂度检测 (byte)
func LinearComplexityTestBytes(data []byte, m int) (float64, float64) {
	return LinearComplexityProto(Byte2BitArr(data), m)
}
