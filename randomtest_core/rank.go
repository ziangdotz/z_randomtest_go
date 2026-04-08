package randomtest_core

import (
	"math"
)

func rowEchelon(matrix [][]int, m int) {
	pivotstartrow := 0
	pivotstartcol := 0
	pivotrow := 0
	for i := 0; i < m; i++ {
		found := false
		for k := pivotstartrow; k < m; k++ {
			if matrix[k][pivotstartcol] == 1 {
				found = true
				pivotrow = k
				break
			}
		}
		if found {
			if pivotrow != pivotstartrow {
				for k := 0; k < m; k++ {
					matrix[pivotrow][k] ^= matrix[pivotstartrow][k]
					matrix[pivotstartrow][k] ^= matrix[pivotrow][k]
					matrix[pivotrow][k] ^= matrix[pivotstartrow][k]
				}
			}
			for j := pivotstartrow + 1; j < m; j++ {
				if matrix[j][pivotstartcol] == 1 {
					for k := 0; k < m; k++ {
						matrix[j][k] = matrix[pivotstartrow][k] ^ matrix[j][k]
					}
				}
			}

			pivotstartcol += 1
			pivotstartrow += 1
		} else {
			pivotstartcol += 1
		}
	}
}

func rank(matrix [][]int, m int) int {
	temp := make([][]int, m)
	for i := 0; i < m; i++ {
		temp[i] = make([]int, m)
		for j := 0; j < m; j++ {
			temp[i][j] = matrix[i][j]
		}
	}
	rowEchelon(temp, m)
	rank := 0
	for i := 0; i < m; i++ {
		notZero := false
		for j := 0; j < m; j++ {
			if temp[i][j] != 0 {
				notZero = true
			}
		}
		if notZero {
			rank++
		}
	}
	return rank
}

// bits: 待检测序列
// R: 矩阵行隶属
// C: 矩阵列隶属
func RankProto(bits []bool, R, C int) (float64, float64) {
	n := len(bits)
	N := n / (R * C)
	if N == 0 {
		return 0.0, 0.0
	}

	var b bool
	matrix := make([][]int, 32)
	for i := 0; i < 32; i++ {
		matrix[i] = make([]int, 32)
	}
	var r int
	var Fm int = 0
	var Fm1 int = 0
	var Fr int = 0
	var V float64 = 0.0
	var P float64 = 0.0
	var Q float64 = 0.0

	for i := 0; i < N; i++ {
		for j := 0; j < R; j++ {
			for k := 0; k < C; k++ {
				b, bits = bits[0], bits[1:]
				if b {
					matrix[j][k] = 1
				} else {
					matrix[j][k] = 0
				}
			}
		}
		r = rank(matrix, R)
		if r == min(R, C) {
			Fm++
		} else if r == (min(R, C) - 1) {
			Fm1++
		} else {
			Fr++
		}
	}
	_N := float64(N)
	V = math.Pow(float64(Fm)-0.2888*_N, 2.0)/(0.2888*_N) +
		math.Pow(float64(Fm1)-0.5776*_N, 2.0)/(0.5776*_N) +
		math.Pow(float64(Fr)-0.1336*_N, 2.0)/(0.1336*_N)
	P = igamc(1, V/2.0)
	Q = P
	debugPrintf("RankTest: P-value: %.6f, Q-value: %.6f", P, Q)
	isPValueOk(P)
	isQValueOk(Q)
	debugPrint("\n")
	return P, Q
}

// RankTest 矩阵秩检测 (bit)
func RankTestBits(bits []bool, R, C int) (float64, float64) {
	return RankProto(bits, R, C)
}

// RankTest 矩阵秩检测 (byte)
func RankTestBytes(data []byte, R, C int) (float64, float64) {
	return RankProto(Byte2BitArr(data), R, C)
}
