package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/archival"
	"github.com/turt2live/matrix-media-repo/common/assets"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/logging"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/synapse"
	"github.com/turt2live/matrix-media-repo/util"
	"golang.org/x/crypto/ssh/terminal"
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
	flag.Parse()

	assets.SetupTemplates(*templatesPath)

	_ = os.MkdirAll(*exportPath, 0755)

	var realPsqlPassword string
	if *postgresPassword == "" {
		if !terminal.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Println("Sorry, your terminal does not support reading passwords. Please supply a -dbPassword or use a different terminal.")
			fmt.Println("If you're on Windows, try using a plain Command Prompt window instead of a bash-like terminal.")
			os.Exit(1)
			return // for good measure
		}
		fmt.Printf("Postgres password: ")
		pass, err := terminal.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			panic(err)
		}
		realPsqlPassword = string(pass[:])
	} else {
		realPsqlPassword = *postgresPassword
	}

	err := logging.Setup(config.Get().General.LogDirectory)
	if err != nil {
		panic(err)
	}

	logrus.Info("Setting up for importing...")

	connectionString := "postgres://" + *postgresUsername + ":" + realPsqlPassword + "@" + *postgresHost + ":" + strconv.Itoa(*postgresPort) + "/" + *postgresDatabase + "?sslmode=disable"

	logrus.Info("Connecting to synapse database...")
	synDb, err := synapse.OpenDatabase(connectionString)
	if err != nil {
		panic(err)
	}

	logrus.Info("Fetching all local media records from synapse...")
	records, err := synDb.GetAllMedia()
	if err != nil {
		panic(err)
	}

	logrus.Info(fmt.Sprintf("Exporting %d media records", len(records)))

	writer := archival.NewV2ArchiveDiskWriter(*exportPath)
	exporter, err := archival.NewV2Export("OOB", *serverName, *partSizeBytes, writer, rcontext.Initial())
	if err != nil {
		logrus.Fatal(err)
	}

	missing := make([]string, 0)

	for _, r := range records {
		// For MediaID AABBCCDD :
		// $importPath/local_content/AA/BB/CCDD
		//
		// For a URL MediaID 2020-08-17_AABBCCDD:
		// $importPath/url_cache/2020-08-17/AABBCCDD

		mxc := fmt.Sprintf("mxc://%s/%s", *serverName, r.MediaId)

		logrus.Info("Copying " + mxc)

		filePath := path.Join(*importPath, "local_content", r.MediaId[0:2], r.MediaId[2:4], r.MediaId[4:])
		if r.UrlCache != "" {
			dateParts := strings.Split(r.MediaId, "_")
			filePath = path.Join(*importPath, "url_cache", dateParts[0], strings.Join(dateParts[1:], "_"))
		}

		f, err := os.Open(filePath)
		if os.IsNotExist(err) && *skipMissing {
			logrus.Warn("File does not appear to exist, skipping: " + filePath)
			missing = append(missing, filePath)
			continue
		}
		if err != nil {
			logrus.Fatal(err)
		}

		d := &bytes.Buffer{}
		_, _ = io.Copy(d, f)
		_ = f.Close()

		temp := bytes.NewBuffer(d.Bytes())
		sha256, err := util.GetSha256HashOfStream(ioutil.NopCloser(temp))
		if err != nil {
			logrus.Fatal(err)
		}

		err = exporter.AppendMedia(*serverName, r.MediaId, r.UploadName, r.ContentType, util.FromMillis(r.CreatedTs), d, sha256, "", r.UserId)
		if err != nil {
			logrus.Fatal(err)
		}
	}

	err = exporter.Finish()
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Info("Done export - cleaning up...")

	// Clean up
	assets.Cleanup()

	// Report missing files
	if len(missing) > 0 {
		for _, m := range missing {
			logrus.Warn("Was not able to find " + m)
		}
	}

	logrus.Info("Export completed")
}
