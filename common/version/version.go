package version

var GitCommit string
var Version string

func SetDefaults() {
	if GitCommit == "" {
		GitCommit = ".dev"
	}
	if Version == "" {
		Version = "unknown"
	}
}
