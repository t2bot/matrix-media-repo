package version

import (
	"fmt"
	"runtime/debug"

	"github.com/sirupsen/logrus"
)

var GitCommit string
var Version string

// DocsVersion The version number used by docs.t2bot.io links throughout the application runtime
const DocsVersion = "v1.3.3"

func SetDefaults() {
	build, infoOk := debug.ReadBuildInfo()

	if GitCommit == "" {
		GitCommit = ".dev"
		if infoOk {
			for _, setting := range build.Settings {
				if setting.Key == "vcs.revision" {
					GitCommit = setting.Value
					break
				}
			}
		}
	}

	if Version == "" {
		Version = "unknown"
	}
}

func Print(usingLogger bool) {
	SetDefaults()

	if usingLogger {
		logrus.Info("Version: " + Version)
		logrus.Info("Commit: " + GitCommit)
	} else {
		fmt.Println("Version: " + Version)
		fmt.Println("Commit: " + GitCommit)
	}
}
