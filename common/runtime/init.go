package runtime

import (
	"github.com/getsentry/sentry-go"
	"github.com/turt2live/matrix-media-repo/database"
	"github.com/turt2live/matrix-media-repo/util/ids"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/common/version"
	"github.com/turt2live/matrix-media-repo/plugins"
)

func RunStartupSequence() {
	version.Print(true)
	CheckIdGenerator()
	config.PrintDomainInfo()
	config.CheckDeprecations()
	LoadDatabase()
	LoadDatastores()
	plugins.ReloadPlugins()
}

func LoadDatabase() {
	logrus.Info("Preparing database...")
	database.GetInstance()
}

func LoadDatastores() {
	mediaDb := database.GetInstance().Media.Prepare(rcontext.Initial())

	logrus.Info("Comparing datastores against config...")
	storeIds, err := mediaDb.GetDistinctDatastoreIds()
	if err != nil {
		sentry.CaptureException(err)
		logrus.Fatal(err)
	}

	dsMap := make(map[string]bool)
	for _, id := range storeIds {
		dsMap[id] = false
	}
	for _, ds := range config.UniqueDatastores() {
		dsMap[ds.Id] = true
	}
	fatal := false
	for id, found := range dsMap {
		if !found {
			logrus.Errorf("No configured datastore for ID %s found - please check your configuration and restart.", id)
			fatal = true
		}
	}
	if fatal {
		logrus.Fatal("One or more datastores are not configured")
	}
}

func CheckIdGenerator() {
	// Create a throwaway ID to ensure no errors
	_, err := ids.NewUniqueId()
	if err != nil {
		panic(err)
	}

	id := ids.GetMachineId()
	logrus.Infof("Running as machine %d for ID generation. This ID must be unique within your cluster.", id)
}
