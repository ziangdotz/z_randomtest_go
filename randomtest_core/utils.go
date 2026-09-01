package randomtest_core

import (
	"fmt"
	"math"
)

const (
	MAXLOG float64 = 7.09782712893383996732224e2 // log(MAXNUM)
	biginv float64 = 2.22044604925031308085e-16
	big    float64 = 4.503599627370496e15
	MACHEP float64 = 1.11022302462515654042e-16
)

func xor(x, y bool) bool {
	return x != y
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func abs(x int) int {
	if x > 0 {
		return x
	}
	return -x
}

func subsequencepattern(bits []bool, m int) int {
	tmp := 0
	var b bool
	for j := 0; j < m; j++ {
		tmp <<= 1
		b, bits = bits[0], bits[1:]
		if b {
			tmp++
		}
	}
	return tmp
}

func logGamma(x float64) float64 {
	res, sign := math.Lgamma(x)
	return res * float64(sign)
}

func igam(a, x float64) float64 {
	var ans, ax, c, r float64
	if (x <= 0) || (a <= 0) {
		return 0.0
	}
	if (x > 1.0) && (x > a) {
		return 1.e0 - igamc(a, x)
	}
	/* Compute  x**a * exp(-x) / gamma(a)  */
	ax = a*math.Log(x) - x - logGamma(a)
	if ax < -MAXLOG {
		return 0.0
	}
	ax = math.Exp(ax)
	/* power series */
	r = a
	c = 1.0
	ans = 1.0
	for {
		r += 1.0
		c *= x / r
		ans += c
		if !(c/ans > MACHEP) {
			break
		}
	}
	return ans * ax / a
}

func igamc(a, x float64) float64 {
	var ans, ax, c, yc, r, t, y, z float64
	var pk, pkm1, pkm2, qk, qkm1, qkm2 float64
	if (x <= 0) || (a <= 0) {
		return (1.0)
	}
	if (x < 1.0) || (x < a) {
		return (1.e0 - igam(a, x))
	}
	ax = a*math.Log(x) - x - logGamma(a)
	if ax < -MAXLOG {
		return 0.0
	}
	ax = math.Exp(ax)
	/* continued fraction */
	y = 1.0 - a
	z = x + y + 1.0
	c = 0.0
	pkm2 = 1.0
	qkm2 = x
	pkm1 = x + 1.0
	qkm1 = z * x
	ans = pkm1 / qkm1
	for {
		c += 1.0
		y += 1.0
		z += 2.0
		yc = y * c
		pk = pkm1*z - pkm2*yc
		qk = qkm1*z - qkm2*yc
		if qk != 0 {
			r = pk / qk
			t = math.Abs((ans - r) / r)
			ans = r
		} else {
			t = 1.0
		}
		pkm2 = pkm1
		pkm1 = pk
		qkm2 = qkm1
		qkm1 = qk
		if math.Abs(pk) > big {
			pkm2 *= biginv
			pkm1 *= biginv
			qkm2 *= biginv
			qkm1 *= biginv
		}
		if !(t > MACHEP) {
			break
		}
	}
	return ans * ax
}

func Bool2Int(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Bit2Byte 比特 (bool 数组) 转换为 字节
func Bit2Byte(arr []bool) byte {
	var res byte = 0
	var v byte = 0
	for _, b := range arr {
		res <<= 1
		if b {
			v = 1
		} else {
			v = 0
		}
		res += v
	}
	return res
}

// bitsExpandTable 字节 → 8 位大端序 0/1 展开查表 (阶段③热路径优化:
// 消除逐位循环移位/逐位 append, 一次查表完成字节展开)。
var bitsExpandTable [256][8]byte

func init() {
	for v := 0; v < 256; v++ {
		for j := 0; j < 8; j++ {
			bitsExpandTable[v][j] = byte((v >> (7 - j)) & 0x01)
		}
	}
}

// BytesToBitBytes 将字节流按大端位序展开为 0/1 字节流写入 dst
// (要求 len(dst) >= len(src)*8), 供 C 互操作等 0/1 字节表示场景复用。
func BytesToBitBytes(dst, src []byte) {
	for i, b := range src {
		copy(dst[i*8:(i+1)*8], bitsExpandTable[b][:])
	}
}

func Byte2BitArr(src []byte) []bool {
	res := make([]bool, len(src)*8)
	for i, b := range src {
		row := &bitsExpandTable[b]
		base := i * 8
		res[base+0] = row[0] != 0
		res[base+1] = row[1] != 0
		res[base+2] = row[2] != 0
		res[base+3] = row[3] != 0
		res[base+4] = row[4] != 0
		res[base+5] = row[5] != 0
		res[base+6] = row[6] != 0
		res[base+7] = row[7] != 0
	}
	return res
}

func Str2Bits(s string) []bool {
	bits := make([]bool, len(s))
	for i, ch := range s {
		if ch == '1' {
			bits[i] = true
		} else {
			bits[i] = false
		}
	}
	return bits
}

func Str2Bytes(s string) ([]byte, error) {
	result := make([]byte, len(s))
	for i, ch := range s {
		if ch == '1' {
			result[i] = 1
		} else if ch == '0' {
			result[i] = 0
		} else {
			return nil, fmt.Errorf("invalid character at position %d: %c", i, ch)
		}
	}
	return result, nil
}

// 验证 P 值是否通过检测
func isPValueOk(pValue float64) {
	if pValue >= Alpha && pValue <= 1 {
		debugPrint("\t✅ pValue Pass")
	} else {
		debugPrint("\t❌ pValue Fail")
	}
}

// 验证 Q 值是否通过检测
func isQValueOk(qValue float64) {
	if qValue >= AlphaT && qValue <= 1 {
		debugPrint("\t✅ qValue Pass")
	} else {
		debugPrint("\t❌ qValue Fail")
	}
}
