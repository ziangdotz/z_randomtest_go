package randomtest_core

// 阶段③热路径基准 (计划 3.4):
//
//	go test -bench . -benchtime 20x
//
// 覆盖 Frequency / DFT / LinearComplexity(500) / NonOverlapping(m=9) / Rank 五个代表性检测,
// 用于位展开查表 (3.1) 与线性复杂度缓冲池化 (3.2) 的前后对比。
import "testing"

// benchData 1Mbit (125000 字节) 确定性随机流 (与 golden 测试同源)。
func benchData() []byte {
	return goldenRandomData(0xB5)
}

func BenchmarkFrequencyTest(b *testing.B) {
	data := benchData()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FrequencyTestBytes(data)
	}
}

func BenchmarkDiscreteFourierTransform(b *testing.B) {
	data := benchData()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DiscreteFourierTransformTestBytes(data)
	}
}

func BenchmarkLinearComplexity500(b *testing.B) {
	data := benchData()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LinearComplexityTestBytes(data, 500)
	}
}

func BenchmarkNonOverlappingTemplateMatchingM9(b *testing.B) {
	data := benchData()
	n := len(data) * 8
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NonOverlappingTemplateMatchingTestBytes(data, n, 9)
	}
}

func BenchmarkRank32x32(b *testing.B) {
	data := benchData()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RankTestBytes(data, 32, 32)
	}
}
