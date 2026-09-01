// Package version holds the application version, injectable at build time.
package version

// Version is the KitQAt release version. Release builds inject the real value from
// the git tag via -ldflags (see .github/workflows/release.yml); a plain local build
// shows "dev" so it is never mistaken for an official release.
//
//	go build -ldflags "-X github.com/wirsal/kitqat/internal/version.Version=v1.2.3" ./cmd/main.go
var Version = "dev"
