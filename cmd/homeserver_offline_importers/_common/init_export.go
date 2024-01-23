package _common

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/t2bot/matrix-media-repo/common/assets"
	"github.com/t2bot/matrix-media-repo/common/config"
	"github.com/t2bot/matrix-media-repo/common/logging"
	"github.com/t2bot/matrix-media-repo/common/version"
	"golang.org/x/term"
)

type ImportOptsPsqlFlatFile struct {
	ServerName       string
	ExportPath       string
	PartSizeBytes    int64
	ImportPath       string
	SkipMissing      bool
	ConnectionString string
}

func InitExportPsqlFlatFile(softwareName string, softwareConfigDir string) *ImportOptsPsqlFlatFile {
	postgresHost := flag.String("dbHost", "localhost", fmt.Sprintf("The hostname for your %s PostgreSQL database.", softwareName))
	postgresPort := flag.Int("dbPort", 5432, fmt.Sprintf("The port for your %s PostgreSQL database.", softwareName))
	postgresUsername := flag.String("dbUsername", strings.ToLower(softwareName), fmt.Sprintf("The username for your %s PostgreSQL database.", softwareName))
	postgresPassword := flag.String("dbPassword", "", fmt.Sprintf("The password for your %s PostgreSQL database. Can be omitted to be prompted when run.", softwareName))
	postgresDatabase := flag.String("dbName", strings.ToLower(softwareName), fmt.Sprintf("The name of your %s PostgreSQL database.", softwareName))
	serverName := flag.String("serverName", "localhost", "The name of your homeserver (eg: matrix.org).")
	templatesPath := flag.String("templates", config.DefaultTemplatesPath, "The absolute path for the MMR templates folder.")
	exportPath := flag.String("destination", "./media-export", "The directory to export the files to (will be created if needed).")
	partSizeBytes := flag.Int64("partSize", 104857600, "The number of bytes (roughly) to split the export files into.")
	importPath := flag.String("mediaDirectory", "./media_store", fmt.Sprintf("The %s for %s.", softwareConfigDir, softwareName))
	skipMissing := flag.Bool("skipMissing", false, "If a media file can't be found, skip it.")
	debug := flag.Bool("debug", false, "Enables debug logging.")
	prettyLog := flag.Bool("prettyLog", false, "Enables pretty logging (colours).")
	flag.Parse()

	config.Runtime.IsImportProcess = true
	version.SetDefaults()
	version.Print(true)

	defer assets.Cleanup()
	assets.SetupTemplates(*templatesPath)

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

	level := "info"
	if *debug {
		level = "debug"
	}
	if err := logging.Setup(
		"-",
		*prettyLog,
		false,
		level,
	); err != nil {
		panic(err)
	}

	connectionString := "postgres://" + *postgresUsername + ":" + realPsqlPassword + "@" + *postgresHost + ":" + strconv.Itoa(*postgresPort) + "/" + *postgresDatabase + "?sslmode=disable"

	return &ImportOptsPsqlFlatFile{
		ServerName:       *serverName,
		ExportPath:       *exportPath,
		PartSizeBytes:    *partSizeBytes,
		ImportPath:       *importPath,
		SkipMissing:      *skipMissing,
		ConnectionString: connectionString,
	}
}
