// Package version reports the build version of the running MyKnownWorld process.
// It is the single source for the version advertised in the MCP handshake.
package version

import "os"

// Version is the version compiled into the binary. Release builds override it:
//
//	go build -ldflags "-X github.com/QuietSugar/my-known-world/internal/version.Version=0.1.0"
//
// A source checkout that was not built with ldflags reports "dev".
var Version = "dev"

// EnvVar overrides the compiled-in version at runtime. Compose deployments use it
// so a container started from a published image reports the same version as the
// image tag without rebuilding.
const EnvVar = "MYKNOWNWORLD_VERSION"

// Current returns the effective version: EnvVar when set and non-empty,
// otherwise the value linked into the binary, otherwise "dev".
func Current() string {
	if v := os.Getenv(EnvVar); v != "" {
		return v
	}
	if Version != "" {
		return Version
	}
	return "dev"
}
