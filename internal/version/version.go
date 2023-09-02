package version

import _ "embed"

//go:generate bash generate_version.sh
//go:embed version.txt
var Version string
