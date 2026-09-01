package randomtest_core

import (
	"embed"
	"fmt"
	"sync"
)

/*
非重叠模板匹配检测的模板文件经 go:embed 嵌入二进制, 不再依赖运行时文件系统路径
(-trimpath、单文件部署、交叉编译均安全)。

仅嵌入纯文本模板 template2~template16, 即 m 的支持范围为 [2,16];
template17~template21 为 .Z 压缩格式, 属历史遗留不可用资源, 不纳入。
*/

//go:embed templates/template2 templates/template3 templates/template4 templates/template5 templates/template6 templates/template7 templates/template8 templates/template9 templates/template10 templates/template11 templates/template12 templates/template13 templates/template14 templates/template15 templates/template16
var templateFS embed.FS

const (
	TemplateMinM = 2  // 支持的最小模板长度
	TemplateMaxM = 16 // 支持的最大模板长度
)

var (
	templateLoadOnce sync.Once
	templateRaw      map[int][]byte // m → 模板文件原始文本
	templateLoadErr  error
)

// loadTemplates 首次调用时解析嵌入的模板文件, 结果进程内复用。
func loadTemplates() (map[int][]byte, error) {
	templateLoadOnce.Do(func() {
		templateRaw = make(map[int][]byte, TemplateMaxM-TemplateMinM+1)
		for m := TemplateMinM; m <= TemplateMaxM; m++ {
			raw, err := templateFS.ReadFile(fmt.Sprintf("templates/template%d", m))
			if err != nil {
				templateLoadErr = fmt.Errorf("加载嵌入模板 template%d 失败: %w", m, err)
				templateRaw = nil
				return
			}
			templateRaw[m] = raw
		}
	})
	return templateRaw, templateLoadErr
}
