//go:build !webembed

// files.go 让纯 API 开发与单元测试无需预先构建前端。
package webui

import "io/fs"

// files 在非发布构建中禁用静态页面，不回退到本机磁盘。
func files() fs.FS { return nil }
