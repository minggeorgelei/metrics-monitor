// Package all aggregates blank imports of every built-in input plugin.
// main.go imports this package once; each individual plugin file in
// this directory carries its own build tag, so the set compiled in is
// chosen at build time:
//
//	go build ./...                                  → every plugin included (default)
//	go build -tags custom ./...                     → no plugins included
//	go build -tags 'custom inputs.cpu' ./...        → only cpu included
//
// To add a new input plugin: drop a one-line file here, e.g.
//
//	//go:build !custom || inputs.disk
//	package all
//	import _ "github.com/minggeorgelei/metrics-monitor/source/plugins/inputs/disk"
//
// This file has no build tag so that `package all` is always
// non-empty — even a fully-custom build with zero input plugins still
// compiles cleanly.
package all
