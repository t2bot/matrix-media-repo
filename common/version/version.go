package version

import (
	"fmt"
	"runtime/debug"

	"github.com/sirupsen/logrus"
)

var GitCommit string
var Version string

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
