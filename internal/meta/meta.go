package meta

const (
	ToolName              = "kforge"
	Vendor                = "KhmerStack / Ing Muyleang"
	URL                   = "https://github.com/MuyleangIng/kforge"
	DefaultReleaseVersion = "v1.1.0"
)

// Version is overridden at build time via:
//
//	-ldflags "-X github.com/MuyleangIng/kforge/internal/meta.Version=vX.Y.Z"
var Version = "dev"

func DisplayVersion() string {
	return Version
}

func DownloadVersion() string {
	if Version == "" || Version == "dev" {
		return DefaultReleaseVersion
	}
	return Version
}
