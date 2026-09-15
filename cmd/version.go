package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	// Version is the current version of agy-sync, set at build time via ldflags.
	Version = "dev"
	// Commit is the git commit sha, set at build time via ldflags.
	Commit = "none"
	// Date is the build date, set at build time via ldflags.
	Date = "unknown"
	// BuiltBy is the tool/user that built the binary.
	BuiltBy = "unknown"
)

// BuildInfo captures runtime and build metadata.
type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	BuiltBy   string `json:"built_by"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// GetBuildInfo resolves the build details combining ldflags and debug.ReadBuildInfo.
func GetBuildInfo() BuildInfo {
	info := BuildInfo{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		BuiltBy:   BuiltBy,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}

	if bi, ok := debug.ReadBuildInfo(); ok {
		if info.Version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			info.Version = bi.Main.Version
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if info.Commit == "none" {
					info.Commit = s.Value
				}
			case "vcs.time":
				if info.Date == "unknown" {
					info.Date = s.Value
				}
			}
		}
	}

	return info
}

func newVersionCommand() *cobra.Command {
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Display version, commit hash, and build information",
		Long:  `Outputs binary build metadata including release tag, commit hash, build date, and Go runtime environment.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			info := GetBuildInfo()

			if globalOpts.JSON {
				data, err := json.MarshalIndent(info, "", "  ")
				if err != nil {
					return fmt.Errorf("failed encoding version info: %w", err)
				}
				cmd.Println(string(data))
				return nil
			}

			cmd.Printf("agy-sync version %s (commit: %s, built at: %s, go: %s, platform: %s/%s)\n",
				info.Version,
				info.Commit,
				info.Date,
				info.GoVersion,
				info.OS,
				info.Arch,
			)
			return nil
		},
	}

	return versionCmd
}
