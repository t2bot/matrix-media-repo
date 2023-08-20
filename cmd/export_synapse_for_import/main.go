package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/turt2live/matrix-media-repo/archival"
	"github.com/turt2live/matrix-media-repo/archival/v2archive"
	"github.com/turt2live/matrix-media-repo/common/assets"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/logging"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/common/version"
	"github.com/turt2live/matrix-media-repo/homeserver_interop/synapse"
	"github.com/turt2live/matrix-media-repo/util"
	"golang.org/x/term"
)

func main() {
	postgresHost := flag.String("dbHost", "localhost", "The PostgresSQL hostname for your Synapse database")
	postgresPort := flag.Int("dbPort", 5432, "The port for your Synapse's PostgreSQL database")
	postgresUsername := flag.String("dbUsername", "synapse", "The username for your Synapse's PostgreSQL database")
	postgresPassword := flag.String("dbPassword", "", "The password for your Synapse's PostgreSQL database. Can be omitted to be prompted when run")
	postgresDatabase := flag.String("dbName", "synapse", "The name of your Synapse database")
	serverName := flag.String("serverName", "localhost", "The name of your homeserver (eg: matrix.org)")
	templatesPath := flag.String("templates", config.DefaultTemplatesPath, "The absolute path for the templates folder")
	exportPath := flag.String("destination", "./media-export", "The directory to export the files to (will be created if needed)")
	importPath := flag.String("mediaDirectory", "./media_store", "The media_store_path for Synapse")
	partSizeBytes := flag.Int64("partSize", 104857600, "The number of bytes (roughly) to split the export files into.")
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
			return // for good measure
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

	ctx := rcontext.InitialNoConfig()

	connectionString := "postgres://" + *postgresUsername + ":" + realPsqlPassword + "@" + *postgresHost + ":" + strconv.Itoa(*postgresPort) + "/" + *postgresDatabase + "?sslmode=disable"

	ctx.Log.Debug("Connecting to synapse database...")
	synDb, err := synapse.OpenDatabase(connectionString)
	if err != nil {
		panic(err)
	}

	ctx.Log.Info("Fetching all local media records from Synapse...")
	records, err := synDb.GetAllMedia()
	if err != nil {
		panic(err)
	}

	ctx.Log.Info(fmt.Sprintf("Exporting %d media records", len(records)))

	archiver, err := v2archive.NewWriter(ctx, "OOB", *serverName, *partSizeBytes, archival.PersistPartsToDirectory(*exportPath))
	if err != nil {
		ctx.Log.Fatal(err)
	}

	missing := make([]string, 0)

	for _, r := range records {
		// For MediaID AABBCCDD :
		// $importPath/local_content/AA/BB/CCDD
		//
		// For a URL MediaID 2020-08-17_AABBCCDD:
		// $importPath/url_cache/2020-08-17/AABBCCDD

		mxc := util.MxcUri(*serverName, r.MediaId)

		ctx.Log.Info("Copying " + mxc)

		filePath := path.Join(*importPath, "local_content", r.MediaId[0:2], r.MediaId[2:4], r.MediaId[4:])
		if r.UrlCache != "" {
			dateParts := strings.Split(r.MediaId, "_")
			filePath = path.Join(*importPath, "url_cache", dateParts[0], strings.Join(dateParts[1:], "_"))
		}

		f, err := os.Open(filePath)
		if os.IsNotExist(err) && *skipMissing {
			ctx.Log.Warn("File does not appear to exist, skipping: " + filePath)
			missing = append(missing, filePath)
			continue
		}
		if err != nil {
			ctx.Log.Fatal(err)
		}

		_, err = archiver.AppendMedia(f, v2archive.MediaInfo{
			Origin:      *serverName,
			MediaId:     r.MediaId,
			FileName:    r.UploadName,
			ContentType: r.ContentType,
			CreationTs:  r.CreatedTs,
			S3Url:       "",
			UserId:      r.UserId,
		})
		if err != nil {
			ctx.Log.Fatal(err)
		}
	}

	err = archiver.Finish()
	if err != nil {
		ctx.Log.Fatal(err)
	}

	ctx.Log.Info("Done export")

	// Report missing files
	if len(missing) > 0 {
		for _, m := range missing {
			ctx.Log.Warn("Was not able to find " + m)
		}
	}

	ctx.Log.Info("Export completed")
}
