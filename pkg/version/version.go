package version

import (
	"fmt"
	"runtime"
)

// Version information set via build flags
var (
	Version   = "v0.1.0"
	GitCommit = "unknown"
	BuildDate = "unknown"
	GoVersion = runtime.Version()
	Platform  = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

// BuildInfo contains all the build information
type BuildInfo struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// GetBuildInfo returns the build information
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		GoVersion: GoVersion,
		Platform:  Platform,
	}
}

// String returns a formatted version string
func (b BuildInfo) String() string {
	return fmt.Sprintf("rag-cli %s (commit: %s, built: %s, go: %s, platform: %s)",
		b.Version, b.GitCommit, b.BuildDate, b.GoVersion, b.Platform)
}
