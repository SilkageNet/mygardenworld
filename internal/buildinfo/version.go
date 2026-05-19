package buildinfo

import "runtime/debug"

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func GetVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return Version
}
