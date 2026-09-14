package randomtest_core

import (
	"fmt"
	"sync"
	"sync/atomic"
)

const (
	DefaultAlpha          = 0.01   // Alpha 显著性水平默认值
	DefaultAlphaT float64 = 0.0001 // AlphaT 分布均匀性的显著性水平默认值
)

var (
	Alpha    = DefaultAlpha     // 可由外部在运行前配置
	AlphaHAT = 1 - DefaultAlpha // 与 Alpha 保持联动
	AlphaT   = DefaultAlphaT    // 可由外部在运行前配置
)

var alphaConfigMu sync.RWMutex

var debugEnabled atomic.Bool

// SetAlphaValue 设置 Alpha 和 AlphaT（建议在检测任务开始前调用）。
func SetAlphaValue(alpha, alphaT float64) error {
	if alpha <= 0 || alpha >= 1 {
		return fmt.Errorf("alpha 必须在 (0,1) 区间内")
	}
	if alphaT <= 0 || alphaT >= 1 {
		return fmt.Errorf("alphaT 必须在 (0,1) 区间内")
	}
	alphaConfigMu.Lock()
	Alpha = alpha
	AlphaHAT = 1 - alpha
	AlphaT = alphaT
	alphaConfigMu.Unlock()
	return nil
}

// GetAlphaValue 返回当前 Alpha/AlphaT/AlphaHAT 配置。
func GetAlphaValue() (alpha, alphaT, alphaHAT float64) {
	alphaConfigMu.RLock()
	defer alphaConfigMu.RUnlock()
	return Alpha, AlphaT, AlphaHAT
}

// ResetAlphaValue 重置为文件内默认值。
func ResetAlphaValue() {
	alphaConfigMu.Lock()
	Alpha = DefaultAlpha
	AlphaHAT = 1 - DefaultAlpha
	AlphaT = DefaultAlphaT
	alphaConfigMu.Unlock()
}

// SetDebug 设置 randomtest_core 包内调试输出开关。
func SetDebug(enabled bool) {
	debugEnabled.Store(enabled)
	syncNistCDebug(enabled)
}

// IsDebug 返回 randomtest_core 包内调试输出开关状态。
func IsDebug() bool {
	return debugEnabled.Load()
}

func debugPrint(args ...interface{}) {
	if !IsDebug() {
		return
	}
	fmt.Print(args...)
}

func debugPrintf(format string, args ...interface{}) {
	if !IsDebug() {
		return
	}
	fmt.Printf(format, args...)
}
