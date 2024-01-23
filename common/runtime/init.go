package runtime

import (
	"github.com/getsentry/sentry-go"
	"github.com/t2bot/matrix-media-repo/database"
	"github.com/t2bot/matrix-media-repo/datastores"
	"github.com/t2bot/matrix-media-repo/errcache"
	"github.com/t2bot/matrix-media-repo/pool"
	"github.com/t2bot/matrix-media-repo/redislib"
	"github.com/t2bot/matrix-media-repo/util/ids"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/rcontext"
	"github.com/t2bot/matrix-media-repo/common/version"
	"github.com/t2bot/matrix-media-repo/plugins"
)

func RunStartupSequence() {
	version.Print(true)
	CheckIdGenerator()
	config.PrintDomainInfo()
	config.CheckDeprecations()
	LoadDatabase()
	LoadDatastores()
	plugins.ReloadPlugins()
	pool.Init()
	errcache.Init()
	redislib.Reconnect()
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
			logrus.Errorf("See https://docs.t2bot.io/matrix-media-repo/%s/upgrading/130 for details", version.DocsVersion)
			fatal = true
		}
	}
	if fatal {
		logrus.Fatal("One or more datastores are not configured")
	}

	datastores.ResetS3Clients()
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
