package main

import (
	"cmp"
	"runtime/debug"
	"strings"
)

// version is set at release build time via
// -ldflags "-X main.version=<tag>".
var version string

// buildVersion returns the version of the running binary, without a leading "v".
func buildVersion() string {
	var moduleVersion string
	if info, ok := debug.ReadBuildInfo(); ok {
		moduleVersion = info.Main.Version
	}
	return resolveVersion(version, moduleVersion)
}

// resolveVersion picks the first available version source: the value embedded
// via ldflags, then the module version recorded by the Go toolchain
// (go install ...@vX.Y.Z, or VCS info for local builds), then "dev".
func resolveVersion(ldflagsVersion, moduleVersion string) string {
	if moduleVersion == "(devel)" {
		moduleVersion = ""
	}
	return strings.TrimPrefix(cmp.Or(ldflagsVersion, moduleVersion, "dev"), "v")
}
