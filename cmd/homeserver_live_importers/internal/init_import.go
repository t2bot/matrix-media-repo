package internal

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/t2bot/matrix-media-repo/common/assets"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/import_cmdline"
	"github.com/t2bot/matrix-media-repo/common/logging"
	"github.com/t2bot/matrix-media-repo/common/runtime"
	"github.com/t2bot/matrix-media-repo/util/ids"
	"golang.org/x/term"
)

type ImportOptsPsqlFlatFile struct {
	ServerName       string
	ApiUrl           string
	NumWorkers       int
	ConnectionString string
}

func InitImportPsqlMatrixDownload(softwareName string) *ImportOptsPsqlFlatFile {
	postgresHost := flag.String("dbHost", "localhost", fmt.Sprintf("The hostname for your %s PostgreSQL database.", softwareName))
	postgresPort := flag.Int("dbPort", 5432, fmt.Sprintf("The port for your %s PostgreSQL database.", softwareName))
	postgresUsername := flag.String("dbUsername", strings.ToLower(softwareName), fmt.Sprintf("The username for your %s PostgreSQL database.", softwareName))
	postgresPassword := flag.String("dbPassword", "", fmt.Sprintf("The password for your %s PostgreSQL database. Can be omitted to be prompted when run.", softwareName))
	postgresDatabase := flag.String("dbName", strings.ToLower(softwareName), fmt.Sprintf("The name of your %s PostgreSQL database.", softwareName))
	serverName := flag.String("serverName", "localhost", "The name of your homeserver (eg: matrix.org).")
	baseUrl := flag.String("baseUrl", "http://localhost:8008", "The base URL to access your homeserver with.")
	configPath := flag.String("config", "media-repo.yaml", "The path to the media repo configuration (configured for the media repo's database).")
	migrationsPath := flag.String("migrations", "./migrations", "The absolute path the media repo's migrations folder.")
	numWorkers := flag.Int("workers", 10, "The number of workers to use when downloading media. Using multiple workers is recommended.")
	flag.Parse()

	// Override config path with config for Docker users
	configEnv := os.Getenv("REPO_CONFIG")
	if configEnv != "" {
		configPath = &configEnv
	}

	config.Runtime.IsImportProcess = true // prevents us from creating media by accident
	config.Path = *configPath

	defer assets.Cleanup()
	assets.SetupMigrations(*migrationsPath)

	if ids.GetMachineId() == 0 {
		import_cmdline.AskMachineId()
	}

	var realPsqlPassword string
	if *postgresPassword == "" {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Println("Sorry, your terminal does not support reading passwords. Please supply a -dbPassword or use a different terminal.")
			fmt.Println("If you're on Windows, try using a plain Command Prompt window instead of a bash-like terminal.")
			os.Exit(1)
			return nil // for good measure
		}
		fmt.Printf("Postgres password: ")
		pass, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			panic(err)
		}
		realPsqlPassword = string(pass[:])
	} else {
		realPsqlPassword = *postgresPassword
	}

	err := logging.Setup(
		config.Get().General.LogDirectory,
		config.Get().General.LogColors,
		config.Get().General.JsonLogs,
		config.Get().General.LogLevel,
	)
	if err != nil {
		panic(err)
	}

	logrus.Info("Starting up...")
	runtime.RunStartupSequence()

	connectionString := "postgres://" + *postgresUsername + ":" + realPsqlPassword + "@" + *postgresHost + ":" + strconv.Itoa(*postgresPort) + "/" + *postgresDatabase + "?sslmode=disable"
	csApiUrl := *baseUrl
	if csApiUrl[len(csApiUrl)-1:] == "/" {
		csApiUrl = csApiUrl[:len(csApiUrl)-1]
	}

	return &ImportOptsPsqlFlatFile{
		ServerName:       *serverName,
		ApiUrl:           csApiUrl,
		NumWorkers:       *numWorkers,
		ConnectionString: connectionString,
	}
}
