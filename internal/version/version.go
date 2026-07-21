// Package version holds the application version, injectable at build time.
package version

// Version is the KitQA release version. Override at build time with:
//
//	go build -ldflags "-X github.com/wirsal/kitqa/internal/version.Version=v1.2.3" ./cmd/main.go
var Version = "v1.0.0"
