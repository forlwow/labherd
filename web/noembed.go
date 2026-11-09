//go:build !webui

// Package web 把前端构建产物（web/dist）嵌入 Go 程序。
//
// 带 webui 构建标签时嵌入 dist/，需要先运行 make web-build；
// 不带标签时（只改后端、未安装 Node 的情况）不嵌入，Dist 返回 false。
package web

import "io/fs"

// Dist 返回构建好的前端文件；未嵌入时第二个返回值为 false。
func Dist() (fs.FS, bool) {
	return nil, false
}
