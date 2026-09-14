// Package report 收敛 CLI / 桌面端 / Web 三端共享的随机性检测判定与报表生成逻辑:
// 样本数阈值 (比例评估法)、单文件检测项判定、统计表与失败清单行生成、ini 路径清洗。
// 渲染与投递仍由各前端负责 (CLI 打印 stdout、桌面端转日志事件、Web 转 JSON 响应)。
package report

import (
	"fmt"
	"math"
	"randomtest_core"
	"strings"
)

/* ---------- 样本数阈值 (NIST SP800-22 比例评估法) ---------- */

// PassCountRangeWithAlpha 按给定显著性水平 alpha 计算跨样本通过数可接受区间 [min, max]
// (3σ 置信区间): max 取 Floor、min 取 Ceil, 边界样本 (如 980/1000) 按统计学口径判为不通过。
// sampleSize<=0 或 alpha 非法时返回 (0, 0)。
func PassCountRangeWithAlpha(alpha float64, sampleSize int) (max, min int) {
	if sampleSize <= 0 || alpha <= 0 || alpha >= 1 {
		return 0, 0
	}
	alphaHAT := 1 - alpha
	n := float64(sampleSize)
	margin := 3.0 * math.Sqrt((alphaHAT*alpha)/n)
	return int(math.Floor((alphaHAT + margin) * n)), int(math.Ceil((alphaHAT - margin) * n))
}

// PassCountRange 按当前全局 Alpha 配置计算通过数可接受区间 [min, max]。
func PassCountRange(sampleSize int) (max, min int) {
	alpha, _, _ := randomtest_core.GetAlphaValue()
	return PassCountRangeWithAlpha(alpha, sampleSize)
}

// CountInRange 判定通过数是否落在阈值区间内。
// sampleSize<=0 (如 RE 子项无适用样本) 视为不可判定, 返回 false。
func CountInRange(passCount, sampleSize int) bool {
	if sampleSize <= 0 {
		return false
	}
	max, min := PassCountRange(sampleSize)
	return passCount >= min && passCount <= max
}

// MinimumPassCount 最小通过样本数 (CountInRange 的下界), 用于前端表单提示。
func MinimumPassCount(sampleSize int) int {
	_, min := PassCountRange(sampleSize)
	return min
}

// FormatPassFraction 格式化 "通过数/样本数" 计数文本。
func FormatPassFraction(passCount, sampleSize int) string {
	return fmt.Sprintf("%d/%d", passCount, sampleSize)
}

/* ---------- 单文件判定 (目录模式) ---------- */

// ItemVerdict 单文件单检测项判定结果。
// QApplicable 显式区分 "Q 值不适用 (NIST 套件不参与判定)" 与 "Q 值不通过",
// 避免两者在数据层共用同一个 false。
type ItemVerdict struct {
	PPassed     bool
	QPassed     bool // 仅当 QApplicable == true 时有意义
	QApplicable bool
}

// JudgeFileItem 目录模式单文件判定, 沿用既有 "PCount/QCount 前后增量" 语义:
//   - RequireAll (SerialTest): 双 P (及 GM 的双 Q) 全过才算通过
//   - 单值检测: p/q 通过即通过
//   - 多值检测 (RE/REV/NonOverlapping): 旧实现 PCount/QCount 不递增, 恒判定为不通过 (保持既有行为)
//   - NIST 套件 (gm=false): Q 值不参与判定, QApplicable=false
func JudgeFileItem(o randomtest_core.Outcome, gm bool) ItemVerdict {
	v := ItemVerdict{QApplicable: gm}
	switch {
	case o.RequireAll:
		v.PPassed = randomtest_core.AllPValuesPassed(o.P)
		if gm {
			v.QPassed = randomtest_core.AllQValuesPassed(o.Q)
		}
	case len(o.P) == 1:
		v.PPassed = randomtest_core.PValuePassed(o.P[0])
		if gm && len(o.Q) == 1 {
			v.QPassed = randomtest_core.QValuePassed(o.Q[0])
		}
	}
	return v
}

// FileDetail 目录模式单文件单检测项判定明细 (失败清单的输入)。
type FileDetail struct {
	FileName    string
	PPassed     bool
	QPassed     bool // 仅当 QApplicable == true 时有意义
	QApplicable bool
}

// NewFileDetail 判定一次检测结果并生成文件级明细。
func NewFileDetail(fileName string, o randomtest_core.Outcome, gm bool) FileDetail {
	v := JudgeFileItem(o, gm)
	return FileDetail{
		FileName:    fileName,
		PPassed:     v.PPassed,
		QPassed:     v.QPassed,
		QApplicable: v.QApplicable,
	}
}

// Failed 明细是否判为不通过 (Q 仅在适用时参与判定)。
func (d FileDetail) Failed() bool {
	return !d.PPassed || (d.QApplicable && !d.QPassed)
}

// FailReason 返回失败原因文本 (如 "P不通过 Q不通过 "), 通过时返回空串。
func (d FileDetail) FailReason() string {
	if !d.Failed() {
		return ""
	}
	reason := ""
	if !d.PPassed {
		reason += "P不通过 "
	}
	if d.QApplicable && !d.QPassed {
		reason += "Q不通过 "
	}
	return reason
}

/* ---------- 报表行 ---------- */

// ItemRow 报表行: 单值检测项一行; NIST 多值检测项按子项展开 (GM 套件无多值项, 不展开)。
type ItemRow struct {
	Name        string // 检测项名 (不含子项下标后缀)
	Multi       bool   // 是否多值子项行
	SubIndex    int    // 子项下标 (Multi 时有意义)
	PCount      int    // P 通过计数
	SampleSize  int    // 统计分母 (RE/REV 为适用计数, 其余为样本数)
	QCount      int    // Q 通过计数 (仅 QApplicable 时有意义)
	QApplicable bool   // Q 是否参与判定 (NIST=false)
	PPassed     bool   // P 计数是否落在阈值区间
	QPassed     bool   // Q 计数是否落在阈值区间 (仅 QApplicable 时有意义)
}

// BuildRows 将聚合统计展开为报表行:
//   - gm=true: 每检测项一行, P/Q 以 testNum 为分母
//   - gm=false (NIST): 多值检测按子项展开, RE/REV 以适用计数 ValuesNum[i] 为分母
//     (p>0 适用语义), 其余多值项 (NonOverlappingTemplateMatching) 以 testNum 为分母; Q 不适用
func BuildRows(testNum int, stats []randomtest_core.ItemStat, gm bool) []ItemRow {
	rows := make([]ItemRow, 0, len(stats))
	for _, item := range stats {
		if !gm && item.ValuesNum != nil {
			isRandomExcursions := item.Name == "RandomExcursionsTest" || item.Name == "RandomExcursionsVariantTest"
			for i, pCount := range item.PCounts {
				sampleSize := testNum
				if isRandomExcursions {
					sampleSize = item.ValuesNum[i]
				}
				rows = append(rows, ItemRow{
					Name:        item.Name,
					Multi:       true,
					SubIndex:    i,
					PCount:      pCount,
					SampleSize:  sampleSize,
					QApplicable: false,
					PPassed:     CountInRange(pCount, sampleSize),
				})
			}
			continue
		}
		rows = append(rows, ItemRow{
			Name:        item.Name,
			PCount:      item.PCount,
			SampleSize:  testNum,
			QCount:      item.QCount,
			QApplicable: gm,
			PPassed:     CountInRange(item.PCount, testNum),
			QPassed:     gm && CountInRange(item.QCount, testNum),
		})
	}
	return rows
}

/* ---------- 统计表与失败清单 ---------- */

// StatsTableLines 生成测试结果统计表文本行 (首行为空行用于排版, 行内容不含换行符)。
// gm=true 使用 GM 双列模板 (85 列宽), 否则使用 NIST 单列模板 (75 列宽)。
func StatsTableLines(level, testNum int, stats []randomtest_core.ItemStat, gm bool) []string {
	rows := BuildRows(testNum, stats, gm)
	lines := make([]string, 0, len(rows)+6)
	lines = append(lines, "")
	if gm {
		lines = append(lines,
			fmt.Sprintf("📊 GM Level%d 测试结果统计", level),
			strings.Repeat("-", 85),
			fmt.Sprintf("%-35s | %21s | %21s", "TestFuncName", "PValue Passed-Count", "QValue Passed-Count"),
			strings.Repeat("-", 85),
		)
		for _, row := range rows {
			pMark := "  "
			if !row.PPassed {
				pMark = " *"
			}
			qMark := "  "
			if !row.QPassed {
				qMark = " *"
			}
			lines = append(lines, fmt.Sprintf("%-35s | %9d/%-9d%2s | %9d/%-9d%2s",
				row.Name, row.PCount, row.SampleSize, pMark, row.QCount, row.SampleSize, qMark))
		}
		lines = append(lines, strings.Repeat("-", 85))
		return lines
	}
	lines = append(lines,
		fmt.Sprintf("📊 NIST Level%d 测试结果统计", level),
		strings.Repeat("-", 75),
		fmt.Sprintf("%-45s | %20s", "TestFuncName", "PValue Passed-Count"),
		strings.Repeat("-", 75),
	)
	for _, row := range rows {
		mark := " "
		if !row.PPassed {
			mark = "*"
		}
		if row.Multi {
			lines = append(lines, fmt.Sprintf("%-41s %3d | %9d/%-9d %s",
				row.Name, row.SubIndex, row.PCount, row.SampleSize, mark))
			continue
		}
		lines = append(lines, fmt.Sprintf("%-45s | %9d/%-9d %s",
			row.Name, row.PCount, row.SampleSize, mark))
	}
	lines = append(lines, strings.Repeat("-", 75))
	return lines
}

// ErrorFileListLines 生成目录模式 "不通过的文件清单" 文本行 (首行为空行用于排版, 行内容不含换行符)。
// 判定以明细自带的 QApplicable 为准: NIST 仅按 P 判定, GM 按 P/Q 双判定。
func ErrorFileListLines(fileDetails [][]FileDetail, stats []randomtest_core.ItemStat) []string {
	lines := make([]string, 0, 4*len(stats)+3)
	lines = append(lines, "", "❌ NIST or GM 测试不通过的文件清单", strings.Repeat("=", 85))
	for i, item := range stats {
		hasFail := false
		lines = append(lines, fmt.Sprintf("测试项: %s", item.Name))
		if i < len(fileDetails) {
			for _, detail := range fileDetails[i] {
				if !detail.Failed() {
					continue
				}
				lines = append(lines, fmt.Sprintf("  - [失败] %-30s: %s", detail.FileName, detail.FailReason()))
				hasFail = true
			}
		}
		if !hasFail {
			lines = append(lines, "  - 所有文件均通过")
		}
		lines = append(lines, strings.Repeat("-", 40))
	}
	return lines
}

/* ---------- 配置清洗 ---------- */

// TrimIniQuotes 剥离 ini 值残留的首尾引号 (如误写为 ""path"" 时解析残留的内层引号),
// 避免路径混入引号字符导致打开失败。
func TrimIniQuotes(s string) string {
	s = strings.TrimSpace(s)
	for len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

// SanitizePaths 批量清理配置路径字段 (nil 指针跳过)。
func SanitizePaths(paths ...*string) {
	for _, p := range paths {
		if p != nil {
			*p = TrimIniQuotes(*p)
		}
	}
}

/* ---------- 单样本值阈值判定 (Web 端) ---------- */

// ValuesAllPassed 判定一组检测值是否全部通过: 空切片视为不适用 (通过),
// NaN/Inf 判为不通过, 其余需满足 threshold <= v <= 1。
func ValuesAllPassed(values []float64, threshold float64) bool {
	if len(values) == 0 {
		return true
	}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
		if value < threshold || value > 1 {
			return false
		}
	}
	return true
}
