//go:build webembed

// embed.go 将构建脚本选择的前端文件编入业务二进制。
package webui

import (
	"embed"
	"io/fs"
)

// bundled 只包含白名单构建目录，不读取运行时工作目录。
//
//go:embed assets
var bundled embed.FS

// files 返回静态资源根目录，编译器保证 assets 已存在。
func files() fs.FS {
	root, err := fs.Sub(bundled, "assets")
	if err != nil {
		return nil
	}
	return root
}
