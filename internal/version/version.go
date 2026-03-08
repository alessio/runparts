package version

import _ "embed"

//go:generate bash generate_version.sh
//go:embed version.txt
var version string

func Version() string {
	if len(version) == 0 {
		return "UNRELEASED"
	}
	return version
}
