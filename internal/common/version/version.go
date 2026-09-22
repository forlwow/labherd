// Package version 保存构建时注入的版本号。
package version

// Version 由 Makefile 通过 -ldflags 注入，默认值为 dev。
var Version = "dev"
