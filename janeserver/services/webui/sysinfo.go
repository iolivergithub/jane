package webui

// SystemInfo describes this running instance of Jane. The values live in package main
// (VERSION, BUILD and RUNSESSION in janeserver.go), which this package cannot import,
// so main passes them in with SetSystemInfo before the Web UI starts.
type SystemInfo struct {
	Version    string
	Build      string
	RunSession string
}

var sysInfo = SystemInfo{Version: "unknown", Build: "unknown", RunSession: "unknown"}

// SetSystemInfo records the version, build and run session identifier shown on the
// home and about pages. Call it before StartWebUI.
func SetSystemInfo(version, build, runsession string) {
	sysInfo = SystemInfo{Version: version, Build: build, RunSession: runsession}
}
