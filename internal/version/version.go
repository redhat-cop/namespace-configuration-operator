/*
Copyright 2020 Red Hat Community of Practice.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package version

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

var (
	// Version is the version of the operator (set via ldflags during build, or from VCS)
	// Defaults to "0.0.1" if not set (matches Makefile VERSION default)
	Version = "0.0.1"
	// Commit is the git commit hash (set via ldflags during build, or from VCS)
	Commit = "unknown"
	// BuildDate is the build date (set via ldflags during build)
	BuildDate = "unknown"
)

// GetCommitHash attempts to get the git commit hash
func GetCommitHash() string {
	if Commit != "unknown" && Commit != "" {
		return Commit
	}
	// Try to get from Go's build info (Go 1.18+)
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				if len(setting.Value) >= 7 {
					return setting.Value[:7] // Short commit hash
				}
				return setting.Value
			}
		}
	}
	return "unknown"
}

// GetVersion returns the version string
func GetVersion() string {
	if Version != "" && Version != "0.0.1" {
		return Version
	}
	// Try to get from Go's build info (Go 1.18+)
	if info, ok := debug.ReadBuildInfo(); ok {
		// Check for version in build info
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
		// Try to get from VCS tag
		for _, setting := range info.Settings {
			if setting.Key == "vcs.tag" && setting.Value != "" {
				return strings.TrimPrefix(setting.Value, "v") // Remove 'v' prefix if present
			}
		}
	}
	return "0.0.1"
}

// GetBuildDate returns the build date
func GetBuildDate() string {
	if BuildDate != "unknown" && BuildDate != "" {
		return BuildDate
	}
	// Try to get from Go's build info (Go 1.18+)
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.time" {
				if t, err := time.Parse(time.RFC3339, setting.Value); err == nil {
					return t.Format("2006-01-02T15:04:05Z")
				}
				return setting.Value
			}
		}
	}
	return "N/A"
}

// PrintStartupBanner prints a large, unmissable startup banner
func PrintStartupBanner() {
	version := GetVersion()
	commit := GetCommitHash()
	buildDate := GetBuildDate()

	// Create a big banner
	banner := fmt.Sprintf(`
╔══════════════════════════════════════════════════════════════════════════════╗
║                                                                              ║
║                    NAMESPACE CONFIGURATION OPERATOR                          ║
║                                                                              ║
╠══════════════════════════════════════════════════════════════════════════════╣
║                                                                              ║
║  VERSION:  %-66s║
║  COMMIT:   %-66s║
║  BUILD:    %-66s║
║                                                                              ║
╚══════════════════════════════════════════════════════════════════════════════╝
`,
		truncate(version, 66),
		truncate(commit, 66),
		truncate(buildDate, 66))

	// Print to stderr so it's always visible even if stdout is redirected
	fmt.Fprint(os.Stderr, banner)
	fmt.Fprint(os.Stderr, "\n")
}

// truncate truncates a string to the specified length
func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	// Pad with spaces to ensure consistent width
	return s + strings.Repeat(" ", maxLen-len(s))
}
