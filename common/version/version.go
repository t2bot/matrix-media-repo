package version

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

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
